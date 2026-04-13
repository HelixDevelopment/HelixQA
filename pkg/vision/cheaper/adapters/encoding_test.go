// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage creates an RGBA image of the given dimensions filled with a
// colour gradient so that the encoded bytes are non-trivial.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 255 / (w - 1 + 1)),
				G: uint8(y * 255 / (h - 1 + 1)),
				B: uint8((x + y) * 255 / (w + h)),
				A: 255,
			})
		}
	}
	return img
}

func TestImageToBase64_ValidImage(t *testing.T) {
	img := newTestImage(10, 10)

	result, err := ImageToBase64(img)

	require.NoError(t, err)
	assert.NotEmpty(t, result, "base64 result should not be empty")
	// PNG files always start with the 8-byte PNG signature; in base64 the
	// first bytes of that signature encode to "iVBOR".
	assert.True(t, len(result) >= 5, "result should be long enough to contain PNG header prefix")
	assert.Equal(t, "iVBOR", result[:5], "PNG base64 should start with iVBOR")
}

func TestImageToBase64_SinglePixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	result, err := ImageToBase64(img)

	require.NoError(t, err)
	assert.NotEmpty(t, result, "single-pixel image should produce non-empty base64")
	assert.Equal(t, "iVBOR", result[:5], "single-pixel PNG base64 should start with iVBOR")
}
