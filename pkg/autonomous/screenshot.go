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

// maxScreenshotWidth is the maximum width (in pixels) for
// screenshots sent to the LLM vision API. Larger images
// are downscaled proportionally using nearest-neighbour
// sampling. 720px keeps file size under ~200KB while
// retaining enough detail for UI analysis.
const maxScreenshotWidth = 720

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
