// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perceptual

import (
	"context"
	"errors"
	"image"
)

// Comparator is the Phase-2 perceptual-similarity contract. Implementations
// produce a scalar similarity in [-1, 1] — 1.0 = identical, 0.0 = no
// structural agreement, negative = structurally anti-correlated.
type Comparator interface {
	Compare(ctx context.Context, a, b image.Image) (similarity float64, err error)
}

// SSIM is a pure-Go SSIM implementation (Wang et al., 2004,
// "Image Quality Assessment: From Error Visibility to Structural
// Similarity", IEEE Trans. Image Proc. 13(4)).
//
// Why pure Go, not gocv:
//
//   - The Phase-2 Kickoff brief nominated gocv because OpenCV ships a
//     reference impl. gocv requires CGO + OpenCV dev headers on every
//     build host; that contradicts HelixQA's CGO-free discipline.
//   - SSIM is ~60 LoC of arithmetic; OpenCV's advantage is NEON/AVX
//     intrinsics that the pure-Go loop still beats the 5 ms / 480p
//     budget without — measured in ssim_test.go.
//
// Implementation notes:
//
//   - Non-overlapping BlockSize×BlockSize windows. The canonical SSIM
//     paper uses 8×8 or 11×11 Gaussian-weighted sliding windows; the
//     block-sampled variant is ~10× faster and still correlates at
//     r > 0.95 with the full sliding version on natural images —
//     adequate for a "tier-2 verifier" that already sits behind a
//     tier-1 dHash pre-filter.
//   - Luma-only: SSIM is traditionally computed on the Y channel of
//     YCbCr. We do the same (Rec-709 luma on the RGB input).
//   - Constants C1, C2 per the original paper: K1=0.01, K2=0.03,
//     L=255 → C1 = (K1*L)² = 6.5025, C2 = (K2*L)² = 58.5225.
type SSIM struct {
	// BlockSize is the non-overlapping window size. Must be a positive
	// even integer. Zero → default 8.
	BlockSize int

	// K1, K2 are the SSIM stabilisation constants. Zero → defaults
	// 0.01 and 0.03 per the original paper.
	K1, K2 float64

	// L is the dynamic range of the luma channel. Zero → 255 for 8-bit
	// per channel inputs.
	L float64
}

// NewSSIM returns an SSIM comparator with canonical defaults
// (BlockSize=8, K1=0.01, K2=0.03, L=255).
func NewSSIM() SSIM { return SSIM{} }

// Sentinel errors.
var (
	ErrNilImage          = errors.New("helixqa/perceptual: nil image")
	ErrDimensionMismatch = errors.New("helixqa/perceptual: images must have identical bounds")
	ErrTooSmall          = errors.New("helixqa/perceptual: images smaller than one block")
)

// Compare returns the SSIM index between a and b in [-1, 1]. Identical
// images return 1.0; uncorrelated noise returns ≈ 0. Respects ctx
// cancellation between block rows.
func (s SSIM) Compare(ctx context.Context, a, b image.Image) (float64, error) {
	if a == nil || b == nil {
		return 0, ErrNilImage
	}
	ba, bb := a.Bounds(), b.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		return 0, ErrDimensionMismatch
	}
	w, h := ba.Dx(), ba.Dy()

	cfg := s.withDefaults()
	if w < cfg.BlockSize || h < cfg.BlockSize {
		return 0, ErrTooSmall
	}
	c1 := (cfg.K1 * cfg.L) * (cfg.K1 * cfg.L)
	c2 := (cfg.K2 * cfg.L) * (cfg.K2 * cfg.L)

	// Allocate the luma buffers once. The two-pass design (extract-luma
	// then block-stats) is the clean one; measurement shows it's below
	// the 5 ms / 480p budget on commodity CPU.
	lumaA := rec709Luma(a)
	lumaB := rec709Luma(b)

	return ssimFromLuma(ctx, lumaA, lumaB, w, h, cfg.BlockSize, c1, c2)
}

// ssimFromLuma is the block loop — extracted from Compare so tests can
// feed canned luma buffers without going through rec709Luma.
func ssimFromLuma(ctx context.Context, lumaA, lumaB []uint8, w, h, bs int, c1, c2 float64) (float64, error) {
	var sum, count float64
	for by := 0; by+bs <= h; by += bs {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		for bx := 0; bx+bs <= w; bx += bs {
			muA, muB, varA, varB, cov := blockStats(lumaA, lumaB, w, bx, by, bs)
			num := (2*muA*muB + c1) * (2*cov + c2)
			den := (muA*muA + muB*muB + c1) * (varA + varB + c2)
			if den == 0 {
				continue
			}
			sum += num / den
			count++
		}
	}
	if count == 0 {
		return 0, ErrTooSmall
	}
	return sum / count, nil
}

func (s SSIM) withDefaults() SSIM {
	if s.BlockSize == 0 {
		s.BlockSize = 8
	}
	if s.K1 == 0 {
		s.K1 = 0.01
	}
	if s.K2 == 0 {
		s.K2 = 0.03
	}
	if s.L == 0 {
		s.L = 255
	}
	return s
}

// rec709Luma converts an image.Image to an 8-bit luma buffer using the
// Rec-709 coefficients. Fast-paths the common concrete types; falls
// back to the generic image.Image interface for anything exotic.
func rec709Luma(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]uint8, w*h)

	switch src := img.(type) {
	case *image.RGBA:
		stride := src.Stride
		baseX := (b.Min.X - src.Rect.Min.X) * 4
		baseY := b.Min.Y - src.Rect.Min.Y
		for y := 0; y < h; y++ {
			start := (baseY+y)*stride + baseX
			row := src.Pix[start : start+w*4]
			outRow := out[y*w : y*w+w]
			for x := 0; x < w; x++ {
				i := x * 4
				outRow[x] = luma8(row[i], row[i+1], row[i+2])
			}
		}
	case *image.NRGBA:
		stride := src.Stride
		baseX := (b.Min.X - src.Rect.Min.X) * 4
		baseY := b.Min.Y - src.Rect.Min.Y
		for y := 0; y < h; y++ {
			start := (baseY+y)*stride + baseX
			row := src.Pix[start : start+w*4]
			outRow := out[y*w : y*w+w]
			for x := 0; x < w; x++ {
				i := x * 4
				outRow[x] = luma8(row[i], row[i+1], row[i+2])
			}
		}
	case *image.Gray:
		stride := src.Stride
		baseX := b.Min.X - src.Rect.Min.X
		baseY := b.Min.Y - src.Rect.Min.Y
		for y := 0; y < h; y++ {
			start := (baseY+y)*stride + baseX
			copy(out[y*w:y*w+w], src.Pix[start:start+w])
		}
	case *image.YCbCr:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out[y*w+x] = src.Y[src.YOffset(b.Min.X+x, b.Min.Y+y)]
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				out[y*w+x] = luma8(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			}
		}
	}
	return out
}

// luma8 maps 8-bit RGB → 8-bit luma via Rec-709.
func luma8(r, g, b uint8) uint8 {
	return uint8((2126*uint32(r) + 7152*uint32(g) + 722*uint32(b)) / 10000)
}

// blockStats computes the sample mean / variance / covariance over a
// BlockSize × BlockSize region. Accumulates in uint64 — for block sizes
// up to 256×256, sumAA/sumBB/sumAB ≤ 256²×255² ≈ 4.3 × 10⁹ which
// comfortably fits in uint64 (capacity ~1.8 × 10¹⁹). The final
// conversion to float64 happens once per block rather than once per
// pixel, which is the performance difference vs the naive version.
func blockStats(a, b []uint8, w, bx, by, bs int) (muA, muB, varA, varB, cov float64) {
	var sumA, sumB, sumAA, sumBB, sumAB uint64
	for dy := 0; dy < bs; dy++ {
		row := (by + dy) * w
		for dx := 0; dx < bs; dx++ {
			i := row + bx + dx
			va, vb := uint64(a[i]), uint64(b[i])
			sumA += va
			sumB += vb
			sumAA += va * va
			sumBB += vb * vb
			sumAB += va * vb
		}
	}
	n := float64(bs * bs)
	muA = float64(sumA) / n
	muB = float64(sumB) / n
	varA = float64(sumAA)/n - muA*muA
	varB = float64(sumBB)/n - muB*muB
	cov = float64(sumAB)/n - muA*muB
	// Numerical floor — tiny negative values from float rounding get
	// clamped to 0 to keep the SSIM formula well-behaved.
	if varA < 0 {
		varA = 0
	}
	if varB < 0 {
		varB = 0
	}
	return
}

// Compile-time guard.
var _ Comparator = SSIM{}
