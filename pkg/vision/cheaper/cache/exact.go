// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cache provides the L1 exact-match image response cache for the
// HelixQA cheaper vision subsystem. An exact cache entry is keyed on a
// combined SHA-256 hash of the raw image pixels and the vision prompt, so
// identical (screenshot, prompt) pairs are served from memory without making
// a live provider call.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// CachedResponse holds a previously obtained vision provider response that
// can be served directly from the L1 cache.
type CachedResponse struct {
	// Text is the primary textual interpretation returned by the provider.
	Text string

	// Model is the exact model identifier that produced this response.
	Model string

	// Duration is the wall-clock time the original provider call took.
	Duration time.Duration

	// Timestamp records when the original provider call was initiated.
	Timestamp time.Time
}

// cacheEntry wraps a CachedResponse together with metadata used for eviction.
type cacheEntry struct {
	resp       *CachedResponse
	insertedAt time.Time
}

// ExactCache is a bounded, concurrency-safe L1 cache keyed on the combined
// hash of an image's pixel data and its associated prompt string. When the
// cache reaches capacity a random existing entry is evicted to make room.
type ExactCache struct {
	entries    map[string]*cacheEntry
	mu         sync.RWMutex
	maxEntries int
}

// NewExactCache returns a new ExactCache that holds at most maxEntries
// entries. If maxEntries is <= 0 it is clamped to 1 so the cache is always
// usable.
func NewExactCache(maxEntries int) *ExactCache {
	if maxEntries <= 0 {
		maxEntries = 1
	}
	return &ExactCache{
		entries:    make(map[string]*cacheEntry, maxEntries),
		maxEntries: maxEntries,
	}
}

// Get looks up a cached response for the given image and prompt. It returns
// the cached response and true when a hit is found, or nil and false on a
// miss.
func (c *ExactCache) Get(img image.Image, promptHash string) (*CachedResponse, bool) {
	key := c.buildKey(img, promptHash)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	return entry.resp, true
}

// Put stores resp in the cache under the key derived from img and
// promptHash. If the cache is already at capacity, one random existing entry
// is evicted before the new entry is inserted.
func (c *ExactCache) Put(img image.Image, promptHash string, resp *CachedResponse) {
	key := c.buildKey(img, promptHash)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict one random entry when at capacity (and the key is new).
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[key] = &cacheEntry{
		resp:       resp,
		insertedAt: time.Now(),
	}
}

// Size returns the current number of entries held in the cache.
func (c *ExactCache) Size() int {
	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	return n
}

// Clear removes all entries from the cache.
func (c *ExactCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry, c.maxEntries)
	c.mu.Unlock()
}

// buildKey returns the composite cache key for (img, promptHash) as
// "<imageHash>:<promptHash>".
func (c *ExactCache) buildKey(img image.Image, promptHash string) string {
	imageHash := memory.ComputeImageHash(img)
	return imageHash + ":" + hashPrompt(promptHash)
}

// hashPrompt returns the first 16 hex characters of the SHA-256 digest of
// the given prompt string. This is sufficient to distinguish prompts while
// keeping cache keys short.
func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])[:16]
}
