// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe

import (
	"testing"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// BenchmarkRing_Push measures the cost of pushing a single event into a
// ring buffer that is already half-full (representative of the steady-state
// observation loop).
func BenchmarkRing_Push(b *testing.B) {
	r := NewRingBuffer(1024)
	// Pre-fill to half capacity.
	base := time.Now()
	for i := 0; i < 512; i++ {
		r.Push(contracts.Event{
			Kind:      contracts.EventKindHook,
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
		})
	}
	e := contracts.Event{Kind: contracts.EventKindHook, Timestamp: time.Now()}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Push(e)
	}
}

// BenchmarkRing_Snapshot measures the cost of snapshotting all events from
// a ring buffer with 256 entries using a zero window (return-all path).
func BenchmarkRing_Snapshot(b *testing.B) {
	r := NewRingBuffer(256)
	base := time.Now()
	for i := 0; i < 256; i++ {
		r.Push(contracts.Event{
			Kind:      contracts.EventKindCDP,
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
		})
	}
	at := base.Add(time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Snapshot(at, 0)
	}
}

// BenchmarkRing_SnapshotWindow measures the windowed snapshot path where
// only a subset of entries pass the timestamp filter.
func BenchmarkRing_SnapshotWindow(b *testing.B) {
	r := NewRingBuffer(1024)
	base := time.Now()
	for i := 0; i < 1024; i++ {
		r.Push(contracts.Event{
			Kind:      contracts.EventKindDBus,
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
		})
	}
	// Window: last 100 ms of the 1024 ms range → ~100 events pass.
	at := base.Add(1023 * time.Millisecond)
	window := 100 * time.Millisecond
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Snapshot(at, window)
	}
}
