// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package text

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixtures + mock sidecar
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

func mockSidecar(response wireResult) (*httptest.Server, *capturedRequest) {
	cap := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/detect" {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	return srv, cap
}

type capturedRequest struct {
	contentType string
	fields      []string
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestDetect_HappyPath(t *testing.T) {
	srv, _ := mockSidecar(wireResult{
		Width: 1920, Height: 1080,
		Regions: []wireRegion{
			{BBox: []int{100, 200, 300, 260}, Text: "Sign In", Confidence: 0.94},
			{BBox: []int{0, 0, 1920, 80}, Text: "", Confidence: 0.88},
		},
	})
	defer srv.Close()

	c := New(srv.URL)
	r, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.Width != 1920 || r.Height != 1080 {
		t.Errorf("dims = %dx%d", r.Width, r.Height)
	}
	if len(r.Regions) != 2 {
		t.Fatalf("regions = %d, want 2", len(r.Regions))
	}
	if r.Regions[0].Text != "Sign In" || r.Regions[0].Confidence != 0.94 {
		t.Errorf("region 0 = %+v", r.Regions[0])
	}
	if r.Regions[0].BBox != image.Rect(100, 200, 300, 260) {
		t.Errorf("region 0 bbox = %v", r.Regions[0].BBox)
	}
}

func TestDetect_MultipartPNGWireFormat(t *testing.T) {
	srv, cap := mockSidecar(wireResult{})
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !strings.HasPrefix(cap.contentType, "multipart/form-data") {
		t.Errorf("content-type = %q", cap.contentType)
	}
	if len(cap.fields) != 1 || cap.fields[0] != "image" {
		t.Errorf("fields = %v, want [image]", cap.fields)
	}
}

func TestDetect_CustomDetectPath(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = r.ParseMultipartForm(10 << 20)
		_ = json.NewEncoder(w).Encode(wireResult{})
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.DetectPath = "/v2/detect-text"
	_, _ = c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if receivedPath != "/v2/detect-text" {
		t.Fatalf("path = %q, want /v2/detect-text", receivedPath)
	}
}

// ---------------------------------------------------------------------------
// Region helpers
// ---------------------------------------------------------------------------

func TestRegion_Center(t *testing.T) {
	r := Region{BBox: image.Rect(100, 200, 300, 260)}
	if c := r.Center(); c != (image.Point{X: 200, Y: 230}) {
		t.Fatalf("Center = %v", c)
	}
}

func TestResult_FindContaining_HitsSmallestEnclosing(t *testing.T) {
	r := Result{
		Regions: []Region{
			{BBox: image.Rect(0, 0, 1000, 1000), Text: "outer"},
			{BBox: image.Rect(100, 100, 200, 200), Text: "inner"},
		},
	}
	got, ok := r.FindContaining(image.Point{X: 150, Y: 150})
	if !ok {
		t.Fatal("expected hit")
	}
	if got.Text != "inner" {
		t.Fatalf("found %q, want 'inner' (smallest enclosing)", got.Text)
	}
}

func TestResult_FindContaining_NoMatch(t *testing.T) {
	r := Result{
		Regions: []Region{
			{BBox: image.Rect(0, 0, 100, 100), Text: "box"},
		},
	}
	if _, ok := r.FindContaining(image.Point{X: 500, Y: 500}); ok {
		t.Fatal("expected no match")
	}
}

func TestResult_FindContaining_Empty(t *testing.T) {
	if _, ok := (Result{}).FindContaining(image.Point{X: 10, Y: 10}); ok {
		t.Fatal("empty result must not hit")
	}
}

func TestBboxArea_InvalidReturnsZero(t *testing.T) {
	inverted := image.Rectangle{Min: image.Point{X: 100, Y: 100}, Max: image.Point{X: 50, Y: 50}}
	if got := bboxArea(inverted); got != 0 {
		t.Fatalf("inverted = %d, want 0", got)
	}
	if got := bboxArea(image.Rect(0, 0, 10, 20)); got != 200 {
		t.Fatalf("normal = %d, want 200", got)
	}
}

func TestPointIn(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	if !pointIn(r, image.Point{X: 10, Y: 10}) {
		t.Error("Min inclusive")
	}
	if pointIn(r, image.Point{X: 20, Y: 20}) {
		t.Error("Max exclusive")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDetect_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	if _, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("empty endpoint: %v", err)
	}
}

func TestDetect_NilImageError(t *testing.T) {
	c := New("http://localhost")
	if _, err := c.Detect(context.Background(), nil); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil image: %v", err)
	}
}

func TestDetect_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestDetect_MalformedJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestDetect_InvalidBBoxError(t *testing.T) {
	srv, _ := mockSidecar(wireResult{
		Regions: []wireRegion{{BBox: []int{1, 2}, Text: "bad"}},
	})
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if !errors.Is(err, ErrInvalidBBox) {
		t.Fatalf("invalid bbox: %v, want ErrInvalidBBox", err)
	}
}

func TestDetect_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Detect(ctx, tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestDetect_InvalidURLError(t *testing.T) {
	c := &Client{Endpoint: "ht!tp://bad\x00url"}
	if _, err := c.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err == nil {
		t.Fatal("invalid URL should fail")
	}
}

// ---------------------------------------------------------------------------
// Constructor + interface conformance
// ---------------------------------------------------------------------------

func TestNew_SetsEndpoint(t *testing.T) {
	c := New("http://example.com")
	if c.Endpoint != "http://example.com" {
		t.Fatalf("Endpoint = %q", c.Endpoint)
	}
}

func TestClient_SatisfiesDetectorInterface(t *testing.T) {
	srv, _ := mockSidecar(wireResult{})
	defer srv.Close()
	var d Detector = New(srv.URL)
	if _, err := d.Detect(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255})); err != nil {
		t.Fatalf("Detect via interface: %v", err)
	}
}
