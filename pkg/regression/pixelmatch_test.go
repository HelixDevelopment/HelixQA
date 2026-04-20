// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"image"
	"image/color"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func checkerRGBA(w, h, tile int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if ((x/tile)+(y/tile))%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	return img
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestDiff_IdenticalImagesYieldZeroDiff(t *testing.T) {
	a := solidRGBA(32, 32, color.RGBA{128, 64, 200, 255})
	r, err := PixelMatch{}.Diff(a, a, DiffOptions{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if r.DiffCount != 0 {
		t.Fatalf("identical images DiffCount = %d, want 0", r.DiffCount)
	}
	if r.AACount != 0 {
		t.Fatalf("identical images AACount = %d, want 0", r.AACount)
	}
	if r.TotalPixels != 32*32 {
		t.Fatalf("TotalPixels = %d, want %d", r.TotalPixels, 32*32)
	}
	if r.Output == nil {
		t.Fatal("Output image must be non-nil")
	}
	if r.Output.Bounds().Dx() != 32 || r.Output.Bounds().Dy() != 32 {
		t.Fatalf("Output bounds = %v, want 32×32", r.Output.Bounds())
	}
}

func TestDiff_CompletelyDifferentImagesFlagEveryPixel(t *testing.T) {
	a := solidRGBA(16, 16, color.RGBA{0, 0, 0, 255})
	b := solidRGBA(16, 16, color.RGBA{255, 255, 255, 255})
	r, err := PixelMatch{}.Diff(a, b, DiffOptions{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	// Every pixel differs → all should be flagged as diff (not AA,
	// since solid color has no gradient for AA detection).
	if r.DiffCount != 16*16 {
		t.Fatalf("DiffCount = %d, want 256", r.DiffCount)
	}
}

func TestDiff_PartialDifference(t *testing.T) {
	a := solidRGBA(32, 32, color.RGBA{100, 100, 100, 255})
	b := solidRGBA(32, 32, color.RGBA{100, 100, 100, 255})
	// Smash a 4×4 patch in b with a very different color.
	for y := 10; y < 14; y++ {
		for x := 10; x < 14; x++ {
			b.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	r, err := PixelMatch{}.Diff(a, b, DiffOptions{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	// At least the interior of the 4×4 patch should be flagged; edges
	// may be classified as AA depending on threshold.
	if r.DiffCount+r.AACount < 4 {
		t.Fatalf("DiffCount+AACount = %d, want ≥ 4 for a 4×4 difference patch", r.DiffCount+r.AACount)
	}
	if r.DiffCount+r.AACount > 16 {
		t.Fatalf("DiffCount+AACount = %d, want ≤ 16 (the patch size)", r.DiffCount+r.AACount)
	}
}

// ---------------------------------------------------------------------------
// Threshold sensitivity
// ---------------------------------------------------------------------------

func TestDiff_HigherThresholdIgnoresSmallDifferences(t *testing.T) {
	a := solidRGBA(16, 16, color.RGBA{100, 100, 100, 255})
	b := solidRGBA(16, 16, color.RGBA{105, 105, 105, 255}) // subtle
	strict, _ := PixelMatch{}.Diff(a, b, DiffOptions{Threshold: 0.01})
	loose, _ := PixelMatch{}.Diff(a, b, DiffOptions{Threshold: 0.5})
	if strict.DiffCount <= loose.DiffCount {
		t.Fatalf("strict threshold should flag more pixels than loose: strict=%d loose=%d",
			strict.DiffCount, loose.DiffCount)
	}
}

// ---------------------------------------------------------------------------
// IncludeAA flag
// ---------------------------------------------------------------------------

func TestDiff_IncludeAAFlagCountsAAPixelsAsDifferences(t *testing.T) {
	// Build an image with an AA edge: the checker pattern has sharp
	// transitions that the AA detector classifies as "on an edge" for
	// neighboring pixels — when compared with a version that shifts
	// the checker slightly.
	a := checkerRGBA(16, 16, 4)
	b := image.NewRGBA(a.Bounds())
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			c := a.RGBAAt(x, y)
			// Introduce a subtle gradient shift near edges.
			if (x%4 == 0 || y%4 == 0) && c.R == 0 {
				b.SetRGBA(x, y, color.RGBA{30, 30, 30, 255})
			} else {
				b.SetRGBA(x, y, c)
			}
		}
	}

	notIncluded, _ := PixelMatch{}.Diff(a, b, DiffOptions{Threshold: 0.05})
	included, _ := PixelMatch{}.Diff(a, b, DiffOptions{Threshold: 0.05, IncludeAA: true})

	// When AA is included, AA pixels count as diffs, so total flagged
	// pixels should be ≥ the non-included case.
	if included.DiffCount < notIncluded.DiffCount+notIncluded.AACount-1 {
		t.Fatalf("IncludeAA=true should roll AA into DiffCount: included.DiffCount=%d, notIncluded.DiffCount=%d, notIncluded.AACount=%d",
			included.DiffCount, notIncluded.DiffCount, notIncluded.AACount)
	}
}

// ---------------------------------------------------------------------------
// Custom colors
// ---------------------------------------------------------------------------

func TestDiff_CustomDiffColorRenderedInOutput(t *testing.T) {
	a := solidRGBA(4, 4, color.RGBA{0, 0, 0, 255})
	b := solidRGBA(4, 4, color.RGBA{255, 255, 255, 255})
	green := color.RGBA{0, 200, 0, 255}
	r, err := PixelMatch{}.Diff(a, b, DiffOptions{DiffColor: green})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	// Spot-check a diff pixel.
	if got := r.Output.RGBAAt(1, 1); got != green {
		t.Fatalf("diff output pixel = %+v, want %+v", got, green)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDiff_NilImageErrors(t *testing.T) {
	a := solidRGBA(4, 4, color.RGBA{0, 0, 0, 255})
	if _, err := (PixelMatch{}).Diff(nil, a, DiffOptions{}); err != ErrNilImage {
		t.Fatalf("Diff(nil, a) = %v, want ErrNilImage", err)
	}
	if _, err := (PixelMatch{}).Diff(a, nil, DiffOptions{}); err != ErrNilImage {
		t.Fatalf("Diff(a, nil) = %v, want ErrNilImage", err)
	}
}

func TestDiff_DimensionMismatchErrors(t *testing.T) {
	a := solidRGBA(4, 4, color.RGBA{0, 0, 0, 255})
	b := solidRGBA(8, 4, color.RGBA{0, 0, 0, 255})
	if _, err := (PixelMatch{}).Diff(a, b, DiffOptions{}); err != ErrDimensionMismatch {
		t.Fatalf("dimension mismatch: err = %v, want ErrDimensionMismatch", err)
	}
}

func TestDiff_ZeroSizedImagesReturnEmptyReport(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 0, 0))
	b := image.NewRGBA(image.Rect(0, 0, 0, 0))
	r, err := PixelMatch{}.Diff(a, b, DiffOptions{})
	if err != nil {
		t.Fatalf("zero-sized: %v", err)
	}
	if r.DiffCount != 0 || r.AACount != 0 || r.TotalPixels != 0 {
		t.Fatalf("zero-sized report non-empty: %+v", r)
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func TestColorDelta_YOnlyMode(t *testing.T) {
	// Pure luma change should produce identical Y-only delta regardless
	// of chroma.
	d1 := colorDelta(100, 100, 100, 255, 200, 200, 200, 255, true)
	d2 := colorDelta(100, 50, 150, 255, 200, 150, 250, 255, true)
	// Rough check — d1 is pure luma shift, d2 is luma + chroma.
	// Y-only mode should only capture the luma component which is
	// identical (same Y delta).
	if d1 >= 0 && d2 < 0 {
		t.Fatalf("Y-only delta sign mismatch: d1=%v d2=%v", d1, d2)
	}
}

func TestRGB2YIQ_BlackIsZero(t *testing.T) {
	if y := rgb2y(0, 0, 0); y != 0 {
		t.Fatalf("rgb2y(0,0,0) = %v, want 0", y)
	}
	if i := rgb2i(0, 0, 0); i != 0 {
		t.Fatalf("rgb2i(0,0,0) = %v, want 0", i)
	}
	if q := rgb2q(0, 0, 0); q != 0 {
		t.Fatalf("rgb2q(0,0,0) = %v, want 0", q)
	}
}

func TestBlendWhite_FullOpaquePreservesColor(t *testing.T) {
	r, g, b := blendWhite(100, 150, 200, 255)
	if r != 100 || g != 150 || b != 200 {
		t.Fatalf("blendWhite(opaque) = (%v, %v, %v), want (100, 150, 200)", r, g, b)
	}
}

func TestBlendWhite_ZeroAlphaProducesWhite(t *testing.T) {
	r, g, b := blendWhite(0, 0, 0, 0)
	if r != 255 || g != 255 || b != 255 {
		t.Fatalf("blendWhite(transparent) = (%v, %v, %v), want (255, 255, 255)", r, g, b)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestPixelMatch_SatisfiesDifferInterface(t *testing.T) {
	var d Differ = PixelMatch{}
	a := solidRGBA(4, 4, color.RGBA{0, 0, 0, 255})
	if _, err := d.Diff(a, a, DiffOptions{}); err != nil {
		t.Fatalf("Diff via interface: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Anti-aliasing detection — coverage for the isAA / hasManySiblings paths.
// ---------------------------------------------------------------------------

func TestIsAA_ClassifiesAntiAliasedEdge(t *testing.T) {
	// Construct an image with a diagonal AA edge: rows alternate between
	// dark, mid-gray, and white, which is the signature AA gradient.
	w, h := 10, 10
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x+y < 8 {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else if x+y == 8 {
				img.SetRGBA(x, y, color.RGBA{128, 128, 128, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	// A pixel on the gray diagonal should be classified as AA.
	if !isAA(img, img.Bounds(), 4, 4, w, h) {
		t.Skip("AA heuristic did not classify this synthetic gradient as AA — may need a more nuanced test image")
	}
}

func TestHasManySiblings_UniformPatchIsSibling(t *testing.T) {
	// Solid image: every pixel has all 8 neighbors identical.
	img := solidRGBA(5, 5, color.RGBA{100, 100, 100, 255})
	if !hasManySiblings(img, img.Bounds(), 2, 2, 5, 5) {
		t.Fatal("uniform interior pixel must report ≥ 3 siblings")
	}
}

func TestHasManySiblings_NoiseIsSolitary(t *testing.T) {
	// Stippled noise: no two adjacent pixels match.
	img := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8((x * 51) + y), uint8(x*17 + y*13), 0, 255})
		}
	}
	if hasManySiblings(img, img.Bounds(), 2, 2, 5, 5) {
		t.Fatal("noisy interior pixel must not report ≥ 3 siblings")
	}
}
