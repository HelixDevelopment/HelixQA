// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// recording-analyzer CLI tests.
//
// Anti-bluff design (Constitution §11.4):
//
//   - Every test asserts a SPECIFIC observable, not just exit code.
//   - PASS/FAIL/UNKNOWN decision logic is tested with synthetic
//     timeline events + synthetic OCR results so the §11.4 enforcement
//     piece itself can't silently regress.
//   - We do NOT spawn ffmpeg in unit tests — the FrameExtractor /
//     AudioExtractor are interfaces, and we substitute stubs.
//   - We do NOT require a live Tesseract / Whisper container — the
//     OCR client is an interface, mocked here.
//
// The result: this test suite runs in <1 s, in air-gapped CI, and
// still locks every load-bearing branch of the analyzer.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/audio"
)

// ---------- Stubs ----------

type stubFrames struct {
	// per-display map: id → frame paths to "produce"
	byDisplay map[int][]string
	err       error
}

func (s *stubFrames) Extract(ctx context.Context, videoPath, outDir string, intervalMS int) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	// Map video path to display ID — videoPath ends with display_<N>.mp4
	id := -1
	if strings.HasSuffix(videoPath, "display_0.mp4") {
		id = 0
	} else if strings.HasSuffix(videoPath, "display_2.mp4") {
		id = 2
	}
	paths := s.byDisplay[id]
	// Materialise each as an empty PNG file so any hypothetical
	// downstream stat() works. (The OCR stub doesn't actually open them.)
	out := make([]string, 0, len(paths))
	for _, name := range paths {
		full := filepath.Join(outDir, name)
		_ = os.MkdirAll(outDir, 0o755)
		_ = os.WriteFile(full, []byte("png-stub"), 0o644)
		out = append(out, full)
	}
	return out, nil
}

type stubAudio struct{}

func (s *stubAudio) Extract(ctx context.Context, videoPath, outWAV string) error {
	// Pretend it worked but write nothing — analyzer treats audio as
	// best-effort.
	return nil
}

// stubOCR returns canned text per frame path basename. If a basename
// is unset the OCR call returns "" (mimics empty OCR).
type stubOCR struct {
	byBasename map[string]string
	err        error
	healthErr  error
}

func (s *stubOCR) OCR(ctx context.Context, imagePath string, opts audio.OCROptions) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.byBasename[filepath.Base(imagePath)], nil
}

func (s *stubOCR) Health(ctx context.Context) (*audio.TesseractHealthResponse, error) {
	if s.healthErr != nil {
		return nil, s.healthErr
	}
	return &audio.TesseractHealthResponse{
		Status:           "ok",
		TesseractVersion: "5.3.4-stub",
		Langs:            []string{"eng", "rus", "osd"},
		DefaultLang:      "eng",
		DefaultPSM:       6,
	}, nil
}

// ---------- Helpers ----------

// writeSyncJSON drops a sync.json into dir, returns its path. Mirrors
// the dual_display_record.sh schema.
func writeSyncJSON(t *testing.T, dir, testName, serial string, secondaryPresent int) string {
	t.Helper()
	primary := filepath.Join(dir, sanitizeTestName(testName)+"__"+serial+"__display_0.mp4")
	secondary := filepath.Join(dir, sanitizeTestName(testName)+"__"+serial+"__display_2.mp4")
	// Materialise the .mp4s so fileExists() returns true.
	_ = os.WriteFile(primary, []byte("primary-mp4-stub-content"), 0o644)
	if secondaryPresent == 1 {
		_ = os.WriteFile(secondary, []byte("secondary-mp4-stub-content"), 0o644)
	}
	syncPath := filepath.Join(dir, sanitizeTestName(testName)+"__"+serial+"__sync.json")
	body := SyncMeta{
		TestName:           testName,
		DeviceSerial:       serial,
		HostTSISO:          "2026-05-06T13:42:00.000Z",
		HostTSEpoch:        "1746536520.000",
		PrimaryDisplayID:   0,
		PrimaryHostPath:    primary,
		SecondaryPresent:   secondaryPresent,
		SecondaryDisplayID: 2,
		SecondaryHostPath:  secondary,
	}
	b, _ := json.Marshal(body)
	if err := os.WriteFile(syncPath, b, 0o644); err != nil {
		t.Fatalf("write sync json: %v", err)
	}
	return syncPath
}

// writeTimeline drops a JSONL timeline file. ts_epoch values are
// supplied as floats relative to the host_ts_epoch=1746536520.000.
func writeTimeline(t *testing.T, dir, testName string, events []struct {
	tsOffsetS float64
	event     string
	detail    string
}) string {
	t.Helper()
	path := filepath.Join(dir, "timeline.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create timeline: %v", err)
	}
	defer f.Close()
	const baseEpoch = 1746536520.0
	for i, ev := range events {
		line := TimelineEvent{
			TSISO:   "2026-05-06T13:42:00.000Z",
			TSEpoch: floatStr(baseEpoch + ev.tsOffsetS),
			Test:    testName,
			Event:   ev.event,
			Detail:  ev.detail,
			Seq:     i + 1,
		}
		b, _ := json.Marshal(line)
		_, _ = f.Write(append(b, '\n'))
	}
	return path
}

func floatStr(f float64) string {
	// Match action_timeline.sh's `date +%s.%N` style — we just need
	// a parseable string.
	return jsonFloat(f)
}

// jsonFloat is a tiny inline that uses the JSON encoder's float
// formatter so we don't need to import strconv just for the test.
func jsonFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// ---------- Tests ----------

// TestParseFlags_Defaults — no argv ⇒ defaults populated, exit 0.
func TestParseFlags_Defaults(t *testing.T) {
	var stderr bytes.Buffer
	flags, code := parseFlags(nil, &stderr)
	if code != 0 {
		t.Fatalf("parseFlags returned %d, stderr=%s", code, stderr.String())
	}
	if flags.RecordingDir != "/tmp/__test_recording" {
		t.Errorf("RecordingDir default = %q", flags.RecordingDir)
	}
	if flags.TimelineFile != "/tmp/__action_timeline.jsonl" {
		t.Errorf("TimelineFile default = %q", flags.TimelineFile)
	}
	if flags.IntervalMS != 500 {
		t.Errorf("IntervalMS default = %d", flags.IntervalMS)
	}
	if flags.WhisperURL != audio.DefaultWhisperBaseURL {
		t.Errorf("WhisperURL default = %q", flags.WhisperURL)
	}
	if flags.TesseractURL != audio.DefaultTesseractBaseURL {
		t.Errorf("TesseractURL default = %q", flags.TesseractURL)
	}
}

// TestParseFlags_BadInterval — negative interval is a caller error.
func TestParseFlags_BadInterval(t *testing.T) {
	var stderr bytes.Buffer
	_, code := parseFlags([]string{"--interval-ms", "0"}, &stderr)
	if code != 2 {
		t.Fatalf("parseFlags exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "interval-ms") {
		t.Errorf("stderr missing interval-ms diagnostic: %s", stderr.String())
	}
}

// TestRun_NoTestName — without --test-name (and not --health-check) we
// exit 2 with a clear error.
func TestRun_NoTestName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), nil, &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr.String(), "--test-name is required") {
		t.Errorf("stderr missing --test-name diagnostic: %s", stderr.String())
	}
}

// TestLoadSyncMeta_MissingFile — clear error rather than nil deref.
func TestLoadSyncMeta_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	_, err := loadSyncMeta(tmp, "no_such_test")
	if err == nil {
		t.Fatalf("loadSyncMeta with no files should error")
	}
	if !strings.Contains(err.Error(), "no sync.json found") {
		t.Errorf("error doesn't mention 'no sync.json found': %v", err)
	}
}

// TestLoadSyncMeta_HappyPath — written sync.json round-trips correctly.
func TestLoadSyncMeta_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	_ = writeSyncJSON(t, tmp, "test_video_streaming", "ABC123", 1)
	meta, err := loadSyncMeta(tmp, "test_video_streaming")
	if err != nil {
		t.Fatalf("loadSyncMeta: %v", err)
	}
	if meta.TestName != "test_video_streaming" {
		t.Errorf("TestName = %q", meta.TestName)
	}
	if meta.DeviceSerial != "ABC123" {
		t.Errorf("DeviceSerial = %q", meta.DeviceSerial)
	}
	if meta.SecondaryPresent != 1 {
		t.Errorf("SecondaryPresent = %d", meta.SecondaryPresent)
	}
}

// TestLoadTimeline_FilterByTest — events belonging to other tests are
// excluded; well-formed events are decoded; junk lines are skipped.
func TestLoadTimeline_FilterByTest(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "tl.jsonl")
	body := strings.Join([]string{
		`{"ts_iso":"x","ts_epoch":"1746536520.000","test":"t1","event":"video_started","detail":"display=2","seq":1}`,
		`{"ts_iso":"x","ts_epoch":"1746536521.000","test":"t2","event":"unrelated","detail":"","seq":2}`,
		`not-json-junk-line`,
		`{"ts_iso":"x","ts_epoch":"1746536522.000","test":"t1","event":"video_paused","detail":"","seq":3}`,
	}, "\n")
	_ = os.WriteFile(path, []byte(body), 0o644)
	events, err := loadTimeline(path, "t1")
	if err != nil {
		t.Fatalf("loadTimeline: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (filtered by test)", len(events))
	}
	if events[0].Event != "video_started" || events[1].Event != "video_paused" {
		t.Errorf("wrong event names: %+v", events)
	}
	if events[0].tsEpochF == 0 {
		t.Errorf("tsEpochF not parsed for events[0]")
	}
}

// TestDecisionLogic_PASS — video_* event on EXPECTED display with
// non-trivial OCR ⇒ PASS. This is the §11.4 happy path.
func TestDecisionLogic_PASS(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 1, SecondaryDisplayID: 2}
	rec := frameRecord{display: 2, idx: 5, tsOffsetS: 2.5, ocrLines: []string{"VK Video", "Now playing"}}
	ev := &TimelineEvent{Event: "video_started", Detail: "display=2 player=exoplayer"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "PASS" {
		t.Fatalf("Decision = %q (reason=%q), want PASS", f.Decision, f.Reason)
	}
	if f.Display != 2 || f.MatchedEvent != "video_started" {
		t.Errorf("Display=%d MatchedEvent=%q", f.Display, f.MatchedEvent)
	}
}

// TestDecisionLogic_FAIL — the Bug #13 catcher. video_* event with
// detail "display=2" but the recording shows the content on display
// 0 ⇒ FAIL.
func TestDecisionLogic_FAIL_WrongDisplay(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 1, SecondaryDisplayID: 2}
	rec := frameRecord{display: 0, idx: 5, tsOffsetS: 2.5, ocrLines: []string{"VK Video", "Now playing"}}
	ev := &TimelineEvent{Event: "video_started", Detail: "display=2 player=exoplayer"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "FAIL" {
		t.Fatalf("Decision = %q, want FAIL (Bug #13 escape catcher)", f.Decision)
	}
	if !strings.Contains(f.Reason, "display 0") || !strings.Contains(f.Reason, "expected display 2") {
		t.Errorf("FAIL reason missing actual/expected detail: %q", f.Reason)
	}
}

// TestDecisionLogic_UNKNOWN_NoVideoPrefix — non-video event ⇒ UNKNOWN.
func TestDecisionLogic_UNKNOWN_NoVideoPrefix(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0}
	rec := frameRecord{display: 0, ocrLines: []string{"Settings"}}
	ev := &TimelineEvent{Event: "app_launched", Detail: "display=0"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "UNKNOWN" {
		t.Errorf("Decision = %q, want UNKNOWN (non-video event)", f.Decision)
	}
}

// TestDecisionLogic_UNKNOWN_OCREmpty — video_* event but OCR empty ⇒
// UNKNOWN (we cannot confirm visible content).
func TestDecisionLogic_UNKNOWN_OCREmpty(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 1, SecondaryDisplayID: 2}
	rec := frameRecord{display: 2, ocrLines: []string{""}}
	ev := &TimelineEvent{Event: "video_started", Detail: "display=2"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "UNKNOWN" {
		t.Errorf("Decision = %q, want UNKNOWN (empty OCR)", f.Decision)
	}
	if !strings.Contains(f.Reason, "OCR empty") {
		t.Errorf("UNKNOWN reason should mention 'OCR empty': %q", f.Reason)
	}
}

// TestDecisionLogic_UNKNOWN_NoDisplayDirective — video_* event without
// display=… ⇒ UNKNOWN (cannot judge correctness).
func TestDecisionLogic_UNKNOWN_NoDisplayDirective(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0}
	rec := frameRecord{display: 0, ocrLines: []string{"Some Visible Text"}}
	ev := &TimelineEvent{Event: "video_started", Detail: "player=exoplayer"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "UNKNOWN" {
		t.Errorf("Decision = %q, want UNKNOWN (no display=… in detail)", f.Decision)
	}
}

// TestDecisionLogic_PrimarySecondaryAlias — display=primary /
// display=secondary alias resolution.
func TestDecisionLogic_PrimarySecondaryAlias(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 1, SecondaryDisplayID: 2}
	rec := frameRecord{display: 2, ocrLines: []string{"YouTube playing"}}
	ev := &TimelineEvent{Event: "video_started", Detail: "display=secondary"}
	f := makeFinding(meta, rec, ev)
	if f.Decision != "PASS" {
		t.Fatalf("Decision = %q (reason=%q), want PASS via display=secondary alias", f.Decision, f.Reason)
	}
	// Primary alias on secondary frame ⇒ FAIL
	ev2 := &TimelineEvent{Event: "video_started", Detail: "display=primary"}
	f2 := makeFinding(meta, rec, ev2)
	if f2.Decision != "FAIL" {
		t.Errorf("display=primary on secondary frame should FAIL (got %q)", f2.Decision)
	}
}

// TestIsOCRTrivial — locks the heuristic so a future tweak doesn't
// silently change anti-bluff behavior.
func TestIsOCRTrivial(t *testing.T) {
	if !isOCRTrivial(nil) {
		t.Errorf("nil should be trivial")
	}
	if !isOCRTrivial([]string{"", "  ", "12:34"}) {
		t.Errorf("only digits/whitespace should be trivial")
	}
	if isOCRTrivial([]string{"VK Video"}) {
		t.Errorf("two letters of 'VK' + 'Video' should NOT be trivial")
	}
}

// TestExpectedDisplayFromDetail_Variants
func TestExpectedDisplayFromDetail_Variants(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 1, SecondaryDisplayID: 2}
	cases := []struct {
		detail string
		wantID int
		wantOK bool
	}{
		{"display=2", 2, true},
		{"display=primary", 0, true},
		{"display=secondary", 2, true},
		{"player=exoplayer", 0, false},
		{"display=bogus", 0, false},
		{"foo display=7 bar", 7, true},
	}
	for _, c := range cases {
		gotID, gotOK := expectedDisplayFromDetail(c.detail, meta)
		if gotID != c.wantID || gotOK != c.wantOK {
			t.Errorf("detail=%q → (%d,%v), want (%d,%v)", c.detail, gotID, gotOK, c.wantID, c.wantOK)
		}
	}
}

// TestExpectedDisplay_SecondaryAlias_NotPresent — when secondary is
// not connected (D2 hardware spec), display=secondary should resolve
// to (_, false) so we don't lie about an absent display.
func TestExpectedDisplay_SecondaryAlias_NotPresent(t *testing.T) {
	meta := &SyncMeta{PrimaryDisplayID: 0, SecondaryPresent: 0}
	id, ok := expectedDisplayFromDetail("display=secondary", meta)
	if ok {
		t.Fatalf("display=secondary on D2-class hardware should be unresolved, got id=%d ok=%v", id, ok)
	}
}

// TestSanitizeTestName — mirrors dual_display_record.sh tr -c.
func TestSanitizeTestName(t *testing.T) {
	got := sanitizeTestName("test video/streaming-1.sh")
	want := "test_video_streaming_1_sh"
	if got != want {
		t.Errorf("sanitizeTestName = %q, want %q", got, want)
	}
}

// TestAnalyze_EndToEnd — the full pipeline with all I/O stubbed.
// Synthesises sync.json + timeline + frames + OCR results and
// verifies the JSONL findings file is populated correctly.
func TestAnalyze_EndToEnd(t *testing.T) {
	tmp := t.TempDir()
	syncPath := writeSyncJSON(t, tmp, "test_pipeline", "DEV1", 1)
	_ = syncPath
	// Two events: one video_* with display=2 (PASS), one video_* with
	// display=2 but the matching frame is on display 0 (FAIL).
	tlPath := writeTimeline(t, tmp, "test_pipeline", []struct {
		tsOffsetS float64
		event     string
		detail    string
	}{
		{tsOffsetS: 0.5, event: "video_started", detail: "display=2"},
		{tsOffsetS: 1.5, event: "video_other", detail: "display=2"},
	})
	meta, err := loadSyncMeta(tmp, "test_pipeline")
	if err != nil {
		t.Fatalf("loadSyncMeta: %v", err)
	}
	events, err := loadTimeline(tlPath, "test_pipeline")
	if err != nil {
		t.Fatalf("loadTimeline: %v", err)
	}

	// Stub OCR: the frame on display 2 (frame_00002) is "VK Video",
	// the frame on display 0 (frame_00004) is "VK Video" too — so
	// the second event finds it on the WRONG display ⇒ FAIL.
	ocr := &stubOCR{
		byBasename: map[string]string{
			"frame_00002.png": "VK Video\nNow playing",
			"frame_00004.png": "VK Video\nNow playing",
		},
	}
	frames := &stubFrames{
		byDisplay: map[int][]string{
			0: {"frame_00001.png", "frame_00002.png", "frame_00003.png", "frame_00004.png"},
			2: {"frame_00001.png", "frame_00002.png", "frame_00003.png", "frame_00004.png"},
		},
	}

	flags := Flags{
		RecordingDir: tmp,
		TimelineFile: tlPath,
		FindingsOut:  filepath.Join(tmp, "findings.jsonl"),
		IntervalMS:   500,
		TestName:     "test_pipeline",
		OCRTO:        100 * 1e9, // 100 ns; stub returns sync.
	}

	findings, err := analyze(context.Background(), flags, meta, events, frames, &stubAudio{}, ocr)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(findings) == 0 {
		t.Fatalf("findings empty — pipeline produced no rows")
	}

	// Assertions on the §11.4 decision logic:
	gotPass := 0
	gotFail := 0
	for _, f := range findings {
		switch f.Decision {
		case "PASS":
			gotPass++
		case "FAIL":
			gotFail++
		}
	}
	if gotPass == 0 {
		t.Errorf("expected at least one PASS finding (display=2 + frame on display 2 + non-trivial OCR)")
	}
	if gotFail == 0 {
		t.Errorf("expected at least one FAIL finding (display=2 expected but frame on display 0 — Bug #13 catcher)")
	}

	// Round-trip: write + re-read JSONL.
	if err := writeFindings(flags.FindingsOut, findings); err != nil {
		t.Fatalf("writeFindings: %v", err)
	}
	data, err := os.ReadFile(flags.FindingsOut)
	if err != nil {
		t.Fatalf("read findings.jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"decision":"FAIL"`) {
		t.Errorf("findings.jsonl missing FAIL row — Bug #13 escape would not be caught:\n%s", string(data))
	}
	if !strings.Contains(string(data), `"decision":"PASS"`) {
		t.Errorf("findings.jsonl missing PASS row:\n%s", string(data))
	}
}

// TestEmitSummary_FailExitsNonzero — when analyze returns any FAIL,
// hasAnyFail must propagate so the orchestrator sees exit 1.
func TestEmitSummary_FailExitsNonzero(t *testing.T) {
	findings := []Finding{
		{Decision: "PASS"},
		{Decision: "FAIL"},
		{Decision: "UNKNOWN"},
	}
	if !hasAnyFail(findings) {
		t.Errorf("hasAnyFail should be true when at least one FAIL present")
	}
	findings2 := []Finding{{Decision: "PASS"}, {Decision: "UNKNOWN"}}
	if hasAnyFail(findings2) {
		t.Errorf("hasAnyFail should be false when no FAIL present")
	}
	// emitSummary should not panic on either input.
	var buf bytes.Buffer
	emitSummary(&buf, Flags{}, &SyncMeta{TestName: "x", DeviceSerial: "y"}, findings)
	if !strings.Contains(buf.String(), "PASS=1") || !strings.Contains(buf.String(), "FAIL=1") {
		t.Errorf("summary missing per-decision counts: %s", buf.String())
	}
}

// TestInWindow — locks the [center+lo, center+hi] semantics so a
// future refactor doesn't accidentally exclude boundary frames.
func TestInWindow(t *testing.T) {
	if !inWindow(10.0, 11.0, -1.0, 5.0) {
		t.Errorf("10.0 should be in [11-1, 11+5]")
	}
	if !inWindow(16.0, 11.0, -1.0, 5.0) {
		t.Errorf("16.0 (boundary) should be in window")
	}
	if inWindow(9.99, 11.0, -1.0, 5.0) {
		t.Errorf("9.99 should NOT be in [10, 16]")
	}
	if inWindow(16.01, 11.0, -1.0, 5.0) {
		t.Errorf("16.01 should NOT be in [10, 16]")
	}
}

// TestRun_NotApplicable_NoVideoEvents — Bug #27 / §11.4.1 fix.
//
// A timeline that carries zero video_* events (e.g. test_lawnchair_recents.sh,
// test_navigation_bar.sh, test_settings_connected_devices.sh) MUST exit 0
// with a NOT_APPLICABLE finding. Pre-fix behaviour was: proceed through
// the full pipeline, find every frame UNKNOWN (no video event in window),
// and orchestrator wrappers treated the analyzer's exit 1 (which can
// fire if any FAIL slips in via stale frames) — or a downstream "no PASS
// rows" check — as ANALYZER FAIL. That was a §11.4.1 FAIL-bluff: a
// non-video test scoring FAIL because the analyzer never applied.
func TestRun_NotApplicable_NoVideoEvents(t *testing.T) {
	tmp := t.TempDir()
	_ = writeSyncJSON(t, tmp, "test_lawnchair_recents", "DEV1", 0)
	// Timeline with NO video events whatsoever.
	tlPath := writeTimeline(t, tmp, "test_lawnchair_recents", []struct {
		tsOffsetS float64
		event     string
		detail    string
	}{
		{tsOffsetS: 0.5, event: "app_launched", detail: "pkg=app.lawnchair"},
		{tsOffsetS: 1.5, event: "key_event", detail: "code=KEYCODE_HOME"},
		{tsOffsetS: 2.5, event: "ui_settled", detail: "window=Recents"},
	})
	findingsPath := filepath.Join(tmp, "findings.jsonl")

	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{
		"--recording-dir", tmp,
		"--timeline-file", tlPath,
		"--findings-out", findingsPath,
		"--test-name", "test_lawnchair_recents",
	}, &stdout, &stderr)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (NOT_APPLICABLE early-return). stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "NOT_APPLICABLE") {
		t.Errorf("stdout missing NOT_APPLICABLE summary: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "no §11.4 enforcement to perform") {
		t.Errorf("stdout missing exemption banner: %s", stdout.String())
	}
	data, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Fatalf("read findings.jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"decision":"NOT_APPLICABLE"`) {
		t.Errorf("findings.jsonl missing NOT_APPLICABLE row:\n%s", string(data))
	}
	// Crucially, the file MUST NOT contain a PASS row for a non-video
	// test — else §11.4.2 cross-checks would mistake non-applicability
	// for recording-validated success.
	if strings.Contains(string(data), `"decision":"PASS"`) {
		t.Errorf("findings.jsonl unexpectedly has PASS row for non-video test:\n%s", string(data))
	}
}

// TestRun_NotApplicable_VideoSubstringHeuristic — even if the consumer's
// timeline event doesn't begin with "video_", a name containing "video"
// or beginning with "playback_" must still trigger full analysis (NOT
// the early-return). Locks the secondary heuristic so a future tweak
// can't accidentally re-introduce the false-positive.
func TestRun_NotApplicable_VideoSubstringHeuristic(t *testing.T) {
	tmp := t.TempDir()
	_ = writeSyncJSON(t, tmp, "test_x", "DEV1", 0)
	// Use an event name that the legacy "video_" prefix check would
	// have missed but the substring heuristic catches.
	tlPath := writeTimeline(t, tmp, "test_x", []struct {
		tsOffsetS float64
		event     string
		detail    string
	}{
		{tsOffsetS: 0.5, event: "playback_started", detail: "display=2"},
	})
	findingsPath := filepath.Join(tmp, "findings.jsonl")
	var stdout, stderr bytes.Buffer
	// We deliberately don't materialise primary mp4 with content
	// ffmpeg can decode — analyze() will fail at frame extraction
	// against the stub mp4 file, returning a non-zero exit. That's
	// fine: the load-bearing assertion is that we did NOT hit the
	// NOT_APPLICABLE early-return (which would emit "no §11.4
	// enforcement to perform"). exit code is intentionally not
	// asserted here.
	_ = run(context.Background(), []string{
		"--recording-dir", tmp,
		"--timeline-file", tlPath,
		"--findings-out", findingsPath,
		"--test-name", "test_x",
		"--ffmpeg", "/bin/false", // force the full path to be exercised
	}, &stdout, &stderr)
	if strings.Contains(stdout.String(), "no §11.4 enforcement to perform") {
		t.Errorf("playback_* event must NOT trigger NOT_APPLICABLE early-return: %s", stdout.String())
	}
}

// Compile-time interface compliance checks — guarantees that if the
// pkg/audio types ever drift, this file fails to build instead of
// silently mocking the wrong shape.
var _ OCRClient = (*audio.TesseractClient)(nil)
var _ ASRClient = (*audio.WhisperClient)(nil)

// Compile-time check that the io.Writer return slot exists (silences
// "declared and not used" if we drop the errOut wiring later).
var _ io.Writer = (*bytes.Buffer)(nil)

// TestAnalyze_NegativeControl_AllGoodDifferentContent — the §11.4.201(1)
// false-positive guard (design Phase-1 item 3 negative-control): a
// genuinely-good clip with DIFFERENT content (Netflix, not the EndToEnd VK
// clip) whose video content appears ONLY on the expected display MUST produce
// PASS findings and ZERO FAIL — proving the analyzer keys on wrong-display
// FIDELITY, not on recognising a specific golden clip. An analyzer that
// false-FAILs a legitimate new clip is itself a bluff (§11.4.107(10)).
func TestAnalyze_NegativeControl_AllGoodDifferentContent(t *testing.T) {
	tmp := t.TempDir()
	_ = writeSyncJSON(t, tmp, "netflix_play", "DEV1", 1)
	tlPath := writeTimeline(t, tmp, "netflix_play", []struct {
		tsOffsetS float64
		event     string
		detail    string
	}{
		{tsOffsetS: 0.5, event: "video_started", detail: "display=2"},
		{tsOffsetS: 1.5, event: "video_playing", detail: "display=2"},
	})
	meta, err := loadSyncMeta(tmp, "netflix_play")
	if err != nil {
		t.Fatalf("loadSyncMeta: %v", err)
	}
	events, err := loadTimeline(tlPath, "netflix_play")
	if err != nil {
		t.Fatalf("loadTimeline: %v", err)
	}

	// Content (non-trivial OCR) ONLY on display-2 frames; display-0 frames use
	// DIFFERENT basenames absent from the OCR map -> empty OCR -> UNKNOWN (never
	// FAIL). Frame time = list index * interval, so distinct names are fine.
	ocr := &stubOCR{byBasename: map[string]string{
		"d2_00002.png": "Netflix\nS01E01 Now Playing",
		"d2_00003.png": "Netflix\nS01E01",
		"d2_00004.png": "Netflix\nEpisode Two",
	}}
	frames := &stubFrames{byDisplay: map[int][]string{
		2: {"d2_00001.png", "d2_00002.png", "d2_00003.png", "d2_00004.png"},
		0: {"d0_00001.png", "d0_00002.png", "d0_00003.png", "d0_00004.png"},
	}}

	flags := Flags{
		RecordingDir: tmp,
		TimelineFile: tlPath,
		FindingsOut:  filepath.Join(tmp, "findings.jsonl"),
		IntervalMS:   500,
		TestName:     "netflix_play",
		OCRTO:        100 * 1e9,
	}

	findings, err := analyze(context.Background(), flags, meta, events, frames, &stubAudio{}, ocr)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	gotPass, gotFail := 0, 0
	for _, f := range findings {
		switch f.Decision {
		case "PASS":
			gotPass++
		case "FAIL":
			gotFail++
		}
	}
	if gotFail != 0 {
		t.Errorf("negative-control (all-good different content) produced %d FAIL finding(s) — FALSE POSITIVE (§11.4.201(1))", gotFail)
	}
	if gotPass == 0 {
		t.Errorf("negative-control expected >=1 PASS finding (Netflix content on expected display 2)")
	}
}
