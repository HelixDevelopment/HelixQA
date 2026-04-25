// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"image"
	"math/bits"
	"sort"
)

// BlockMeanHasher implements the BlockMean perceptual hash (Yang 2006,
// "Block Mean Value Based Image Perceptual Hashing"). The image is
// divided into a grid of BlockSize × BlockSize tiles; each tile's
// mean luminance is compared to the median of all tile means; the
// result is a BlockSize² bit hash.
//
// Why HelixQA carries BlockMean alongside dHash and pHash:
//
//   - dHash captures row-gradient structure. Good for detecting
//     any change; blind to where the change happened.
//   - pHash captures global frequency content. Shift/rotation-
//     robust but expensive.
//   - BlockMean preserves spatial layout — each tile owns a
//     specific region of the screen. XOR of two BlockMeans tells
//     you WHICH tile changed, making it ideal for partial-screen
//     change detection (e.g. "the lower-right tile flipped — a
//     toast notification appeared").
//
// BlockSize defaults to 8 → 64-bit hash compatible with the
// Hasher interface.
type BlockMeanHasher struct {
	// BlockSize is the grid resolution. Zero → 8 (64-bit output).
	// Values > 8 produce hashes wider than 64 bits which the
	// Hasher interface can't carry — use ChangedTiles instead for
	// wider grids.
	BlockSize int
}

// Hash computes a 64-bit BlockMean hash. Requires BlockSize = 0
// (default 8) or 8 — any other value exceeds the 64-bit Hasher
// contract and returns ErrWrongBlockSize.
func (h BlockMeanHasher) Hash(img image.Image) (uint64, error) {
	if img == nil {
		return 0, ErrNilImage
	}
	bs := h.BlockSize
	if bs == 0 {
		bs = 8
	}
	if bs != 8 {
		return 0, ErrWrongBlockSize
	}

	means, err := blockMeans(img, bs)
	if err != nil {
		return 0, err
	}

	median := medianOf(means)
	var out uint64
	for i, m := range means {
		if m > median {
			out |= uint64(1) << uint(63-i)
		}
	}
	return out, nil
}

// Distance returns the Hamming distance between two 64-bit
// BlockMean hashes.
func (BlockMeanHasher) Distance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// ChangedTiles returns the 1-indexed tile coordinates ((col, row))
// whose hash bit differs between a and b. Useful for partial-screen
// change detection — given the XOR of two BlockMeans, it tells the
// caller exactly which cells of the grid flipped. Zero-allocation
// for up-to-8×8 grids.
//
// The returned TileChange records use (Col, Row) in [0, BlockSize)
// with (0, 0) = top-left.
func (h BlockMeanHasher) ChangedTiles(a, b uint64) []TileChange {
	bs := h.BlockSize
	if bs == 0 {
		bs = 8
	}
	diff := a ^ b
	var out []TileChange
	for i := 0; i < bs*bs; i++ {
		bit := uint64(1) << uint(63-i)
		if diff&bit != 0 {
			out = append(out, TileChange{
				Col: i % bs,
				Row: i / bs,
			})
		}
	}
	return out
}

// TileChange is one tile flipped between two BlockMean hashes.
type TileChange struct {
	Col int
	Row int
}

// ErrWrongBlockSize is returned when Hash is called on a
// BlockMeanHasher whose BlockSize can't fit in a uint64 (i.e. any
// value other than 0 or 8). Call ChangedTiles directly if you need
// wider grids — the TileChange output is unbounded.
var ErrWrongBlockSize = errorString("helixqa/vision/hash: BlockMean requires BlockSize = 8 (64-bit Hasher contract)")

// errorString is a sentinel-error helper so this file doesn't pull
// in the errors package for a single value; `_ = fmt.Errorf` is
// already referenced across the package.
type errorString string

func (e errorString) Error() string { return string(e) }

// blockMeans computes the mean luma of each BlockSize × BlockSize
// tile. Returns a flat slice of size BlockSize² in row-major order.
func blockMeans(img image.Image, bs int) ([]float64, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, ErrZeroBounds
	}
	means := make([]float64, bs*bs)

	// Reuse the resize kernel at grid resolution — the box-average
	// logic of resizeGray when called with w, h < BlockSize would
	// produce the tile-mean directly. But resizeGray does nearest-
	// neighbor now, so we compute tile means explicitly here.
	for ty := 0; ty < bs; ty++ {
		y0 := b.Min.Y + ty*h/bs
		y1 := b.Min.Y + (ty+1)*h/bs
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for tx := 0; tx < bs; tx++ {
			x0 := b.Min.X + tx*w/bs
			x1 := b.Min.X + (tx+1)*w/bs
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum, count float64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					r, g, bl, _ := img.At(xx, yy).RGBA()
					l := lumaRec709(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
					sum += float64(l)
					count++
				}
			}
			means[ty*bs+tx] = sum / count
		}
	}
	return means, nil
}

// medianOf returns the median of a float64 slice. Allocates a
// single sorted copy; the caller's slice is not mutated.
func medianOf(values []float64) float64 {
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

// Compile-time guard: BlockMeanHasher satisfies the package-wide
// Hasher interface (for 64-bit hashes).
var _ Hasher = BlockMeanHasher{}
