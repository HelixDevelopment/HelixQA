// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"time"
)

// StagnationDetector tracks screen states and detects when UI is stuck
type StagnationDetector struct {
	history      []screenSnapshot
	maxHistory   int
	stagnantTime time.Duration
}

// screenSnapshot captures a point-in-time screen state
type screenSnapshot struct {
	timestamp  time.Time
	data       []byte
	size       int
	hash       uint64
	screenName string
}

// NewStagnationDetector creates a new detector
func NewStagnationDetector() *StagnationDetector {
	return &StagnationDetector{
		history:      make([]screenSnapshot, 0, 30),
		maxHistory:   30,
		stagnantTime: 10 * time.Second, // Consider stagnant after 10s of no change
	}
}

// AddScreenshot records a new screen state
func (sd *StagnationDetector) AddScreenshot(data []byte, screenName string) {
	if len(data) == 0 {
		return
	}
	
	snapshot := screenSnapshot{
		timestamp:  time.Now(),
		data:       data,
		size:       len(data),
		hash:       sd.computeHash(data),
		screenName: screenName,
	}
	
	sd.history = append(sd.history, snapshot)
	if len(sd.history) > sd.maxHistory {
		sd.history = sd.history[1:]
	}
}

// IsStagnant returns true if screen hasn't changed in stagnantTime
func (sd *StagnationDetector) IsStagnant() bool {
	if len(sd.history) < 3 {
		return false
	}
	
	// Get snapshots from last stagnantTime period
	cutoff := time.Now().Add(-sd.stagnantTime)
	var recent []screenSnapshot
	for _, snap := range sd.history {
		if snap.timestamp.After(cutoff) {
			recent = append(recent, snap)
		}
	}
	
	// Need at least 3 snapshots to determine stagnation
	if len(recent) < 3 {
		return false
	}
	
	// Check if all recent snapshots are identical
	first := recent[0]
	for _, snap := range recent[1:] {
		if snap.hash != first.hash || snap.size != first.size {
			return false // Screen changed
		}
	}
	
	return true // Stagnant
}

// GetStagnantDuration returns how long screen has been stagnant
func (sd *StagnationDetector) GetStagnantDuration() time.Duration {
	if !sd.IsStagnant() {
		return 0
	}
	
	if len(sd.history) == 0 {
		return 0
	}
	
	// Find when stagnation started
	latest := sd.history[len(sd.history)-1]
	for i := len(sd.history) - 2; i >= 0; i-- {
		if sd.history[i].hash != latest.hash {
			// Stagnation started after this point
			return time.Since(sd.history[i+1].timestamp)
		}
	}
	
	return time.Since(sd.history[0].timestamp)
}

// GetCurrentScreen returns the most recent screen name
func (sd *StagnationDetector) GetCurrentScreen() string {
	if len(sd.history) == 0 {
		return "unknown"
	}
	return sd.history[len(sd.history)-1].screenName
}

// computeHash creates a simple hash of screenshot data
func (sd *StagnationDetector) computeHash(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	
	// Use sampling for performance
	h := uint64(14695981039346656037) // FNV offset basis
	samplePoints := []int{0, len(data) / 4, len(data) / 2, 3 * len(data) / 4, len(data) - 1}
	
	for _, idx := range samplePoints {
		if idx < len(data) {
			h ^= uint64(data[idx])
			h *= 1099511628211 // FNV prime
		}
	}
	
	return h
}

// Reset clears the history
func (sd *StagnationDetector) Reset() {
	sd.history = make([]screenSnapshot, 0, sd.maxHistory)
}
