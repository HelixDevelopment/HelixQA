// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command recording-analyzer correlates dual-display screen recordings
// produced by scripts/testing/dual_display_record.sh (Bug #14.A) with
// the structured action timeline emitted by
// scripts/testing/action_timeline.sh (Bug #14.B). It is the §11.4
// enforcement piece that catches PASS-bluffs where a feature claims
// to have worked but the user-visible recording disagrees.
//
// Bug #14.C / Phase 34.G — would have caught Bug #13 (VK Video routes
// to PRIMARY instead of SECONDARY) automatically.
//
// Workflow:
//
//  1. Read <recording-dir>/<test_name>__<serial>__sync.json
//  2. For each present display (.mp4):
//       - Extract frames at --interval-ms via ffmpeg
//       - OCR each frame via Tesseract container
//       - Extract audio track to wav (analyzer-side; consumer may also
//         transcribe via Whisper container)
//  3. Read --timeline-file (JSONL) into memory
//  4. For each timeline event at ts T, look at frames in window
//     [T-1s, T+5s] across both displays
//  5. Emit one JSONL finding per (frame, matched_event) tuple:
//
//       {"ts":"…","display":0|2,"frame_idx":N,"frame_ts_offset_s":0.5,
//        "ocr":["..."],"matched_event":"vk_video_launched",
//        "decision":"PASS|FAIL|UNKNOWN","reason":"…"}
//
// Decision logic (the load-bearing §11.4 piece):
//
//   - PASS  : matched_event prefix "video_*" AND OCR non-trivial AND
//             on the EXPECTED display per the event detail field
//             (e.g. "display=2" or "display=primary"/"display=secondary")
//   - FAIL  : matched_event prefix "video_*" AND on the WRONG display
//   - UNKNOWN: no matched_event in window OR OCR empty OR no display
//             expectation present
//
// Constraint: PROJECT-AGNOSTIC per HelixQA charter — no
// `com.atmosphere.*`, no `ru.kinopoisk` hardcoded. All product names
// arrive via timeline event names, which the consumer registers.
//
// Health-check mode: --health-check probes Whisper + Tesseract /health
// only and exits 0 / 1. Useful for CI gating.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/audio"
)

// SyncMeta mirrors the structure that dual_display_record.sh writes
// to <test>__<serial>__sync.json. We only need a subset.
type SyncMeta struct {
	TestName            string `json:"test_name"`
	DeviceSerial        string `json:"device_serial"`
	HostTSISO           string `json:"host_ts_iso"`
	HostTSEpoch         string `json:"host_ts_epoch"`
	PrimaryDisplayID    int    `json:"primary_display_id"`
	PrimaryHostPath     string `json:"primary_host_path"`
	SecondaryPresent    int    `json:"secondary_present"`
	SecondaryDisplayID  int    `json:"secondary_display_id"`
	SecondaryHostPath   string `json:"secondary_host_path"`
}

// TimelineEvent mirrors one JSONL line from action_timeline.sh.
type TimelineEvent struct {
	TSISO    string `json:"ts_iso"`
	TSEpoch  string `json:"ts_epoch"` // string-encoded float seconds
	Test     string `json:"test"`
	Event    string `json:"event"`
	Detail   string `json:"detail"`
	Seq      int    `json:"seq"`
	tsEpochF float64
}

// Finding is one JSONL line emitted to --findings-out.
type Finding struct {
	TS              string   `json:"ts"`
	Display         int      `json:"display"`
	FrameIdx        int      `json:"frame_idx"`
	FrameTSOffsetS  float64  `json:"frame_ts_offset_s"`
	OCR             []string `json:"ocr"`
	MatchedEvent    string   `json:"matched_event,omitempty"`
	Decision        string   `json:"decision"`
	Reason          string   `json:"reason"`
}

// Flags collects parsed CLI inputs. Split out so tests can construct
// a Flags directly without re-parsing argv.
type Flags struct {
	RecordingDir string
	TimelineFile string
	FindingsOut  string
	IntervalMS   int
	WhisperURL   string
	TesseractURL string
	TestName     string
	HealthCheck  bool
	FFmpeg       string
	HealthTO     time.Duration
	FFmpegTO     time.Duration
	OCRTO        time.Duration

	// --- §11.4.107 post-hoc liveness correlator inputs ---
	// When PostAnalyze is set, run() dispatches to runPostAnalyze instead
	// of the OCR/timeline correlator. The post-hoc correlator answers a
	// different question: did a user-visible feature appear, LIVE (not a
	// frozen/stale frame), on the INTENDED display — using ffprobe-style
	// frame analysis (§11.4.107 freeze/SSIM + not-stale-vs-previous).
	PostAnalyze     bool
	Recording       string // mp4 to analyze (the intended-display capture)
	PrevRecording   string // optional: previous content's mp4 (not-stale cross-check)
	ExpectedDisplay int    // display id the recording was captured from (informational)
	ExpectedCodec   string // optional codec the timeline claims (informational)
	FFprobe         string // path to ffprobe binary (or 'ffprobe' to use $PATH)
	FreezeSSIM      float64 // adjacent-frame similarity ≥ this for ≥FreezeWindow ⇒ frozen
	FreezeWindowS   float64 // sustained-freeze window in seconds
	NotStaleSSIM    float64 // first-frame vs prev-last-frame ≥ this ⇒ stale (failed)
	MinFrames       int     // decoded-frame-count floor for a live PASS
}

// FrameExtractor extracts frames from a video at a fixed interval.
// Defined as an interface so unit tests can stub it without spawning
// ffmpeg.
type FrameExtractor interface {
	// Extract returns absolute paths to PNG frames written to outDir.
	// Caller owns lifecycle (creation + removal) of outDir.
	Extract(ctx context.Context, videoPath, outDir string, intervalMS int) ([]string, error)
}

// FFmpegFrameExtractor invokes the ffmpeg binary on disk.
type FFmpegFrameExtractor struct {
	Bin     string
	Timeout time.Duration
}

// Extract spawns ffmpeg with -vf fps=<rate> and writes frame_%05d.png
// into outDir. fps is 1000/intervalMS.
func (f *FFmpegFrameExtractor) Extract(ctx context.Context, videoPath, outDir string, intervalMS int) ([]string, error) {
	if intervalMS <= 0 {
		return nil, fmt.Errorf("recording-analyzer: intervalMS must be > 0 (got %d)", intervalMS)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("recording-analyzer: mkdir %q: %w", outDir, err)
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rate := 1000.0 / float64(intervalMS)
	pattern := filepath.Join(outDir, "frame_%05d.png")
	// -y overwrite, -loglevel error keeps stderr quiet, -vf fps=<rate>
	// samples uniformly. We deliberately run ffmpeg single-threaded
	// (-threads 1) so a 60s analyzer pass can't multiply CPU load by
	// nproc — predictable resource ceiling per §12.6 in spirit.
	cmd := exec.CommandContext(cctx, f.Bin,
		"-y",
		"-loglevel", "error",
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%g", rate),
		"-threads", "1",
		pattern,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("recording-analyzer: ffmpeg frame extract failed (%w): %s", err, strings.TrimSpace(string(output)))
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "frame_*.png"))
	if err != nil {
		return nil, fmt.Errorf("recording-analyzer: glob frames: %w", err)
	}
	sort.Strings(matches)
	return matches, nil
}

// AudioExtractor extracts the audio track of a video into a WAV.
// Defined as interface for testability.
type AudioExtractor interface {
	Extract(ctx context.Context, videoPath, outWAV string) error
}

// FFmpegAudioExtractor invokes ffmpeg to extract audio.
type FFmpegAudioExtractor struct {
	Bin     string
	Timeout time.Duration
}

// Extract spawns ffmpeg to extract the audio track to a 16 kHz mono
// WAV — the format Whisper accepts most reliably.
func (f *FFmpegAudioExtractor) Extract(ctx context.Context, videoPath, outWAV string) error {
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, f.Bin,
		"-y",
		"-loglevel", "error",
		"-i", videoPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		"-threads", "1",
		outWAV,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Some recordings have no audio track — that's a SKIP, not a
		// FAIL. Surface the underlying ffmpeg error verbatim so the
		// operator can tell apart "no audio stream" from "ffmpeg crash".
		return fmt.Errorf("recording-analyzer: ffmpeg audio extract failed (%w): %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// OCRClient is the subset of TesseractClient we use; an interface
// makes unit tests independent of the live container.
type OCRClient interface {
	OCR(ctx context.Context, imagePath string, opts audio.OCROptions) (string, error)
	Health(ctx context.Context) (*audio.TesseractHealthResponse, error)
}

// ASRClient is the subset of WhisperClient we use.
type ASRClient interface {
	Transcribe(ctx context.Context, path string, opts audio.TranscribeOptions) (*audio.TranscribeResult, error)
	Health(ctx context.Context) (*audio.HealthResponse, error)
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point.
func run(ctx context.Context, argv []string, out, errOut io.Writer) int {
	flags, code := parseFlags(argv, errOut)
	if code != 0 {
		return code
	}

	if flags.HealthCheck {
		return runHealthCheck(ctx, flags, out, errOut)
	}

	if flags.PostAnalyze {
		return runPostAnalyze(ctx, flags, out, errOut)
	}

	if flags.TestName == "" {
		fmt.Fprintln(errOut, "recording-analyzer: --test-name is required (or use --health-check)")
		return 2
	}

	syncMeta, err := loadSyncMeta(flags.RecordingDir, flags.TestName)
	if err != nil {
		fmt.Fprintf(errOut, "recording-analyzer: load sync.json: %v\n", err)
		return 1
	}

	events, err := loadTimeline(flags.TimelineFile, flags.TestName)
	if err != nil {
		fmt.Fprintf(errOut, "recording-analyzer: load timeline: %v\n", err)
		return 1
	}

	// Bug #27 fix (Phase 34): a test with no video timeline events is
	// not a video test. The analyzer cannot enforce video routing on
	// it (Tier 1 / Tier 2 secondary-display redirect is irrelevant for
	// a non-video test like test_lawnchair_recents.sh,
	// test_navigation_bar.sh, test_settings_connected_devices.sh).
	// Without this exemption, run() would proceed with zero video
	// events, frames would have no matched event in window, every
	// finding would be UNKNOWN, and any operator-side wrapper that
	// treats exit 1 as FAIL would fire a §11.4.1 FAIL-bluff (false-
	// positive ANALYZER FAIL banner on a non-video test). Emit one
	// informational NOT_APPLICABLE finding and exit 0 cleanly.
	//
	// CM-ANALYZER-NON-VIDEO-EXEMPT — pre-build gate enforces presence
	// of `videoEventCount == 0` literal + NOT_APPLICABLE token below.
	videoEventCount := 0
	for _, ev := range events {
		if isVideoEvent(ev.Event) {
			videoEventCount++
			continue
		}
		// Defensive secondary heuristic — catches consumer-written
		// timeline events that didn't follow the "video_" prefix
		// convention but still mention video / playback semantics.
		lower := strings.ToLower(ev.Event)
		if strings.HasPrefix(lower, "playback_") || strings.Contains(lower, "video") {
			videoEventCount++
		}
	}
	if videoEventCount == 0 {
		return emitNotApplicable(out, errOut, flags, "no video timeline events found — test is non-video")
	}

	tess := audio.NewTesseractClient(flags.TesseractURL)
	frameExtractor := &FFmpegFrameExtractor{Bin: flags.FFmpeg, Timeout: flags.FFmpegTO}
	audioExtractor := &FFmpegAudioExtractor{Bin: flags.FFmpeg, Timeout: flags.FFmpegTO}

	report, err := analyze(ctx, flags, syncMeta, events, frameExtractor, audioExtractor, tess)
	if err != nil {
		fmt.Fprintf(errOut, "recording-analyzer: analyze: %v\n", err)
		return 1
	}

	if err := writeFindings(flags.FindingsOut, report); err != nil {
		fmt.Fprintf(errOut, "recording-analyzer: write findings: %v\n", err)
		return 1
	}

	// Deterministic summary on stdout for operator + orchestrator.
	emitSummary(out, flags, syncMeta, report)
	if hasAnyFail(report) {
		return 1
	}
	return 0
}

// parseFlags is split so tests can drive run() with minimal setup.
func parseFlags(argv []string, errOut io.Writer) (Flags, int) {
	fs := flag.NewFlagSet("recording-analyzer", flag.ContinueOnError)
	fs.SetOutput(errOut)

	rec := fs.String("recording-dir", "/tmp/__test_recording",
		"Directory containing <test_name>__<serial>__sync.json + display .mp4 files (dual_display_record.sh output).")
	tl := fs.String("timeline-file", "/tmp/__action_timeline.jsonl",
		"JSONL file written by action_timeline.sh.")
	out := fs.String("findings-out", "/tmp/__recording_findings.jsonl",
		"Where to write per-frame findings (JSONL, one line per (frame, matched_event)).")
	interval := fs.Int("interval-ms", 500,
		"Frame sampling interval in milliseconds. fps = 1000 / interval.")
	whisper := fs.String("whisper-url", audio.DefaultWhisperBaseURL,
		"Whisper container base URL.")
	tess := fs.String("tesseract-url", audio.DefaultTesseractBaseURL,
		"Tesseract container base URL.")
	testName := fs.String("test-name", "",
		"Required: which <test>__<serial>__sync.json to analyze.")
	health := fs.Bool("health-check", false,
		"Probe Whisper + Tesseract /health and exit 0/1 (no analysis).")
	ffmpegBin := fs.String("ffmpeg", "ffmpeg",
		"Path to ffmpeg binary (or just 'ffmpeg' to use $PATH).")
	healthTO := fs.Duration("health-timeout", 5*time.Second,
		"Per-service /health probe timeout for --health-check.")
	ffmpegTO := fs.Duration("ffmpeg-timeout", 60*time.Second,
		"Hard cap per ffmpeg invocation (frame extract OR audio extract).")
	ocrTO := fs.Duration("ocr-timeout", 30*time.Second,
		"Per-frame OCR HTTP timeout.")

	// --- §11.4.107 post-hoc liveness correlator flags ---
	postAnalyze := fs.Bool("post-analyze", false,
		"Run the §11.4.107 post-hoc liveness correlator instead of the OCR/timeline correlator. "+
			"Requires --recording; correlates frame-advance + freeze + not-stale against --timeline-file.")
	recording := fs.String("recording", "",
		"Post-analyze: path to the intended-display mp4 to analyze (required with --post-analyze).")
	prevRecording := fs.String("prev-recording", "",
		"Post-analyze (optional): previous content's mp4 — first frame of --recording must differ from this file's last frame (not-stale §11.4.107 cross-check).")
	expectedDisplay := fs.Int("expected-display", -1,
		"Post-analyze (informational): display id the --recording was captured from.")
	expectedCodec := fs.String("expected-codec", "",
		"Post-analyze (informational): codec the timeline claims for the content.")
	ffprobeBin := fs.String("ffprobe", "ffprobe",
		"Post-analyze: path to ffprobe binary (or 'ffprobe' to use $PATH). Absent ⇒ honest SKIP, never fake PASS (§11.4.3).")
	freezeSSIM := fs.Float64("freeze-ssim", 0.999,
		"Post-analyze: adjacent-frame similarity ≥ this for the freeze window ⇒ frozen (§11.4.107).")
	freezeWindowS := fs.Float64("freeze-window-s", 1.0,
		"Post-analyze: sustained-freeze window in seconds.")
	notStaleSSIM := fs.Float64("not-stale-ssim", 0.999,
		"Post-analyze: --recording first frame vs --prev-recording last frame ≥ this ⇒ stale (FAIL).")
	minFrames := fs.Int("min-frames", 2,
		"Post-analyze: decoded-frame-count floor for a live PASS (0-frame mp4 = Bug #24 PASS-bluff).")

	if err := fs.Parse(argv); err != nil {
		return Flags{}, 2
	}
	if *interval <= 0 {
		fmt.Fprintln(errOut, "recording-analyzer: --interval-ms must be > 0")
		return Flags{}, 2
	}

	return Flags{
		RecordingDir: *rec,
		TimelineFile: *tl,
		FindingsOut:  *out,
		IntervalMS:   *interval,
		WhisperURL:   *whisper,
		TesseractURL: *tess,
		TestName:     *testName,
		HealthCheck:  *health,
		FFmpeg:       *ffmpegBin,
		HealthTO:     *healthTO,
		FFmpegTO:     *ffmpegTO,
		OCRTO:        *ocrTO,

		PostAnalyze:     *postAnalyze,
		Recording:       *recording,
		PrevRecording:   *prevRecording,
		ExpectedDisplay: *expectedDisplay,
		ExpectedCodec:   *expectedCodec,
		FFprobe:         *ffprobeBin,
		FreezeSSIM:      *freezeSSIM,
		FreezeWindowS:   *freezeWindowS,
		NotStaleSSIM:    *notStaleSSIM,
		MinFrames:       *minFrames,
	}, 0
}

// runHealthCheck probes both containers and exits 0 iff both healthy.
func runHealthCheck(ctx context.Context, flags Flags, out, errOut io.Writer) int {
	w := audio.NewWhisperClient(flags.WhisperURL)
	t := audio.NewTesseractClient(flags.TesseractURL)
	wctx, wcancel := context.WithTimeout(ctx, flags.HealthTO)
	defer wcancel()
	wh, werr := w.Health(wctx)
	tctx, tcancel := context.WithTimeout(ctx, flags.HealthTO)
	defer tcancel()
	th, terr := t.Health(tctx)

	wState := "FAIL"
	wDetail := ""
	if werr == nil {
		wState = "PASS"
		wDetail = fmt.Sprintf("backend=%s default_model=%s", wh.Backend, wh.DefaultModel)
	} else {
		wDetail = werr.Error()
	}
	tState := "FAIL"
	tDetail := ""
	if terr == nil {
		tState = "PASS"
		tDetail = fmt.Sprintf("version=%s langs=%v", th.TesseractVersion, th.Langs)
	} else {
		tDetail = terr.Error()
	}

	fmt.Fprintln(out, "recording-analyzer health-check")
	fmt.Fprintf(out, "  %-4s whisper   @ %s — %s\n", wState, flags.WhisperURL, wDetail)
	fmt.Fprintf(out, "  %-4s tesseract @ %s — %s\n", tState, flags.TesseractURL, tDetail)
	if werr != nil || terr != nil {
		_ = errOut
		return 1
	}
	return 0
}

// loadSyncMeta reads the sync.json that dual_display_record.sh
// produces. The file name pattern is fixed; we glob for the matching
// test_name (any serial).
func loadSyncMeta(recordingDir, testName string) (*SyncMeta, error) {
	safe := sanitizeTestName(testName)
	pattern := filepath.Join(recordingDir, safe+"__*__sync.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no sync.json found matching %q", pattern)
	}
	// Pick the most recently modified one if multiple devices recorded.
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return matches[i] < matches[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return loadSyncMetaFromFile(matches[0])
}

// loadSyncMetaFromFile is the testable inner half of loadSyncMeta.
func loadSyncMetaFromFile(path string) (*SyncMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	var m SyncMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode %q: %w", path, err)
	}
	if m.TestName == "" || m.DeviceSerial == "" {
		return nil, fmt.Errorf("sync.json %q missing test_name or device_serial", path)
	}
	return &m, nil
}

// loadTimeline reads the JSONL timeline and returns the events
// belonging to the given test. Filters out lines that fail to decode
// (rather than aborting the whole analysis on one bad line — the
// timeline is concurrent-write).
func loadTimeline(path, testName string) ([]TimelineEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()
	var events []TimelineEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev TimelineEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // best-effort
		}
		if testName != "" && ev.Test != testName {
			continue
		}
		ev.tsEpochF, _ = strconv.ParseFloat(ev.TSEpoch, 64)
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan timeline %q: %w", path, err)
	}
	return events, nil
}

// frameRecord is the per-frame analyzer state we build before
// emitting findings — kept private to the analyzer.
type frameRecord struct {
	display       int
	idx           int
	tsOffsetS     float64 // seconds since recording started
	absoluteTSF   float64 // host_ts_epoch + tsOffsetS
	ocrLines      []string
}

// analyze drives the full pipeline. Public-ish for testability.
// Returns the list of findings (one per (frame, matched_event) tuple,
// or one per frame when no event matched — UNKNOWN).
func analyze(
	ctx context.Context,
	flags Flags,
	meta *SyncMeta,
	events []TimelineEvent,
	frames FrameExtractor,
	audioX AudioExtractor,
	ocr OCRClient,
) ([]Finding, error) {
	var findings []Finding

	hostStart, _ := strconv.ParseFloat(meta.HostTSEpoch, 64)

	displays := []displayJob{}
	if meta.PrimaryHostPath != "" && fileExists(meta.PrimaryHostPath) {
		displays = append(displays, displayJob{id: meta.PrimaryDisplayID, path: meta.PrimaryHostPath})
	}
	if meta.SecondaryPresent == 1 && meta.SecondaryHostPath != "" && fileExists(meta.SecondaryHostPath) {
		displays = append(displays, displayJob{id: meta.SecondaryDisplayID, path: meta.SecondaryHostPath})
	}
	if len(displays) == 0 {
		return nil, fmt.Errorf("no display recordings found on disk for test %q", meta.TestName)
	}

	tmpRoot, err := os.MkdirTemp("", "recording-analyzer-")
	if err != nil {
		return nil, fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	for _, d := range displays {
		dDir := filepath.Join(tmpRoot, fmt.Sprintf("display_%d", d.id))
		framesPaths, err := frames.Extract(ctx, d.path, dDir, flags.IntervalMS)
		if err != nil {
			// Attach UNKNOWN finding so the operator sees the failure
			// surface in the JSONL — never silently drop.
			findings = append(findings, Finding{
				TS:       meta.HostTSISO,
				Display:  d.id,
				Decision: "UNKNOWN",
				Reason:   fmt.Sprintf("frame extraction failed: %v", err),
			})
			continue
		}

		// Best-effort audio extraction (only for primary OR if the
		// recording has audio). If audioX is nil the caller doesn't
		// want audio — skip cleanly.
		if audioX != nil {
			wavPath := filepath.Join(dDir, "audio.wav")
			_ = audioX.Extract(ctx, d.path, wavPath) // best-effort
		}

		intervalS := float64(flags.IntervalMS) / 1000.0
		records := make([]frameRecord, 0, len(framesPaths))
		for i, fp := range framesPaths {
			ocrCtx, cancel := context.WithTimeout(ctx, flags.OCRTO)
			text, err := ocr.OCR(ocrCtx, fp, audio.OCROptions{})
			cancel()
			lines := []string{}
			if err == nil {
				for _, l := range strings.Split(text, "\n") {
					l = strings.TrimSpace(l)
					if l != "" {
						lines = append(lines, l)
					}
				}
			}
			tsOffset := float64(i) * intervalS
			records = append(records, frameRecord{
				display:     d.id,
				idx:         i,
				tsOffsetS:   tsOffset,
				absoluteTSF: hostStart + tsOffset,
				ocrLines:    lines,
			})
		}

		// Correlate against timeline events. For each event, look at
		// frames in window [T-1s, T+5s] on this display.
		for _, ev := range events {
			matchedAny := false
			for _, rec := range records {
				if !inWindow(rec.absoluteTSF, ev.tsEpochF, -1.0, 5.0) {
					continue
				}
				matchedAny = true
				findings = append(findings, makeFinding(meta, rec, &ev))
			}
			if !matchedAny {
				// Event with no frame in window — still record the
				// gap so the operator can see it's missing evidence.
				findings = append(findings, Finding{
					TS:           ev.TSISO,
					Display:      d.id,
					FrameIdx:     -1,
					MatchedEvent: ev.Event,
					Decision:     "UNKNOWN",
					Reason:       "no frame in [T-1s, T+5s] window for this event on this display",
				})
			}
		}

		// Frames with no matched event — UNKNOWN baseline so the
		// findings file is a complete trace, not just event-driven.
		for _, rec := range records {
			if frameMatchedAnyEvent(rec, events) {
				continue
			}
			findings = append(findings, Finding{
				TS:             tsToISO(rec.absoluteTSF),
				Display:        rec.display,
				FrameIdx:       rec.idx,
				FrameTSOffsetS: rec.tsOffsetS,
				OCR:            rec.ocrLines,
				Decision:       "UNKNOWN",
				Reason:         "frame has no matching timeline event in window",
			})
		}
	}

	return findings, nil
}

type displayJob struct {
	id   int
	path string
}

// makeFinding applies the §11.4 decision logic.
func makeFinding(meta *SyncMeta, rec frameRecord, ev *TimelineEvent) Finding {
	f := Finding{
		TS:             tsToISO(rec.absoluteTSF),
		Display:        rec.display,
		FrameIdx:       rec.idx,
		FrameTSOffsetS: rec.tsOffsetS,
		OCR:            rec.ocrLines,
		MatchedEvent:   ev.Event,
	}

	// Default UNKNOWN until we positively match PASS/FAIL.
	f.Decision = "UNKNOWN"
	f.Reason = "default — no PASS/FAIL signal"

	if !isVideoEvent(ev.Event) {
		f.Reason = "matched event is not video_*"
		return f
	}
	if len(rec.ocrLines) == 0 || isOCRTrivial(rec.ocrLines) {
		f.Reason = "OCR empty or trivial — cannot confirm visible content"
		return f
	}
	expected, ok := expectedDisplayFromDetail(ev.Detail, meta)
	if !ok {
		f.Reason = "video_* event has no display=… expectation in detail"
		return f
	}
	if rec.display == expected {
		f.Decision = "PASS"
		f.Reason = fmt.Sprintf("video_* event observed on expected display %d with non-trivial OCR", expected)
	} else {
		f.Decision = "FAIL"
		f.Reason = fmt.Sprintf("video_* event content rendered on display %d but expected display %d", rec.display, expected)
	}
	return f
}

// isVideoEvent matches event names that begin with "video_" — the
// product-agnostic convention the timeline consumer adopts.
func isVideoEvent(name string) bool {
	return strings.HasPrefix(name, "video_")
}

// isOCRTrivial returns true if the OCR output carries no useful
// signal (empty, just status-bar timestamps, etc.). Heuristic:
// fewer than 3 letters total across all lines.
func isOCRTrivial(lines []string) bool {
	letters := 0
	for _, l := range lines {
		for _, r := range l {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				(r >= 'А' && r <= 'я') {
				letters++
			}
		}
	}
	return letters < 3
}

// expectedDisplayFromDetail parses the event's detail string for a
// display=… directive. Recognises:
//
//	display=2          → 2
//	display=primary    → meta.PrimaryDisplayID
//	display=secondary  → meta.SecondaryDisplayID (if present)
//
// Detail is space-separated `key=value` per action_timeline.sh
// convention. Returns (displayID, true) if recognised.
func expectedDisplayFromDetail(detail string, meta *SyncMeta) (int, bool) {
	for _, tok := range strings.Fields(detail) {
		if !strings.HasPrefix(tok, "display=") {
			continue
		}
		val := strings.TrimPrefix(tok, "display=")
		switch val {
		case "primary":
			return meta.PrimaryDisplayID, true
		case "secondary":
			if meta.SecondaryPresent == 1 {
				return meta.SecondaryDisplayID, true
			}
			return 0, false
		default:
			n, err := strconv.Atoi(val)
			if err != nil {
				return 0, false
			}
			return n, true
		}
	}
	return 0, false
}

// inWindow tests whether t is within [center+lo, center+hi].
func inWindow(t, center, lo, hi float64) bool {
	return t >= center+lo && t <= center+hi
}

// frameMatchedAnyEvent — true if rec is in any event's window.
func frameMatchedAnyEvent(rec frameRecord, events []TimelineEvent) bool {
	for _, ev := range events {
		if inWindow(rec.absoluteTSF, ev.tsEpochF, -1.0, 5.0) {
			return true
		}
	}
	return false
}

// tsToISO converts an epoch float seconds value to RFC3339 with
// milliseconds. Used for finding rows where the ts is computed from
// host_ts_epoch + offset (no original ISO available).
func tsToISO(epoch float64) string {
	if epoch <= 0 {
		return ""
	}
	sec := int64(epoch)
	nsec := int64((epoch - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC().Format("2006-01-02T15:04:05.000Z")
}

// fileExists is a tiny wrapper to keep the analyzer readable.
func fileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Size() > 0
}

// sanitizeTestName mirrors dual_display_record.sh's $TEST_SAFE
// transformation: tr -c 'A-Za-z0-9_' '_'.
func sanitizeTestName(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		isAlnum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '_'
		if isAlnum {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}

// writeFindings serializes findings to JSONL.
func writeFindings(path string, findings []Finding) error {
	if path == "" {
		return errors.New("findings-out path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir parent of %q: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, fnd := range findings {
		if err := enc.Encode(fnd); err != nil {
			return fmt.Errorf("encode finding: %w", err)
		}
	}
	return nil
}

// hasAnyFail returns true if any finding decision is "FAIL".
func hasAnyFail(findings []Finding) bool {
	for _, f := range findings {
		if f.Decision == "FAIL" {
			return true
		}
	}
	return false
}

// emitNotApplicable is the Bug #27 (§11.4.1) early-return helper.
//
// It writes a single NOT_APPLICABLE finding to flags.FindingsOut, prints
// a one-line summary to out, and returns exit code 0. Used when the
// timeline carries zero video events — the analyzer has nothing to
// enforce, so emitting exit 1 (which any operator-side wrapper would
// surface as ANALYZER FAIL) would be a §11.4.1 FAIL-bluff: the test
// isn't broken, the analyzer simply doesn't apply.
//
// The NOT_APPLICABLE row is mechanically distinguishable from PASS so
// downstream §11.4.2 cross-checking does not mistake a non-video test
// for a recording-validated pass.
func emitNotApplicable(out, errOut io.Writer, flags Flags, reason string) int {
	finding := Finding{
		TS:       time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Display:  -1,
		FrameIdx: -1,
		Decision: "NOT_APPLICABLE",
		Reason:   reason,
	}
	if err := writeFindings(flags.FindingsOut, []Finding{finding}); err != nil {
		fmt.Fprintf(errOut, "recording-analyzer: write NOT_APPLICABLE finding: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "recording-analyzer — summary")
	fmt.Fprintf(out, "  test          : %s\n", flags.TestName)
	fmt.Fprintf(out, "  decision      : NOT_APPLICABLE\n")
	fmt.Fprintf(out, "  reason        : %s\n", reason)
	fmt.Fprintf(out, "  findings-out  : %s\n", flags.FindingsOut)
	fmt.Fprintln(out, "  → analyzer exempt: timeline carries no video events; no §11.4 enforcement to perform")
	return 0
}

// emitSummary prints a one-screen summary on stdout.
func emitSummary(out io.Writer, flags Flags, meta *SyncMeta, findings []Finding) {
	pass, fail, unknown := 0, 0, 0
	for _, f := range findings {
		switch f.Decision {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		default:
			unknown++
		}
	}
	fmt.Fprintln(out, "recording-analyzer — summary")
	fmt.Fprintf(out, "  test          : %s\n", meta.TestName)
	fmt.Fprintf(out, "  device        : %s\n", meta.DeviceSerial)
	fmt.Fprintf(out, "  recording-dir : %s\n", flags.RecordingDir)
	fmt.Fprintf(out, "  timeline-file : %s\n", flags.TimelineFile)
	fmt.Fprintf(out, "  findings-out  : %s\n", flags.FindingsOut)
	fmt.Fprintf(out, "  PASS=%d  FAIL=%d  UNKNOWN=%d  total=%d\n",
		pass, fail, unknown, len(findings))
	if fail > 0 {
		fmt.Fprintln(out, "  → at least one FAIL: feature claimed by timeline disagrees with recorded display")
	}
}
