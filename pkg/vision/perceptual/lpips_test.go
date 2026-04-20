// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perceptual

import (
	"context"
	"encoding/json"
	"errors"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock Triton LPIPS sidecar
// ---------------------------------------------------------------------------

func newMockLPIPS(distance float64, outputName string) (*httptest.Server, *string) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		captured = string(body)
		resp := lpipsResponse{
			Outputs: []lpipsOutput{{Name: outputName, Datatype: "FP32", Shape: []int{1}, Data: []float64{distance}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &captured
}

// ---------------------------------------------------------------------------
// distance → similarity mapping
// ---------------------------------------------------------------------------

func TestDistanceToSimilarity_ZeroMapsToOne(t *testing.T) {
	if got := distanceToSimilarity(0, 1); got != 1 {
		t.Fatalf("d=0 → %v, want 1", got)
	}
}

func TestDistanceToSimilarity_HalfMapsToZero(t *testing.T) {
	if got := distanceToSimilarity(0.5, 1); math.Abs(got) > 1e-9 {
		t.Fatalf("d=0.5, max=1 → %v, want 0", got)
	}
}

func TestDistanceToSimilarity_MaxMapsToMinusOne(t *testing.T) {
	if got := distanceToSimilarity(1, 1); got != -1 {
		t.Fatalf("d=max → %v, want -1", got)
	}
}

func TestDistanceToSimilarity_BeyondMaxClamps(t *testing.T) {
	if got := distanceToSimilarity(5, 1); got != -1 {
		t.Fatalf("d=5, max=1 → %v, want -1 (clamped)", got)
	}
}

func TestDistanceToSimilarity_NegativeDistanceTreatedAsIdentical(t *testing.T) {
	// Shouldn't happen in practice but the guarded branch returns 1.
	if got := distanceToSimilarity(-0.5, 1); got != 1 {
		t.Fatalf("d=-0.5 → %v, want 1 (defensive)", got)
	}
}

// ---------------------------------------------------------------------------
// Compare — happy path via mock sidecar
// ---------------------------------------------------------------------------

func TestLPIPS_IdenticalImagesReturnOne(t *testing.T) {
	srv, _ := newMockLPIPS(0, "DISTANCE")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	s, err := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if s != 1 {
		t.Fatalf("similarity = %v, want 1", s)
	}
}

func TestLPIPS_MidDistanceReturnsZero(t *testing.T) {
	srv, _ := newMockLPIPS(0.5, "DISTANCE")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	s, _ := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{255, 255, 255, 255}))
	if math.Abs(s) > 1e-9 {
		t.Fatalf("similarity = %v, want 0", s)
	}
}

func TestLPIPS_MaxDistanceReturnsMinusOne(t *testing.T) {
	srv, _ := newMockLPIPS(1, "DISTANCE")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	s, _ := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{255, 255, 255, 255}))
	if s != -1 {
		t.Fatalf("similarity = %v, want -1", s)
	}
}

func TestLPIPS_CustomMaxDistance(t *testing.T) {
	srv, _ := newMockLPIPS(1.0, "DISTANCE")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	l.MaxDistance = 2.0 // distance=1, max=2 → similarity = 1 - 1 = 0
	s, _ := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{255, 255, 255, 255}))
	if math.Abs(s) > 1e-9 {
		t.Fatalf("similarity = %v, want 0 (custom MaxDistance=2)", s)
	}
}

func TestLPIPS_LowercaseDistanceOutput(t *testing.T) {
	srv, _ := newMockLPIPS(0.25, "distance")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	s, err := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// d=0.25, max=1 → similarity = 1 - 0.5 = 0.5.
	if math.Abs(s-0.5) > 1e-9 {
		t.Fatalf("similarity = %v, want 0.5", s)
	}
}

func TestLPIPS_PreInvertedSimilarityOutput(t *testing.T) {
	// Some sidecars pre-invert — emit SIMILARITY already in the
	// canonical [-1, 1] range. The client reverses back to distance
	// before re-applying its own maxDist map.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(lpipsResponse{
			Outputs: []lpipsOutput{{Name: "SIMILARITY", Data: []float64{0.5}}},
		})
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	s, err := l.Compare(context.Background(), tinyRGBA(color.RGBA{0, 0, 0, 255}), tinyRGBA(color.RGBA{0, 0, 0, 255}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// Sidecar said similarity=0.5 → distance = (1 - 0.5)/2 = 0.25.
	// Client re-applies maxDist=1: similarity = 1 - 2*0.25 = 0.5.
	if math.Abs(s-0.5) > 1e-9 {
		t.Fatalf("similarity = %v, want 0.5", s)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestLPIPS_NilImagesError(t *testing.T) {
	l := NewLPIPS("http://localhost")
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), nil, img); err != ErrNilImage {
		t.Fatalf("nil a = %v, want ErrNilImage", err)
	}
	if _, err := l.Compare(context.Background(), img, nil); err != ErrNilImage {
		t.Fatalf("nil b = %v, want ErrNilImage", err)
	}
}

func TestLPIPS_EmptyEndpointError(t *testing.T) {
	l := &LPIPS{}
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); !errors.Is(err, ErrLPIPSEndpoint) {
		t.Fatalf("empty endpoint = %v", err)
	}
}

func TestLPIPS_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	_, err := l.Compare(context.Background(), img, img)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestLPIPS_MalformedResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestLPIPS_NoOutputsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(lpipsResponse{Outputs: []lpipsOutput{}})
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); !errors.Is(err, ErrLPIPSResponse) {
		t.Fatalf("no outputs = %v, want ErrLPIPSResponse", err)
	}
}

func TestLPIPS_EmptyDistanceDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(lpipsResponse{
			Outputs: []lpipsOutput{{Name: "DISTANCE", Data: []float64{}}},
		})
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); !errors.Is(err, ErrLPIPSResponse) {
		t.Fatalf("empty DISTANCE = %v, want ErrLPIPSResponse", err)
	}
}

func TestLPIPS_EmptySimilarityDataError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(lpipsResponse{
			Outputs: []lpipsOutput{{Name: "SIMILARITY", Data: []float64{}}},
		})
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); !errors.Is(err, ErrLPIPSResponse) {
		t.Fatalf("empty SIMILARITY = %v, want ErrLPIPSResponse", err)
	}
}

func TestLPIPS_OutputNameFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(lpipsResponse{
			Outputs: []lpipsOutput{{Name: "custom_out", Data: []float64{0.5}}},
		})
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	s, err := l.Compare(context.Background(), img, img)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	// Fallback treats 0.5 as distance → similarity = 1 - 2*0.5 = 0.
	if math.Abs(s) > 1e-9 {
		t.Fatalf("similarity = %v, want 0", s)
	}
}

func TestLPIPS_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(ctx, img, img); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestLPIPS_InvalidEndpointURLError(t *testing.T) {
	l := &LPIPS{Endpoint: "ht!tp://bad\x00url"}
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := l.Compare(context.Background(), img, img); err == nil {
		t.Fatal("invalid URL should fail")
	}
}

func TestLPIPS_ZeroMaxDistanceDefaultsToOne(t *testing.T) {
	srv, _ := newMockLPIPS(0.5, "DISTANCE")
	defer srv.Close()
	l := NewLPIPS(srv.URL)
	// MaxDistance=0 should default to 1.
	l.MaxDistance = 0
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	s, _ := l.Compare(context.Background(), img, img)
	if math.Abs(s) > 1e-9 {
		t.Fatalf("similarity = %v, want 0 (default MaxDistance=1)", s)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestLPIPS_SatisfiesComparatorInterface(t *testing.T) {
	srv, _ := newMockLPIPS(0, "DISTANCE")
	defer srv.Close()
	var c Comparator = NewLPIPS(srv.URL)
	img := tinyRGBA(color.RGBA{0, 0, 0, 255})
	if _, err := c.Compare(context.Background(), img, img); err != nil {
		t.Fatalf("Compare via interface: %v", err)
	}
}
