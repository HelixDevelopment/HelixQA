// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"sync"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// FrameRing is a bounded, goroutine-safe ring buffer storing
// contracts.Frame values. When full, Push evicts the oldest entry.
type FrameRing struct {
	mu   sync.Mutex
	buf  []contracts.Frame
	head int // next slot to write
	size int // valid entries currently stored
	cap  int // total capacity
}

// NewFrameRing creates a FrameRing with the given capacity.
// capacity must be > 0; if ≤ 0 it is clamped to 1.
func NewFrameRing(capacity int) *FrameRing {
	if capacity <= 0 {
		capacity = 1
	}
	return &FrameRing{
		buf: make([]contracts.Frame, capacity),
		cap: capacity,
	}
}

// Push inserts f into the buffer. If the buffer is full, the oldest
// entry is silently evicted.
func (r *FrameRing) Push(f contracts.Frame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = f
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// SnapshotAround returns all frames whose Timestamp falls in the
// half-open window (around-window/2, around+window/2]. A zero
// window returns all stored frames.
func (r *FrameRing) SnapshotAround(around time.Time, window time.Duration) []contracts.Frame {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return nil
	}

	var earliest, latest time.Time
	if window > 0 {
		half := window / 2
		earliest = around.Add(-half)
		latest = around.Add(half)
	}

	out := make([]contracts.Frame, 0, r.size)
	// Walk from oldest to newest.
	start := (r.head - r.size + r.cap) % r.cap
	for i := 0; i < r.size; i++ {
		idx := (start + i) % r.cap
		f := r.buf[idx]
		if window > 0 {
			// half-open: strictly after earliest, at or before latest
			if !f.Timestamp.After(earliest) || f.Timestamp.After(latest) {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// Len returns the number of frames currently stored.
func (r *FrameRing) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
