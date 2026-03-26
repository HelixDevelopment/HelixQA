// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package performance provides types and utilities for
// collecting and analysing runtime performance metrics
// across platforms (Android, web, desktop).
package performance

import (
	"time"
)

// MetricType identifies the kind of measurement in a snapshot.
type MetricType string

const (
	// MetricMemoryRSS is the resident set size in kilobytes.
	MetricMemoryRSS MetricType = "memory_rss_kb"

	// MetricMemoryHeap is the heap allocation in kilobytes.
	MetricMemoryHeap MetricType = "memory_heap_kb"

	// MetricCPUPercent is the CPU usage as a percentage.
	MetricCPUPercent MetricType = "cpu_percent"

	// MetricNetworkRxKB is the network bytes received in KB.
	MetricNetworkRxKB MetricType = "network_rx_kb"

	// MetricNetworkTxKB is the network bytes transmitted in KB.
	MetricNetworkTxKB MetricType = "network_tx_kb"

	// MetricFPS is the rendered frames per second.
	MetricFPS MetricType = "fps"

	// MetricThreadCount is the number of active threads.
	MetricThreadCount MetricType = "thread_count"
)

// MetricSnapshot holds a single measurement at a point in time.
type MetricSnapshot struct {
	// Type identifies what was measured.
	Type MetricType `json:"type"`

	// Value is the numeric measurement.
	Value float64 `json:"value"`

	// Timestamp is when the measurement was taken.
	Timestamp time.Time `json:"timestamp"`

	// Platform is the platform on which the measurement was
	// taken (e.g. "android", "web", "desktop").
	Platform string `json:"platform"`

	// Label is an optional human-readable annotation.
	Label string `json:"label,omitempty"`
}

// MetricsTimeline is an ordered sequence of snapshots for a
// single platform, collected over a test run.
type MetricsTimeline struct {
	// Platform identifies the platform for this timeline.
	Platform string `json:"platform"`

	// Snapshots contains all collected measurements in the
	// order they were added.
	Snapshots []MetricSnapshot `json:"snapshots"`
}

// Add appends a snapshot to the timeline.
func (t *MetricsTimeline) Add(s MetricSnapshot) {
	t.Snapshots = append(t.Snapshots, s)
}

// OfType returns all snapshots whose Type matches the given
// MetricType.
func (t *MetricsTimeline) OfType(mt MetricType) []MetricSnapshot {
	var result []MetricSnapshot
	for _, s := range t.Snapshots {
		if s.Type == mt {
			result = append(result, s)
		}
	}
	return result
}

// LeakIndicator describes the outcome of a memory-leak
// analysis over the timeline.
type LeakIndicator struct {
	// Platform is the platform that was analysed.
	Platform string `json:"platform"`

	// StartKB is the first memory reading in kilobytes.
	StartKB float64 `json:"start_kb"`

	// EndKB is the last memory reading in kilobytes.
	EndKB float64 `json:"end_kb"`

	// GrowthPercent is the percentage growth from start to end.
	GrowthPercent float64 `json:"growth_percent"`

	// DurationSecs is the elapsed time between first and last
	// sample, in seconds.
	DurationSecs float64 `json:"duration_secs"`

	// IsLeak is true when GrowthPercent exceeds the configured
	// threshold.
	IsLeak bool `json:"is_leak"`
}

// DetectMemoryLeak analyses RSS snapshots in the timeline and
// returns a LeakIndicator. It returns nil when fewer than two
// MetricMemoryRSS snapshots are present (not enough data).
// thresholdPercent is the minimum growth percentage that is
// considered a leak (e.g. 10.0 for 10%).
func (t *MetricsTimeline) DetectMemoryLeak(
	thresholdPercent float64,
) *LeakIndicator {
	samples := t.OfType(MetricMemoryRSS)
	if len(samples) < 2 {
		return nil
	}

	first := samples[0]
	last := samples[len(samples)-1]

	var growthPct float64
	if first.Value > 0 {
		growthPct = (last.Value - first.Value) / first.Value * 100.0
	}

	durationSecs := last.Timestamp.Sub(first.Timestamp).Seconds()

	return &LeakIndicator{
		Platform:      t.Platform,
		StartKB:       first.Value,
		EndKB:         last.Value,
		GrowthPercent: growthPct,
		DurationSecs:  durationSecs,
		IsLeak:        growthPct >= thresholdPercent,
	}
}
