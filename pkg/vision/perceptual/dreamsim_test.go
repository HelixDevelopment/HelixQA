// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perceptual

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixture helpers
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

// newMockTriton builds an httptest server that mimics the Triton KServe v2
// /infer endpoint — returns the similarity value handed in, and records
// the last request body for assertion.
func newMockTriton(similarity float64) (*httptest.Server, *string) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = string(body)
		resp := tritonResponse{
			Outputs: []tritonOutput{
				{Name: "SIMILARITY", Datatype: "FP32", Shape: []int{1}, Data: []float64{similarity}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &captured
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestDreamSim_IdenticalImagesMapToOne(t *testing.T) {
	srv, _ := newMockTriton(1.0)
	defer srv.Close()

	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{100, 100, 100, 255})
	sim, err := d.Compare(context.Background(), img, img)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// Raw 1.0 → canonical 2*1 - 1 = 1.0.
	if sim != 1.0 {
		t.Fatalf("raw=1.0 → canonical=%v, want 1.0", sim)
	}
}

func TestDreamSim_HalfSimilarityMapsToZero(t *testing.T) {
	srv, _ := newMockTriton(0.5)
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{50, 200, 50, 255})
	sim, _ := d.Compare(context.Background(), img, img)
	if sim != 0.0 {
		t.Fatalf("raw=0.5 → canonical=%v, want 0.0", sim)
	}
}

func TestDreamSim_DisagreementMapsToMinusOne(t *testing.T) {
	srv, _ := newMockTriton(0.0)
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	sim, _ := d.Compare(context.Background(), img, img)
	if sim != -1.0 {
		t.Fatalf("raw=0.0 → canonical=%v, want -1.0", sim)
	}
}

func TestDreamSim_RequestShapeIsKServeV2(t *testing.T) {
	srv, captured := newMockTriton(0.87)
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	a := tinyRGBA(color.RGBA{200, 100, 50, 255})
	b := tinyRGBA(color.RGBA{50, 200, 100, 255})
	if _, err := d.Compare(context.Background(), a, b); err != nil {
		t.Fatalf("Compare: %v", err)
	}

	var req tritonRequest
	if err := json.Unmarshal([]byte(*captured), &req); err != nil {
		t.Fatalf("captured body isn't valid KServe request: %v\n%s", err, *captured)
	}
	if len(req.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(req.Inputs))
	}
	names := []string{req.Inputs[0].Name, req.Inputs[1].Name}
	if names[0] != "IMAGE_A" || names[1] != "IMAGE_B" {
		t.Fatalf("input names = %v, want [IMAGE_A, IMAGE_B]", names)
	}
	for i, in := range req.Inputs {
		if in.Datatype != "BYTES" {
			t.Errorf("input %d datatype = %q, want BYTES", i, in.Datatype)
		}
		if len(in.Shape) != 1 || in.Shape[0] != 1 {
			t.Errorf("input %d shape = %v, want [1]", i, in.Shape)
		}
		if len(in.Data) != 1 || !strings.HasPrefix(in.Data[0], "iVBOR") {
			t.Errorf("input %d data is not PNG base64 (should start with 'iVBOR' for PNG)", i)
		}
	}
}

func TestDreamSim_URLIncludesModelName(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(tritonResponse{
			Outputs: []tritonOutput{{Name: "SIMILARITY", Data: []float64{0.5}}},
		})
	}))
	defer srv.Close()

	d := NewDreamSim(srv.URL)
	d.Model = "custom-dreamsim-v2"
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	_, _ = d.Compare(context.Background(), img, img)
	want := "/v2/models/custom-dreamsim-v2/infer"
	if receivedPath != want {
		t.Fatalf("path = %q, want %q", receivedPath, want)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDreamSim_NilImagesError(t *testing.T) {
	d := NewDreamSim("http://localhost:8000")
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(context.Background(), nil, img); err != ErrNilImage {
		t.Fatalf("nil a: %v, want ErrNilImage", err)
	}
	if _, err := d.Compare(context.Background(), img, nil); err != ErrNilImage {
		t.Fatalf("nil b: %v, want ErrNilImage", err)
	}
}

func TestDreamSim_EmptyEndpointError(t *testing.T) {
	d := &DreamSim{}
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(context.Background(), img, img); !errors.Is(err, ErrDreamSimEndpoint) {
		t.Fatalf("empty endpoint: %v, want ErrDreamSimEndpoint", err)
	}
}

func TestDreamSim_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	_, err := d.Compare(context.Background(), img, img)
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("HTTP 500 not propagated: %v", err)
	}
}

func TestDreamSim_MalformedResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(context.Background(), img, img); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestDreamSim_NoOutputsReturnsErrDreamSimResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tritonResponse{Outputs: []tritonOutput{}})
	}))
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	_, err := d.Compare(context.Background(), img, img)
	if !errors.Is(err, ErrDreamSimResponse) {
		t.Fatalf("no outputs: %v, want ErrDreamSimResponse", err)
	}
}

func TestDreamSim_EmptySimilarityDataReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tritonResponse{
			Outputs: []tritonOutput{{Name: "SIMILARITY", Data: []float64{}}},
		})
	}))
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(context.Background(), img, img); !errors.Is(err, ErrDreamSimResponse) {
		t.Fatalf("empty data: %v, want ErrDreamSimResponse", err)
	}
}

func TestDreamSim_ContextCanceled(t *testing.T) {
	// Use a server that blocks — gives the ctx cancellation time to fire.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	d := NewDreamSim(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(ctx, img, img); err == nil {
		t.Fatal("canceled ctx must fail")
	}
}

// ---------------------------------------------------------------------------
// Fallback output-name matching
// ---------------------------------------------------------------------------

func TestDreamSim_LowercaseSimilarityOutputName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tritonResponse{
			Outputs: []tritonOutput{{Name: "similarity", Data: []float64{0.75}}},
		})
	}))
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	sim, err := d.Compare(context.Background(), img, img)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// 2*0.75 - 1 = 0.5
	if sim != 0.5 {
		t.Fatalf("sim = %v, want 0.5", sim)
	}
}

func TestDreamSim_OutputNameFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tritonResponse{
			Outputs: []tritonOutput{{Name: "weird_name", Data: []float64{0.25}}},
		})
	}))
	defer srv.Close()
	d := NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	sim, err := d.Compare(context.Background(), img, img)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// Fallback: first non-empty output used.
	if sim != 2*0.25-1 {
		t.Fatalf("sim = %v, want %v", sim, 2*0.25-1)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestDreamSim_SatisfiesComparatorInterface(t *testing.T) {
	srv, _ := newMockTriton(1.0)
	defer srv.Close()
	var c Comparator = NewDreamSim(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := c.Compare(context.Background(), img, img); err != nil {
		t.Fatalf("Compare via interface: %v", err)
	}
}

// ---------------------------------------------------------------------------
// encodePNGBase64 sanity
// ---------------------------------------------------------------------------

func TestEncodePNGBase64_NotEmpty(t *testing.T) {
	img := tinyRGBA(color.RGBA{100, 150, 200, 255})
	s, err := encodePNGBase64(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) < 20 {
		t.Fatalf("base64 PNG too short: %q", s)
	}
	if !strings.HasPrefix(s, "iVBOR") {
		t.Fatalf("base64 PNG should start with 'iVBOR' (PNG magic), got %q", s[:10])
	}
}

// TestDreamSim_DefaultsAppliedWhenFieldsZeroed exercises the defaulting
// branches inside Compare — Model="" → "dreamsim", HTTPClient=nil →
// 200 ms client. Uses a struct literal (not NewDreamSim) to start from
// a zero-value DreamSim.
func TestDreamSim_DefaultsAppliedWhenFieldsZeroed(t *testing.T) {
	srv, _ := newMockTriton(1.0)
	defer srv.Close()
	d := &DreamSim{Endpoint: srv.URL} // Model + HTTPClient left zero
	img := tinyRGBA(color.RGBA{100, 100, 100, 255})
	sim, err := d.Compare(context.Background(), img, img)
	if err != nil {
		t.Fatalf("Compare with zeroed defaults: %v", err)
	}
	if sim != 1.0 {
		t.Fatalf("sim = %v, want 1.0", sim)
	}
}

// TestDreamSim_InvalidEndpointURLError exercises the http.NewRequest error
// path — an endpoint containing invalid URL characters fails before any
// network I/O.
func TestDreamSim_InvalidEndpointURLError(t *testing.T) {
	d := &DreamSim{Endpoint: "ht!tp://invalid\x00url", Model: "m"}
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := d.Compare(context.Background(), img, img); err == nil {
		t.Fatal("invalid URL should fail")
	}
}
