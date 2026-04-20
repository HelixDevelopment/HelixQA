// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"image"
	"image/color"
	"math/bits"
	"math/rand/v2"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixture synthesis — avoids checking in 1080p PNG binaries.
// ---------------------------------------------------------------------------

// gradientRGBA produces a deterministic 1920×1080 gradient image: R grows with
// x, G grows with y, B is the XOR pattern. Good dHash input because the per-
// row luminance differences are stable.
func gradientRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / (w - 1)),
				G: uint8((y * 255) / (h - 1)),
				B: uint8((x ^ y) & 0xFF),
				A: 255,
			})
		}
	}
	return img
}

// shiftRGBA returns a copy of src shifted right by dx, down by dy, with the
// exposed edge filled with black. Simulates a 1-pixel UI jitter.
func shiftRGBA(src *image.RGBA, dx, dy int) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sx, sy := x-dx, y-dy
			if sx < b.Min.X || sx >= b.Max.X || sy < b.Min.Y || sy >= b.Max.Y {
				out.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				continue
			}
			out.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return out
}

// randomRGBA seeds a deterministic PRNG for a reproducible "completely
// different" image.
func randomRGBA(w, h int, seed uint64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, seed*0x9E3779B97F4A7C15))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(r.UintN(256)),
				G: uint8(r.UintN(256)),
				B: uint8(r.UintN(256)),
				A: 255,
			})
		}
	}
	return img
}

// ---------------------------------------------------------------------------
// DHash-64 — happy path
// ---------------------------------------------------------------------------

func TestDHash64_IdenticalImages_DistanceZero(t *testing.T) {
	h := DHasher{Kind: DHash64}
	img := gradientRGBA(640, 360)

	a, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash (second call): %v", err)
	}
	if got := h.Distance(a, b); got != 0 {
		t.Fatalf("identical images must have distance 0, got %d", got)
	}
}

func TestDHash64_ShiftedByOnePixel_DistanceSmall(t *testing.T) {
	h := DHasher{Kind: DHash64}
	src := gradientRGBA(640, 360)
	shifted := shiftRGBA(src, 1, 0)

	a, err := h.Hash(src)
	if err != nil {
		t.Fatalf("src: %v", err)
	}
	b, err := h.Hash(shifted)
	if err != nil {
		t.Fatalf("shifted: %v", err)
	}
	d := h.Distance(a, b)
	// dHash is designed to be robust to tiny shifts — 1px on a 640-px image
	// downsampled to 9×8 should almost never flip more than a handful of bits.
	if d > 8 {
		t.Fatalf("1-pixel shift produced distance %d — dHash should be robust", d)
	}
}

func TestDHash64_CompletelyDifferent_DistanceLarge(t *testing.T) {
	h := DHasher{Kind: DHash64}
	a, err := h.Hash(gradientRGBA(640, 360))
	if err != nil {
		t.Fatalf("gradient: %v", err)
	}
	b, err := h.Hash(randomRGBA(640, 360, 0xC4A41062))
	if err != nil {
		t.Fatalf("random: %v", err)
	}
	d := h.Distance(a, b)
	// A deterministic gradient vs uniform RGB noise should disagree wildly.
	// The expected Hamming distance on uncorrelated 64-bit hashes is ~32; we
	// use a conservative floor of 20 to allow for occasional lucky collisions
	// while still failing if the impl returns identical hashes.
	if d < 20 {
		t.Fatalf("completely different images produced suspiciously low distance %d", d)
	}
}

// ---------------------------------------------------------------------------
// DHash-256 — happy path
// ---------------------------------------------------------------------------

func TestDHash256_IdenticalImages_DistanceZero(t *testing.T) {
	h := DHasher{Kind: DHash256}
	img := gradientRGBA(640, 360)

	a, err := h.Hash256(img)
	if err != nil {
		t.Fatalf("Hash256: %v", err)
	}
	b, err := h.Hash256(img)
	if err != nil {
		t.Fatalf("Hash256 (second): %v", err)
	}
	if got := a.Distance(*b); got != 0 {
		t.Fatalf("identical Hash256: distance %d, want 0", got)
	}
}

func TestDHash256_CompletelyDifferent_DistanceLarge(t *testing.T) {
	h := DHasher{Kind: DHash256}
	a, err := h.Hash256(gradientRGBA(640, 360))
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	b, err := h.Hash256(randomRGBA(640, 360, 0xC4A41062))
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	d := a.Distance(*b)
	// Expected ~128 bits out of 256 for uncorrelated inputs. Floor at 80.
	if d < 80 {
		t.Fatalf("completely different Hash256: distance %d, want ≥ 80", d)
	}
}

func TestBigHash_Distance_Symmetric(t *testing.T) {
	x := BigHash{0x1111_1111_1111_1111, 0xFFFF_FFFF_FFFF_FFFF, 0, 0x5A5A_5A5A_5A5A_5A5A}
	y := BigHash{0x0000_0000_0000_0000, 0x0F0F_0F0F_0F0F_0F0F, 0x1234_5678_9ABC_DEF0, 0xFFFF_FFFF_FFFF_FFFF}
	if x.Distance(y) != y.Distance(x) {
		t.Fatalf("Distance not symmetric")
	}
	var expected int
	for i := 0; i < 4; i++ {
		expected += bits.OnesCount64(x[i] ^ y[i])
	}
	if got := x.Distance(y); got != expected {
		t.Fatalf("Distance = %d, want %d", got, expected)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestHasher_WrongKind_Hash64OnDHash256Hasher(t *testing.T) {
	h := DHasher{Kind: DHash256}
	if _, err := h.Hash(gradientRGBA(64, 64)); err != ErrWrongKind64 {
		t.Fatalf("want ErrWrongKind64, got %v", err)
	}
}

func TestHasher_WrongKind_Hash256OnDHash64Hasher(t *testing.T) {
	h := DHasher{Kind: DHash64}
	if _, err := h.Hash256(gradientRGBA(64, 64)); err != ErrWrongKind256 {
		t.Fatalf("want ErrWrongKind256, got %v", err)
	}
}

func TestHasher_NilImage_Hash(t *testing.T) {
	h := DHasher{Kind: DHash64}
	if _, err := h.Hash(nil); err != ErrNilImage {
		t.Fatalf("Hash(nil) = %v, want ErrNilImage", err)
	}
}

func TestHasher_NilImage_Hash256(t *testing.T) {
	h := DHasher{Kind: DHash256}
	if _, err := h.Hash256(nil); err != ErrNilImage {
		t.Fatalf("Hash256(nil) = %v, want ErrNilImage", err)
	}
}

func TestHasher_ZeroBounds(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	h := DHasher{Kind: DHash64}
	if _, err := h.Hash(empty); err != ErrZeroBounds {
		t.Fatalf("zero-bounds Hash = %v, want ErrZeroBounds", err)
	}
	h256 := DHasher{Kind: DHash256}
	if _, err := h256.Hash256(empty); err != ErrZeroBounds {
		t.Fatalf("zero-bounds Hash256 = %v, want ErrZeroBounds", err)
	}
}

func TestDistance64_Examples(t *testing.T) {
	h := DHasher{Kind: DHash64}
	cases := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0xFFFF_FFFF_FFFF_FFFF, 0, 64},
		{0xAAAA_AAAA_AAAA_AAAA, 0x5555_5555_5555_5555, 64},
		{0xFF, 0x01, 7},
	}
	for _, c := range cases {
		if got := h.Distance(c.a, c.b); got != c.want {
			t.Errorf("Distance(%#x, %#x) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Shape coverage — Gray, NRGBA, tiny images
// ---------------------------------------------------------------------------

func TestDHash64_HandlesVariousImageTypes(t *testing.T) {
	h := DHasher{Kind: DHash64}
	for _, name := range []string{"RGBA", "NRGBA", "Gray"} {
		t.Run(name, func(t *testing.T) {
			src := gradientRGBA(128, 96)
			var img image.Image = src
			switch name {
			case "NRGBA":
				nrgba := image.NewNRGBA(src.Rect)
				for y := 0; y < src.Rect.Dy(); y++ {
					for x := 0; x < src.Rect.Dx(); x++ {
						c := src.RGBAAt(x, y)
						nrgba.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, c.A})
					}
				}
				img = nrgba
			case "Gray":
				gray := image.NewGray(src.Rect)
				for y := 0; y < src.Rect.Dy(); y++ {
					for x := 0; x < src.Rect.Dx(); x++ {
						c := src.RGBAAt(x, y)
						y := (2126*int(c.R) + 7152*int(c.G) + 722*int(c.B)) / 10000
						gray.SetGray(x, y, color.Gray{uint8(y)})
					}
				}
				img = gray
			}
			if _, err := h.Hash(img); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
}

func TestDHash64_YCbCr(t *testing.T) {
	h := DHasher{Kind: DHash64}
	// Synthesize a YCbCr image from the gradient; the fast-path reads
	// src.Y directly so Cb/Cr content doesn't influence the hash — this
	// test just asserts that the path runs end-to-end.
	src := gradientRGBA(128, 96)
	yc := image.NewYCbCr(src.Rect, image.YCbCrSubsampleRatio420)
	for y := 0; y < src.Rect.Dy(); y++ {
		for x := 0; x < src.Rect.Dx(); x++ {
			c := src.RGBAAt(x, y)
			yi := yc.YOffset(x, y)
			yc.Y[yi] = lumaRec709(c.R, c.G, c.B)
		}
	}
	if _, err := h.Hash(yc); err != nil {
		t.Fatalf("YCbCr Hash: %v", err)
	}
	if _, err := (DHasher{Kind: DHash256}).Hash256(yc); err != nil {
		t.Fatalf("YCbCr Hash256: %v", err)
	}
}

// paletteImage is a minimal image.Image that does NOT match any of the
// fast-path concrete types — it exists to exercise the default branch of
// resizeGray (generic .At().RGBA() access).
type paletteImage struct{ rect image.Rectangle }

func (p paletteImage) ColorModel() color.Model { return color.RGBAModel }
func (p paletteImage) Bounds() image.Rectangle { return p.rect }
func (p paletteImage) At(x, y int) color.Color {
	v := uint8((x*7 + y*13) & 0xFF)
	return color.RGBA{v, v, v, 255}
}

func TestDHash64_GenericImageInterface(t *testing.T) {
	h := DHasher{Kind: DHash64}
	img := paletteImage{rect: image.Rect(0, 0, 256, 128)}
	if _, err := h.Hash(img); err != nil {
		t.Fatalf("generic Hash: %v", err)
	}
	if _, err := (DHasher{Kind: DHash256}).Hash256(img); err != nil {
		t.Fatalf("generic Hash256: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Hasher interface conformance
// ---------------------------------------------------------------------------

func TestDHasher_SatisfiesHasherInterface(t *testing.T) {
	var h Hasher = DHasher{Kind: DHash64}
	out, err := h.Hash(gradientRGBA(64, 64))
	if err != nil {
		t.Fatalf("Hash via interface: %v", err)
	}
	if h.Distance(out, out) != 0 {
		t.Fatal("Distance(self,self) != 0 via interface")
	}
}

// ---------------------------------------------------------------------------
// Benchmarks — Article V category 8 (< 5 ms @ 1080p CPU).
// ---------------------------------------------------------------------------

func BenchmarkDHash64_1080p(b *testing.B) {
	h := DHasher{Kind: DHash64}
	img := gradientRGBA(1920, 1080)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.Hash(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDHash256_1080p(b *testing.B) {
	h := DHasher{Kind: DHash256}
	img := gradientRGBA(1920, 1080)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.Hash256(img); err != nil {
			b.Fatal(err)
		}
	}
}

// TestPerformance_DHash64_Under5msPer1080pFrame enforces the Article V
// benchmark target as a regular test so CI (which runs `go test`) catches
// regressions without needing a separate bench harness.
func TestPerformance_DHash64_Under5msPer1080pFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("long perf test — skip in short mode")
	}
	h := DHasher{Kind: DHash64}
	img := gradientRGBA(1920, 1080)

	const runs = 20
	start := time.Now()
	for i := 0; i < runs; i++ {
		if _, err := h.Hash(img); err != nil {
			t.Fatalf("Hash: %v", err)
		}
	}
	avg := time.Since(start) / runs
	t.Logf("DHash64 @ 1080p average: %s", avg)
	if avg > 5*time.Millisecond {
		t.Fatalf("DHash64 averaged %s/frame at 1080p — budget is 5ms (OpenClawing4.md §5.8 tier-1)", avg)
	}
}
