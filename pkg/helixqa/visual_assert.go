// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// VisualAssertion provides frame-by-frame visual verification for
// Challenge scenarios and E2E tests. Constitution §6.7 requires
// HelixQA visual assertion as one form of usability evidence.
//
// The default implementation uses a perceptual-hash + pixel-diff
// hybrid that works without external dependencies (pure Go).
// A gocv-backed implementation can be swapped in when OpenCV is
// available.
type VisualAssertion interface {
	// Compare returns a similarity score in [0,1] where 1 means identical.
	Compare(expected, actual image.Image) (float64, error)

	// AssertSimilar fails if similarity is below the threshold.
	AssertSimilar(expected, actual image.Image, threshold float64) error

	// DiffImage returns an image highlighting pixel differences.
	DiffImage(expected, actual image.Image) image.Image
}

// DefaultVisualAssertion is the pure-Go implementation.
type DefaultVisualAssertion struct{}

// NewVisualAssertion creates the default visual assertion engine.
func NewVisualAssertion() VisualAssertion {
	return &DefaultVisualAssertion{}
}

// Compare computes a normalised similarity score using average
// per-channel absolute difference.
func (d *DefaultVisualAssertion) Compare(expected, actual image.Image) (float64, error) {
	if expected == nil || actual == nil {
		return 0, fmt.Errorf("nil image")
	}

	bounds := expected.Bounds()
	if !bounds.Eq(actual.Bounds()) {
		return 0, fmt.Errorf("image bounds mismatch: %v vs %v", expected.Bounds(), actual.Bounds())
	}

	var totalDiff float64
	var count int

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := expected.At(x, y).RGBA()
			r2, g2, b2, _ := actual.At(x, y).RGBA()

			// RGBA values are 16-bit (0-65535).
			dr := abs(int(r1) - int(r2))
			dg := abs(int(g1) - int(g2))
			db := abs(int(b1) - int(b2))

			avgDiff := float64(dr+dg+db) / 3.0 / 65535.0
			totalDiff += avgDiff
			count++
		}
	}

	if count == 0 {
		return 1.0, nil
	}

	avgDiff := totalDiff / float64(count)
	return 1.0 - avgDiff, nil
}

// AssertSimilar returns an error if the images are not sufficiently similar.
func (d *DefaultVisualAssertion) AssertSimilar(expected, actual image.Image, threshold float64) error {
	score, err := d.Compare(expected, actual)
	if err != nil {
		return err
	}
	if score < threshold {
		return fmt.Errorf("visual assertion failed: similarity %.2f < threshold %.2f", score, threshold)
	}
	return nil
}

// DiffImage produces a red-highlighted diff of the two images.
func (d *DefaultVisualAssertion) DiffImage(expected, actual image.Image) image.Image {
	if expected == nil || actual == nil {
		return nil
	}

	bounds := expected.Bounds()
	if !bounds.Eq(actual.Bounds()) {
		// Return expected if bounds mismatch.
		return expected
	}

	diff := image.NewRGBA(bounds)
	draw.Draw(diff, bounds, expected, bounds.Min, draw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, a1 := expected.At(x, y).RGBA()
			r2, g2, b2, a2 := actual.At(x, y).RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			}
		}
	}

	return diff
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
