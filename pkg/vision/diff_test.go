// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package vision

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

// makePNG creates a PNG-encoded image filled with one color.
func makePNG(
	t *testing.T, w, h int, c color.Color,
) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// makePNGWithRect creates a PNG with a background color and
// a colored rectangle drawn at (rx, ry, rw, rh).
func makePNGWithRect(
	t *testing.T,
	w, h int,
	bg, fg color.Color,
	rx, ry, rw, rh int,
) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := ry; y < ry+rh && y < h; y++ {
		for x := rx; x < rx+rw && x < w; x++ {
			img.Set(x, y, fg)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// --- NewScreenDiffer tests ---

func TestNewScreenDiffer_DefaultThreshold(t *testing.T) {
	sd := NewScreenDiffer(0)
	assert.InDelta(
		t, defaultSimilarityThreshold,
		sd.Threshold(), 0.001,
	)
}

func TestNewScreenDiffer_CustomThreshold(t *testing.T) {
	sd := NewScreenDiffer(0.95)
	assert.InDelta(t, 0.95, sd.Threshold(), 0.001)
}

func TestNewScreenDiffer_InvalidThreshold_Negative(
	t *testing.T,
) {
	sd := NewScreenDiffer(-0.5)
	assert.InDelta(
		t, defaultSimilarityThreshold,
		sd.Threshold(), 0.001,
	)
}

func TestNewScreenDiffer_InvalidThreshold_TooHigh(
	t *testing.T,
) {
	sd := NewScreenDiffer(1.5)
	assert.InDelta(
		t, defaultSimilarityThreshold,
		sd.Threshold(), 0.001,
	)
}

func TestNewScreenDiffer_ThresholdExactlyOne(t *testing.T) {
	sd := NewScreenDiffer(1.0)
	assert.InDelta(t, 1.0, sd.Threshold(), 0.001)
}

// --- IsSameScreen tests ---

func TestIsSameScreen_FirstCall_AlwaysFalse(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.White)

	assert.False(t, sd.IsSameScreen(img))
	assert.True(t, sd.HasPrevious())
}

func TestIsSameScreen_IdenticalImages(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 80, 80, color.RGBA{100, 150, 200, 255})

	// First call stores the image.
	sd.IsSameScreen(img)

	// Second call compares identical images.
	assert.True(t, sd.IsSameScreen(img))
}

func TestIsSameScreen_CompletelyDifferent(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	white := makePNG(t, 60, 60, color.White)
	black := makePNG(t, 60, 60, color.Black)

	sd.IsSameScreen(white)
	assert.False(t, sd.IsSameScreen(black))
}

func TestIsSameScreen_MinorDifference(t *testing.T) {
	sd := NewScreenDiffer(0.95)
	// Two very similar colors (within channel tolerance).
	imgA := makePNG(
		t, 100, 100, color.RGBA{128, 128, 128, 255},
	)
	imgB := makePNG(
		t, 100, 100, color.RGBA{130, 130, 130, 255},
	)

	sd.IsSameScreen(imgA)
	assert.True(t, sd.IsSameScreen(imgB))
}

func TestIsSameScreen_SmallRegionChanged(t *testing.T) {
	sd := NewScreenDiffer(0.90)
	sd.SetSampleCount(2000) // more samples for accuracy

	base := makePNG(t, 200, 200, color.White)
	// Change a small 20x20 region (2% of pixels).
	changed := makePNGWithRect(
		t, 200, 200,
		color.White, color.Black,
		90, 90, 20, 20,
	)

	sd.IsSameScreen(base)
	// With only 2% changed, at 0.90 threshold the screens
	// should still be "same" (98% match > 90%).
	assert.True(t, sd.IsSameScreen(changed))
}

func TestIsSameScreen_LargeRegionChanged(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetSampleCount(1000)

	base := makePNG(t, 100, 100, color.White)
	// Change half the screen.
	changed := makePNGWithRect(
		t, 100, 100,
		color.White, color.Black,
		0, 0, 50, 100,
	)

	sd.IsSameScreen(base)
	assert.False(t, sd.IsSameScreen(changed))
}

// --- Compare tests ---

func TestCompare_ReturnsDetailedResult(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.White)

	// First call: no previous.
	result := sd.Compare(img)
	assert.False(t, result.IsSame)
	assert.InDelta(t, 0.0, result.Similarity, 0.001)

	// Second call: identical.
	result = sd.Compare(img)
	assert.True(t, result.IsSame)
	assert.InDelta(t, 1.0, result.Similarity, 0.01)
}

func TestCompare_SizeDiffers(t *testing.T) {
	sd := NewScreenDiffer(0.99)

	small := makePNG(t, 30, 30, color.White)
	large := makePNG(t, 200, 200, color.White)

	sd.Compare(small)
	result := sd.Compare(large)
	// PNG of 200x200 is much larger than 30x30 in bytes.
	assert.False(t, result.IsSame)
	assert.True(t, result.SizeDiffers)
}

func TestCompare_DimensionMismatch(t *testing.T) {
	sd := NewScreenDiffer(0.99)

	imgA := makePNG(t, 100, 100, color.White)
	imgB := makePNG(t, 100, 200, color.White)

	sd.Compare(imgA)
	result := sd.Compare(imgB)
	// Different dimensions are treated as different screens
	// even if the byte-length check passes.
	assert.False(t, result.IsSame)
}

func TestCompare_EmptyNewImage(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.White)

	sd.Compare(img)
	result := sd.Compare(nil)
	assert.False(t, result.IsSame)
	assert.False(t, sd.HasPrevious())
}

func TestCompare_InvalidPNG(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	valid := makePNG(t, 50, 50, color.White)

	sd.Compare(valid)
	result := sd.Compare([]byte("not a png"))
	assert.False(t, result.IsSame)
}

// --- Update tests ---

func TestUpdate_StoresImage(t *testing.T) {
	sd := NewScreenDiffer(0.99)

	assert.False(t, sd.HasPrevious())
	img := makePNG(t, 50, 50, color.White)
	sd.Update(img)
	assert.True(t, sd.HasPrevious())
}

func TestUpdate_ThenCompareIdentical(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.RGBA{60, 60, 60, 255})

	sd.Update(img)
	assert.True(t, sd.IsSameScreen(img))
}

// --- Reset tests ---

func TestReset_ClearsPrevious(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.White)

	sd.Update(img)
	assert.True(t, sd.HasPrevious())

	sd.Reset()
	assert.False(t, sd.HasPrevious())
}

func TestReset_ClearsStats(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	img := makePNG(t, 50, 50, color.White)

	sd.IsSameScreen(img) // stores
	sd.IsSameScreen(img) // same

	same, diff := sd.Stats()
	assert.Equal(t, 1, same)
	assert.Equal(t, 0, diff)

	sd.Reset()
	same, diff = sd.Stats()
	assert.Equal(t, 0, same)
	assert.Equal(t, 0, diff)
}

// --- Stats tests ---

func TestStats_CountsSameAndDiff(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	white := makePNG(t, 50, 50, color.White)
	black := makePNG(t, 50, 50, color.Black)

	sd.IsSameScreen(white)  // first call, stores
	sd.IsSameScreen(white)  // same
	sd.IsSameScreen(white)  // same
	sd.IsSameScreen(black)  // diff
	sd.IsSameScreen(black)  // same (black vs black)

	same, diff := sd.Stats()
	assert.Equal(t, 3, same)
	assert.Equal(t, 1, diff)
}

// --- SetSampleCount / SetTolerance tests ---

func TestSetSampleCount_Valid(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetSampleCount(1000)
	assert.Equal(t, 1000, sd.sampleCount)
}

func TestSetSampleCount_Invalid(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	original := sd.sampleCount
	sd.SetSampleCount(0)
	assert.Equal(t, original, sd.sampleCount)
	sd.SetSampleCount(-5)
	assert.Equal(t, original, sd.sampleCount)
}

func TestSetTolerance_Valid(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetTolerance(20)
	assert.Equal(t, 20, sd.tolerance)
}

func TestSetTolerance_Invalid(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	original := sd.tolerance
	sd.SetTolerance(-1)
	assert.Equal(t, original, sd.tolerance)
	sd.SetTolerance(256)
	assert.Equal(t, original, sd.tolerance)
}

func TestSetTolerance_Zero(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetTolerance(0)
	assert.Equal(t, 0, sd.tolerance)
}

// --- Edge cases ---

func TestIsSameScreen_VerySmallImage(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetSampleCount(1)
	// Smallest valid PNG: 1x1 pixel.
	tiny := makePNG(t, 1, 1, color.White)

	sd.IsSameScreen(tiny)
	result := sd.Compare(tiny)
	assert.True(t, result.IsSame)
}

func TestIsSameScreen_SinglePixel(t *testing.T) {
	sd := NewScreenDiffer(0.99)
	sd.SetSampleCount(1)

	pixel := makePNG(
		t, 1, 1, color.RGBA{128, 128, 128, 255},
	)

	sd.IsSameScreen(pixel)
	assert.True(t, sd.IsSameScreen(pixel))
}

func TestIsSameScreen_SequenceOfScreens(t *testing.T) {
	sd := NewScreenDiffer(0.99)

	red := makePNG(
		t, 100, 100, color.RGBA{255, 0, 0, 255},
	)
	green := makePNG(
		t, 100, 100, color.RGBA{0, 255, 0, 255},
	)
	blue := makePNG(
		t, 100, 100, color.RGBA{0, 0, 255, 255},
	)

	sd.IsSameScreen(red)    // stores red
	assert.False(t, sd.IsSameScreen(green))  // red vs green: diff
	assert.False(t, sd.IsSameScreen(blue))   // green vs blue: diff
	assert.True(t, sd.IsSameScreen(blue))    // blue vs blue: same
}

// --- copyBytes tests ---

func TestCopyBytes_Nil(t *testing.T) {
	assert.Nil(t, copyBytes(nil))
}

func TestCopyBytes_Independent(t *testing.T) {
	original := []byte{1, 2, 3}
	copied := copyBytes(original)
	assert.Equal(t, original, copied)

	// Mutating the copy should not affect the original.
	copied[0] = 99
	assert.Equal(t, byte(1), original[0])
}

// --- absDiffU32 tests ---

func TestAbsDiffU32(t *testing.T) {
	assert.Equal(t, uint32(5), absDiffU32(10, 5))
	assert.Equal(t, uint32(5), absDiffU32(5, 10))
	assert.Equal(t, uint32(0), absDiffU32(7, 7))
}
