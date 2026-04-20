// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"errors"
	"image"
	"image/color"
	"math/bits"
	"testing"
)

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestBlockMean_IdenticalImagesReturnZeroDistance(t *testing.T) {
	h := BlockMeanHasher{}
	img := gradientRGBA(128, 96)
	a, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, _ := h.Hash(img)
	if d := h.Distance(a, b); d != 0 {
		t.Fatalf("identical = %d, want 0", d)
	}
}

func TestBlockMean_HalfSplit_ProducesConsistentHash(t *testing.T) {
	// Left half black, right half white. The BlockMean hash should
	// produce a deterministic result with bits set only for tiles
	// on the white side.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x >= 32 {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	h := BlockMeanHasher{}
	got, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	// Exactly half the 8×8 tiles should be "above median" (the
	// white side). 32 bits set in a 64-bit hash.
	if bits.OnesCount64(got) != 32 {
		t.Fatalf("half-split popcount = %d, want 32", bits.OnesCount64(got))
	}
}

func TestBlockMean_DifferentImagesReturnNonZeroDistance(t *testing.T) {
	h := BlockMeanHasher{}
	a, _ := h.Hash(gradientRGBA(128, 96))
	b, _ := h.Hash(randomRGBA(128, 96, 0xC0FFEE))
	if h.Distance(a, b) == 0 {
		t.Fatal("different images should produce non-zero distance")
	}
}

func TestBlockMean_ShiftedImagesStaySimilar(t *testing.T) {
	h := BlockMeanHasher{}
	src := gradientRGBA(256, 192)
	shifted := shiftRGBA(src, 1, 0)
	a, _ := h.Hash(src)
	b, _ := h.Hash(shifted)
	// 1-pixel shift on a 256-wide source → ~1/32nd of a tile move
	// → at most a few bits flip near tile boundaries.
	if d := h.Distance(a, b); d > 8 {
		t.Fatalf("1-pixel shift distance = %d, want ≤ 8", d)
	}
}

// ---------------------------------------------------------------------------
// ChangedTiles — partial-screen change detection
// ---------------------------------------------------------------------------

func TestChangedTiles_EmptyXOR(t *testing.T) {
	h := BlockMeanHasher{}
	if got := h.ChangedTiles(0xABCD, 0xABCD); len(got) != 0 {
		t.Fatalf("identical hashes should produce 0 changes, got %v", got)
	}
}

func TestChangedTiles_SingleBitFlip(t *testing.T) {
	h := BlockMeanHasher{}
	// Bit at MSB position (0th tile = top-left).
	a := uint64(0)
	b := uint64(1 << 63)
	tiles := h.ChangedTiles(a, b)
	if len(tiles) != 1 {
		t.Fatalf("single bit → %d tiles, want 1", len(tiles))
	}
	if tiles[0] != (TileChange{Col: 0, Row: 0}) {
		t.Fatalf("expected top-left, got %+v", tiles[0])
	}
}

func TestChangedTiles_BottomRightBit(t *testing.T) {
	h := BlockMeanHasher{}
	// Bit 0 (LSB) = 64th tile = bottom-right in 8×8 grid.
	a := uint64(0)
	b := uint64(1)
	tiles := h.ChangedTiles(a, b)
	if len(tiles) != 1 {
		t.Fatalf("single bit → %d tiles, want 1", len(tiles))
	}
	if tiles[0] != (TileChange{Col: 7, Row: 7}) {
		t.Fatalf("expected bottom-right, got %+v", tiles[0])
	}
}

func TestChangedTiles_AllBitsFlipped(t *testing.T) {
	h := BlockMeanHasher{}
	tiles := h.ChangedTiles(0, ^uint64(0))
	if len(tiles) != 64 {
		t.Fatalf("all-bits XOR → %d tiles, want 64", len(tiles))
	}
	// Verify exhaustive coverage: every (col, row) in [0, 8) appears.
	seen := make(map[TileChange]bool, 64)
	for _, t := range tiles {
		seen[t] = true
	}
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if !seen[TileChange{Col: c, Row: r}] {
				t.Errorf("missing tile (%d, %d)", c, r)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestBlockMean_NilImageError(t *testing.T) {
	if _, err := (BlockMeanHasher{}).Hash(nil); err != ErrNilImage {
		t.Fatalf("nil = %v, want ErrNilImage", err)
	}
}

func TestBlockMean_ZeroBoundsError(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if _, err := (BlockMeanHasher{}).Hash(empty); err != ErrZeroBounds {
		t.Fatalf("empty = %v, want ErrZeroBounds", err)
	}
}

func TestBlockMean_WrongBlockSizeError(t *testing.T) {
	for _, bs := range []int{4, 16, 32, -1} {
		h := BlockMeanHasher{BlockSize: bs}
		if _, err := h.Hash(gradientRGBA(64, 64)); !errors.Is(err, ErrWrongBlockSize) {
			t.Errorf("BlockSize=%d: %v, want ErrWrongBlockSize", bs, err)
		}
	}
}

func TestBlockMean_DefaultBlockSizeIsEight(t *testing.T) {
	h := BlockMeanHasher{}
	if _, err := h.Hash(gradientRGBA(64, 64)); err != nil {
		t.Fatalf("default BlockSize should succeed: %v", err)
	}
}

func TestBlockMean_ExplicitEightBlockSizeWorks(t *testing.T) {
	h := BlockMeanHasher{BlockSize: 8}
	if _, err := h.Hash(gradientRGBA(64, 64)); err != nil {
		t.Fatalf("BlockSize=8: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestMedianOf_OddLength(t *testing.T) {
	if got := medianOf([]float64{3, 1, 2}); got != 2 {
		t.Fatalf("median of [1,2,3] = %v, want 2", got)
	}
}

func TestMedianOf_EvenLength(t *testing.T) {
	if got := medianOf([]float64{4, 2, 1, 3}); got != 2.5 {
		t.Fatalf("median of [1,2,3,4] = %v, want 2.5", got)
	}
}

func TestMedianOf_DoesNotMutateInput(t *testing.T) {
	input := []float64{3, 1, 2}
	_ = medianOf(input)
	if input[0] != 3 || input[1] != 1 || input[2] != 2 {
		t.Fatalf("medianOf mutated input: %v", input)
	}
}

func TestBlockMeans_ZeroBoundsError(t *testing.T) {
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if _, err := blockMeans(empty, 8); err != ErrZeroBounds {
		t.Fatalf("empty = %v, want ErrZeroBounds", err)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestBlockMean_SatisfiesHasherInterface(t *testing.T) {
	var h Hasher = BlockMeanHasher{}
	img := gradientRGBA(64, 64)
	a, err := h.Hash(img)
	if err != nil {
		t.Fatalf("Hash via interface: %v", err)
	}
	if h.Distance(a, a) != 0 {
		t.Fatal("Distance(self, self) != 0")
	}
}

// ---------------------------------------------------------------------------
// errorString string-method sanity
// ---------------------------------------------------------------------------

func TestErrWrongBlockSize_HasErrorMessage(t *testing.T) {
	if msg := ErrWrongBlockSize.Error(); msg == "" {
		t.Fatal("ErrWrongBlockSize.Error() empty")
	}
}
