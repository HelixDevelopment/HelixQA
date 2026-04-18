// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func makeFrame(seq uint64, ts time.Time) contracts.Frame {
	return contracts.Frame{
		Seq:       seq,
		Timestamp: ts,
		Width:     1920,
		Height:    1080,
	}
}

// TestFrameRing_PushAndSnapshotAround_ZeroWindow verifies that a zero
// window returns all stored frames in insertion order.
func TestFrameRing_PushAndSnapshotAround_ZeroWindow(t *testing.T) {
	r := NewFrameRing(8)
	base := time.Now()
	for i := 0; i < 5; i++ {
		r.Push(makeFrame(uint64(i), base.Add(time.Duration(i)*time.Second)))
	}
	snap := r.SnapshotAround(base.Add(10*time.Hour), 0)
	require.Len(t, snap, 5)
	for i, f := range snap {
		assert.Equal(t, uint64(i), f.Seq)
	}
}

// TestFrameRing_SnapshotAround_Window verifies timestamp filtering.
// Window = 4s centred on t+2s → (t+0s, t+4s] → frames at t+1s, t+2s, t+3s, t+4s.
// Frame at t+0s is strictly excluded (not after earliest=t+0); t+4s is included
// (at or before latest=t+4s).
func TestFrameRing_SnapshotAround_Window(t *testing.T) {
	r := NewFrameRing(16)
	base := time.Now()
	// Push frames at t+0 … t+4 (5 frames, 1s apart)
	for i := 0; i < 5; i++ {
		r.Push(makeFrame(uint64(i), base.Add(time.Duration(i)*time.Second)))
	}
	// Window = 4s centred on t+2s → half=2s → earliest=t+0s, latest=t+4s
	// Half-open (earliest, latest]: excludes t+0s, includes t+1s..t+4s → 4 frames
	snap := r.SnapshotAround(base.Add(2*time.Second), 4*time.Second)
	require.Len(t, snap, 4, "expected frames at t+1s, t+2s, t+3s, t+4s")
	assert.Equal(t, uint64(1), snap[0].Seq)
	assert.Equal(t, uint64(2), snap[1].Seq)
	assert.Equal(t, uint64(3), snap[2].Seq)
	assert.Equal(t, uint64(4), snap[3].Seq)
}

// TestFrameRing_Eviction verifies oldest entries are evicted when full.
func TestFrameRing_Eviction(t *testing.T) {
	r := NewFrameRing(4)
	base := time.Now()
	for i := 0; i < 6; i++ {
		r.Push(makeFrame(uint64(i), base.Add(time.Duration(i)*time.Second)))
	}
	assert.Equal(t, 4, r.Len())
	snap := r.SnapshotAround(base.Add(10*time.Hour), 0)
	require.Len(t, snap, 4)
	// Oldest 2 (seq 0,1) evicted; remaining seq 2,3,4,5
	assert.Equal(t, uint64(2), snap[0].Seq)
	assert.Equal(t, uint64(5), snap[3].Seq)
}

// TestFrameRing_SnapshotAround_Empty verifies no panic on empty ring.
func TestFrameRing_SnapshotAround_Empty(t *testing.T) {
	r := NewFrameRing(16)
	snap := r.SnapshotAround(time.Now(), time.Second)
	assert.Nil(t, snap)
}

// TestFrameRing_ConcurrentPush verifies no data races under -race.
func TestFrameRing_ConcurrentPush(t *testing.T) {
	r := NewFrameRing(256)
	base := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Push(makeFrame(uint64(n), base.Add(time.Duration(n)*time.Millisecond)))
		}(i)
	}
	wg.Wait()
	assert.LessOrEqual(t, r.Len(), 64)
}
