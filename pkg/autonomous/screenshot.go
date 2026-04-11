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

	// Sample pixels across a dense 9x9 grid (81 points) so we catch
	// UI widgets even on dark-themed login screens where most of the
	// background is near-black. Reference-to-first sampling gave false
	// "uniform" verdicts when the reference pixel and most samples hit
	// the background; we now compute the per-channel range (max - min)
	// across all samples, which is robust to uniform backgrounds with
	// small bright UI elements.
	const gridN = 9
	var rMin, gMin, bMin uint32 = 255, 255, 255
	var rMax, gMax, bMax uint32
	for iy := 1; iy <= gridN; iy++ {
		for ix := 1; ix <= gridN; ix++ {
			x := bounds.Min.X + width*ix/(gridN+1)
			y := bounds.Min.Y + height*iy/(gridN+1)
			r, g, b, _ := img.At(x, y).RGBA()
			r, g, b = r>>8, g>>8, b>>8
			if r < rMin {
				rMin = r
			}
			if r > rMax {
				rMax = r
			}
			if g < gMin {
				gMin = g
			}
			if g > gMax {
				gMax = g
			}
			if b < bMin {
				bMin = b
			}
			if b > bMax {
				bMax = b
			}
		}
	}

	// If any single channel shows >= 20 levels of variation across
	// the grid, there is real content on screen. The 20-level
	// threshold catches faint widgets (input borders, subtle
	// gradients) while still rejecting truly blank frames.
	maxRange := rMax - rMin
	if gMax-gMin > maxRange {
		maxRange = gMax - gMin
	}
	if bMax-bMin > maxRange {
		maxRange = bMax - bMin
	}
	return maxRange < 20
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
