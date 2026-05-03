// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisualAssertionCompareIdentical(t *testing.T) {
	va := NewVisualAssertion()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 64, B: 32, A: 255})
		}
	}

	score, err := va.Compare(img, img)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, score, 0.001)
}

func TestVisualAssertionCompareDifferent(t *testing.T) {
	va := NewVisualAssertion()

	img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img1.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			img2.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}

	score, err := va.Compare(img1, img2)
	require.NoError(t, err)
	assert.Less(t, score, 0.5)
}

func TestVisualAssertionAssertSimilar(t *testing.T) {
	va := NewVisualAssertion()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}

	// Identical images should pass at any threshold.
	require.NoError(t, va.AssertSimilar(img, img, 0.99))
}

func TestVisualAssertionDiffImage(t *testing.T) {
	va := NewVisualAssertion()

	img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img2 := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img1.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			img2.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	// Make one pixel different.
	img2.Set(5, 5, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	diff := va.DiffImage(img1, img2)
	require.NotNil(t, diff)

	// The differing pixel should be red in the diff image.
	c := diff.At(5, 5)
	r, g, b, _ := c.RGBA()
	assert.Equal(t, uint32(65535), r)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
}
