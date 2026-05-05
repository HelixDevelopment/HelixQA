// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package omniparser

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixture helpers + mock OmniParser server
// ---------------------------------------------------------------------------

func tinyRGBA(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// mockOmniParser builds an httptest server that accepts
// multipart/form-data uploads on /parse and returns the given scripted
// wire payload. Captures the last form field names + content type.
func mockOmniParser(wire wireResult) (*httptest.Server, *serverCapture) {
	cap := &serverCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		cap.contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for name := range r.MultipartForm.File {
			cap.fields = append(cap.fields, name)
		}
		if f, _, err := r.FormFile("image"); err == nil {
			b, _ := io.ReadAll(f)
			cap.imageSize = len(b)
			f.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wire)
	}))
	return srv, cap
}

type serverCapture struct {
	contentType string
	fields      []string
	imageSize   int
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestParse_HappyPath(t *testing.T) {
	srv, _ := mockOmniParser(wireResult{
		Width: 1920, Height: 1080,
		Elements: []wireElement{
			{BBox: []int{100, 200, 300, 260}, Type: "button", Text: "Sign in", Interactive: true, Confidence: 0.94},
			{BBox: []int{0, 0, 1920, 80}, Type: "toolbar", Text: "", Interactive: false, Confidence: 0.88},
		},
	})
	defer srv.Close()

	c := New(srv.URL)
	r, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.Width != 1920 || r.Height != 1080 {
		t.Errorf("dimensions = %dx%d, want 1920x1080", r.Width, r.Height)
	}
	if len(r.Elements) != 2 {
		t.Fatalf("element count = %d, want 2", len(r.Elements))
	}
	btn := r.Elements[0]
	if btn.BBox != image.Rect(100, 200, 300, 260) {
		t.Errorf("button bbox = %v", btn.BBox)
	}
	if btn.Type != "button" || btn.Text != "Sign in" || !btn.Interactive || btn.Confidence != 0.94 {
		t.Errorf("button fields wrong: %+v", btn)
	}
}

func TestParse_RequestShapeIsMultipartPNG(t *testing.T) {
	srv, cap := mockOmniParser(wireResult{Width: 100, Height: 100})
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(cap.contentType, "multipart/form-data") {
		t.Errorf("content-type = %q, want multipart/form-data", cap.contentType)
	}
	if len(cap.fields) != 1 || cap.fields[0] != "image" {
		t.Errorf("fields = %v, want [image]", cap.fields)
	}
	if cap.imageSize < 50 {
		t.Errorf("image payload too small (%d bytes) — PNG encoding broken?", cap.imageSize)
	}
}

func TestParse_CustomParsePath(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = r.ParseMultipartForm(10 << 20)
		_ = json.NewEncoder(w).Encode(wireResult{})
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.ParsePath = "/v2/detect"
	_, _ = c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if receivedPath != "/v2/detect" {
		t.Fatalf("path = %q, want /v2/detect", receivedPath)
	}
}

// ---------------------------------------------------------------------------
// Element + Result helpers
// ---------------------------------------------------------------------------

func TestElement_Center(t *testing.T) {
	e := Element{BBox: image.Rect(100, 200, 300, 260)}
	if got := e.Center(); got != (image.Point{X: 200, Y: 230}) {
		t.Fatalf("Center = %v", got)
	}
}

func TestResult_FindInteractiveContaining_HitsSmallestEnclosing(t *testing.T) {
	// Two interactive elements — one nested inside the other. The
	// query should return the inner (smaller) one.
	r := Result{
		Elements: []Element{
			{BBox: image.Rect(0, 0, 1000, 1000), Type: "group", Interactive: true},
			{BBox: image.Rect(100, 100, 200, 200), Type: "button", Interactive: true},
			{BBox: image.Rect(500, 500, 600, 600), Type: "button", Interactive: true},
		},
	}
	got, ok := r.FindInteractiveContaining(image.Point{X: 150, Y: 150})
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.BBox != image.Rect(100, 100, 200, 200) {
		t.Fatalf("hit = %+v, want the inner 100,100-200,200 button", got)
	}
}

func TestResult_FindInteractiveContaining_NoMatch(t *testing.T) {
	r := Result{
		Elements: []Element{
			{BBox: image.Rect(0, 0, 100, 100), Interactive: true},
		},
	}
	if _, ok := r.FindInteractiveContaining(image.Point{X: 500, Y: 500}); ok {
		t.Fatal("expected no match for point outside every bbox")
	}
}

func TestResult_FindInteractiveContaining_IgnoresNonInteractive(t *testing.T) {
	r := Result{
		Elements: []Element{
			{BBox: image.Rect(0, 0, 1000, 1000), Type: "toolbar", Interactive: false},
		},
	}
	if _, ok := r.FindInteractiveContaining(image.Point{X: 500, Y: 500}); ok {
		t.Fatal("non-interactive elements must not produce a hit")
	}
}

func TestResult_FindInteractiveContaining_EmptyResult(t *testing.T) {
	if _, ok := (Result{}).FindInteractiveContaining(image.Point{X: 10, Y: 10}); ok {
		t.Fatal("empty result must not hit")
	}
}

func TestBboxArea_InvalidReturnsZero(t *testing.T) {
	// Zero-sized / inverted rectangles should return 0 — used as a
	// safety net for malformed server responses. image.Rect()
	// canonicalizes swapped corners, so the inverted case is built
	// via a direct struct literal.
	cases := []image.Rectangle{
		{Min: image.Point{X: 100, Y: 100}, Max: image.Point{X: 50, Y: 50}}, // inverted (Max < Min)
		image.Rect(10, 10, 10, 20), // zero width
		image.Rect(10, 10, 20, 10), // zero height
	}
	for i, rc := range cases {
		if got := bboxArea(rc); got != 0 {
			t.Errorf("case %d (%v): area = %d, want 0", i, rc, got)
		}
	}
	if got := bboxArea(image.Rect(0, 0, 10, 20)); got != 200 {
		t.Fatalf("normal bbox area = %d, want 200", got)
	}
}

func TestPointIn(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	// Min is inclusive, Max is exclusive per standard image.Rectangle.
	if !pointIn(r, image.Point{X: 10, Y: 10}) {
		t.Error("Min should be inclusive")
	}
	if pointIn(r, image.Point{X: 20, Y: 20}) {
		t.Error("Max should be exclusive")
	}
	if !pointIn(r, image.Point{X: 15, Y: 15}) {
		t.Error("interior point should be in")
	}
	if pointIn(r, image.Point{X: 5, Y: 15}) {
		t.Error("point left of rect should be out")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestParse_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	if _, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("empty endpoint: %v, want ErrEmptyEndpoint", err)
	}
}

func TestParse_NilScreenshotError(t *testing.T) {
	c := New("http://localhost")
	if _, err := c.Parse(context.Background(), nil); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil image: %v, want ErrNilImage", err)
	}
}

func TestParse_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestParse_MalformedJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestParse_InvalidBBoxShapeError(t *testing.T) {
	srv, _ := mockOmniParser(wireResult{
		Width: 100, Height: 100,
		Elements: []wireElement{
			{BBox: []int{10, 20, 30}, Type: "button"}, // only 3 coords
		},
	})
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if !errors.Is(err, ErrInvalidBBox) {
		t.Fatalf("invalid bbox: %v, want ErrInvalidBBox", err)
	}
}

func TestParse_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Parse(ctx, tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestParse_InvalidEndpointURLError(t *testing.T) {
	c := &Client{Endpoint: "ht!tp://bad\x00url"}
	if _, err := c.Parse(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("invalid URL should fail")
	}
}

// ---------------------------------------------------------------------------
// buildMultipartPNG unit test
// ---------------------------------------------------------------------------

func TestBuildMultipartPNG_WellFormed(t *testing.T) {
	img := tinyRGBA(color.RGBA{100, 200, 50, 255})
	body, ct, err := buildMultipartPNG(img)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
		t.Fatalf("content type = %q", ct)
	}

	// Parse it back with mime/multipart to ensure the form is valid.
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	boundary := ct[len("multipart/form-data; boundary="):]
	r := multipart.NewReader(strings.NewReader(string(raw)), boundary)
	part, err := r.NextPart()
	if err != nil {
		t.Fatalf("NextPart: %v", err)
	}
	if part.FormName() != "image" {
		t.Errorf("form name = %q, want image", part.FormName())
	}
	data, _ := io.ReadAll(part)
	// PNG magic is 0x89 'P' 'N' 'G'.
	if len(data) < 8 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Errorf("first 4 bytes are not PNG magic: %v", data[:4])
	}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

