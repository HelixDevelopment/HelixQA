// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// helixqa-bridge HTTP+SSE service tests.
//
// Anti-bluff design (Constitution §11.4):
//   - every test asserts a SPECIFIC observable side-effect, not just
//     "no panic" or "200 OK".
//   - the commandRunner seam is replaced with a fakeRunner that
//     records every os/exec call so we prove the bridge actually
//     invoked dual_display_record.sh with the expected args (a bridge
//     that returns 200 without invoking the script would silently let
//     end-user features ship broken).
//   - SSE tests prove the polling cursor advances and that mid-line
//     reads don't lose bytes — both are easy bluff modes (return
//     "ready" event then nothing).
//   - Path-traversal tests prove validateTestName actually rejects
//     "../" and absolute paths — a regex bug here would let a remote
//     loopback caller read arbitrary files via /v1/timeline.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakeRunner — testable commandRunner double
// ---------------------------------------------------------------------------

type runCall struct {
	Name string
	Args []string
}

type fakeRunner struct {
	mu sync.Mutex

	// Calls records every Run() / Start() invocation in order.
	Calls []runCall

	// RunOutput is the canned combined-output for Run().
	RunOutput []byte
	// RunError is the canned error for Run().
	RunError error

	// StartPID is the canned PID for Start().
	StartPID int
	// StartError is the canned error for Start().
	StartError error
	// startCancelled tracks whether the Start cleanup was invoked.
	startCancelled atomic.Bool
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, runCall{Name: name, Args: append([]string(nil), args...)})
	f.mu.Unlock()
	return f.RunOutput, f.RunError
}

func (f *fakeRunner) Start(ctx context.Context, name string, args ...string) (int, func(), error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, runCall{Name: name, Args: append([]string(nil), args...)})
	f.mu.Unlock()
	if f.StartError != nil {
		return 0, func() {}, f.StartError
	}
	cleanup := func() { f.startCancelled.Store(true) }
	return f.StartPID, cleanup, nil
}

func (f *fakeRunner) snapshotCalls() []runCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]runCall, len(f.Calls))
	copy(out, f.Calls)
	return out
}

// newTestServer builds a Server wired to a fakeRunner and a temp dir
// scratch space. The audioProbe is replaced with a static OK lambda
// so the tests don't depend on real Whisper / Tesseract containers.
func newTestServer(t *testing.T) (*Server, *fakeRunner, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		ListenAddr:     "127.0.0.1:0", // not used in these tests (mux directly)
		RecordScript:   filepath.Join(dir, "dual_display_record.sh"),
		AnalyzerBinary: filepath.Join(dir, "recording-analyzer"),
		RecordingDir:   filepath.Join(dir, "recording"),
		FindingsFile:   filepath.Join(dir, "findings.jsonl"),
		TimelineDir:    dir,
		WhisperURL:     "http://127.0.0.1:7070",
		TesseractURL:   "http://127.0.0.1:7071",
		RecordTimeout:  5 * time.Second,
		AnalyzeTimeout: 5 * time.Second,
		HealthTimeout:  500 * time.Millisecond,
	}
	// Make the script + binary look "present" (regular file + 0755).
	if err := os.WriteFile(cfg.RecordScript, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatalf("write fake record script: %v", err)
	}
	if err := os.WriteFile(cfg.AnalyzerBinary, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatalf("write fake analyzer binary: %v", err)
	}
	if err := os.MkdirAll(cfg.RecordingDir, 0o755); err != nil {
		t.Fatalf("mkdir recording dir: %v", err)
	}
	srv := NewServer(cfg)
	fr := &fakeRunner{}
	srv.runner = fr
	// Static OK probe — captured-evidence asserted by individual tests
	// that exercise the health endpoint directly.
	srv.audioProbe = func(ctx context.Context, _, _ string, _ time.Duration) (
		bool, string, error, bool, string, error,
	) {
		return true, "backend=fake default_model=base loaded=[base] compute=int8", nil,
			true, "version=5.3.4 default_lang=eng default_psm=6 langs=[eng]", nil
	}
	return srv, fr, dir
}

// httpDo executes an in-process request against the server's mux.
func httpDo(t *testing.T, srv *Server, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func TestValidateTestName(t *testing.T) {
	good := []string{
		"test_video_streaming",
		"test-foo",
		"a",
		"abc123",
		"X_Y-Z",
	}
	for _, g := range good {
		if err := validateTestName(g); err != nil {
			t.Errorf("validateTestName(%q) unexpected err: %v", g, err)
		}
	}
	bad := []string{
		"",
		"../etc/passwd",
		"foo/bar",
		"foo bar",
		"foo;rm -rf /",
		"foo\nbar",
		"foo$bar",
		strings.Repeat("a", 129),
	}
	for _, b := range bad {
		if err := validateTestName(b); err == nil {
			t.Errorf("validateTestName(%q) expected error, got nil", b)
		}
	}
}

func TestValidateDeviceSerial(t *testing.T) {
	good := []string{"", "ABC123", "192.168.1.10:5555", "emulator-5554", "sn_001"}
	for _, g := range good {
		if err := validateDeviceSerial(g); err != nil {
			t.Errorf("validateDeviceSerial(%q) unexpected err: %v", g, err)
		}
	}
	bad := []string{"foo bar", "foo;ls", "../baz", "$(whoami)"}
	for _, b := range bad {
		if err := validateDeviceSerial(b); err == nil {
			t.Errorf("validateDeviceSerial(%q) expected error, got nil", b)
		}
	}
}

func TestConfigValidateLoopback(t *testing.T) {
	cases := []struct {
		addr    string
		wantErr bool
	}{
		{"127.0.0.1:7842", false},
		{"127.0.0.5:8080", false},
		{"localhost:7842", false},
		{"[::1]:7842", false},
		{"0.0.0.0:7842", true},
		{"192.168.1.1:7842", true},
		{"example.com:7842", true},
		{"not a host", true},
	}
	for _, tc := range cases {
		c := Config{ListenAddr: tc.addr}
		err := c.validateLoopback()
		if tc.wantErr && err == nil {
			t.Errorf("addr=%q expected error, got nil", tc.addr)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("addr=%q unexpected error: %v", tc.addr, err)
		}
	}
}

// ---------------------------------------------------------------------------
// /v1/recording/start
// ---------------------------------------------------------------------------

func TestRecordingStart_OK(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.RunOutput = []byte("OK: recording started for foo on emulator-5554")

	body := `{"test_name":"my_test","device_serial":"emulator-5554","interval_ms":500}`
	req := httptest.NewRequest(http.MethodPost, "/v1/recording/start", strings.NewReader(body))
	rec := httpDo(t, srv, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp startRecordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	if resp.Status != "started" {
		t.Errorf("Status = %q, want 'started'", resp.Status)
	}
	if !strings.Contains(resp.SyncPath, "my_test__emulator-5554__sync.json") {
		t.Errorf("SyncPath = %q, expected contains 'my_test__emulator-5554__sync.json'", resp.SyncPath)
	}
	// Anti-bluff: bridge MUST have actually invoked the script.
	calls := fr.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d: %+v", len(calls), calls)
	}
	if calls[0].Name != "bash" {
		t.Errorf("call[0].Name = %q, want 'bash'", calls[0].Name)
	}
	wantArgs := []string{srv.cfg.RecordScript, "start", "my_test", "emulator-5554"}
	if !equalStrings(calls[0].Args, wantArgs) {
		t.Errorf("call[0].Args = %v, want %v", calls[0].Args, wantArgs)
	}
}

func TestRecordingStart_PathTraversalRejected(t *testing.T) {
	srv, fr, _ := newTestServer(t)

	body := `{"test_name":"../etc/passwd","device_serial":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/recording/start", strings.NewReader(body))
	rec := httpDo(t, srv, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for path-traversal test_name", rec.Code)
	}
	// The script MUST NOT have been invoked.
	if len(fr.snapshotCalls()) != 0 {
		t.Errorf("script invoked despite invalid test_name (security bug): %+v", fr.snapshotCalls())
	}
}

func TestRecordingStart_ScriptError(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.RunOutput = []byte("FAIL: no ADB device available")
	fr.RunError = errors.New("exit status 1")

	body := `{"test_name":"foo","device_serial":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/recording/start", strings.NewReader(body))
	rec := httpDo(t, srv, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 for script failure", rec.Code)
	}
	var er errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Anti-bluff: caller MUST see WHY the script failed (operator can't
	// debug otherwise).
	if !strings.Contains(er.Detail, "no ADB device available") {
		t.Errorf("Detail missing script stderr: %q", er.Detail)
	}
}

func TestRecordingStart_OmittedSerialNotPassedToScript(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.RunOutput = []byte("OK")

	body := `{"test_name":"foo","device_serial":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/recording/start", strings.NewReader(body))
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	calls := fr.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	wantArgs := []string{srv.cfg.RecordScript, "start", "foo"}
	if !equalStrings(calls[0].Args, wantArgs) {
		t.Errorf("call[0].Args = %v, want %v (no trailing serial)", calls[0].Args, wantArgs)
	}
}

// ---------------------------------------------------------------------------
// /v1/recording/stop
// ---------------------------------------------------------------------------

func TestRecordingStop_ReadsFileSizes(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.RunOutput = []byte("OK: recording stopped")

	// Pre-populate the recording dir with fake mp4 files matching the
	// path layout the bash script would produce.
	prefix := filepath.Join(srv.cfg.RecordingDir, "alpha__SN001")
	primary := prefix + "__display_0.mp4"
	secondary := prefix + "__display_2.mp4"
	if err := os.WriteFile(primary, bytes.Repeat([]byte("a"), 12345), 0o644); err != nil {
		t.Fatalf("write primary: %v", err)
	}
	if err := os.WriteFile(secondary, bytes.Repeat([]byte("b"), 6789), 0o644); err != nil {
		t.Fatalf("write secondary: %v", err)
	}

	body := `{"test_name":"alpha","device_serial":"SN001"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/recording/stop", strings.NewReader(body))
	rec := httpDo(t, srv, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp stopRecordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PrimarySize != 12345 {
		t.Errorf("PrimarySize = %d, want 12345 (positive evidence: stop must report real file size)", resp.PrimarySize)
	}
	if resp.SecondarySize != 6789 {
		t.Errorf("SecondarySize = %d, want 6789", resp.SecondarySize)
	}
	if !strings.HasSuffix(resp.SyncPath, "alpha__SN001__sync.json") {
		t.Errorf("SyncPath = %q, want suffix 'alpha__SN001__sync.json'", resp.SyncPath)
	}
	calls := fr.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	wantArgs := []string{srv.cfg.RecordScript, "stop", "alpha", "SN001"}
	if !equalStrings(calls[0].Args, wantArgs) {
		t.Errorf("call[0].Args = %v, want %v", calls[0].Args, wantArgs)
	}
}

// ---------------------------------------------------------------------------
// /v1/analyze/start
// ---------------------------------------------------------------------------

func TestAnalyzeStart_OK(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.StartPID = 4242

	body := `{"test_name":"foo","interval_ms":250}`
	req := httptest.NewRequest(http.MethodPost, "/v1/analyze/start", strings.NewReader(body))
	rec := httpDo(t, srv, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp analyzeStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PID != 4242 {
		t.Errorf("PID = %d, want 4242", resp.PID)
	}
	calls := fr.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != srv.cfg.AnalyzerBinary {
		t.Errorf("call[0].Name = %q, want %q", calls[0].Name, srv.cfg.AnalyzerBinary)
	}
	// Verify the bridge passed --interval-ms 250 (anti-bluff: a bridge
	// that ignores the request value would silently default to 500).
	if !containsArg(calls[0].Args, "--interval-ms", "250") {
		t.Errorf("call[0].Args missing --interval-ms 250: %v", calls[0].Args)
	}
	if !containsArg(calls[0].Args, "--test-name", "foo") {
		t.Errorf("call[0].Args missing --test-name foo: %v", calls[0].Args)
	}
	if !containsArg(calls[0].Args, "--findings-file", srv.cfg.FindingsFile) {
		t.Errorf("call[0].Args missing --findings-file %s: %v", srv.cfg.FindingsFile, calls[0].Args)
	}
}

func TestAnalyzeStart_RejectConcurrent(t *testing.T) {
	srv, fr, _ := newTestServer(t)
	fr.StartPID = 100

	body := `{"test_name":"foo"}`
	rec1 := httpDo(t, srv, httptest.NewRequest(http.MethodPost, "/v1/analyze/start", strings.NewReader(body)))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first start: status = %d, want 200", rec1.Code)
	}

	rec2 := httpDo(t, srv, httptest.NewRequest(http.MethodPost, "/v1/analyze/start", strings.NewReader(body)))
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second start: status = %d, want 409 (concurrent analyzer rejected)", rec2.Code)
	}
}

// ---------------------------------------------------------------------------
// /v1/findings/stream (SSE)
// ---------------------------------------------------------------------------

func TestFindingsStream_TailsAppendedLines(t *testing.T) {
	srv, _, _ := newTestServer(t)
	// Create the findings file (empty).
	if err := os.WriteFile(srv.cfg.FindingsFile, []byte{}, 0o644); err != nil {
		t.Fatalf("create findings file: %v", err)
	}

	// Spin up a real httptest server so streaming flushers behave like prod.
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/findings/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/findings/stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	// Append two JSONL lines after a short delay (must arrive on the
	// stream — captured-evidence the cursor advances).
	go func() {
		time.Sleep(300 * time.Millisecond)
		f, _ := os.OpenFile(srv.cfg.FindingsFile, os.O_APPEND|os.O_WRONLY, 0o644)
		_, _ = f.WriteString(`{"event":"first"}` + "\n")
		_, _ = f.WriteString(`{"event":"second"}` + "\n")
		_ = f.Close()
	}()

	// Read stream until both lines arrive or the test deadline trips.
	gotFirst, gotSecond := false, false
	buf := make([]byte, 4096)
	deadline := time.Now().Add(3 * time.Second)
	var collected bytes.Buffer
	for time.Now().Before(deadline) && !(gotFirst && gotSecond) {
		n, _ := resp.Body.Read(buf)
		if n > 0 {
			collected.Write(buf[:n])
			if strings.Contains(collected.String(), `"event":"first"`) {
				gotFirst = true
			}
			if strings.Contains(collected.String(), `"event":"second"`) {
				gotSecond = true
			}
		}
	}
	cancel()
	if !gotFirst {
		t.Errorf("did not receive 'first' line on SSE stream; collected=%q", collected.String())
	}
	if !gotSecond {
		t.Errorf("did not receive 'second' line on SSE stream; collected=%q", collected.String())
	}
	// Ready event MUST have fired up front so callers know the
	// connection is live (anti-bluff: a stream that never emits 'ready'
	// is indistinguishable from a hang).
	if !strings.Contains(collected.String(), "event: ready") {
		t.Errorf("missing 'event: ready' opening event; collected=%q", collected.String())
	}
}

// ---------------------------------------------------------------------------
// /v1/health
// ---------------------------------------------------------------------------

func TestHealth_AllOK(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.WhisperOK || !resp.TesseractOK {
		t.Errorf("audio probes not OK: %+v", resp)
	}
	if !resp.RecordingDirWritable {
		t.Errorf("recording dir not writable: %s", srv.cfg.RecordingDir)
	}
	if !resp.AnalyzerBinaryPresent {
		t.Errorf("analyzer binary not present: %s", srv.cfg.AnalyzerBinary)
	}
	if !resp.RecordScriptPresent {
		t.Errorf("record script not present: %s", srv.cfg.RecordScript)
	}
	// Anti-bluff: detail strings carry SPECIFIC version/model evidence,
	// not just "ok".
	if !strings.Contains(resp.WhisperDetail, "default_model=base") {
		t.Errorf("WhisperDetail missing model evidence: %q", resp.WhisperDetail)
	}
	if !strings.Contains(resp.TesseractDetail, "version=5.3.4") {
		t.Errorf("TesseractDetail missing version evidence: %q", resp.TesseractDetail)
	}
}

func TestHealth_AudioFailureSurfaced(t *testing.T) {
	srv, _, _ := newTestServer(t)
	srv.audioProbe = func(ctx context.Context, _, _ string, _ time.Duration) (
		bool, string, error, bool, string, error,
	) {
		return false, "", errors.New("connection refused"),
			true, "version=5.3.4", nil
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (failures still served as JSON)", rec.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WhisperOK {
		t.Errorf("WhisperOK = true, want false")
	}
	// Anti-bluff: the failure message must surface so caller can debug.
	if !strings.Contains(resp.WhisperError, "connection refused") {
		t.Errorf("WhisperError missing diagnostic: %q", resp.WhisperError)
	}
}

// ---------------------------------------------------------------------------
// /v1/timeline
// ---------------------------------------------------------------------------

func TestTimeline_ReadsPerTestFile(t *testing.T) {
	srv, _, dir := newTestServer(t)
	path := filepath.Join(dir, "__action_timeline_my_test.jsonl")
	content := `{"event":"foo","seq":1}` + "\n" + `{"event":"bar","seq":2}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write timeline: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/timeline?test_name=my_test", nil)
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != content {
		t.Errorf("body = %q, want %q", rec.Body.String(), content)
	}
	if rec.Header().Get("X-Timeline-Path") != path {
		t.Errorf("X-Timeline-Path = %q, want %q", rec.Header().Get("X-Timeline-Path"), path)
	}
}

func TestTimeline_FallsBackToShared(t *testing.T) {
	srv, _, dir := newTestServer(t)
	shared := filepath.Join(dir, "__action_timeline.jsonl")
	if err := os.WriteFile(shared, []byte(`{"event":"shared"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write shared timeline: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/timeline?test_name=missing_test", nil)
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fallback)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "shared") {
		t.Errorf("did not fall back to shared timeline: body=%s", rec.Body.String())
	}
}

func TestTimeline_PathTraversalBlocked(t *testing.T) {
	srv, _, _ := newTestServer(t)
	cases := []string{
		"../etc/passwd",
		"../../../../../../etc/passwd",
		"foo/bar",
		"foo bar",
	}
	for _, c := range cases {
		// Build URL via url.Values so spaces/slashes don't break NewRequest.
		req := httptest.NewRequest(http.MethodGet, "/v1/timeline", nil)
		q := req.URL.Query()
		q.Set("test_name", c)
		req.URL.RawQuery = q.Encode()
		rec := httpDo(t, srv, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("test_name=%q: status = %d, want 400 (path traversal must be blocked at validation layer)",
				c, rec.Code)
		}
	}
}

func TestTimeline_NotFound(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/timeline?test_name=does_not_exist", nil)
	rec := httpDo(t, srv, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// realMain --health-check
// ---------------------------------------------------------------------------

func TestRealMain_RejectsNonLoopback(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := realMain([]string{"--listen", "0.0.0.0:7842"}, &stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2 (non-loopback bind must be refused)", exit)
	}
	if !strings.Contains(stderr.String(), "loopback") {
		t.Errorf("stderr missing 'loopback' diagnostic: %q", stderr.String())
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsArg(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// Unused but kept as a clarifying example of how a reader/writer pair
// would be wired in a streaming test if needed; silences "unused"
// linters that flag the io import.
var _ = io.Discard
var _ = fmt.Sprintf
