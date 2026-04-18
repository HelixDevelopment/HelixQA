// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe

import (
	"sync"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// RingBuffer is a bounded, goroutine-safe ring buffer storing
// contracts.Event values. When full, Push evicts the oldest entry.
type RingBuffer struct {
	mu   sync.Mutex
	buf  []contracts.Event
	head int // index of the next slot to write
	size int // number of valid entries currently stored
	cap  int // total capacity
}

// NewRingBuffer creates a RingBuffer with the given capacity.
// cap must be > 0; if ≤ 0 it is clamped to 1.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		buf: make([]contracts.Event, capacity),
		cap: capacity,
	}
}

// Push inserts e into the buffer. If the buffer is full, the oldest
// entry is silently evicted.
func (r *RingBuffer) Push(e contracts.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = e
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// Snapshot returns all events whose Timestamp falls in the half-open
// window (at-window, at]. at is the end of the window; window is its
// duration. A zero window returns all stored events.
func (r *RingBuffer) Snapshot(at time.Time, window time.Duration) []contracts.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == 0 {
		return nil
	}

	// Compute the earliest timestamp to include.
	var earliest time.Time
	if window > 0 {
		earliest = at.Add(-window)
	}

	out := make([]contracts.Event, 0, r.size)
	// Walk from oldest to newest: oldest entry is at (head - size + cap) % cap.
	start := (r.head - r.size + r.cap) % r.cap
	for i := 0; i < r.size; i++ {
		idx := (start + i) % r.cap
		e := r.buf[idx]
		if window > 0 {
			// Window is half-open: (at-window, at] — strictly after earliest,
			// at or before at.
			if !e.Timestamp.After(earliest) || e.Timestamp.After(at) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// Len returns the number of events currently stored.
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
