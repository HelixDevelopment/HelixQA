// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"errors"
	"image"
	"math/bits"
)

// Kind selects between the 64-bit and 256-bit dHash variants.
type Kind int

const (
	// DHash64 is an 8×8 difference hash packed into one uint64. Sub-millisecond
	// per 1080p frame and plenty of discrimination for "did the screen change?"
	// Phase-2 tier-1 decisions.
	DHash64 Kind = iota
	// DHash256 is a 16×16 difference hash packed into four uint64 words. Kept
	// for the rare cases where tier-1 DHash64 gives ambiguous answers (~Hamming
	// ≤ 5 on near-duplicates); tier-2 SSIM typically takes over first.
	DHash256
)

// BigHash is a 256-bit perceptual hash stored as four 64-bit words, most
// significant word first. Emitted by DHasher{Kind: DHash256}.Hash256.
type BigHash [4]uint64

// Distance returns the Hamming distance between two BigHashes.
func (a BigHash) Distance(b BigHash) int {
	d := 0
	for i := 0; i < 4; i++ {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}

// Hasher is the Phase-2 tier-1 interface as advertised in doc.go. Implemented
// by DHasher; future phash/wHash/BlockMean wrappers land in sibling files.
type Hasher interface {
	Hash(img image.Image) (uint64, error)
	Distance(a, b uint64) int
}

// DHasher wraps the dHash algorithm for both the 64-bit and 256-bit variants.
// Safe to share across goroutines — no internal state.
type DHasher struct {
	Kind Kind
}

// Sentinel errors.
var (
	ErrNilImage      = errors.New("helixqa/vision/hash: nil image")
	ErrZeroBounds    = errors.New("helixqa/vision/hash: image has zero bounds")
	ErrWrongKind64   = errors.New("helixqa/vision/hash: Hash called on DHasher{Kind: DHash256} — use Hash256")
	ErrWrongKind256  = errors.New("helixqa/vision/hash: Hash256 called on DHasher{Kind: DHash64} — use Hash")
)

// Hash returns a 64-bit dHash. Requires Kind == DHash64.
func (h DHasher) Hash(img image.Image) (uint64, error) {
	if h.Kind != DHash64 {
		return 0, ErrWrongKind64
	}
	if img == nil {
		return 0, ErrNilImage
	}
	px, err := resizeGray(img, 9, 8)
	if err != nil {
		return 0, err
	}
	var out uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			out <<= 1
			if px[y*9+x] > px[y*9+x+1] {
				out |= 1
			}
		}
	}
	return out, nil
}

// Hash256 returns a 256-bit dHash. Requires Kind == DHash256.
func (h DHasher) Hash256(img image.Image) (*BigHash, error) {
	if h.Kind != DHash256 {
		return nil, ErrWrongKind256
	}
	if img == nil {
		return nil, ErrNilImage
	}
	px, err := resizeGray(img, 17, 16)
	if err != nil {
		return nil, err
	}
	var out BigHash
	bit := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			word := bit / 64
			shift := uint(63 - (bit % 64))
			if px[y*17+x] > px[y*17+x+1] {
				out[word] |= uint64(1) << shift
			}
			bit++
		}
	}
	return &out, nil
}

// Distance returns the Hamming distance between two 64-bit dHashes.
func (h DHasher) Distance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// resizeGray downsamples img to w×h in Rec-709 luma. The output buffer is
// row-major, one byte per pixel.
//
// For dHash the 8×8 / 16×16 output grid means each output cell represents a
// region of thousands of source pixels, so nearest-neighbor at the region
// center is sufficient — a 1-pixel source shift moves far less than one
// output cell, preserving the Hamming-distance robustness dHash promises.
// Fast paths for the common concrete image types (RGBA, NRGBA, Gray, YCbCr)
// avoid interface dispatch + 16-bit channel expansion and keep 1080p hashing
// below 1 ms on commodity CPU.
func resizeGray(img image.Image, w, h int) ([]uint8, error) {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return nil, ErrZeroBounds
	}
	out := make([]uint8, w*h)

	switch src := img.(type) {
	case *image.RGBA:
		for ty := 0; ty < h; ty++ {
			sy := b.Min.Y + (ty*sh+sh/2)/h
			for tx := 0; tx < w; tx++ {
				sx := b.Min.X + (tx*sw+sw/2)/w
				i := src.PixOffset(sx, sy)
				r, g, bl := src.Pix[i], src.Pix[i+1], src.Pix[i+2]
				out[ty*w+tx] = lumaRec709(r, g, bl)
			}
		}
	case *image.NRGBA:
		for ty := 0; ty < h; ty++ {
			sy := b.Min.Y + (ty*sh+sh/2)/h
			for tx := 0; tx < w; tx++ {
				sx := b.Min.X + (tx*sw+sw/2)/w
				i := src.PixOffset(sx, sy)
				r, g, bl := src.Pix[i], src.Pix[i+1], src.Pix[i+2]
				out[ty*w+tx] = lumaRec709(r, g, bl)
			}
		}
	case *image.Gray:
		for ty := 0; ty < h; ty++ {
			sy := b.Min.Y + (ty*sh+sh/2)/h
			for tx := 0; tx < w; tx++ {
				sx := b.Min.X + (tx*sw+sw/2)/w
				out[ty*w+tx] = src.Pix[src.PixOffset(sx, sy)]
			}
		}
	case *image.YCbCr:
		for ty := 0; ty < h; ty++ {
			sy := b.Min.Y + (ty*sh+sh/2)/h
			for tx := 0; tx < w; tx++ {
				sx := b.Min.X + (tx*sw+sw/2)/w
				out[ty*w+tx] = src.Y[src.YOffset(sx, sy)]
			}
		}
	default:
		for ty := 0; ty < h; ty++ {
			sy := b.Min.Y + (ty*sh+sh/2)/h
			for tx := 0; tx < w; tx++ {
				sx := b.Min.X + (tx*sw+sw/2)/w
				r, g, bl, _ := img.At(sx, sy).RGBA()
				// RGBA() returns 16-bit channels; collapse to 8 bits first.
				out[ty*w+tx] = lumaRec709(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			}
		}
	}
	return out, nil
}

// lumaRec709 converts 8-bit RGB to 8-bit luminance using Rec-709 coefficients.
// Integer-only; no CGO.
func lumaRec709(r, g, b uint8) uint8 {
	return uint8((2126*uint32(r) + 7152*uint32(g) + 722*uint32(b)) / 10000)
}

// Compile-time guard: DHasher{Kind: DHash64} satisfies the Hasher interface.
// DHasher with Kind: DHash256 intentionally does NOT satisfy Hasher — clients
// that need 256-bit hashes use Hash256 + BigHash.Distance directly.
var _ Hasher = DHasher{Kind: DHash64}
