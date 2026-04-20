// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// gradient produces a deterministic image with sharp step edges
// — required for Lucas-Kanade to have trackable features. A smooth
// gradient alone has no local corners; LK needs both spatial
// derivatives non-zero in the window (a 2-D structure tensor).
// Stripes every 8 pixels provide horizontal AND vertical edges.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Checker-like pattern with 8-pixel cells: provides both
			// horizontal + vertical edges that LK can latch onto.
			base := uint8(200)
			if ((x/8)+(y/8))%2 == 0 {
				base = 50
			}
			img.SetRGBA(x, y, color.RGBA{base, base, base, 255})
		}
	}
	return img
}

// shift returns a copy of src translated by (dx, dy). The edge
// exposed by the shift is filled with the average image color (not
// black) to keep LK gradients well-behaved near boundaries.
func shift(src *image.RGBA, dx, dy int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x-dx, y-dy
			if sx >= 0 && sx < w && sy >= 0 && sy < h {
				out.SetRGBA(x, y, src.RGBAAt(sx, sy))
			} else {
				out.SetRGBA(x, y, color.RGBA{128, 128, 128, 255})
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestSparse_DetectsHorizontalMotionSign(t *testing.T) {
	// LK assumes infinitesimal motion; on our 1-pixel-per-step
	// checkerboard a single-pixel shift triggers the valid "small
	// motion" regime. Anything larger breaks the linearization and
	// LK's direct solution reverts to aliasing. Tests assert the
	// motion SIGN and non-trivial magnitude rather than exact pixels
	// — the downstream summary layer always uses the median sign
	// anyway (scroll up vs scroll down).
	prev := gradient(128, 96)
	curr := shift(prev, 1, 0)
	vectors, err := (Computer{}).Sparse(context.Background(), prev, curr, 8)
	if err != nil {
		t.Fatalf("Sparse: %v", err)
	}
	if len(vectors) == 0 {
		t.Fatal("no flow vectors computed")
	}
	u, v := Median(vectors)
	if u <= 0 {
		t.Errorf("median U = %v, want > 0 for rightward shift", u)
	}
	if math.Abs(v) > math.Abs(u) {
		t.Errorf("median V = %v dominates U = %v — direction wrong", v, u)
	}
}

func TestSparse_DetectsVerticalMotionSign(t *testing.T) {
	prev := gradient(128, 96)
	curr := shift(prev, 0, 1)
	vectors, _ := (Computer{}).Sparse(context.Background(), prev, curr, 8)
	u, v := Median(vectors)
	if v <= 0 {
		t.Errorf("median V = %v, want > 0 for downward shift", v)
	}
	if math.Abs(u) > math.Abs(v) {
		t.Errorf("median U = %v dominates V = %v — direction wrong", u, v)
	}
}

func TestSparse_IdenticalFramesProduceZeroFlow(t *testing.T) {
	img := gradient(128, 96)
	vectors, err := (Computer{}).Sparse(context.Background(), img, img, 8)
	if err != nil {
		t.Fatalf("Sparse: %v", err)
	}
	u, v := Median(vectors)
	if math.Abs(u) > 0.01 || math.Abs(v) > 0.01 {
		t.Fatalf("identical frames median = (%v, %v), want ≈ (0, 0)", u, v)
	}
}

// ---------------------------------------------------------------------------
// Grid parameter
// ---------------------------------------------------------------------------

func TestSparse_FinerGridProducesMoreVectors(t *testing.T) {
	prev := gradient(256, 192)
	curr := shift(prev, 1, 1)
	sparse, _ := (Computer{}).Sparse(context.Background(), prev, curr, 32)
	dense, _ := (Computer{}).Sparse(context.Background(), prev, curr, 8)
	if len(dense) <= len(sparse) {
		t.Fatalf("dense (grid=8) = %d, sparse (grid=32) = %d — dense should have more", len(dense), len(sparse))
	}
}

func TestSparse_CustomWindowSize(t *testing.T) {
	prev := gradient(128, 96)
	curr := shift(prev, 2, 0)
	small, _ := (Computer{WindowSize: 3}).Sparse(context.Background(), prev, curr, 8)
	large, _ := (Computer{WindowSize: 15}).Sparse(context.Background(), prev, curr, 8)
	// Both should produce vectors; custom window shouldn't break LK.
	if len(small) == 0 || len(large) == 0 {
		t.Fatalf("custom window sizes produced no vectors: small=%d large=%d", len(small), len(large))
	}
}

func TestSparse_EvenWindowSizeForcedOdd(t *testing.T) {
	prev := gradient(64, 64)
	curr := shift(prev, 1, 0)
	// WindowSize=6 should be forced to 7 internally. No error,
	// produces vectors.
	vectors, err := (Computer{WindowSize: 6}).Sparse(context.Background(), prev, curr, 8)
	if err != nil {
		t.Fatalf("Sparse: %v", err)
	}
	if len(vectors) == 0 {
		t.Fatal("even WindowSize should still produce vectors (forced odd)")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestSparse_NilImageError(t *testing.T) {
	img := gradient(16, 16)
	if _, err := (Computer{}).Sparse(context.Background(), nil, img, 4); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil prev: %v, want ErrNilImage", err)
	}
	if _, err := (Computer{}).Sparse(context.Background(), img, nil, 4); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil curr: %v, want ErrNilImage", err)
	}
}

func TestSparse_DimensionMismatchError(t *testing.T) {
	a := gradient(16, 16)
	b := gradient(32, 16)
	if _, err := (Computer{}).Sparse(context.Background(), a, b, 4); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("mismatch: %v, want ErrDimensionMismatch", err)
	}
}

func TestSparse_TooSmallError(t *testing.T) {
	a := gradient(4, 4)
	b := gradient(4, 4)
	if _, err := (Computer{}).Sparse(context.Background(), a, b, 1); !errors.Is(err, ErrTooSmall) {
		t.Fatalf("tiny: %v, want ErrTooSmall", err)
	}
}

func TestSparse_InvalidGridError(t *testing.T) {
	a := gradient(32, 32)
	b := gradient(32, 32)
	if _, err := (Computer{}).Sparse(context.Background(), a, b, 0); !errors.Is(err, ErrInvalidGrid) {
		t.Fatalf("grid=0: %v, want ErrInvalidGrid", err)
	}
	if _, err := (Computer{}).Sparse(context.Background(), a, b, -1); !errors.Is(err, ErrInvalidGrid) {
		t.Fatalf("grid=-1: %v, want ErrInvalidGrid", err)
	}
}

func TestSparse_ContextCanceled(t *testing.T) {
	// Need > 64 rows to reach the ctx check.
	a := gradient(64, 128)
	b := shift(a, 1, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Computer{}).Sparse(ctx, a, b, 2); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// solveLK degenerate branch
// ---------------------------------------------------------------------------

func TestSolveLK_UniformWindowReturnsFalse(t *testing.T) {
	// All-zero gradients → det(A^T A) = 0 → returns (0, 0, false).
	ix := make([]float64, 100)
	iy := make([]float64, 100)
	it := make([]float64, 100)
	u, v, ok := solveLK(ix, iy, it, 10, 5, 5, 2)
	if ok {
		t.Fatalf("uniform gradients: solveLK returned ok=true (u=%v, v=%v)", u, v)
	}
}

// ---------------------------------------------------------------------------
// Median + helpers
// ---------------------------------------------------------------------------

func TestMedian_EmptyReturnsZero(t *testing.T) {
	if u, v := Median(nil); u != 0 || v != 0 {
		t.Fatalf("empty median = (%v, %v)", u, v)
	}
}

func TestMedian_OddCountUsesMiddleElement(t *testing.T) {
	vecs := []Vector{
		{U: 1, V: 10},
		{U: 3, V: 30},
		{U: 2, V: 20},
	}
	u, v := Median(vecs)
	if u != 2 || v != 20 {
		t.Fatalf("median = (%v, %v), want (2, 20)", u, v)
	}
}

func TestMedian_EvenCountAveragesMiddlePair(t *testing.T) {
	vecs := []Vector{
		{U: 1, V: 10}, {U: 2, V: 20}, {U: 3, V: 30}, {U: 4, V: 40},
	}
	u, v := Median(vecs)
	if u != 2.5 || v != 25 {
		t.Fatalf("median = (%v, %v), want (2.5, 25)", u, v)
	}
}

func TestMedianOf_Empty(t *testing.T) {
	if got := medianOf(nil); got != 0 {
		t.Fatalf("medianOf(nil) = %v, want 0", got)
	}
}

func TestInsertionSort(t *testing.T) {
	a := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	insertionSort(a)
	want := []float64{1, 1, 2, 3, 4, 5, 6, 9}
	for i := range want {
		if a[i] != want[i] {
			t.Fatalf("sorted[%d] = %v, want %v", i, a[i], want[i])
		}
	}
}

func TestRec709Luma_Black(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for _, v := range rec709Luma(img) {
		if v != 0 {
			t.Errorf("black = %d, want 0", v)
		}
	}
}
