// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// imgSize is the side length used for differential cache test images.
// With patchSize=24 this gives a 10×10 = 100 patch grid, so a single
// changed patch represents 1% — well below the 5% default changeThreshold.
const imgSize = 240

// newDiffImage returns an imgSize×imgSize RGBA image filled with the given colour,
// sized to give a 100-patch grid with the default patchSize=24.
func newDiffImage(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))
	for y := 0; y < imgSize; y++ {
		for x := 0; x < imgSize; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// copyDiffImage returns a deep copy of an imgSize×imgSize RGBA image.
func copyDiffImage(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))
	for y := 0; y < imgSize; y++ {
		for x := 0; x < imgSize; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(x, y))
		}
	}
	return dst
}

// diffSampleResponse returns a CachedResponse for differential cache tests.
func diffSampleResponse(text string) *CachedResponse {
	return &CachedResponse{
		Text:      text,
		Model:     "test-model",
		Duration:  10 * time.Millisecond,
		Timestamp: time.Now(),
	}
}

// TestDiffCache_NoHistory verifies that GetCachedResponse returns a miss when
// the cache is empty (no frame has been stored yet).
func TestDiffCache_NoHistory(t *testing.T) {
	dc := NewDifferentialCache(0.05)
	img := newDiffImage(color.RGBA{R: 128, G: 128, B: 128, A: 255})

	got, ok := dc.GetCachedResponse(nil, img)

	assert.False(t, ok, "expected miss when no frame has been stored")
	assert.Nil(t, got)
}

// TestDiffCache_IdenticalFrame verifies that storing a frame and then querying
// with the same image returns a cache hit with the stored response.
func TestDiffCache_IdenticalFrame(t *testing.T) {
	dc := NewDifferentialCache(0.05)
	img := newDiffImage(color.RGBA{R: 200, G: 100, B: 50, A: 255})
	resp := diffSampleResponse("identical frame response")

	dc.StoreFrame(img, resp)

	// Same pixel content — must be a hit.
	got, ok := dc.GetCachedResponse(nil, img)

	require.True(t, ok, "expected cache hit for identical frame")
	require.NotNil(t, got)
	assert.Equal(t, resp.Text, got.Text)
	assert.Equal(t, resp.Model, got.Model)
}

// TestDiffCache_SmallChange verifies that a frame with fewer changed patches
// than the change threshold (default 5%) returns a cache hit.
//
// With imgSize=240 and patchSize=24 there are 100 patches. Changing only 2
// adjacent pixels that both fall inside patch (0,0) results in exactly 1
// changed patch out of 100 = 1% — below the 5% threshold.
func TestDiffCache_SmallChange(t *testing.T) {
	dc := NewDifferentialCache(0.05)
	base := newDiffImage(color.RGBA{R: 100, G: 100, B: 100, A: 255})
	resp := diffSampleResponse("small change response")

	dc.StoreFrame(base, resp)

	// Copy the base image and change 2 pixels that both land in patch (0,0),
	// i.e. coordinates (0,0) and (1,0) — same 24×24 tile, so only 1/100
	// patches hash differently (1% < 5% threshold).
	slightly := copyDiffImage(base)
	slightly.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	slightly.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	got, ok := dc.GetCachedResponse(nil, slightly)

	require.True(t, ok, "expected cache hit for sub-threshold change")
	require.NotNil(t, got)
	assert.Equal(t, resp.Text, got.Text)
}

// TestDiffCache_LargeChange verifies that a frame with changes exceeding the
// threshold returns a cache miss. We flip the entire image to a different
// colour — 100% of patches change.
func TestDiffCache_LargeChange(t *testing.T) {
	dc := NewDifferentialCache(0.05)
	base := newDiffImage(color.RGBA{R: 10, G: 10, B: 10, A: 255})
	resp := diffSampleResponse("large change response")

	dc.StoreFrame(base, resp)

	// Completely different image — all 100 patches change.
	different := newDiffImage(color.RGBA{R: 255, G: 255, B: 0, A: 255})

	got, ok := dc.GetCachedResponse(nil, different)

	assert.False(t, ok, "expected cache miss when change exceeds threshold")
	assert.Nil(t, got)
}

// TestDiffCache_StoreAndRetrieve verifies full store → retrieve round-trip for
// multiple successive frames: each new StoreFrame replaces the previous one.
func TestDiffCache_StoreAndRetrieve(t *testing.T) {
	dc := NewDifferentialCache(0.05)

	first := newDiffImage(color.RGBA{R: 10, G: 20, B: 30, A: 255})
	firstResp := diffSampleResponse("first frame")
	dc.StoreFrame(first, firstResp)

	// Retrieve the first frame (identical patches — must hit).
	got, ok := dc.GetCachedResponse(nil, first)
	require.True(t, ok)
	assert.Equal(t, "first frame", got.Text)

	// Store a completely different second frame.
	second := newDiffImage(color.RGBA{R: 200, G: 100, B: 50, A: 255})
	secondResp := diffSampleResponse("second frame")
	dc.StoreFrame(second, secondResp)

	// Querying with the second frame (identical to the stored one) must hit.
	got2, ok2 := dc.GetCachedResponse(nil, second)
	require.True(t, ok2)
	assert.Equal(t, "second frame", got2.Text)

	// Querying with the first frame is now a large change (all patches differ).
	got3, ok3 := dc.GetCachedResponse(nil, first)
	assert.False(t, ok3, "first frame should now be a miss (large change from second)")
	assert.Nil(t, got3)
}

// TestDiffCache_DetectChangeRatio is a direct unit test for detectChangeRatio.
// It validates boundary values and an intermediate case.
func TestDiffCache_DetectChangeRatio(t *testing.T) {
	dc := NewDifferentialCache(0.05)

	// All patches identical → ratio 0.0.
	same := []string{"aaa", "bbb", "ccc", "ddd"}
	assert.Equal(t, 0.0, dc.detectChangeRatio(same, same))

	// All patches different → ratio 1.0.
	a := []string{"aaa", "bbb", "ccc"}
	b := []string{"xxx", "yyy", "zzz"}
	assert.Equal(t, 1.0, dc.detectChangeRatio(a, b))

	// 2 out of 4 patches changed → ratio 0.5.
	current := []string{"aaa", "NEW1", "ccc", "NEW2"}
	previous := []string{"aaa", "bbb", "ccc", "ddd"}
	ratio := dc.detectChangeRatio(current, previous)
	assert.InDelta(t, 0.5, ratio, 1e-9)

	// Empty slices → ratio 0.0 (nothing changed).
	assert.Equal(t, 0.0, dc.detectChangeRatio([]string{}, []string{}))
}
