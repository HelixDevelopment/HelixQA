// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// qa-audio-probe CLI tests.
//
// Anti-bluff design: every PASS asserts a SPECIFIC string from the
// emitted output AND the exit code, not just "no panic". A run() that
// returned 0 but emitted nothing should fail these tests.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServers spins up two httptest servers (one for each container)
// with caller-controllable handlers. Returns the URLs and a cleanup fn.
type fakeServers struct {
	whisperURL   string
	tesseractURL string
	cleanup      func()
}

func newFakeServers(t *testing.T, whisperHandler, tesseractHandler http.HandlerFunc) *fakeServers {
	t.Helper()
	if whisperHandler == nil {
		whisperHandler = func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","backend":"faster-whisper","default_model":"base","loaded_models":["base"],"compute_type":"int8","models_dir":"/models"}`)
		}
	}
	if tesseractHandler == nil {
		tesseractHandler = func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"status":"ok","tesseract_version":"5.3.4","langs":["eng","rus","osd"],"default_lang":"eng+rus","default_psm":6}`)
		}
	}
	w := httptest.NewServer(whisperHandler)
	te := httptest.NewServer(tesseractHandler)
	return &fakeServers{
		whisperURL:   w.URL,
		tesseractURL: te.URL,
		cleanup:      func() { w.Close(); te.Close() },
	}
}

// TestRun_BothPass — happy path. Asserts exit 0 + both PASS lines
// + the "all required PASS" footer + Whisper detail string carries
// the model name (captured-evidence per Constitution §11.4).
func TestRun_BothPass(t *testing.T) {
	srv := newFakeServers(t, nil, nil)
	defer srv.cleanup()

	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{"--whisper-url", srv.whisperURL, "--tesseract-url", srv.tesseractURL},
		&stdout, &stderr)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", exit, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PASS  whisper") {
		t.Errorf("stdout missing 'PASS  whisper' line:\n%s", out)
	}
	if !strings.Contains(out, "PASS  tesseract") {
		t.Errorf("stdout missing 'PASS  tesseract' line:\n%s", out)
	}
	// Captured-evidence assertions — the detail strings MUST carry
	// the model name + version, not just "ok".
	if !strings.Contains(out, "default_model=base") {
		t.Errorf("Whisper PASS detail missing model name (bluff: would PASS without specific evidence):\n%s", out)
	}
	if !strings.Contains(out, "version=5.3.4") {
		t.Errorf("Tesseract PASS detail missing version (bluff: would PASS without specific evidence):\n%s", out)
	}
	if !strings.Contains(out, "all required (tesseract,whisper) PASS") {
		t.Errorf("missing 'all required PASS' footer:\n%s", out)
	}
}

// TestRun_WhisperFail_TesseractPass — required-fail produces exit 1
// AND surfaces the server-side error in the detail line so an
// operator can see WHY without re-running with debugging.
func TestRun_WhisperFail_TesseractPass(t *testing.T) {
	srv := newFakeServers(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "model not loaded yet")
		},
		nil,
	)
	defer srv.cleanup()

	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{"--whisper-url", srv.whisperURL, "--tesseract-url", srv.tesseractURL},
		&stdout, &stderr)

	if exit != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%s", exit, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FAIL  whisper") {
		t.Errorf("stdout missing 'FAIL  whisper' line:\n%s", out)
	}
	if !strings.Contains(out, "PASS  tesseract") {
		t.Errorf("stdout missing 'PASS  tesseract' line:\n%s", out)
	}
	// Anti-bluff: the failure body MUST surface so operator can debug.
	if !strings.Contains(out, "model not loaded yet") {
		t.Errorf("FAIL output missing server diagnostic (operator can't debug):\n%s", out)
	}
}

// TestRun_NotRequired_FailsAsSkip — when a service is not in
// --require, its FAIL becomes SKIP and does not affect exit code.
func TestRun_NotRequired_FailsAsSkip(t *testing.T) {
	srv := newFakeServers(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		nil,
	)
	defer srv.cleanup()

	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{
			"--whisper-url", srv.whisperURL,
			"--tesseract-url", srv.tesseractURL,
			"--require", "tesseract",
		},
		&stdout, &stderr)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (whisper not required, FAIL→SKIP); stdout=%s",
			exit, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "SKIP  whisper") {
		t.Errorf("stdout missing 'SKIP  whisper' (whisper not required → FAIL must downgrade to SKIP):\n%s", out)
	}
	if !strings.Contains(out, "PASS  tesseract") {
		t.Errorf("stdout missing 'PASS  tesseract':\n%s", out)
	}
}

// TestRun_JSONOutput — --json emits a single parseable document
// with both services' results. Asserts the structured fields.
func TestRun_JSONOutput(t *testing.T) {
	srv := newFakeServers(t, nil, nil)
	defer srv.cleanup()

	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{
			"--whisper-url", srv.whisperURL,
			"--tesseract-url", srv.tesseractURL,
			"--json",
		},
		&stdout, &stderr)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}

	var report ProbeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON: %v\nstdout=%s", err, stdout.String())
	}
	if !report.AllOK {
		t.Errorf("AllOK = false, want true")
	}
	if len(report.Services) != 2 {
		t.Fatalf("Services len = %d, want 2", len(report.Services))
	}
	for _, s := range report.Services {
		if s.State != "PASS" {
			t.Errorf("Service %s State = %q, want PASS", s.Service, s.State)
		}
	}
}

// TestRun_InvalidRequire — --require=bogus is a caller error,
// exit 2 (not 1, which is reserved for "service down").
func TestRun_InvalidRequire(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{"--require", "bogus"},
		&stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2 (invalid --require)", exit)
	}
	if !strings.Contains(stderr.String(), "unknown service") {
		t.Errorf("stderr missing 'unknown service':\n%s", stderr.String())
	}
}

// TestRun_EmptyRequire — empty --require means "no services
// required" which would silently always-pass — caller error.
func TestRun_EmptyRequire(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(),
		[]string{"--require", ""},
		&stdout, &stderr)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2 (empty --require)", exit)
	}
	if !strings.Contains(stderr.String(), "must list at least one service") {
		t.Errorf("stderr missing 'must list at least one' diagnostic:\n%s", stderr.String())
	}
}

// TestSplitCSV_Dedup — "whisper,whisper,tesseract" → 2 services
// (sorted, deduped). Locks the dedup behavior so a careless
// --require value doesn't double-count.
func TestSplitCSV_Dedup(t *testing.T) {
	got := splitCSV("whisper, whisper ,tesseract,whisper")
	want := []string{"tesseract", "whisper"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
