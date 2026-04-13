// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage creates a deterministic RGBA image of size w×h where each
// pixel's color components are derived from its coordinates and the given seed.
func newTestImage(w, h int, seed byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: byte(x) ^ seed,
				G: byte(y) ^ seed,
				B: byte(x+y) ^ seed,
				A: 255,
			})
		}
	}
	return img
}

// TestComputeImageHash_Deterministic verifies that hashing the same image
// twice always produces the same result.
func TestComputeImageHash_Deterministic(t *testing.T) {
	img := newTestImage(64, 64, 0x42)

	hash1 := ComputeImageHash(img)
	hash2 := ComputeImageHash(img)

	require.NotEmpty(t, hash1)
	assert.Equal(t, hash1, hash2, "same image must produce the same hash on every call")
}

// TestComputeImageHash_DifferentImages verifies that images with different
// pixel content produce different hashes.
func TestComputeImageHash_DifferentImages(t *testing.T) {
	imgA := newTestImage(64, 64, 0x01)
	imgB := newTestImage(64, 64, 0x02)

	hashA := ComputeImageHash(imgA)
	hashB := ComputeImageHash(imgB)

	require.NotEmpty(t, hashA)
	require.NotEmpty(t, hashB)
	assert.NotEqual(t, hashA, hashB, "images with different pixels must produce different hashes")
}

// TestComputeImageHash_SinglePixel verifies correct operation on the smallest
// possible image (1×1) and checks that the returned string is a valid 64-char
// lowercase hex SHA-256 digest.
func TestComputeImageHash_SinglePixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 128, A: 255})

	hash := ComputeImageHash(img)

	require.NotEmpty(t, hash)
	assert.Len(t, hash, 64, "SHA-256 hex digest must be exactly 64 characters")

	// Hashing an identical single-pixel image must yield the same result.
	img2 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img2.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 128, A: 255})
	assert.Equal(t, hash, ComputeImageHash(img2))

	// A differently-coloured single pixel must differ.
	img3 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img3.SetRGBA(0, 0, color.RGBA{R: 0, G: 255, B: 128, A: 255})
	assert.NotEqual(t, hash, ComputeImageHash(img3))
}
