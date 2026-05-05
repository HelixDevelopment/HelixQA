// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Tesseract HTTP client tests — same anti-bluff design as
// whisper_client_test.go: every PASS asserts a specific value
// from the response, not "no error".

package audio

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
)

func TestTesseractHealth_ParsesContainerSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{
			"status": "ok",
			"tesseract_version": "5.3.4",
			"langs": ["eng", "rus", "osd"],
			"default_lang": "eng+rus",
			"default_psm": 6
		}`)
	}))
	defer srv.Close()

	c := NewTesseractClient(srv.URL)
	got, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q", got.Status)
	}
	if got.TesseractVersion != "5.3.4" {
		t.Errorf("TesseractVersion = %q", got.TesseractVersion)
	}
	if got.DefaultLang != "eng+rus" {
		t.Errorf("DefaultLang = %q", got.DefaultLang)
	}
	if got.DefaultPSM != 6 {
		t.Errorf("DefaultPSM = %d, want 6", got.DefaultPSM)
	}
	if len(got.Langs) != 3 {
		t.Errorf("Langs len = %d, want 3 (eng, rus, osd)", len(got.Langs))
	}
}

func TestTesseractOCR_RawTextResponse(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "frame.png")
	imgBytes := []byte("\x89PNG\r\n\x1a\nfake-png-payload-for-test")
	if err := os.WriteFile(imgPath, imgBytes, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var (
		gotMethod   string
		gotPath     string
		gotQuery    string
		gotPartName string
		gotFilename string
		gotBody     []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			t.Errorf("server: %v", err)
			return
		}
		fhs := r.MultipartForm.File["image"]
		if len(fhs) != 1 {
			t.Errorf("server: expected 1 'image' part, got %d", len(fhs))
			return
		}
		gotPartName = "image"
		gotFilename = fhs[0].Filename
		f, err := fhs[0].Open()
		if err != nil {
			t.Errorf("server: open: %v", err)
			return
		}
		defer f.Close()
		gotBody, _ = io.ReadAll(f)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Anti-bluff: Tesseract output preserves trailing newline +
		// internal whitespace. Client must NOT trim it (a reviewer
		// needs the exact engine output).
		_, _ = io.WriteString(w, "Hello, world!\nLine two\n")
	}))
	defer srv.Close()

	c := NewTesseractClient(srv.URL)
	text, err := c.OCR(context.Background(), imgPath, OCROptions{
		Lang:   "eng+rus",
		PSM:    6,
		SetPSM: true,
	})
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/ocr" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "lang=eng%2Brus") && !strings.Contains(gotQuery, "lang=eng+rus") {
		t.Errorf("query missing lang=eng+rus (got %q)", gotQuery)
	}
	if !strings.Contains(gotQuery, "psm=6") {
		t.Errorf("query missing psm=6 (got %q)", gotQuery)
	}
	if gotPartName != "image" {
		t.Errorf("part name = %q", gotPartName)
	}
	if gotFilename != "frame.png" {
		t.Errorf("filename = %q", gotFilename)
	}
	if string(gotBody) != string(imgBytes) {
		t.Errorf("body bytes mismatch")
	}
	// Anti-bluff: full raw text including trailing newline.
	want := "Hello, world!\nLine two\n"
	if text != want {
		t.Errorf("OCR text = %q, want %q (post-processing leaked into client?)", text, want)
	}
}

func TestTesseractOCR_PSMZero_HonoredOnlyWhenSetPSM(t *testing.T) {
	// When SetPSM=false, the client must NOT emit `psm=0` (which
	// would override the server's TESSERACT_PSM env default with
	// the orientation-detection PSM, breaking text extraction).
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "x.png")
	if err := os.WriteFile(imgPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	c := NewTesseractClient(srv.URL)
	if _, err := c.OCR(context.Background(), imgPath, OCROptions{PSM: 0, SetPSM: false}); err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if strings.Contains(gotQuery, "psm=") {
		t.Errorf("PSM unset but query has psm=: %q", gotQuery)
	}

	// Now SetPSM=true with PSM=0 — client MUST emit psm=0 explicitly.
	if _, err := c.OCR(context.Background(), imgPath, OCROptions{PSM: 0, SetPSM: true}); err != nil {
		t.Fatalf("OCR (SetPSM=true): %v", err)
	}
	if !strings.Contains(gotQuery, "psm=0") {
		t.Errorf("SetPSM=true PSM=0 should send psm=0; query = %q", gotQuery)
	}
}

func TestTesseractOCR_NonOK_SurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"TesseractError: language data 'klingon' not found"}`)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "x.png")
	if err := os.WriteFile(imgPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	c := NewTesseractClient(srv.URL)
	_, err := c.OCR(context.Background(), imgPath, OCROptions{Lang: "klingon"})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q missing status code", err.Error())
	}
	if !strings.Contains(err.Error(), "klingon") {
		t.Errorf("error %q missing server diagnostic", err.Error())
	}
}

func TestTesseractOCRVideo_ParsesFramesSchema(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "clip.mp4")
	if err := os.WriteFile(videoPath, []byte("\x00\x00\x00 ftypmp42fake-mp4"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ocr-video" {
			t.Errorf("path = %q", r.URL.Path)
		}
		// fps + lang + psm assertions are exercised by the more
		// detailed OCR test above; here we focus on response
		// parsing of the per-frame map.
		_ = json.NewEncoder(w).Encode(OCRVideoResult{
			FPS:  2.0,
			Lang: "eng",
			PSM:  6,
			Frames: map[string]string{
				"0001": "Frame one text",
				"0002": "Frame two text",
				"0003": "Frame three text",
			},
			FrameCount: 3,
		})
	}))
	defer srv.Close()

	c := NewTesseractClient(srv.URL)
	got, err := c.OCRVideo(context.Background(), videoPath, OCROptions{
		Lang: "eng", PSM: 6, SetPSM: true, FPS: 2.0,
	})
	if err != nil {
		t.Fatalf("OCRVideo: %v", err)
	}
	if got.FPS != 2.0 {
		t.Errorf("FPS = %v, want 2.0", got.FPS)
	}
	if got.FrameCount != 3 {
		t.Errorf("FrameCount = %d, want 3", got.FrameCount)
	}
	if got.Frames["0002"] != "Frame two text" {
		t.Errorf("Frames[0002] = %q, want \"Frame two text\"", got.Frames["0002"])
	}
	// Anti-bluff: the keys MUST be zero-padded 4-digit strings
	// per server.py contract — a regression to e.g. "1" / "2" / "3"
	// would silently break callers iterating in order.
	for k := range got.Frames {
		if len(k) != 4 {
			t.Errorf("frame key %q is not 4-char zero-padded", k)
		}
	}
}

func TestTesseractOCRVideo_OmitsFPSWhenZero(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(OCRVideoResult{Frames: map[string]string{}})
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "x.mp4")
	if err := os.WriteFile(videoPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	c := NewTesseractClient(srv.URL)
	if _, err := c.OCRVideo(context.Background(), videoPath, OCROptions{Lang: "eng"}); err != nil {
		t.Fatalf("OCRVideo: %v", err)
	}
	if strings.Contains(gotQuery, "fps=") {
		t.Errorf("FPS=0 should omit query; got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "lang=eng") {
		t.Errorf("Lang should be present; got %q", gotQuery)
	}
}
