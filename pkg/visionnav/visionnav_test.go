// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Phase 26.5 — pkg/visionnav tests. Anti-bluff design (Constitution
// §11.4): every PASS asserts a SPECIFIC value (not "no error"); the
// validation tests confirm that bluff-shaped Evidence is REJECTED.

package visionnav

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/audio"
)

// --- Validate() tests ----------------------------------------------

func TestValidate_NilEvidence(t *testing.T) {
	var e *Evidence
	err := e.Validate()
	if err == nil || !strings.Contains(err.Error(), "nil Evidence") {
		t.Fatalf("nil Evidence: got %v, want error mentioning 'nil Evidence'", err)
	}
}

func TestValidate_EmptyDescription(t *testing.T) {
	e := &Evidence{Verdict: "pass", OCRSnapshot: "x"}
	err := e.Validate()
	if err == nil || !strings.Contains(err.Error(), "Description is empty") {
		t.Fatalf("empty Description: got %v", err)
	}
}

func TestValidate_EmptyVerdict(t *testing.T) {
	e := &Evidence{Description: "found something", OCRSnapshot: "x"}
	err := e.Validate()
	if err == nil || !strings.Contains(err.Error(), "Verdict is empty") {
		t.Fatalf("empty Verdict: got %v", err)
	}
}

func TestValidate_BogusVerdict(t *testing.T) {
	e := &Evidence{Description: "x", Verdict: "maybe", OCRSnapshot: "x"}
	err := e.Validate()
	if err == nil || !strings.Contains(err.Error(), "Verdict") || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("bogus Verdict: got %v", err)
	}
}

func TestValidate_NeitherTranscriptNorOCR_RejectedAsBluff(t *testing.T) {
	// This is the §11.4 enforcement at the data layer. Even an
	// explorer that ignores conventions can't ship pass/fail
	// without captured evidence.
	e := &Evidence{Description: "found something", Verdict: "pass"}
	err := e.Validate()
	if err == nil {
		t.Fatal("expected bluff-rejection: pass verdict with neither transcript nor OCR")
	}
	if !strings.Contains(err.Error(), "bluff verdict") {
		t.Errorf("error %q does not mention 'bluff verdict' — explanation matters", err.Error())
	}
}

func TestValidate_OCROnly_Accepted(t *testing.T) {
	e := &Evidence{Description: "x", Verdict: "fail", OCRSnapshot: "Settings"}
	if err := e.Validate(); err != nil {
		t.Fatalf("OCR-only Evidence rejected: %v", err)
	}
}

func TestValidate_TranscriptOnly_Accepted(t *testing.T) {
	e := &Evidence{
		Description: "x", Verdict: "needs-review",
		Transcript: &audio.TranscribeResult{Text: "hello"},
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("Transcript-only Evidence rejected: %v", err)
	}
}

// --- FileSink tests ------------------------------------------------

func TestFileSink_Record_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	sink, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	e := &Evidence{
		Description: "Settings dialog opened with title 'Network'",
		Verdict:     "pass",
		OCRSnapshot: "Network\nWiFi\nEthernet",
	}
	if err := sink.Record(context.Background(), e); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if sink.Count() != 1 {
		t.Fatalf("Count = %d, want 1", sink.Count())
	}

	// Find the written file.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in sink dir, got %d", len(entries))
	}
	name := entries[0].Name()
	// Filename pattern: <ts>_<verdict>_<short>.json
	if !strings.Contains(name, "_pass_") {
		t.Errorf("filename %q missing '_pass_' verdict marker", name)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Errorf("filename %q missing .json suffix", name)
	}

	// Captured-evidence: the JSON content must round-trip with the
	// SPECIFIC Description we set.
	body, _ := os.ReadFile(filepath.Join(dir, name))
	var rt Evidence
	if err := json.Unmarshal(body, &rt); err != nil {
		t.Fatalf("decode: %v\n%s", err, body)
	}
	if rt.Description != e.Description {
		t.Errorf("Description round-trip: got %q, want %q", rt.Description, e.Description)
	}
	if rt.OCRSnapshot != e.OCRSnapshot {
		t.Errorf("OCRSnapshot round-trip mismatch")
	}
}

func TestFileSink_Record_RejectsBluffEvidence(t *testing.T) {
	dir := t.TempDir()
	sink, err := NewFileSink(dir)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}
	// Bluff: pass verdict with neither transcript nor OCR.
	bluff := &Evidence{Description: "x", Verdict: "pass"}
	err = sink.Record(context.Background(), bluff)
	if err == nil {
		t.Fatal("FileSink accepted bluff Evidence — §11.4 enforcement broken")
	}
	if sink.Count() != 0 {
		t.Errorf("Count = %d after bluff rejection, want 0", sink.Count())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("FileSink wrote a file for bluff Evidence — should not")
	}
}

func TestFileSink_NewFileSink_EmptyDirRejected(t *testing.T) {
	_, err := NewFileSink("")
	if err == nil {
		t.Fatal("NewFileSink('') accepted empty dir — caller error not surfaced")
	}
}

func TestSafeFilenameSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello world", "hello-world"},
		{"foo/bar/../baz", "foo-bar-baz"},
		{"alphanumeric_123-test", "alphanumeric_123-test"},
		{"!!!", "-"},
		{"", "unnamed"},
	}
	for _, c := range cases {
		got := safeFilenameSegment(c.in)
		if got != c.want {
			t.Errorf("safeFilenameSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- DefaultExplorer tests -----------------------------------------

func TestNewDefaultExplorer_RejectsAllNilSources(t *testing.T) {
	dir := t.TempDir()
	sink, _ := NewFileSink(dir)
	_, err := NewDefaultExplorer("test", nil, nil, sink)
	if err == nil {
		t.Fatal("expected error: explorer with nil whisper AND nil tesseract is bluff-by-construction")
	}
	if !strings.Contains(err.Error(), "captured-evidence") {
		t.Errorf("error %q should mention captured-evidence rule", err.Error())
	}
}

func TestNewDefaultExplorer_RequiresName(t *testing.T) {
	dir := t.TempDir()
	sink, _ := NewFileSink(dir)
	_, err := NewDefaultExplorer("", nil, audio.NewTesseractClient(""), sink)
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected name-required error, got %v", err)
	}
}

func TestNewDefaultExplorer_RequiresSink(t *testing.T) {
	_, err := NewDefaultExplorer("test", nil, audio.NewTesseractClient(""), nil)
	if err == nil || !strings.Contains(err.Error(), "EvidenceSink") {
		t.Fatalf("expected sink-required error, got %v", err)
	}
}

func TestDefaultExplorer_CaptureFinding_OCRPath(t *testing.T) {
	// Synthetic Tesseract /ocr server that returns a known string.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Settings\nWiFi\nEthernet\n")
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "snap.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	sinkDir := t.TempDir()
	sink, _ := NewFileSink(sinkDir)
	tess := audio.NewTesseractClient(srv.URL)

	exp, err := NewDefaultExplorer("test-explorer", nil, tess, sink)
	if err != nil {
		t.Fatalf("NewDefaultExplorer: %v", err)
	}

	ev, err := exp.CaptureFinding(context.Background(), FindingOptions{
		Description: "Settings dialog discovered",
		Verdict:     "pass",
		ImagePath:   imgPath,
	})
	if err != nil {
		t.Fatalf("CaptureFinding: %v", err)
	}
	// Captured-evidence assertion — the OCR text must be the SPECIFIC
	// string the synthetic server emitted, not a stub or empty.
	if !strings.Contains(ev.OCRSnapshot, "Settings") {
		t.Errorf("OCRSnapshot %q missing 'Settings'", ev.OCRSnapshot)
	}
	if !strings.Contains(ev.OCRSnapshot, "Ethernet") {
		t.Errorf("OCRSnapshot %q missing 'Ethernet'", ev.OCRSnapshot)
	}
	// Verify the sink persisted it.
	if sink.Count() != 1 {
		t.Errorf("sink Count = %d, want 1", sink.Count())
	}
}

func TestDefaultExplorer_Name(t *testing.T) {
	dir := t.TempDir()
	sink, _ := NewFileSink(dir)
	exp, _ := NewDefaultExplorer("anthropic-claude-vision-nav-v1", nil, audio.NewTesseractClient(""), sink)
	if exp.Name() != "anthropic-claude-vision-nav-v1" {
		t.Errorf("Name = %q", exp.Name())
	}
}
