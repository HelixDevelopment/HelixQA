// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package memory provides image hashing utilities for the HelixQA L1 exact
// image cache. It produces deterministic, content-addressable hashes over raw
// pixel data so that visually identical screenshots always map to the same
// cache key regardless of how the image.Image was obtained.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
)

// ComputeImageHash returns a SHA-256 hex-encoded hash computed over the raw
// RGBA pixel bytes of img. The iteration order is row-major (y outer, x inner)
// over the full image bounds. Each pixel contributes four bytes: R, G, B, A
// (each pre-multiplied alpha component in the range [0, 255]).
//
// The function never returns an error because all operations are purely
// in-memory; hashing in-memory data cannot fail.
func ComputeImageHash(img image.Image) string {
	bounds := img.Bounds()
	h := sha256.New()

	buf := make([]byte, 4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// RGBA() returns values in [0, 65535]; shift right by 8 to get [0, 255].
			buf[0] = byte(r >> 8)
			buf[1] = byte(g >> 8)
			buf[2] = byte(b >> 8)
			buf[3] = byte(a >> 8)
			_, _ = h.Write(buf)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}
