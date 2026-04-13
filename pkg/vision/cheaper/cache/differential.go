// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cache provides the L1 exact-match and L2 differential image
// response caches for the HelixQA cheaper vision subsystem. The differential
// cache avoids redundant provider calls when consecutive screenshots differ by
// less than a configurable fraction of their patch grid.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

const (
	// lastFrameKey is the go-cache key used to store the most recent frame state.
	lastFrameKey = "last_frame"

	// defaultTTL is the time-to-live for cached frame states.
	defaultTTL = 5 * time.Minute

	// defaultCleanupInterval is how often go-cache evicts expired entries.
	defaultCleanupInterval = 10 * time.Minute

	// defaultPatchSize is the width and height (in pixels) of each patch in the
	// grid used to compute differential change ratios.
	defaultPatchSize = 24
)

// FrameState holds the hashed representation of a single captured frame
// together with the provider response that was returned for it.
type FrameState struct {
	// FrameID is an opaque identifier for the frame (unused internally; exposed
	// for callers that want to tag frames).
	FrameID string

	// ImageHash is the full-image SHA-256 hash (hex-encoded).
	ImageHash string

	// PatchHashes holds one SHA-256 hash per patch in the patchSize × patchSize
	// grid, in row-major order.
	PatchHashes []string

	// Timestamp records when the frame was stored.
	Timestamp time.Time

	// FullResponse is the provider response associated with this frame.
	FullResponse *CachedResponse
}

// DifferentialCache is an L2 cache that reuses provider responses when
// successive screenshots are visually similar. Similarity is determined by
// comparing per-patch SHA-256 hashes: if the fraction of changed patches is
// below changeThreshold the previous response is returned directly.
type DifferentialCache struct {
	frameCache      *gocache.Cache
	mu              sync.RWMutex
	changeThreshold float64
	patchSize       int
}

// NewDifferentialCache returns a new DifferentialCache with the given change
// threshold (a value in [0, 1] representing the maximum fraction of changed
// patches for a cache hit). A patchSize of 24 pixels and a default TTL of
// 5 minutes are used.
func NewDifferentialCache(changeThreshold float64) *DifferentialCache {
	return &DifferentialCache{
		frameCache:      gocache.New(defaultTTL, defaultCleanupInterval),
		changeThreshold: changeThreshold,
		patchSize:       defaultPatchSize,
	}
}

// GetCachedResponse checks whether the given image is similar enough to the
// most recently stored frame to reuse its response. It first checks for an
// exact patch-hash match (zero-cost identical frame), and then falls back to
// comparing the change ratio against changeThreshold. ctx is accepted for
// interface compatibility but is not used internally.
//
// Returns the cached CachedResponse and true on a hit, or nil and false on a
// miss.
func (d *DifferentialCache) GetCachedResponse(_ context.Context, img image.Image) (*CachedResponse, bool) {
	d.mu.RLock()
	raw, found := d.frameCache.Get(lastFrameKey)
	d.mu.RUnlock()

	if !found {
		return nil, false
	}

	prev, ok := raw.(*FrameState)
	if !ok || prev.FullResponse == nil {
		return nil, false
	}

	current := d.computePatchHashes(img)

	// Fast path: identical patch hashes → definite hit.
	if patchSlicesEqual(current, prev.PatchHashes) {
		return prev.FullResponse, true
	}

	// Slow path: compute change ratio and compare against threshold.
	ratio := d.detectChangeRatio(current, prev.PatchHashes)
	if ratio < d.changeThreshold {
		return prev.FullResponse, true
	}

	return nil, false
}

// StoreFrame computes the patch hashes for img and stores a FrameState under
// the "last_frame" key in the underlying go-cache, replacing any previous
// entry.
func (d *DifferentialCache) StoreFrame(img image.Image, response *CachedResponse) {
	patches := d.computePatchHashes(img)

	state := &FrameState{
		PatchHashes:  patches,
		Timestamp:    time.Now(),
		FullResponse: response,
	}

	d.mu.Lock()
	d.frameCache.Set(lastFrameKey, state, gocache.DefaultExpiration)
	d.mu.Unlock()
}

// computePatchHashes divides img into a grid of patchSize × patchSize pixel
// patches and returns a slice of hex-encoded SHA-256 hashes, one per patch,
// in row-major order. Boundary patches (at the right/bottom edges) may be
// smaller than patchSize if the image dimensions are not exact multiples.
func (d *DifferentialCache) computePatchHashes(img image.Image) []string {
	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y

	// Compute the number of patch columns and rows, rounding up.
	cols := (w + d.patchSize - 1) / d.patchSize
	rows := (h + d.patchSize - 1) / d.patchSize

	if cols == 0 || rows == 0 {
		return nil
	}

	hashes := make([]string, 0, cols*rows)
	buf := make([]byte, 4)

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x0 := bounds.Min.X + col*d.patchSize
			y0 := bounds.Min.Y + row*d.patchSize
			x1 := x0 + d.patchSize
			y1 := y0 + d.patchSize

			if x1 > bounds.Max.X {
				x1 = bounds.Max.X
			}
			if y1 > bounds.Max.Y {
				y1 = bounds.Max.Y
			}

			h := sha256.New()
			for py := y0; py < y1; py++ {
				for px := x0; px < x1; px++ {
					r, g, b, a := img.At(px, py).RGBA()
					buf[0] = byte(r >> 8)
					buf[1] = byte(g >> 8)
					buf[2] = byte(b >> 8)
					buf[3] = byte(a >> 8)
					_, _ = h.Write(buf)
				}
			}

			hashes = append(hashes, hex.EncodeToString(h.Sum(nil)))
		}
	}

	return hashes
}

// detectChangeRatio compares two patch-hash slices of equal length and
// returns the fraction of patches that differ. A value of 0.0 means all
// patches are identical; 1.0 means all patches have changed. If either slice
// is empty the ratio is 0.0.
func (d *DifferentialCache) detectChangeRatio(current []string, previous []string) float64 {
	total := len(previous)
	if total == 0 || len(current) == 0 {
		return 0.0
	}

	// Use the shorter length to avoid an index-out-of-bounds if slices differ
	// in size (e.g. different resolution frames).
	if len(current) < total {
		total = len(current)
	}

	changed := 0
	for i := 0; i < total; i++ {
		if current[i] != previous[i] {
			changed++
		}
	}

	return float64(changed) / float64(total)
}

// patchSlicesEqual returns true when both slices have the same length and all
// corresponding elements are equal.
func patchSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
