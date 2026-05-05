// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Whisper client tests use httptest to stand in for the qa-whisper
// container. This avoids depending on a running container in CI
// while still asserting wire-level behaviour:
//
//   - request shape (method, path, query string, multipart body)
//   - response parsing (full TranscribeResult with nested Words)
//   - error propagation (non-200 surfaces body in error message)
//
// Constitution §11.4 captured-evidence rule: each test asserts a
// SPECIFIC value extracted from the response, not "no error". A
// PASS that only checks err == nil is a bluff (we wouldn't notice
// if the server returned an empty body or wrong language).

package audio

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNewWhisperClient_DefaultBaseURL confirms the loopback URL
// from the changelog (1.1.5-dev-0.0.3) is honored when caller
// passes empty string.

// TestHealth_ParsesContainerSchema verifies the client decodes the
// real /health JSON shape from server.py lines 88-99.
func TestHealth_ParsesContainerSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{
			"status": "ok",
			"backend": "faster-whisper",
			"default_model": "base",
			"loaded_models": ["base", "medium"],
			"compute_type": "int8",
			"models_dir": "/models"
		}`)
	}))
	defer srv.Close()

	c := NewWhisperClient(srv.URL)
	got, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q, want \"ok\"", got.Status)
	}
	if got.Backend != "faster-whisper" {
		t.Errorf("Backend = %q, want \"faster-whisper\"", got.Backend)
	}
	if got.DefaultModel != "base" {
		t.Errorf("DefaultModel = %q, want \"base\"", got.DefaultModel)
	}
	if len(got.LoadedModels) != 2 || got.LoadedModels[1] != "medium" {
		t.Errorf("LoadedModels = %v, want [base medium]", got.LoadedModels)
	}
	if got.ComputeType != "int8" {
		t.Errorf("ComputeType = %q, want \"int8\"", got.ComputeType)
	}
}

// TestHealth_NonOK_SurfacesBody validates that the error message
// includes server body so an operator can see WHY /health failed.
// Anti-bluff: a generic "non-200" error would be a usability bluff
// (failure surfaced but unactionable).
func TestHealth_NonOK_SurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "model not loaded yet")
	}))
	defer srv.Close()

	c := NewWhisperClient(srv.URL)
	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatalf("expected error for 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error %q does not contain status code", err.Error())
	}
	if !strings.Contains(err.Error(), "model not loaded yet") {
		t.Errorf("error %q does not contain server body — operator can't debug", err.Error())
	}
}

// TestTranscribe_RequestShape_AndResponseParsing exercises the
// full happy path: server receives correct multipart + query, client
// parses the full structured TranscribeResult including nested Words.
func TestTranscribe_RequestShape_AndResponseParsing(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "sample.wav")
	wantBytes := []byte("RIFF$\x00\x00\x00WAVEfmt fake-audio-payload")
	if err := os.WriteFile(audioPath, wantBytes, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var (
		gotMethod    string
		gotPath      string
		gotQuery     string
		gotFilename  string
		gotPartName  string
		gotBodyBytes []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery

		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Errorf("server: ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		// The client always uses "video" — assert by name.
		fhs := r.MultipartForm.File["video"]
		if len(fhs) != 1 {
			t.Errorf("server: expected 1 'video' file part, got %d", len(fhs))
			return
		}
		gotPartName = "video"
		gotFilename = fhs[0].Filename
		f, err := fhs[0].Open()
		if err != nil {
			t.Errorf("server: open uploaded part: %v", err)
			return
		}
		defer f.Close()
		gotBodyBytes, _ = io.ReadAll(f)

		// Reply with a full TranscribeResult so the client can prove
		// it parses the nested Segment + Word arrays.
		_ = json.NewEncoder(w).Encode(TranscribeResult{
			Model:               "medium",
			Backend:             "faster-whisper",
			Language:            "ru",
			LanguageProbability: 0.97,
			Duration:            12.34,
			Text:                "привет мир",
			Segments: []Segment{
				{
					ID:               0,
					Start:            0.0,
					End:              2.5,
					Text:             " привет мир",
					AvgLogProb:       -0.21,
					CompressionRatio: 1.05,
					NoSpeechProb:     0.01,
					Words: []Word{
						{Start: 0.0, End: 0.6, Word: " привет", Probability: 0.92},
						{Start: 0.6, End: 1.4, Word: " мир", Probability: 0.88},
					},
				},
			},
			SRT:            "1\n00:00:00,000 --> 00:00:02,500\nпривет мир\n",
			InputFilename:  "sample.wav",
			InputSizeBytes: int64(len(wantBytes)),
		})
	}))
	defer srv.Close()

	// 30s test budget — plenty for the in-process httptest round-trip.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := NewWhisperClient(srv.URL)
	got, err := c.Transcribe(ctx, audioPath, TranscribeOptions{
		Model:    "medium",
		Language: "ru",
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	// Request shape assertions.
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/transcribe" {
		t.Errorf("path = %q, want /transcribe", gotPath)
	}
	if !strings.Contains(gotQuery, "model=medium") || !strings.Contains(gotQuery, "language=ru") {
		t.Errorf("query = %q missing model+language", gotQuery)
	}
	if gotPartName != "video" {
		t.Errorf("part name = %q, want \"video\"", gotPartName)
	}
	if gotFilename != "sample.wav" {
		t.Errorf("filename = %q, want sample.wav", gotFilename)
	}
	if string(gotBodyBytes) != string(wantBytes) {
		t.Errorf("body bytes mismatch — got %d bytes %q, want %d bytes %q",
			len(gotBodyBytes), string(gotBodyBytes), len(wantBytes), string(wantBytes))
	}

	// Response parsing assertions — captured-evidence values.
	if got.Language != "ru" {
		t.Errorf("Language = %q, want \"ru\"", got.Language)
	}
	if got.Text != "привет мир" {
		t.Errorf("Text = %q, want \"привет мир\"", got.Text)
	}
	if len(got.Segments) != 1 {
		t.Fatalf("Segments len = %d, want 1", len(got.Segments))
	}
	seg := got.Segments[0]
	if len(seg.Words) != 2 {
		t.Fatalf("seg.Words len = %d, want 2", len(seg.Words))
	}
	if seg.Words[0].Word != " привет" {
		t.Errorf("Words[0] = %q, want \" привет\"", seg.Words[0].Word)
	}
	if seg.Words[0].Probability < 0.9 {
		t.Errorf("Words[0].Probability = %v, want ≥0.9", seg.Words[0].Probability)
	}
	if !strings.HasPrefix(got.SRT, "1\n00:00:00,000") {
		t.Errorf("SRT does not start with frame 1 timestamp: %q", got.SRT)
	}
}

// TestTranscribe_NonOK_SurfacesBody — same anti-bluff treatment as
// Health: server returns 422 with diagnostic body, client must
// surface it in the error.
func TestTranscribe_NonOK_SurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error":"missing 'video' or 'audio' multipart part"}`)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "empty.wav")
	if err := os.WriteFile(audioPath, []byte{}, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	c := NewWhisperClient(srv.URL)
	_, err := c.Transcribe(context.Background(), audioPath, TranscribeOptions{})
	if err == nil {
		t.Fatalf("expected error for 422, got nil")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error %q missing status code", err.Error())
	}
	if !strings.Contains(err.Error(), "multipart part") {
		t.Errorf("error %q missing server diagnostic", err.Error())
	}
}

// TestTranscribe_OmitsQueryWhenOptsZero — when caller passes a
// zero-value TranscribeOptions, the URL must NOT carry "?model=&language=".
// Empty params would override the server's WHISPER_MODEL env default
// with empty-string and crash _get_model("").
func TestTranscribe_OmitsQueryWhenOptsZero(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(TranscribeResult{Text: "ok"})
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "x.wav")
	if err := os.WriteFile(audioPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	c := NewWhisperClient(srv.URL)
	if _, err := c.Transcribe(context.Background(), audioPath, TranscribeOptions{}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty when opts zero", gotQuery)
	}
}

// TestTranscribe_FileNotFound — surface OS-level error before
// constructing the multipart body.
func TestTranscribe_FileNotFound(t *testing.T) {
	c := NewWhisperClient("http://no-server")
	_, err := c.Transcribe(context.Background(), "/nonexistent/file.wav", TranscribeOptions{})
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error %q does not mention 'open'", err.Error())
	}
}

// silenceMultipartImports — prevents `mime/multipart` from being
// flagged unused if assertions above are ever simplified. The
// import is used implicitly by ParseMultipartForm in the test
// server handler.
var _ = multipart.ErrMessageTooLarge
