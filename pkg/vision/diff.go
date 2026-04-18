// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package vision provides screenshot diffing capabilities
// for the HelixQA autonomous pipeline. The primary use case
// is detecting whether a UI action had any visible effect
// by comparing the current screenshot with the previous one.
// When screens are nearly identical the pipeline can inform
// the LLM that the previous action had no effect, saving
// unnecessary LLM calls and improving exploration quality.
package vision

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"sync"
)

// defaultSimilarityThreshold is the fraction of matching
// pixel samples above which two screenshots are considered
// the same screen. At 0.99 (99%) only screens that are
// virtually identical are flagged.
const defaultSimilarityThreshold = 0.99

// defaultSampleCount is the number of pixel positions to
// sample when comparing images. Sampling avoids decoding
// the entire image and keeps comparison fast.
const defaultSampleCount = 500

// defaultChannelTolerance is the per-channel (8-bit) color
// difference allowed when comparing two sampled pixels.
// Small differences from JPEG compression or minor
// rendering variations are expected.
const defaultChannelTolerance = 12

// ScreenDiffer compares screenshots to detect if a UI
// action had any visible effect. It stores the previous
// screenshot and compares it against each new one using
// fast pixel sampling.
//
// Thread-safe: all state is protected by a mutex.
type ScreenDiffer struct {
	mu              sync.Mutex
	previousImage   []byte
	previousDecoded image.Image
	threshold       float64
	sampleCount     int
	tolerance       int
	sameCount       int
	diffCount       int
}

// NewScreenDiffer creates a ScreenDiffer with the given
// similarity threshold. If threshold is outside (0, 1] it
// defaults to 0.99.
func NewScreenDiffer(threshold float64) *ScreenDiffer {
	if threshold <= 0 || threshold > 1.0 {
		threshold = defaultSimilarityThreshold
	}
	return &ScreenDiffer{
		threshold:   threshold,
		sampleCount: defaultSampleCount,
		tolerance:   defaultChannelTolerance,
	}
}

// SetSampleCount overrides the number of pixel positions
// sampled during comparison. Higher values increase
// accuracy but take longer.
func (sd *ScreenDiffer) SetSampleCount(n int) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if n > 0 {
		sd.sampleCount = n
	}
}

// SetTolerance overrides the per-channel pixel tolerance.
func (sd *ScreenDiffer) SetTolerance(t int) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if t >= 0 && t <= 255 {
		sd.tolerance = t
	}
}

// DiffResult describes the outcome of a screen comparison.
type DiffResult struct {
	// IsSame is true when the screens are nearly identical.
	IsSame bool

	// Similarity is the fraction of matching samples (0-1).
	Similarity float64

	// SizeDiffers is true when the raw byte lengths differ
	// by more than 10%.
	SizeDiffers bool
}

// IsSameScreen returns true if the new image is nearly
// identical to the previous one. On the first call (no
// previous image stored) it returns false and stores the
// image for future comparison.
//
// The comparison uses a multi-stage approach:
//  1. Fast byte-length check: if sizes differ by >10%
//     the screens are different (PNG compression varies
//     with content).
//  2. PNG decoding: both images are decoded.
//  3. Dimension check: different dimensions means different
//     screens.
//  4. Pixel sampling: N random-but-deterministic pixel
//     positions are compared with channel tolerance.
//  5. If >threshold fraction matches, screens are the same.
//
// After comparison the new image replaces the stored one.
func (sd *ScreenDiffer) IsSameScreen(
	newImage []byte,
) bool {
	result := sd.Compare(newImage)
	return result.IsSame
}

// Compare performs the full comparison and returns detailed
// results. It also updates the stored image.
func (sd *ScreenDiffer) Compare(
	newImage []byte,
) DiffResult {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	result := DiffResult{}

	if sd.previousImage == nil {
		sd.previousImage = copyBytes(newImage)
		sd.previousDecoded = nil // force re-decode
		return result
	}

	if len(newImage) == 0 {
		sd.previousImage = nil
		sd.previousDecoded = nil
		return result
	}

	// Stage 1: Fast byte-length comparison.
	prevLen := len(sd.previousImage)
	newLen := len(newImage)
	if prevLen > 0 {
		diff := math.Abs(float64(newLen - prevLen))
		if diff > float64(prevLen)/10.0 {
			result.SizeDiffers = true
			sd.previousImage = copyBytes(newImage)
			sd.previousDecoded = nil
			sd.diffCount++
			return result
		}
	}

	// Stage 2: Decode images.
	prevImg := sd.previousDecoded
	if prevImg == nil {
		var err error
		prevImg, err = decodePNG(sd.previousImage)
		if err != nil {
			sd.previousImage = copyBytes(newImage)
			sd.previousDecoded = nil
			sd.diffCount++
			return result
		}
	}

	newImg, err := decodePNG(newImage)
	if err != nil {
		sd.previousImage = copyBytes(newImage)
		sd.previousDecoded = nil
		sd.diffCount++
		return result
	}

	// Stage 3: Dimension check.
	prevBounds := prevImg.Bounds()
	newBounds := newImg.Bounds()
	if prevBounds.Dx() != newBounds.Dx() ||
		prevBounds.Dy() != newBounds.Dy() {
		sd.previousImage = copyBytes(newImage)
		sd.previousDecoded = newImg
		sd.diffCount++
		return result
	}

	// Stage 4: Pixel sampling.
	w := prevBounds.Dx()
	h := prevBounds.Dy()
	totalPixels := w * h

	if totalPixels == 0 {
		result.IsSame = true
		result.Similarity = 1.0
		sd.sameCount++
		return result
	}

	samples := sd.sampleCount
	if samples > totalPixels {
		samples = totalPixels
	}

	matchCount := 0
	for i := 0; i < samples; i++ {
		// Deterministic but distributed sampling using a
		// prime-stride walk through the pixel space.
		idx := (i * 7919) % totalPixels
		x := idx%w + prevBounds.Min.X
		y := idx/w + prevBounds.Min.Y

		rP, gP, bP, _ := prevImg.At(x, y).RGBA()
		rN, gN, bN, _ := newImg.At(x, y).RGBA()

		// RGBA() returns 16-bit values; scale to 8-bit.
		tol := uint32(sd.tolerance) * 257
		if absDiffU32(rP, rN) <= tol &&
			absDiffU32(gP, gN) <= tol &&
			absDiffU32(bP, bN) <= tol {
			matchCount++
		}
	}

	similarity := float64(matchCount) / float64(samples)
	result.Similarity = similarity
	result.IsSame = similarity >= sd.threshold

	// Update stored image.
	sd.previousImage = copyBytes(newImage)
	sd.previousDecoded = newImg

	if result.IsSame {
		sd.sameCount++
	} else {
		sd.diffCount++
	}

	return result
}

// Update stores the current image for future comparison
// without performing a diff. Useful for resetting the
// baseline after a known screen transition.
func (sd *ScreenDiffer) Update(image []byte) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.previousImage = copyBytes(image)
	sd.previousDecoded = nil
}

// Reset clears the stored image and counters.
func (sd *ScreenDiffer) Reset() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.previousImage = nil
	sd.previousDecoded = nil
	sd.sameCount = 0
	sd.diffCount = 0
}

// Stats returns the number of same-screen and different-
// screen detections since creation or last reset.
func (sd *ScreenDiffer) Stats() (same, diff int) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.sameCount, sd.diffCount
}

// Threshold returns the configured similarity threshold.
func (sd *ScreenDiffer) Threshold() float64 {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.threshold
}

// HasPrevious reports whether a previous image is stored.
func (sd *ScreenDiffer) HasPrevious() bool {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.previousImage != nil
}

// decodePNG decodes PNG data into an image.Image.
func decodePNG(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("png decode: %w", err)
	}
	return img, nil
}

// absDiffU32 returns |a - b| for unsigned 32-bit values.
func absDiffU32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// copyBytes returns a copy of the byte slice.
func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
