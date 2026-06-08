// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// posthoc.go — the §11.4.107 post-hoc liveness correlator.
//
// The sibling main.go correlator answers "did OCR-readable content appear
// in the timeline window on the expected display". This file answers the
// harder §11.4.107 question that a single screenshot CANNOT answer:
//
//   Did the captured recording show LIVE, ADVANCING frames — not a frozen
//   or stale-from-previous frame — so the user actually saw the feature
//   working, not a stuck decoder painting one stale picture?
//
// Forensic anchor (§11.4.107): a test PASSed on a single captured frame
// showing "a picture" that was a FROZEN / STALE frame from the previously
// played content (stuck-decoder / stale-producer). The feature was broken
// for the user while the test was green. §11.4.5 mandates captured evidence
// + a presence pass; §11.4.107 raises the bar to liveness + not-stale +
// self-validated-analyzer.
//
// Inputs (run() dispatches here when --post-analyze is set):
//
//	--recording        the intended-display mp4 to analyze (required)
//	--prev-recording   optional previous-content mp4 (not-stale cross-check)
//	--timeline-file    optional action-timeline JSONL (frame-advance window)
//	--expected-display informational display id
//	--expected-codec   informational codec claim
//	--findings-out     JSONL findings sink
//
// Verdict logic (the load-bearing §11.4.107 piece):
//
//	SKIP  : ffprobe/ffmpeg absent (ToolAbsentError) OR recording missing on
//	        disk → honest SKIP-with-reason per §11.4.3, NEVER a fake PASS.
//	FAIL  : decoded-frame count < MinFrames (0-byte / 0-frame Bug #24), OR
//	        adjacent frames stay near-identical for the freeze window (frozen
//	        decoder), OR first frame == previous content's last frame (stale).
//	PASS  : decoded frames ≥ MinFrames AND frames advance (not frozen) AND
//	        not-stale-vs-previous. Live, advancing, fresh content.
//
// Self-validation (§11.4.107 clause 10): posthoc_test.go drives a
// golden-good FrameSource (advancing distinct frames → PASS) AND a
// golden-bad FrameSource (frozen / zero-frame → FAIL). An analyzer that
// PASSes its golden-bad fixture is itself the bluff this rule forbids;
// the test proves this analyzer is not a no-op.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ToolAbsentError is returned when the frame-analysis backend (ffprobe,
// with ffmpeg fallback) is not available on the host. The correlator
// converts it into an honest SKIP (§11.4.3) — never a fake PASS. It is a
// typed error so callers can distinguish "tool missing" from "tool ran
// and the recording is broken".
type ToolAbsentError struct {
	Tool   string
	Detail string
}

func (e *ToolAbsentError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("frame-analysis tool %q is absent: %s", e.Tool, e.Detail)
	}
	return fmt.Sprintf("frame-analysis tool %q is absent", e.Tool)
}

// asToolAbsent reports whether err is (or wraps) a *ToolAbsentError.
func asToolAbsent(err error) (*ToolAbsentError, bool) {
	var tae *ToolAbsentError
	if errors.As(err, &tae) {
		return tae, true
	}
	return nil, false
}

// FrameStats is the per-recording analysis result a FrameSource produces.
// It is the minimal observable surface the liveness verdict needs, kept
// deliberately tiny so unit tests can construct golden-good / golden-bad
// fixtures directly without spawning any binary.
type FrameStats struct {
	// DecodedFrames is the number of decoded video frames in the clip.
	// 0 ⇒ the canonical Bug #24 0-frame PASS-bluff.
	DecodedFrames int
	// FrameMeans holds the per-sampled-frame mean luma (0..255). Adjacent
	// near-equal means across the freeze window ⇒ frozen. Used for both
	// freeze detection and the first/last-frame not-stale cross-check.
	// Sampled at the analyzer's interval (not necessarily every frame).
	FrameMeans []float64
	// FPS is the recording's nominal frame rate (informational).
	FPS float64
	// Codec is the detected video codec (informational, may be "").
	Codec string
}

// FrameSource extracts FrameStats from a recording. The real
// implementation shells out to ffprobe/ffmpeg; tests substitute a stub so
// the §11.4.107 self-validation runs in air-gapped CI with no binaries.
type FrameSource interface {
	Analyze(ctx context.Context, recordingPath string) (FrameStats, error)
}

// PostFinding is one JSONL row emitted by the post-hoc correlator. It is a
// distinct shape from main.go's Finding so a downstream reader never
// confuses an OCR/timeline finding with a liveness finding.
type PostFinding struct {
	TS              string  `json:"ts"`
	Kind            string  `json:"kind"` // always "liveness"
	Recording       string  `json:"recording"`
	ExpectedDisplay int     `json:"expected_display"`
	ExpectedCodec   string  `json:"expected_codec,omitempty"`
	DetectedCodec   string  `json:"detected_codec,omitempty"`
	DecodedFrames   int     `json:"decoded_frames"`
	FPS             float64 `json:"fps,omitempty"`
	MinSSIMAdjacent float64 `json:"min_adjacent_similarity"` // lowest adjacent-frame similarity (lower = more motion)
	MaxFreezeRunS   float64 `json:"max_freeze_run_s"`        // longest sustained near-identical run, seconds
	NotStaleScore   float64 `json:"not_stale_similarity"`    // first-vs-prev-last similarity; <NotStaleSSIM = fresh
	Decision        string  `json:"decision"` // PASS | FAIL | SKIP
	Reason          string  `json:"reason"`
}

// runPostAnalyze is the §11.4.107 entry point dispatched from run().
// Exit codes: 0 = PASS, 1 = FAIL, 2 = usage error, 3 = SKIP (tool absent
// / recording missing — honest, distinct from PASS so an operator wrapper
// never counts a SKIP as a pass).
func runPostAnalyze(ctx context.Context, flags Flags, out, errOut io.Writer) int {
	if flags.Recording == "" {
		fmt.Fprintln(errOut, "recording-analyzer: --post-analyze requires --recording")
		return 2
	}

	src := &FFprobeFrameSource{
		FFprobe:    flags.FFprobe,
		FFmpeg:     flags.FFmpeg,
		Timeout:    flags.FFmpegTO,
		IntervalMS: flags.IntervalMS,
	}
	return runPostAnalyzeWith(ctx, flags, src, out, errOut)
}

// runPostAnalyzeWith is the testable core — accepts an injected FrameSource
// so posthoc_test.go can drive golden-good / golden-bad fixtures.
func runPostAnalyzeWith(ctx context.Context, flags Flags, src FrameSource, out, errOut io.Writer) int {
	finding := PostFinding{
		TS:              time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Kind:            "liveness",
		Recording:       flags.Recording,
		ExpectedDisplay: flags.ExpectedDisplay,
		ExpectedCodec:   flags.ExpectedCodec,
	}

	// Recording must exist + be non-empty on disk. Missing ⇒ honest SKIP
	// (the recorder may have been topology-skipped per §11.4.3), NOT FAIL
	// and NOT PASS.
	if !fileExists(flags.Recording) {
		finding.Decision = "SKIP"
		finding.Reason = "recording absent or empty on disk — nothing to analyze (topology-skipped recorder?)"
		return finishPost(flags, finding, out, errOut, 3)
	}

	stats, err := src.Analyze(ctx, flags.Recording)
	if err != nil {
		if tae, ok := asToolAbsent(err); ok {
			finding.Decision = "SKIP"
			finding.Reason = "frame-analysis tool absent (" + tae.Error() + ") — honest SKIP per §11.4.3, no fake PASS"
			return finishPost(flags, finding, out, errOut, 3)
		}
		// A genuine analysis error (corrupt mp4, decode failure) is a real
		// product/evidence defect → FAIL, surfaced verbatim.
		finding.Decision = "FAIL"
		finding.Reason = "frame analysis failed: " + err.Error()
		return finishPost(flags, finding, out, errOut, 1)
	}

	finding.DecodedFrames = stats.DecodedFrames
	finding.FPS = stats.FPS
	finding.DetectedCodec = stats.Codec

	// (1) Frame-advance floor. 0-frame mp4 (Bug #24) or below the floor ⇒
	// the recording captured nothing the user could see.
	if stats.DecodedFrames < flags.MinFrames {
		finding.Decision = "FAIL"
		finding.Reason = fmt.Sprintf("decoded-frame count %d < floor %d — 0/low-frame recording (Bug #24 class)",
			stats.DecodedFrames, flags.MinFrames)
		return finishPost(flags, finding, out, errOut, 1)
	}

	// (2) Freeze detection (§11.4.107). Compute the longest sustained run of
	// adjacent frames whose similarity ≥ FreezeSSIM. If that run covers the
	// freeze window, the decoder painted one stale picture ⇒ FAIL.
	minAdj, maxFreezeRunS := freezeAnalysis(stats, flags)
	finding.MinSSIMAdjacent = minAdj
	finding.MaxFreezeRunS = maxFreezeRunS
	if maxFreezeRunS+1e-9 >= flags.FreezeWindowS {
		finding.Decision = "FAIL"
		finding.Reason = fmt.Sprintf(
			"frozen: adjacent frames ≥ %.4f similar for %.2fs ≥ freeze window %.2fs (stuck decoder / single stale frame)",
			flags.FreezeSSIM, maxFreezeRunS, flags.FreezeWindowS)
		return finishPost(flags, finding, out, errOut, 1)
	}

	// (3) Not-stale-vs-previous cross-check (§11.4.107). The new content's
	// first frame must differ from the previous content's last frame; an
	// identical first frame means the new playback never actually started
	// (stale producer carried over). Only checked when --prev-recording is
	// supplied AND analyzable.
	if flags.PrevRecording != "" {
		notStale, perr := notStaleScore(ctx, src, flags, stats)
		switch {
		case perr != nil:
			if _, ok := asToolAbsent(perr); ok {
				// Tool can analyze the main recording but not the prev one —
				// surface as informational; do not fake a stale verdict.
				finding.NotStaleScore = -1
			} else {
				finding.Decision = "FAIL"
				finding.Reason = "previous-recording analysis failed: " + perr.Error()
				return finishPost(flags, finding, out, errOut, 1)
			}
		default:
			finding.NotStaleScore = notStale
			if notStale+1e-9 >= flags.NotStaleSSIM {
				finding.Decision = "FAIL"
				finding.Reason = fmt.Sprintf(
					"stale: first frame ≥ %.4f similar to previous content's last frame (%.4f) — new playback never started",
					flags.NotStaleSSIM, notStale)
				return finishPost(flags, finding, out, errOut, 1)
			}
		}
	}

	// All §11.4.107 checks passed: live, advancing, fresh frames.
	finding.Decision = "PASS"
	finding.Reason = fmt.Sprintf(
		"live: %d decoded frames, max freeze run %.2fs < window %.2fs, min adjacent similarity %.4f — advancing content",
		stats.DecodedFrames, maxFreezeRunS, flags.FreezeWindowS, minAdj)
	return finishPost(flags, finding, out, errOut, 0)
}

// freezeAnalysis walks the sampled per-frame means, computes the lowest
// adjacent-frame similarity (proxy for "most motion seen"), and the longest
// sustained run, in seconds, of adjacent frames whose similarity ≥
// FreezeSSIM. Similarity is 1 - normalized |Δmean| over the full 0..255
// luma range — a cheap, deterministic, dependency-free SSIM-class proxy
// computed from ffprobe/ffmpeg frame-mean metadata.
func freezeAnalysis(stats FrameStats, flags Flags) (minAdj, maxFreezeRunS float64) {
	means := stats.FrameMeans
	if len(means) < 2 {
		// Below-floor handled earlier; with <2 sampled means we cannot see
		// motion, treat as fully frozen for the run length we do have.
		return 1.0, sampleSpacingS(stats, flags) // one-sample run
	}
	minAdj = 1.0
	spacing := sampleSpacingS(stats, flags)
	curRun := 0.0
	for i := 1; i < len(means); i++ {
		sim := meanSimilarity(means[i-1], means[i])
		if sim < minAdj {
			minAdj = sim
		}
		if sim >= flags.FreezeSSIM {
			curRun += spacing
			if curRun > maxFreezeRunS {
				maxFreezeRunS = curRun
			}
		} else {
			curRun = 0
		}
	}
	return minAdj, maxFreezeRunS
}

// sampleSpacingS returns the wall-clock seconds between two sampled means.
// Prefers IntervalMS (the analyzer's fixed sampling cadence). Falls back to
// 1/FPS, then to a conservative 0.1s.
func sampleSpacingS(stats FrameStats, flags Flags) float64 {
	if flags.IntervalMS > 0 {
		return float64(flags.IntervalMS) / 1000.0
	}
	if stats.FPS > 0 {
		return 1.0 / stats.FPS
	}
	return 0.1
}

// meanSimilarity maps |a-b| over the 0..255 luma range to a 0..1 similarity
// (1 = identical, 0 = maximally different). Deterministic, no deps.
func meanSimilarity(a, b float64) float64 {
	d := math.Abs(a-b) / 255.0
	if d > 1 {
		d = 1
	}
	return 1.0 - d
}

// notStaleScore analyzes the previous recording and returns the similarity
// between this recording's FIRST sampled mean and the previous recording's
// LAST sampled mean. High ⇒ stale (the new content's first frame is the
// old content's last frame).
func notStaleScore(ctx context.Context, src FrameSource, flags Flags, cur FrameStats) (float64, error) {
	if !fileExists(flags.PrevRecording) {
		// No prev recording on disk → cannot be stale-from-it. Score 0 (fresh).
		return 0, nil
	}
	prev, err := src.Analyze(ctx, flags.PrevRecording)
	if err != nil {
		return 0, err
	}
	if len(cur.FrameMeans) == 0 || len(prev.FrameMeans) == 0 {
		return 0, nil
	}
	first := cur.FrameMeans[0]
	last := prev.FrameMeans[len(prev.FrameMeans)-1]
	return meanSimilarity(first, last), nil
}

// finishPost writes the finding JSONL, prints a one-line summary, returns
// the exit code.
func finishPost(flags Flags, finding PostFinding, out, errOut io.Writer, code int) int {
	if flags.FindingsOut != "" {
		if err := writePostFindings(flags.FindingsOut, finding); err != nil {
			fmt.Fprintf(errOut, "recording-analyzer: write post findings: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	fmt.Fprintln(out, "recording-analyzer — post-hoc liveness correlator (§11.4.107)")
	fmt.Fprintf(out, "  recording      : %s\n", finding.Recording)
	fmt.Fprintf(out, "  expected-disp  : %d\n", finding.ExpectedDisplay)
	if finding.DetectedCodec != "" || finding.ExpectedCodec != "" {
		fmt.Fprintf(out, "  codec          : detected=%q expected=%q\n", finding.DetectedCodec, finding.ExpectedCodec)
	}
	fmt.Fprintf(out, "  decoded-frames : %d\n", finding.DecodedFrames)
	fmt.Fprintf(out, "  max-freeze-run : %.2fs (window %.2fs)\n", finding.MaxFreezeRunS, flags.FreezeWindowS)
	fmt.Fprintf(out, "  min-adj-sim    : %.4f\n", finding.MinSSIMAdjacent)
	if flags.PrevRecording != "" {
		fmt.Fprintf(out, "  not-stale-sim  : %.4f (stale ≥ %.4f)\n", finding.NotStaleScore, flags.NotStaleSSIM)
	}
	fmt.Fprintf(out, "  DECISION       : %s — %s\n", finding.Decision, finding.Reason)
	return code
}

// writePostFindings serializes one PostFinding as a single JSONL line.
func writePostFindings(path string, finding PostFinding) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir parent of %q: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(finding)
}

// ---------------------------------------------------------------------------
// FFprobeFrameSource — the real backend.
// ---------------------------------------------------------------------------

// FFprobeFrameSource extracts FrameStats using ffprobe (preferred) with an
// ffmpeg fallback for the per-frame mean-luma samples. If NEITHER binary is
// resolvable it returns a *ToolAbsentError so the correlator emits an
// honest SKIP rather than a fake PASS.
type FFprobeFrameSource struct {
	FFprobe    string
	FFmpeg     string
	Timeout    time.Duration
	IntervalMS int
}

// Analyze runs ffprobe to count decoded frames + read codec/fps, then
// ffmpeg's signalstats to sample per-frame mean luma at the analyzer
// interval. Either binary missing ⇒ *ToolAbsentError.
func (s *FFprobeFrameSource) Analyze(ctx context.Context, recordingPath string) (FrameStats, error) {
	var stats FrameStats
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	probe := s.FFprobe
	if probe == "" {
		probe = "ffprobe"
	}
	if _, err := exec.LookPath(probe); err != nil {
		return stats, &ToolAbsentError{Tool: probe, Detail: err.Error()}
	}

	// (a) ffprobe: decoded frame count + codec + fps.
	pctx, pcancel := context.WithTimeout(ctx, timeout)
	defer pcancel()
	// -count_frames forces a full decode count (nb_read_frames). We read
	// codec_name + avg_frame_rate in the same call.
	probeCmd := exec.CommandContext(pctx, probe,
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames,codec_name,avg_frame_rate",
		"-of", "default=noprint_wrappers=1:nokey=0",
		recordingPath,
	)
	probeOut, perr := probeCmd.CombinedOutput()
	if perr != nil {
		// ffprobe present but failed to decode → genuine analysis error.
		return stats, fmt.Errorf("ffprobe failed (%w): %s", perr, strings.TrimSpace(string(probeOut)))
	}
	parseProbeOutput(string(probeOut), &stats)

	// (b) ffmpeg signalstats: per-frame mean luma (YAVG), sampled at the
	// analyzer interval. Used for freeze + not-stale. ffmpeg-absent here is
	// also a ToolAbsentError (we cannot judge liveness without per-frame
	// means).
	ff := s.FFmpeg
	if ff == "" {
		ff = "ffmpeg"
	}
	if _, err := exec.LookPath(ff); err != nil {
		return stats, &ToolAbsentError{Tool: ff, Detail: err.Error()}
	}
	means, merr := s.sampleFrameMeans(ctx, ff, recordingPath, timeout)
	if merr != nil {
		return stats, merr
	}
	stats.FrameMeans = means
	return stats, nil
}

// parseProbeOutput parses ffprobe key=value lines into stats.
func parseProbeOutput(text string, stats *FrameStats) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "nb_read_frames":
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				stats.DecodedFrames = n
			}
		case "codec_name":
			stats.Codec = strings.TrimSpace(v)
		case "avg_frame_rate":
			stats.FPS = parseRational(strings.TrimSpace(v))
		}
	}
}

// parseRational parses ffprobe "num/den" frame-rate strings into a float.
func parseRational(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	n, e1 := strconv.ParseFloat(num, 64)
	d, e2 := strconv.ParseFloat(den, 64)
	if e1 != nil || e2 != nil || d == 0 {
		return 0
	}
	return n / d
}

// sampleFrameMeans runs ffmpeg signalstats and parses the YAVG (mean luma)
// metadata for frames sampled at the analyzer interval.
func (s *FFprobeFrameSource) sampleFrameMeans(ctx context.Context, ffmpegBin, recordingPath string, timeout time.Duration) ([]float64, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rate := 2.0 // default 2 samples/s
	if s.IntervalMS > 0 {
		rate = 1000.0 / float64(s.IntervalMS)
	}
	// fps filter downsamples; signalstats writes lavfi.signalstats.YAVG into
	// frame metadata; metadata=print emits it to stderr in key=value form.
	vf := fmt.Sprintf("fps=%g,signalstats,metadata=print:key=lavfi.signalstats.YAVG", rate)
	cmd := exec.CommandContext(cctx, ffmpegBin,
		"-v", "info",
		"-i", recordingPath,
		"-vf", vf,
		"-an",
		"-f", "null",
		"-",
	)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg signalstats failed (%w): %s", err, strings.TrimSpace(string(combined)))
	}
	return parseYAVG(string(combined)), nil
}

// parseYAVG extracts per-frame YAVG mean-luma values from ffmpeg
// metadata=print output. Lines look like:
//
//	[Parsed_metadata_2 @ 0x..] lavfi.signalstats.YAVG=123.456
func parseYAVG(text string) []float64 {
	var means []float64
	for _, line := range strings.Split(text, "\n") {
		idx := strings.Index(line, "lavfi.signalstats.YAVG=")
		if idx < 0 {
			continue
		}
		val := strings.TrimSpace(line[idx+len("lavfi.signalstats.YAVG="):])
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			means = append(means, f)
		}
	}
	return means
}
