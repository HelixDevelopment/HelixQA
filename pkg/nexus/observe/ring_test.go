// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func makeEvent(kind contracts.EventKind, ts time.Time) contracts.Event {
	return contracts.Event{Kind: kind, Timestamp: ts}
}

func TestRing_PushAndSnapshotHappyPath(t *testing.T) {
	r := NewRingBuffer(8)
	now := time.Now()
	r.Push(makeEvent(contracts.EventKindHook, now))
	r.Push(makeEvent(contracts.EventKindDBus, now.Add(time.Second)))

	// Zero window returns all events.
	snap := r.Snapshot(now.Add(2*time.Second), 0)
	require.Len(t, snap, 2)
	require.Equal(t, contracts.EventKindHook, snap[0].Kind)
	require.Equal(t, contracts.EventKindDBus, snap[1].Kind)
}

func TestRing_WindowFiltering(t *testing.T) {
	r := NewRingBuffer(16)
	base := time.Now()
	// Push 5 events 1 second apart.
	for i := 0; i < 5; i++ {
		r.Push(makeEvent(contracts.EventKindCDP, base.Add(time.Duration(i)*time.Second)))
	}
	// Window of 2 seconds ending at base+3s should include events at +2s and +3s.
	at := base.Add(3 * time.Second)
	snap := r.Snapshot(at, 2*time.Second)
	require.Len(t, snap, 2, "only events within (at-window, at] should be returned")
	require.Equal(t, base.Add(2*time.Second), snap[0].Timestamp)
	require.Equal(t, base.Add(3*time.Second), snap[1].Timestamp)
}

func TestRing_EvictionWhenFull(t *testing.T) {
	cap := 4
	r := NewRingBuffer(cap)
	base := time.Now()
	// Push cap+2 events; oldest 2 should be evicted.
	for i := 0; i < cap+2; i++ {
		r.Push(makeEvent(contracts.EventKindAXTree, base.Add(time.Duration(i)*time.Second)))
	}
	require.Equal(t, cap, r.Len(), "ring must not exceed capacity")
	snap := r.Snapshot(base.Add(10*time.Second), 0)
	require.Len(t, snap, cap)
	// Oldest surviving event is at index 2 (the first 2 were evicted).
	require.Equal(t, base.Add(2*time.Second), snap[0].Timestamp)
}

func TestRing_SnapshotEmpty(t *testing.T) {
	r := NewRingBuffer(8)
	snap := r.Snapshot(time.Now(), 0)
	require.Nil(t, snap, "snapshot of empty ring must return nil")
}

func TestRing_SnapshotWindowExcludesAll(t *testing.T) {
	r := NewRingBuffer(8)
	base := time.Now()
	r.Push(makeEvent(contracts.EventKindSyscall, base))
	// Window that doesn't overlap the stored event.
	snap := r.Snapshot(base.Add(-time.Hour), time.Second)
	require.Empty(t, snap)
}

func TestRing_ConcurrentPush(t *testing.T) {
	r := NewRingBuffer(256)
	var wg sync.WaitGroup
	const goroutines = 64
	const pushesEach = 32
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < pushesEach; j++ {
				r.Push(makeEvent(contracts.EventKindHook, time.Now()))
			}
		}()
	}
	wg.Wait()
	// Ring capacity is 256; goroutines*pushes = 2048 — ring should be full.
	require.Equal(t, 256, r.Len())
}
