// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	// Register JPEG decoder so image.Decode can handle
	// JPEG screenshots returned by some ADB versions.
	_ "image/jpeg"
)

// IsBlankScreenshot checks if a screenshot is blank/uniform color.
// It returns true if the image is all white, all black, or uniform color.
func IsBlankScreenshot(data []byte) bool {
	if len(data) < 1000 {
		// Too small to contain meaningful content
		return true
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Can't decode, assume it's blank to be safe
		return true
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width < 10 || height < 10 {
		return true
	}

	// Sample pixels at different positions
	samplePoints := []struct{ x, y int }{
		{width / 4, height / 4},
		{width / 2, height / 4},
		{3 * width / 4, height / 4},
		{width / 4, height / 2},
		{width / 2, height / 2},
		{3 * width / 4, height / 2},
		{width / 4, 3 * height / 4},
		{width / 2, 3 * height / 4},
		{3 * width / 4, 3 * height / 4},
	}

	// Get color of first sample point
	r0, g0, b0, _ := img.At(bounds.Min.X+samplePoints[0].x, bounds.Min.Y+samplePoints[0].y).RGBA()
	// Convert to 8-bit
	r0, g0, b0 = r0>>8, g0>>8, b0>>8

	var totalDiff uint32
	for i, pt := range samplePoints {
		if i == 0 {
			continue
		}
		r, g, b, _ := img.At(bounds.Min.X+pt.x, bounds.Min.Y+pt.y).RGBA()
		r, g, b = r>>8, g>>8, b>>8
		diff := absDiff(r, r0) + absDiff(g, g0) + absDiff(b, b0)
		totalDiff += uint32(diff)
	}

	// If average difference across samples is less than threshold,
	// image is likely blank/uniform
	avgDiff := totalDiff / uint32(len(samplePoints)-1)
	// Threshold: average difference less than 10 per channel = uniform
	return avgDiff < 30
}

// absDiff returns absolute difference
func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// maxScreenshotWidth is the maximum width (in pixels) for
// screenshots sent to the LLM vision API. Larger images
// are downscaled proportionally using nearest-neighbour
// sampling. 480px keeps file size under ~50KB for fast
// CPU-based inference while retaining UI readability.
const maxScreenshotWidth = 480

// resizeScreenshot downscales a PNG image to at most
// maxScreenshotWidth pixels wide, preserving aspect ratio.
// If the image is already small enough or cannot be
// decoded, the original bytes are returned unchanged.
func resizeScreenshot(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	if origW <= maxScreenshotWidth {
		return data
	}

	// Compute new dimensions preserving aspect ratio.
	newW := maxScreenshotWidth
	newH := origH * newW / origW

	// Nearest-neighbour downscale — fast and sufficient
	// for LLM vision which does not need anti-aliasing.
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * origH / newH
		for x := 0; x < newW; x++ {
			srcX := x * origW / newW
			dst.Set(x, y, img.At(
				bounds.Min.X+srcX,
				bounds.Min.Y+srcY,
			))
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return data
	}

	fmt.Printf(
		"    [resize] %dx%d -> %dx%d (%dKB -> %dKB)\n",
		origW, origH, newW, newH,
		len(data)/1024, buf.Len()/1024,
	)
	return buf.Bytes()
}
