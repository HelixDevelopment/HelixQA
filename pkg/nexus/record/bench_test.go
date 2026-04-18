// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"bytes"
	"testing"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func benchFrame(seq uint64, ts time.Time) contracts.Frame {
	return contracts.Frame{Seq: seq, Timestamp: ts, Width: 1920, Height: 1080}
}

// BenchmarkFrameRing_Push measures the cost of pushing a single frame into a
// ring buffer that is already half-full (steady-state recording loop).
func BenchmarkFrameRing_Push(b *testing.B) {
	r := NewFrameRing(1024)
	base := time.Now()
	for i := 0; i < 512; i++ {
		r.Push(benchFrame(uint64(i), base.Add(time.Duration(i)*time.Millisecond)))
	}
	f := benchFrame(512, time.Now())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Push(f)
	}
}

// BenchmarkFrameRing_SnapshotAround_ZeroWindow measures the cost of
// snapshotting all 256 entries (return-all path).
func BenchmarkFrameRing_SnapshotAround_ZeroWindow(b *testing.B) {
	r := NewFrameRing(256)
	base := time.Now()
	for i := 0; i < 256; i++ {
		r.Push(benchFrame(uint64(i), base.Add(time.Duration(i)*time.Millisecond)))
	}
	at := base.Add(time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.SnapshotAround(at, 0)
	}
}

// BenchmarkFrameRing_SnapshotAround_Window measures the windowed path where
// only a subset of entries pass the timestamp filter (~100 of 1000).
func BenchmarkFrameRing_SnapshotAround_Window(b *testing.B) {
	r := NewFrameRing(1000)
	base := time.Now()
	for i := 0; i < 1000; i++ {
		r.Push(benchFrame(uint64(i), base.Add(time.Duration(i)*time.Millisecond)))
	}
	at := base.Add(500 * time.Millisecond)
	window := 200 * time.Millisecond // ~200 frames pass
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.SnapshotAround(at, window)
	}
}

// BenchmarkClip_1000Frames measures the full Clip path (SnapshotAround +
// JSON encode) for 1000 frames in the ring.
func BenchmarkClip_1000Frames(b *testing.B) {
	r := NewFrameRing(1000)
	base := time.Now()
	for i := 0; i < 1000; i++ {
		r.Push(benchFrame(uint64(i), base.Add(time.Duration(i)*time.Millisecond)))
	}
	around := base.Add(500 * time.Millisecond)
	window := time.Duration(0) // all frames
	opts := contracts.ClipOptions{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = clipWrite(r, around, window, &buf, opts)
	}
}
