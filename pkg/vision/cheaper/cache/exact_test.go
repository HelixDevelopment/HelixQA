// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSolidImage returns a 16×16 RGBA image filled with the given colour.
func newSolidImage(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// sampleResponse returns a non-nil CachedResponse suitable for use in tests.
func sampleResponse(text, model string) *CachedResponse {
	return &CachedResponse{
		Text:      text,
		Model:     model,
		Duration:  42 * time.Millisecond,
		Timestamp: time.Now(),
	}
}

// TestExactCache_Miss verifies that Get returns a miss for an entry that has
// never been stored.
func TestExactCache_Miss(t *testing.T) {
	c := NewExactCache(10)
	img := newSolidImage(color.RGBA{R: 255, A: 255})

	got, ok := c.Get(img, "some prompt")

	assert.False(t, ok, "expected cache miss for unseen key")
	assert.Nil(t, got)
}

// TestExactCache_Hit verifies that Put followed by Get with the same image
// and prompt returns the stored response.
func TestExactCache_Hit(t *testing.T) {
	c := NewExactCache(10)
	img := newSolidImage(color.RGBA{G: 200, A: 255})
	resp := sampleResponse("click the button", "glm4v")

	c.Put(img, "tap the login button", resp)
	got, ok := c.Get(img, "tap the login button")

	require.True(t, ok, "expected cache hit after Put")
	require.NotNil(t, got)
	assert.Equal(t, resp.Text, got.Text)
	assert.Equal(t, resp.Model, got.Model)
}

// TestExactCache_DifferentPrompt verifies that the same image stored under
// one prompt is not returned when queried with a different prompt.
func TestExactCache_DifferentPrompt(t *testing.T) {
	c := NewExactCache(10)
	img := newSolidImage(color.RGBA{B: 128, A: 255})
	resp := sampleResponse("navigate home", "qwen25vl")

	c.Put(img, "original prompt", resp)
	got, ok := c.Get(img, "different prompt")

	assert.False(t, ok, "different prompt must produce a cache miss")
	assert.Nil(t, got)
}

// TestExactCache_Eviction verifies that the cache never exceeds maxEntries.
// With maxEntries=2 inserting 3 distinct entries must keep the size at ≤ 2.
func TestExactCache_Eviction(t *testing.T) {
	c := NewExactCache(2)

	images := []*image.RGBA{
		newSolidImage(color.RGBA{R: 100, A: 255}),
		newSolidImage(color.RGBA{G: 100, A: 255}),
		newSolidImage(color.RGBA{B: 100, A: 255}),
	}
	prompts := []string{"prompt-a", "prompt-b", "prompt-c"}

	for i, img := range images {
		c.Put(img, prompts[i], sampleResponse("text", "model"))
	}

	assert.LessOrEqual(t, c.Size(), 2, "cache size must not exceed maxEntries after eviction")
}

// TestExactCache_Clear verifies that Clear removes all entries and resets
// the size to zero.
func TestExactCache_Clear(t *testing.T) {
	c := NewExactCache(10)
	img := newSolidImage(color.RGBA{R: 50, G: 50, B: 50, A: 255})

	c.Put(img, "p1", sampleResponse("a", "m"))
	c.Put(img, "p2", sampleResponse("b", "m"))
	require.Greater(t, c.Size(), 0, "cache must be non-empty before Clear")

	c.Clear()

	assert.Equal(t, 0, c.Size(), "cache must be empty after Clear")

	// Subsequent Get must miss.
	_, ok := c.Get(img, "p1")
	assert.False(t, ok, "Get after Clear must return a miss")
}

// TestExactCache_ConcurrentAccess verifies that simultaneous reads and writes
// from 50 goroutines do not cause data races.
func TestExactCache_ConcurrentAccess(t *testing.T) {
	c := NewExactCache(20)
	img := newSolidImage(color.RGBA{R: 77, G: 77, B: 77, A: 255})
	resp := sampleResponse("concurrent text", "uitars")

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			prompt := "shared prompt"
			// Even goroutines write; odd goroutines read.
			if id%2 == 0 {
				c.Put(img, prompt, resp)
			} else {
				_, _ = c.Get(img, prompt)
			}
		}(i)
	}

	wg.Wait()
	// The cache must be in a valid state after concurrent access.
	assert.LessOrEqual(t, c.Size(), 20)
}
