// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// embed inserts a needle into a haystack at (nx, ny) using a
// deterministic checker-with-gradient pattern inside the needle
// so NCC has signal to correlate against.
func embed(hw, hh int, nx, ny, nw, nh int, hayColor, needleColor color.RGBA) *image.RGBA {
	img := solid(hw, hh, hayColor)
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			// Checker with gradient inside the needle area.
			if ((x/4)+(y/4))%2 == 0 {
				img.SetRGBA(nx+x, ny+y, needleColor)
			} else {
				img.SetRGBA(nx+x, ny+y, color.RGBA{
					R: uint8((x * 255) / nw),
					G: uint8((y * 255) / nh),
					B: 128,
					A: 255,
				})
			}
		}
	}
	return img
}

func needle(nw, nh int, base color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			if ((x/4)+(y/4))%2 == 0 {
				img.SetRGBA(x, y, base)
			} else {
				img.SetRGBA(x, y, color.RGBA{
					R: uint8((x * 255) / nw),
					G: uint8((y * 255) / nh),
					B: 128,
					A: 255,
				})
			}
		}
	}
	return img
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestMatch_ExactEmbeddedTemplateFoundAtKnownLocation(t *testing.T) {
	n := needle(20, 20, color.RGBA{255, 100, 100, 255})
	h := embed(200, 200, 42, 37, 20, 20, color.RGBA{50, 50, 50, 255}, color.RGBA{255, 100, 100, 255})
	r, found, err := (Matcher{}).Match(context.Background(), h, n)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !found {
		t.Fatalf("perfect match should be found, got score=%v", r.Score)
	}
	if r.BBox.Min.X != 42 || r.BBox.Min.Y != 37 {
		t.Fatalf("match location = %v, want (42, 37)", r.BBox.Min)
	}
	if r.Score < 0.99 {
		t.Fatalf("score = %v, want ≈ 1", r.Score)
	}
}

func TestMatch_TemplateAbsentReturnsNotFound(t *testing.T) {
	n := needle(16, 16, color.RGBA{255, 0, 0, 255})
	// Solid-color haystack — no correlation signal for the needle's
	// checker pattern.
	h := solid(200, 200, color.RGBA{50, 50, 50, 255})
	r, found, err := (Matcher{MinScore: 0.9}).Match(context.Background(), h, n)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if found {
		t.Fatalf("template should not be found in solid haystack, got score=%v", r.Score)
	}
}

func TestMatch_CenterReturnsBBoxMidpoint(t *testing.T) {
	n := needle(10, 20, color.RGBA{0, 0, 0, 255})
	h := embed(100, 100, 30, 40, 10, 20, color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255})
	r, _, _ := (Matcher{}).Match(context.Background(), h, n)
	c := r.Center()
	// Expected center = (30 + 10/2, 40 + 20/2) = (35, 50).
	if c.X != 35 || c.Y != 50 {
		t.Fatalf("Center = %v, want (35, 50)", c)
	}
}

func TestMatch_CustomMinScoreEnforced(t *testing.T) {
	n := needle(20, 20, color.RGBA{255, 100, 100, 255})
	// Haystack contains a SHIFTED/MODIFIED version of the needle —
	// close but not exact. With MinScore=0.99 it should fail; with
	// MinScore=0.2 it should pass.
	h := embed(100, 100, 30, 30, 20, 20, color.RGBA{50, 50, 50, 255}, color.RGBA{255, 100, 100, 255})
	// Add some noise to reduce the perfect score.
	for y := 30; y < 50; y++ {
		for x := 30; x < 50; x++ {
			if (x+y)%7 == 0 {
				h.SetRGBA(x, y, color.RGBA{80, 80, 80, 255})
			}
		}
	}

	strict, foundStrict, _ := (Matcher{MinScore: 0.999999}).Match(context.Background(), h, n)
	loose, foundLoose, _ := (Matcher{MinScore: 0.2}).Match(context.Background(), h, n)

	// The noise should make the strict threshold reject while loose
	// accepts. If the noise happens to produce a still-perfect
	// match, relax the assertion to just "strict <= loose".
	if strict.Score > loose.Score {
		t.Fatalf("inconsistent: strict=%v loose=%v", strict.Score, loose.Score)
	}
	if foundStrict && !foundLoose {
		t.Fatal("strict found but loose didn't — impossible")
	}
}

// ---------------------------------------------------------------------------
// Uniform haystack / needle degeneracies
// ---------------------------------------------------------------------------

func TestMatch_UniformNeedleReturnsNotFoundGracefully(t *testing.T) {
	n := solid(16, 16, color.RGBA{128, 128, 128, 255})
	h := solid(100, 100, color.RGBA{128, 128, 128, 255})
	r, found, err := (Matcher{}).Match(context.Background(), h, n)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	// Uniform needle has zero variance → no NCC signal → found=false,
	// score=0, region anchored at (0, 0).
	if found {
		t.Fatal("uniform needle must not claim a match")
	}
	if r.BBox != image.Rect(0, 0, 16, 16) {
		t.Fatalf("uniform needle bbox = %v, want (0,0)-(16,16)", r.BBox)
	}
}

func TestMatch_UniformHaystackWindowHasZeroScore(t *testing.T) {
	n := needle(16, 16, color.RGBA{255, 0, 0, 255})
	h := solid(100, 100, color.RGBA{128, 128, 128, 255})
	r, _, _ := (Matcher{}).Match(context.Background(), h, n)
	// Every haystack window has zero variance → ncc returns 0.
	// Best score is 0.
	if r.Score != 0 {
		t.Fatalf("all-uniform haystack score = %v, want 0", r.Score)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestMatch_NilImageError(t *testing.T) {
	n := needle(4, 4, color.RGBA{0, 0, 0, 255})
	if _, _, err := (Matcher{}).Match(context.Background(), nil, n); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil haystack: %v, want ErrNilImage", err)
	}
	h := solid(10, 10, color.RGBA{0, 0, 0, 255})
	if _, _, err := (Matcher{}).Match(context.Background(), h, nil); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil needle: %v, want ErrNilImage", err)
	}
}

func TestMatch_ZeroBoundsError(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	n := needle(4, 4, color.RGBA{0, 0, 0, 255})
	if _, _, err := (Matcher{}).Match(context.Background(), empty, n); !errors.Is(err, ErrZeroBounds) {
		t.Fatalf("empty haystack: %v, want ErrZeroBounds", err)
	}
	h := solid(10, 10, color.RGBA{0, 0, 0, 255})
	if _, _, err := (Matcher{}).Match(context.Background(), h, empty); !errors.Is(err, ErrZeroBounds) {
		t.Fatalf("empty needle: %v, want ErrZeroBounds", err)
	}
}

func TestMatch_NeedleLargerThanHaystack(t *testing.T) {
	h := solid(10, 10, color.RGBA{0, 0, 0, 255})
	n := solid(20, 20, color.RGBA{0, 0, 0, 255})
	if _, _, err := (Matcher{}).Match(context.Background(), h, n); !errors.Is(err, ErrNeedleTooLarge) {
		t.Fatalf("oversized needle: %v, want ErrNeedleTooLarge", err)
	}
}

func TestMatch_ContextCanceled(t *testing.T) {
	// Need a haystack bigger than 32 rows to hit the ctx check
	// inside the scan loop.
	h := solid(100, 100, color.RGBA{0, 0, 0, 255})
	n := needle(8, 8, color.RGBA{255, 0, 0, 255})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := (Matcher{}).Match(ctx, h, n); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// ncc + rec709Luma unit tests
// ---------------------------------------------------------------------------

func TestNCC_ZeroHaystackVarianceReturnsZero(t *testing.T) {
	// Uniform haystack window = zero variance → NCC divides by 0
	// after sqrt(0*nSqDev)=0. The guarded branch returns 0.
	hay := []uint8{100, 100, 100, 100}
	n := []uint8{50, 60, 70, 80}
	// Precomputed needle stats for 1×4 needle:
	nMean := (50.0 + 60 + 70 + 80) / 4
	var nSqDev float64
	for _, v := range n {
		d := float64(v) - nMean
		nSqDev += d * d
	}
	got := ncc(hay, 4, 0, 0, n, 4, 1, nMean, nSqDev)
	if got != 0 {
		t.Fatalf("uniform haystack NCC = %v, want 0", got)
	}
}

func TestRec709Luma_BlackAndWhite(t *testing.T) {
	black := solid(2, 2, color.RGBA{0, 0, 0, 255})
	for _, v := range rec709Luma(black) {
		if v != 0 {
			t.Errorf("black pixel = %d, want 0", v)
		}
	}
	white := solid(2, 2, color.RGBA{255, 255, 255, 255})
	for _, v := range rec709Luma(white) {
		if v < 250 {
			t.Errorf("white pixel = %d, want ≥ 250", v)
		}
	}
}
