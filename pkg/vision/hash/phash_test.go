// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"image"
	"image/color"
	"math/bits"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestPHash_IdenticalImagesReturnZeroDistance(t *testing.T) {
	h := PHasher{}
	img := gradientRGBA(128, 96)
	a, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, _ := h.Hash(img)
	if d := h.Distance(a, b); d != 0 {
		t.Fatalf("identical pHash distance = %d, want 0", d)
	}
}

func TestPHash_CompletelyDifferentImagesReturnLargeDistance(t *testing.T) {
	h := PHasher{}
	a, _ := h.Hash(gradientRGBA(128, 96))
	b, _ := h.Hash(randomRGBA(128, 96, 0xDEADBEEF))
	d := h.Distance(a, b)
	// Uncorrelated 64-bit pHashes have expected Hamming distance
	// around 32; use a conservative floor of 15 to avoid flaky tests
	// on lucky seeds.
	if d < 15 {
		t.Fatalf("different images pHash distance = %d, want ≥ 15", d)
	}
}

func TestPHash_ShiftedImagesStillSimilar(t *testing.T) {
	// pHash is shift-robust *in principle*, but on small source
	// images a 1-pixel shift becomes a significant fraction of the
	// 32×32 downsampled grid. We assert the shift stays well under
	// the ~32-bit expected Hamming distance of uncorrelated inputs
	// — i.e. the pHash still recognizes the two images as related,
	// not identical.
	h := PHasher{}
	src := gradientRGBA(512, 384) // larger source → shift is truly 1 pixel in 512
	shifted := shiftRGBA(src, 1, 0)
	a, _ := h.Hash(src)
	b, _ := h.Hash(shifted)
	d := h.Distance(a, b)
	if d > 16 {
		t.Fatalf("1-pixel shift pHash distance = %d, want ≤ 16 (should remain < uncorrelated baseline of ~32)", d)
	}
}

func TestPHash_HammingPackedCorrectly(t *testing.T) {
	// A black vs white image should produce opposite-mask hashes
	// after DC exclusion. The DC coefficient is massive for both
	// (all-black has 0, all-white has 255 summed), so bit 63 (the
	// DC slot) will be the same for both. The remaining 63 bits
	// capture the difference — for two uniform images the DCT has
	// zero AC components so all 63 bits flip consistently; we
	// just check that distance is non-zero.
	h := PHasher{}
	black := image.NewRGBA(image.Rect(0, 0, 64, 64))
	white := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			white.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	a, _ := h.Hash(black)
	b, _ := h.Hash(white)
	// Uniform images have degenerate ACs — the median split against
	// ties produces a hash pattern but the key invariant is: the
	// two hashes must not be completely identical (that would mean
	// black and white collapse to the same 64-bit output).
	if bits.OnesCount64(a) == bits.OnesCount64(b) && a == b {
		t.Fatalf("black and white hashes are identical — DCT pipeline broken")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestPHash_NilImageError(t *testing.T) {
	if _, err := (PHasher{}).Hash(nil); err != ErrNilImage {
		t.Fatalf("nil image = %v, want ErrNilImage", err)
	}
}

func TestPHash_ZeroBoundsError(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if _, err := (PHasher{}).Hash(empty); err != ErrZeroBounds {
		t.Fatalf("empty image = %v, want ErrZeroBounds", err)
	}
}

// ---------------------------------------------------------------------------
// Image type coverage
// ---------------------------------------------------------------------------

func TestPHash_HandlesConcreteImageTypes(t *testing.T) {
	h := PHasher{}
	src := gradientRGBA(64, 64)

	nrgba := image.NewNRGBA(src.Rect)
	for y := 0; y < src.Rect.Dy(); y++ {
		for x := 0; x < src.Rect.Dx(); x++ {
			c := src.RGBAAt(x, y)
			nrgba.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, c.A})
		}
	}
	gray := image.NewGray(src.Rect)
	for y := 0; y < src.Rect.Dy(); y++ {
		for x := 0; x < src.Rect.Dx(); x++ {
			c := src.RGBAAt(x, y)
			gray.SetGray(x, y, color.Gray{Y: lumaRec709(c.R, c.G, c.B)})
		}
	}

	for _, tc := range []struct {
		name string
		img  image.Image
	}{
		{"RGBA", src},
		{"NRGBA", nrgba},
		{"Gray", gray},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := h.Hash(tc.img)
			if err != nil {
				t.Fatalf("Hash: %v", err)
			}
			b, _ := h.Hash(tc.img)
			if h.Distance(a, b) != 0 {
				t.Fatalf("%s self-pHash distance != 0", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Hasher interface conformance
// ---------------------------------------------------------------------------

func TestPHash_SatisfiesHasherInterface(t *testing.T) {
	var h Hasher = PHasher{}
	img := gradientRGBA(64, 64)
	a, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash via interface: %v", err)
	}
	if h.Distance(a, a) != 0 {
		t.Fatal("Distance(self, self) != 0 via interface")
	}
}

// ---------------------------------------------------------------------------
// DCT unit test
// ---------------------------------------------------------------------------

func TestDCT32_ConstantInputHasOnlyDC(t *testing.T) {
	// A constant image produces zero AC coefficients — the DCT's
	// hallmark property.
	pixels := make([]uint8, 32*32)
	for i := range pixels {
		pixels[i] = 128
	}
	dct := dct32(pixels)

	// DC (index 0) must be large and positive.
	if dct[0] <= 0 {
		t.Fatalf("DC = %v, want > 0 for constant input", dct[0])
	}
	// Every other coefficient should be within rounding of zero.
	for i := 1; i < len(dct); i++ {
		if abs(dct[i]) > 1e-8 {
			t.Errorf("AC coefficient %d = %v, want ≈ 0", i, dct[i])
		}
	}
}

func TestDCT32_PanicsOnWrongSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("dct32 should panic on wrong-size input")
		}
	}()
	dct32(make([]uint8, 10))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ---------------------------------------------------------------------------
// Benchmarks + perf
// ---------------------------------------------------------------------------

func BenchmarkPHash_1080p(b *testing.B) {
	h := PHasher{}
	img := gradientRGBA(1920, 1080)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.Hash(img); err != nil {
			b.Fatal(err)
		}
	}
}

// TestPerformance_PHash_Under25msPer1080pFrame enforces a reasonable
// perf ceiling for pHash. The canonical expectation is < 5 ms
// (approximately 5× dHash's cost), but the DCT setup + two passes
// put it around 1-3 ms on commodity CPU; 25 ms is a generous
// regression guard.
func TestPerformance_PHash_Under25msPer1080pFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("long perf test — skip in short mode")
	}
	h := PHasher{}
	img := gradientRGBA(1920, 1080)

	const runs = 10
	start := time.Now()
	for i := 0; i < runs; i++ {
		if _, err := h.Hash(img); err != nil {
			t.Fatalf("Hash: %v", err)
		}
	}
	avg := time.Since(start) / runs
	t.Logf("PHash @ 1080p average: %s", avg)
	if avg > 25*time.Millisecond {
		t.Fatalf("PHash averaged %s/frame — expected < 25 ms on commodity CPU", avg)
	}
}
