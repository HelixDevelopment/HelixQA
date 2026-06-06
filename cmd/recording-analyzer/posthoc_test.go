// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// posthoc_test.go — §11.4.107 self-validation of the post-hoc liveness
// correlator.
//
// Anti-bluff design (§11.4.107 clause 10): the analyzer MUST be proven
// against a golden-GOOD fixture (clean, advancing frames → PASS) AND a
// golden-BAD fixture (frozen / zero-frame → FAIL). An analyzer that PASSes
// its golden-bad fixture is itself a bluff. These tests substitute a
// stubFrameSource so the self-validation runs with NO external binary —
// the golden fixtures are constructed programmatically as FrameStats.
//
// We additionally drive the real FFprobeFrameSource against the host to
// PROVE the tool-absent path produces an honest SKIP (not a fake PASS)
// when ffprobe is missing — the live forensic on this host (ffprobe is a
// broken symlink) makes that the actually-exercised path.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubFrameSource returns canned FrameStats per recording path. Lets the
// golden-good / golden-bad fixtures drive the verdict logic with no ffmpeg.
type stubFrameSource struct {
	byPath map[string]FrameStats
	errs   map[string]error
}

func (s *stubFrameSource) Analyze(_ context.Context, path string) (FrameStats, error) {
	if s.errs != nil {
		if e, ok := s.errs[path]; ok && e != nil {
			return FrameStats{}, e
		}
	}
	if fs, ok := s.byPath[path]; ok {
		return fs, nil
	}
	return FrameStats{}, &ToolAbsentError{Tool: "stub", Detail: "no canned stats for " + path}
}

// defaultPostFlags builds a Flags value with the §11.4.107 thresholds the
// CLI defaults to, pointed at a real on-disk recording file (content
// irrelevant — the stub FrameSource decides the stats).
func defaultPostFlags(t *testing.T, recording string) Flags {
	t.Helper()
	out := filepath.Join(t.TempDir(), "post_findings.jsonl")
	return Flags{
		PostAnalyze:   true,
		Recording:     recording,
		FindingsOut:   out,
		IntervalMS:    500, // 0.5s spacing between sampled means
		FreezeSSIM:    0.999,
		FreezeWindowS: 1.0,
		NotStaleSSIM:  0.999,
		MinFrames:     2,
		FFprobe:       "ffprobe",
		FFmpeg:        "ffmpeg",
	}
}

// writeNonEmpty creates a non-empty file so fileExists() passes; the stub
// FrameSource — not the bytes — decides the verdict.
func writeNonEmpty(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("\x00\x00mp4-stub-bytes\x00\x00"), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// goldenGoodStats: many decoded frames whose per-frame mean luma changes
// substantially every sample → live, advancing content. NO sustained
// freeze run.
func goldenGoodStats() FrameStats {
	means := make([]float64, 0, 20)
	for i := 0; i < 20; i++ {
		// Alternate between two well-separated luma levels so every adjacent
		// pair is clearly NOT near-identical (similarity well below 0.999).
		if i%2 == 0 {
			means = append(means, 40.0)
		} else {
			means = append(means, 200.0)
		}
	}
	return FrameStats{DecodedFrames: 240, FrameMeans: means, FPS: 30, Codec: "h264"}
}

// goldenBadStatsFrozen: frames decoded, but EVERY sampled mean is identical
// → the decoder painted one stale picture. A long sustained freeze run.
func goldenBadStatsFrozen() FrameStats {
	means := make([]float64, 0, 20)
	for i := 0; i < 20; i++ {
		means = append(means, 128.0) // identical every sample → frozen
	}
	return FrameStats{DecodedFrames: 240, FrameMeans: means, FPS: 30, Codec: "h264"}
}

// goldenBadStatsZeroFrame: the canonical Bug #24 0-frame mp4.
func goldenBadStatsZeroFrame() FrameStats {
	return FrameStats{DecodedFrames: 0, FrameMeans: nil, FPS: 0, Codec: ""}
}

func readPostFinding(t *testing.T, path string) PostFinding {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read findings %s: %v", path, err)
	}
	var pf PostFinding
	if err := json.Unmarshal(bytes.TrimSpace(data), &pf); err != nil {
		t.Fatalf("decode finding %q: %v", string(data), err)
	}
	return pf
}

// ---------------------------------------------------------------------------
// §11.4.107 clause 10 — golden-good PASSes, golden-bad FAILs.
// ---------------------------------------------------------------------------

func TestPostAnalyze_GoldenGood_LivePass(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "good.mp4")
	flags := defaultPostFlags(t, rec)
	src := &stubFrameSource{byPath: map[string]FrameStats{rec: goldenGoodStats()}}

	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 0 {
		t.Fatalf("golden-good must PASS (exit 0); got %d. stderr=%s out=%s", code, errOut.String(), out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "PASS" {
		t.Fatalf("golden-good decision = %q, want PASS (reason=%q)", pf.Decision, pf.Reason)
	}
	if pf.DecodedFrames != 240 {
		t.Fatalf("decoded frames = %d, want 240", pf.DecodedFrames)
	}
	if pf.MaxFreezeRunS >= flags.FreezeWindowS {
		t.Fatalf("golden-good must not be frozen: max freeze run %.2fs >= window %.2fs", pf.MaxFreezeRunS, flags.FreezeWindowS)
	}
}

func TestPostAnalyze_GoldenBadFrozen_FailsNotPass(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "frozen.mp4")
	flags := defaultPostFlags(t, rec)
	src := &stubFrameSource{byPath: map[string]FrameStats{rec: goldenBadStatsFrozen()}}

	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	// The load-bearing §11.4.107 assertion: a frozen recording MUST FAIL.
	// If this ever PASSes, the analyzer is a no-op bluff.
	if code != 1 {
		t.Fatalf("golden-bad (frozen) must FAIL (exit 1); got %d — ANALYZER IS A BLUFF. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "FAIL" {
		t.Fatalf("frozen decision = %q, want FAIL (reason=%q)", pf.Decision, pf.Reason)
	}
	if !strings.Contains(pf.Reason, "frozen") {
		t.Fatalf("frozen reason should mention 'frozen': %q", pf.Reason)
	}
}

func TestPostAnalyze_GoldenBadZeroFrame_FailsNotPass(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "zero.mp4")
	flags := defaultPostFlags(t, rec)
	src := &stubFrameSource{byPath: map[string]FrameStats{rec: goldenBadStatsZeroFrame()}}

	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 1 {
		t.Fatalf("golden-bad (0-frame Bug #24) must FAIL (exit 1); got %d. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "FAIL" {
		t.Fatalf("0-frame decision = %q, want FAIL (reason=%q)", pf.Decision, pf.Reason)
	}
	if !strings.Contains(pf.Reason, "frame count") {
		t.Fatalf("0-frame reason should mention 'frame count': %q", pf.Reason)
	}
}

// ---------------------------------------------------------------------------
// not-stale-vs-previous cross-check.
// ---------------------------------------------------------------------------

func TestPostAnalyze_StaleFromPrevious_Fails(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "new.mp4")
	prev := writeNonEmpty(t, dir, "prev.mp4")
	flags := defaultPostFlags(t, rec)
	flags.PrevRecording = prev

	// New content's FIRST mean == prev content's LAST mean → stale.
	// New content otherwise advances (so it would PASS on liveness alone),
	// proving the stale check is what FAILs it.
	newStats := FrameStats{DecodedFrames: 240, FPS: 30, Codec: "h264",
		FrameMeans: []float64{128.0, 40.0, 200.0, 40.0, 200.0}}
	prevStats := FrameStats{DecodedFrames: 240, FPS: 30, Codec: "h264",
		FrameMeans: []float64{10.0, 60.0, 128.0}} // last == 128 == new first

	src := &stubFrameSource{byPath: map[string]FrameStats{rec: newStats, prev: prevStats}}
	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 1 {
		t.Fatalf("stale-from-previous must FAIL (exit 1); got %d. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "FAIL" || !strings.Contains(pf.Reason, "stale") {
		t.Fatalf("stale decision=%q reason=%q, want FAIL mentioning 'stale'", pf.Decision, pf.Reason)
	}
}

func TestPostAnalyze_FreshFromPrevious_Pass(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "new.mp4")
	prev := writeNonEmpty(t, dir, "prev.mp4")
	flags := defaultPostFlags(t, rec)
	flags.PrevRecording = prev

	newStats := goldenGoodStats()                 // first mean 40
	prevStats := FrameStats{DecodedFrames: 240, FrameMeans: []float64{10, 60, 230}, FPS: 30} // last 230 != 40
	src := &stubFrameSource{byPath: map[string]FrameStats{rec: newStats, prev: prevStats}}

	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 0 {
		t.Fatalf("fresh-from-previous must PASS (exit 0); got %d. out=%s", code, out.String())
	}
}

// ---------------------------------------------------------------------------
// honest SKIP — tool absent / recording missing → NOT a fake PASS.
// ---------------------------------------------------------------------------

func TestPostAnalyze_ToolAbsent_SkipsNotPass(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "rec.mp4")
	flags := defaultPostFlags(t, rec)
	src := &stubFrameSource{errs: map[string]error{rec: &ToolAbsentError{Tool: "ffprobe", Detail: "not found"}}}

	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 3 {
		t.Fatalf("tool-absent must SKIP (exit 3), never PASS/FAIL; got %d. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "SKIP" {
		t.Fatalf("tool-absent decision = %q, want SKIP", pf.Decision)
	}
}

func TestPostAnalyze_RecordingMissing_Skips(t *testing.T) {
	flags := defaultPostFlags(t, filepath.Join(t.TempDir(), "does_not_exist.mp4"))
	src := &stubFrameSource{}
	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 3 {
		t.Fatalf("missing recording must SKIP (exit 3); got %d. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "SKIP" {
		t.Fatalf("missing-recording decision = %q, want SKIP", pf.Decision)
	}
}

func TestPostAnalyze_AnalysisError_Fails(t *testing.T) {
	dir := t.TempDir()
	rec := writeNonEmpty(t, dir, "corrupt.mp4")
	flags := defaultPostFlags(t, rec)
	// A NON-ToolAbsent error (e.g. decode failure on a corrupt mp4) is a real
	// defect → FAIL, distinct from the SKIP path.
	src := &stubFrameSource{errs: map[string]error{rec: errCorrupt}}
	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 1 {
		t.Fatalf("genuine analysis error must FAIL (exit 1), not SKIP; got %d. out=%s", code, out.String())
	}
}

var errCorrupt = &decodeError{"moov atom not found"}

type decodeError struct{ msg string }

func (e *decodeError) Error() string { return "decode failed: " + e.msg }

// ---------------------------------------------------------------------------
// usage + flag wiring.
// ---------------------------------------------------------------------------

func TestPostAnalyze_RequiresRecording(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(context.Background(), []string{"--post-analyze"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("--post-analyze without --recording must be usage error (exit 2); got %d", code)
	}
	if !strings.Contains(errOut.String(), "requires --recording") {
		t.Fatalf("usage error should mention --recording: %q", errOut.String())
	}
}

func TestParseFlags_PostAnalyzeWired(t *testing.T) {
	var errOut bytes.Buffer
	flags, code := parseFlags([]string{
		"--post-analyze",
		"--recording", "/tmp/x.mp4",
		"--prev-recording", "/tmp/p.mp4",
		"--expected-display", "2",
		"--expected-codec", "h264",
		"--min-frames", "5",
		"--freeze-window-s", "2.0",
	}, &errOut)
	if code != 0 {
		t.Fatalf("parseFlags returned %d: %s", code, errOut.String())
	}
	if !flags.PostAnalyze || flags.Recording != "/tmp/x.mp4" || flags.PrevRecording != "/tmp/p.mp4" {
		t.Fatalf("post-analyze flags not wired: %+v", flags)
	}
	if flags.ExpectedDisplay != 2 || flags.ExpectedCodec != "h264" || flags.MinFrames != 5 || flags.FreezeWindowS != 2.0 {
		t.Fatalf("post-analyze flag values wrong: %+v", flags)
	}
}

// ---------------------------------------------------------------------------
// Real backend: prove the tool-absent path on THIS host is an honest SKIP,
// and (if ffmpeg+ffprobe both work) prove an end-to-end real PASS on a
// programmatically-generated advancing clip + FAIL on a frozen clip.
// ---------------------------------------------------------------------------

func TestFFprobeFrameSource_ToolAbsentIsHonestSkip(t *testing.T) {
	// Point at a binary name that cannot exist on PATH.
	flags := defaultPostFlags(t, writeNonEmpty(t, t.TempDir(), "rec.mp4"))
	flags.FFprobe = "ffprobe-definitely-not-installed-xyz"
	src := &FFprobeFrameSource{FFprobe: flags.FFprobe, FFmpeg: flags.FFmpeg, IntervalMS: flags.IntervalMS}
	var out, errOut bytes.Buffer
	code := runPostAnalyzeWith(context.Background(), flags, src, &out, &errOut)
	if code != 3 {
		t.Fatalf("absent ffprobe must SKIP (exit 3), never PASS; got %d. out=%s", code, out.String())
	}
	pf := readPostFinding(t, flags.FindingsOut)
	if pf.Decision != "SKIP" || !strings.Contains(pf.Reason, "absent") {
		t.Fatalf("absent-tool decision=%q reason=%q, want SKIP mentioning 'absent'", pf.Decision, pf.Reason)
	}
}

// TestFFprobeFrameSource_RealClips runs the REAL ffmpeg/ffprobe backend on
// programmatically-generated mp4s (advancing testsrc → PASS; single-color
// → FAIL frozen). Skipped if either binary is unavailable on the host —
// that SKIP is honest (§11.4.3), not a hidden pass.
func TestFFprobeFrameSource_RealClips(t *testing.T) {
	ffmpeg, e1 := exec.LookPath("ffmpeg")
	ffprobe, e2 := exec.LookPath("ffprobe")
	if e1 != nil || e2 != nil {
		t.Skipf("SKIP-OK real-backend: ffmpeg=%v ffprobe=%v (honest SKIP per §11.4.3)", e1, e2)
	}
	dir := t.TempDir()

	good := filepath.Join(dir, "advancing.mp4")
	// testsrc = animated test pattern → every frame differs (live).
	mk := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=128x128:rate=10:duration=2",
		"-pix_fmt", "yuv420p", good)
	if b, err := mk.CombinedOutput(); err != nil {
		t.Skipf("SKIP-OK could not synthesize advancing clip: %v %s", err, b)
	}

	bad := filepath.Join(dir, "frozen.mp4")
	// color=gray = single static color for every frame → frozen.
	mk2 := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "color=c=gray:size=128x128:rate=10:duration=2",
		"-pix_fmt", "yuv420p", bad)
	if b, err := mk2.CombinedOutput(); err != nil {
		t.Skipf("SKIP-OK could not synthesize frozen clip: %v %s", err, b)
	}

	src := &FFprobeFrameSource{FFprobe: ffprobe, FFmpeg: ffmpeg, IntervalMS: 200}

	// advancing → PASS
	fg := defaultPostFlags(t, good)
	fg.FFprobe = ffprobe
	fg.FFmpeg = ffmpeg
	fg.IntervalMS = 200
	var go1, ge1 bytes.Buffer
	if code := runPostAnalyzeWith(context.Background(), fg, src, &go1, &ge1); code != 0 {
		t.Fatalf("real advancing clip must PASS; got %d. out=%s err=%s", code, go1.String(), ge1.String())
	}

	// frozen → FAIL
	fb := defaultPostFlags(t, bad)
	fb.FFprobe = ffprobe
	fb.FFmpeg = ffmpeg
	fb.IntervalMS = 200
	var go2, ge2 bytes.Buffer
	if code := runPostAnalyzeWith(context.Background(), fb, src, &go2, &ge2); code != 1 {
		t.Fatalf("real frozen clip must FAIL; got %d. out=%s err=%s", code, go2.String(), ge2.String())
	}
}
