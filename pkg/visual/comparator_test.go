// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visual

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

// makePNG creates a PNG-encoded image of the given size
// filled with the specified color.
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

// makePNGWithRegion creates a PNG filled with bg, then
// paints a rectangle at (rx,ry,rw,rh) with fg.
func makePNGWithRegion(
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

// --- NewScreenshotComparator tests ---

func TestNewScreenshotComparator_Defaults(t *testing.T) {
	sc := NewScreenshotComparator()
	assert.Equal(t, defaultTolerance, sc.tolerance)
	assert.Equal(t, defaultBlackThreshold, sc.blackThreshold)
	assert.InDelta(t, defaultBlackPercent, sc.blackPercent, 0.001)
}

func TestNewScreenshotComparator_WithOptions(t *testing.T) {
	sc := NewScreenshotComparator(
		WithTolerance(20),
		WithBlackThreshold(50),
		WithBlackPercent(0.80),
	)
	assert.Equal(t, 20, sc.tolerance)
	assert.Equal(t, 50, sc.blackThreshold)
	assert.InDelta(t, 0.80, sc.blackPercent, 0.001)
}

func TestNewScreenshotComparator_InvalidOptions(t *testing.T) {
	sc := NewScreenshotComparator(
		WithTolerance(-5),
		WithBlackThreshold(300),
		WithBlackPercent(2.0),
	)
	// Invalid values should be ignored, defaults kept.
	assert.Equal(t, defaultTolerance, sc.tolerance)
	assert.Equal(t, defaultBlackThreshold, sc.blackThreshold)
	assert.InDelta(t, defaultBlackPercent, sc.blackPercent, 0.001)
}

// --- Compare tests ---

func TestCompare_IdenticalImages(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	img := makePNG(t, 100, 100, color.RGBA{128, 64, 32, 255})

	result, err := sc.Compare(ctx, img, img)
	require.NoError(t, err)
	assert.True(t, result.Match)
	assert.InDelta(t, 1.0, result.Similarity, 0.001)
	assert.Equal(t, 0, result.DiffPixelCount)
	assert.Empty(t, result.DiffRegions)
	assert.Equal(t, 100, result.Width)
	assert.Equal(t, 100, result.Height)
}

func TestCompare_CompletelyDifferent(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	white := makePNG(t, 50, 50, color.White)
	black := makePNG(t, 50, 50, color.Black)

	result, err := sc.Compare(ctx, white, black)
	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.InDelta(t, 0.0, result.Similarity, 0.01)
	assert.Equal(t, 50*50, result.DiffPixelCount)
	assert.NotEmpty(t, result.DiffRegions)
}

func TestCompare_WithinTolerance(t *testing.T) {
	sc := NewScreenshotComparator(WithTolerance(15))
	ctx := context.Background()

	imgA := makePNG(
		t, 40, 40, color.RGBA{100, 100, 100, 255},
	)
	imgB := makePNG(
		t, 40, 40, color.RGBA{110, 105, 95, 255},
	)

	result, err := sc.Compare(ctx, imgA, imgB)
	require.NoError(t, err)
	assert.True(t, result.Match)
	assert.InDelta(t, 1.0, result.Similarity, 0.001)
}

func TestCompare_BeyondTolerance(t *testing.T) {
	sc := NewScreenshotComparator(WithTolerance(5))
	ctx := context.Background()

	imgA := makePNG(
		t, 40, 40, color.RGBA{100, 100, 100, 255},
	)
	imgB := makePNG(
		t, 40, 40, color.RGBA{120, 100, 100, 255},
	)

	result, err := sc.Compare(ctx, imgA, imgB)
	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.Equal(t, 40*40, result.DiffPixelCount)
}

func TestCompare_DimensionMismatch(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	imgA := makePNG(t, 100, 100, color.White)
	imgB := makePNG(t, 200, 100, color.White)

	_, err := sc.Compare(ctx, imgA, imgB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")
}

func TestCompare_InvalidPNG(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	valid := makePNG(t, 10, 10, color.White)

	_, err := sc.Compare(ctx, []byte("not a png"), valid)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode before")

	_, err = sc.Compare(ctx, valid, []byte("not a png"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode after")
}

func TestCompare_EmptyData(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	valid := makePNG(t, 10, 10, color.White)

	_, err := sc.Compare(ctx, nil, valid)
	assert.Error(t, err)

	_, err = sc.Compare(ctx, valid, nil)
	assert.Error(t, err)
}

func TestCompare_PartialDiff_HasRegions(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	// Image A: all white.
	imgA := makePNG(t, 100, 100, color.White)

	// Image B: white with a black rectangle in center.
	imgB := makePNGWithRegion(
		t, 100, 100,
		color.White, color.Black,
		30, 30, 40, 40,
	)

	result, err := sc.Compare(ctx, imgA, imgB)
	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.Equal(t, 40*40, result.DiffPixelCount)
	assert.NotEmpty(t, result.DiffRegions)

	// The diff region should cover roughly the center.
	foundCenter := false
	for _, r := range result.DiffRegions {
		if r.X <= 30 && r.Y <= 30 &&
			r.X+r.Width >= 70 && r.Y+r.Height >= 70 {
			foundCenter = true
		}
	}
	assert.True(t, foundCenter,
		"diff region should cover center rectangle")
}

// --- IsBlack tests ---

func TestIsBlack_AllBlack(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	img := makePNG(t, 50, 50, color.Black)

	black, err := sc.IsBlack(ctx, img)
	require.NoError(t, err)
	assert.True(t, black)
}

func TestIsBlack_AllWhite(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	img := makePNG(t, 50, 50, color.White)

	black, err := sc.IsBlack(ctx, img)
	require.NoError(t, err)
	assert.False(t, black)
}

func TestIsBlack_NearlyBlack(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	// Dark gray (below threshold).
	img := makePNG(
		t, 50, 50, color.RGBA{10, 10, 10, 255},
	)

	black, err := sc.IsBlack(ctx, img)
	require.NoError(t, err)
	assert.True(t, black)
}

func TestIsBlack_SmallBrightRegion(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	// Black image with a small white dot (1% bright).
	// 100x100 = 10000px, 1x1 bright = 0.01% -- still
	// >95% dark.
	img := makePNGWithRegion(
		t, 100, 100,
		color.Black, color.White,
		50, 50, 1, 1,
	)

	black, err := sc.IsBlack(ctx, img)
	require.NoError(t, err)
	assert.True(t, black)
}

func TestIsBlack_InvalidPNG(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	_, err := sc.IsBlack(ctx, []byte("garbage"))
	assert.Error(t, err)
}

// --- HasContent tests ---

func TestHasContent_WhiteImage(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	img := makePNG(t, 50, 50, color.White)

	content, err := sc.HasContent(ctx, img)
	require.NoError(t, err)
	assert.True(t, content)
}

func TestHasContent_BlackImage(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	img := makePNG(t, 50, 50, color.Black)

	content, err := sc.HasContent(ctx, img)
	require.NoError(t, err)
	assert.False(t, content)
}

func TestHasContent_InvalidPNG(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	_, err := sc.HasContent(ctx, nil)
	assert.Error(t, err)
}

// --- CompareDisplays tests ---

func TestCompareDisplays_VideoOnSecondary(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	// Primary: mostly black (Presenter background) with a
	// small overlay region (10% bright).
	primary := makePNGWithRegion(
		t, 100, 100,
		color.Black,
		color.RGBA{200, 200, 200, 255},
		0, 0, 10, 100,
	)

	// Secondary: colorful video content.
	secondary := makePNG(
		t, 100, 100, color.RGBA{150, 80, 40, 255},
	)

	result, err := sc.CompareDisplays(
		ctx, primary, secondary,
	)
	require.NoError(t, err)
	assert.Equal(t, DisplayStateContent, result.SecondaryState)
	assert.True(t, result.SecondaryHasVideo)
	assert.Equal(t, DisplayStateContent, result.PrimaryState)
	assert.True(t, result.PrimaryHasOverlay)
	assert.True(t, result.SecondaryBrightness > 0)
}

func TestCompareDisplays_BothBlack(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	black := makePNG(t, 80, 80, color.Black)

	result, err := sc.CompareDisplays(
		ctx, black, black,
	)
	require.NoError(t, err)
	assert.Equal(t, DisplayStateBlack, result.PrimaryState)
	assert.Equal(t, DisplayStateBlack, result.SecondaryState)
	assert.False(t, result.SecondaryHasVideo)
	assert.False(t, result.PrimaryHasOverlay)
	assert.InDelta(t, 0.0, result.PrimaryBrightness, 1.0)
	assert.InDelta(t, 0.0, result.SecondaryBrightness, 1.0)
}

func TestCompareDisplays_SecondaryBlack(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	primary := makePNG(
		t, 80, 80, color.RGBA{100, 120, 140, 255},
	)
	secondary := makePNG(t, 80, 80, color.Black)

	result, err := sc.CompareDisplays(
		ctx, primary, secondary,
	)
	require.NoError(t, err)
	assert.Equal(t, DisplayStateContent, result.PrimaryState)
	assert.Equal(t, DisplayStateBlack, result.SecondaryState)
	assert.False(t, result.SecondaryHasVideo)
}

func TestCompareDisplays_InvalidPrimary(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	secondary := makePNG(t, 50, 50, color.White)

	_, err := sc.CompareDisplays(
		ctx, []byte("bad"), secondary,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "primary")
}

func TestCompareDisplays_InvalidSecondary(t *testing.T) {
	sc := NewScreenshotComparator()
	ctx := context.Background()

	primary := makePNG(t, 50, 50, color.White)

	_, err := sc.CompareDisplays(
		ctx, primary, []byte("bad"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secondary")
}

// --- Region tests ---

func TestRegion_Area(t *testing.T) {
	tests := []struct {
		name     string
		region   Region
		expected int
	}{
		{
			name:     "normal",
			region:   Region{X: 0, Y: 0, Width: 10, Height: 20},
			expected: 200,
		},
		{
			name:     "zero width",
			region:   Region{X: 5, Y: 5, Width: 0, Height: 10},
			expected: 0,
		},
		{
			name:     "negative",
			region:   Region{X: 0, Y: 0, Width: -1, Height: 10},
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.region.Area())
		})
	}
}

// --- Luminance tests ---

func TestLuminance_Black(t *testing.T) {
	lum := Luminance(color.Black)
	assert.InDelta(t, 0.0, lum, 1.0)
}

func TestLuminance_White(t *testing.T) {
	lum := Luminance(color.White)
	assert.InDelta(t, 255.0, lum, 1.0)
}

func TestLuminance_PureRed(t *testing.T) {
	// BT.601: 0.299 * 255 = 76.245
	lum := Luminance(color.RGBA{255, 0, 0, 255})
	assert.InDelta(t, 76.0, lum, 2.0)
}

func TestLuminance_PureGreen(t *testing.T) {
	// BT.601: 0.587 * 255 = 149.685
	lum := Luminance(color.RGBA{0, 255, 0, 255})
	assert.InDelta(t, 150.0, lum, 2.0)
}

// --- DisplayState constants ---

func TestDisplayState_Constants(t *testing.T) {
	assert.Equal(t,
		DisplayState("black"), DisplayStateBlack,
	)
	assert.Equal(t,
		DisplayState("content"), DisplayStateContent,
	)
	assert.Equal(t,
		DisplayState("unknown"), DisplayStateUnknown,
	)
}

// --- meanLuminance tests ---

func TestMeanLuminance_UniformImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	gray := color.RGBA{128, 128, 128, 255}
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, gray)
		}
	}
	lum := meanLuminance(img)
	assert.InDelta(t, 128.0, lum, 2.0)
}

func TestMeanLuminance_EmptyImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 0, 0))
	lum := meanLuminance(img)
	assert.InDelta(t, 0.0, lum, 0.001)
}
