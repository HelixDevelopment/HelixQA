// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"image"
	"time"

	"digital.vasic.helixqa/pkg/vision/hash"
)

// StagnationDetector tracks screen states and flags both:
//
//   - window-based stagnation — "no pixels have changed in ≥ stagnantTime"
//     (cruder, drives the 10-second "UI is stuck" heuristic);
//   - online change-point detection — Bayesian Online Change-Point
//     Detection (Adams & MacKay 2007) over the dHash Hamming-distance
//     stream. Emits a per-frame change probability that flips high on the
//     exact frame the UI transitioned, without the 10-second wait.
//
// The BOCPD channel is optional — it activates once the first image.Image
// is fed via AddFrame(). Callers that only need the legacy byte-hash
// behaviour can keep using AddScreenshot() and never touch BOCPD.
type StagnationDetector struct {
	history      []screenSnapshot
	maxHistory   int
	stagnantTime time.Duration

	hasher     hash.DHasher
	bocpd      *BOCPD
	lastDHash  uint64
	hasDHash   bool
	lastCPProb float64
}

// screenSnapshot captures a point-in-time screen state
type screenSnapshot struct {
	timestamp  time.Time
	data       []byte
	size       int
	hash       uint64
	screenName string
}

// NewStagnationDetector creates a new detector with sensible defaults
// (maxHistory=30, stagnantTime=10s, BOCPD hazard=1/250 dHash-64). Errors
// surface only on bad cfg math in BOCPD; the default cfg is clean.
func NewStagnationDetector() *StagnationDetector {
	b, _ := NewBOCPD(BOCPDConfig{})
	return &StagnationDetector{
		history:      make([]screenSnapshot, 0, 30),
		maxHistory:   30,
		stagnantTime: 10 * time.Second,
		hasher:       hash.DHasher{Kind: hash.DHash64},
		bocpd:        b,
	}
}

// AddScreenshot records a new screen state from raw pixel bytes (legacy
// path — the crude 5-sample FNV hash). Prefer AddFrame when an
// image.Image is available; it feeds the BOCPD channel and produces
// per-frame change probabilities.
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

// AddFrame records a new frame via its image.Image. Computes a dHash-64
// for window-stagnation bookkeeping, then feeds the Hamming distance
// from the previous frame into the BOCPD channel. After the first call,
// LastChangeProbability() returns a meaningful per-frame change
// probability — high means "the UI just changed".
//
// Returns the BOCPD change probability for this frame, or 0 on the very
// first frame (there is no distance to feed yet).
func (sd *StagnationDetector) AddFrame(img image.Image, screenName string) (float64, error) {
	h, err := sd.hasher.Hash(img)
	if err != nil {
		return 0, err
	}

	snapshot := screenSnapshot{
		timestamp:  time.Now(),
		hash:       h,
		screenName: screenName,
	}
	sd.history = append(sd.history, snapshot)
	if len(sd.history) > sd.maxHistory {
		sd.history = sd.history[1:]
	}

	if !sd.hasDHash {
		sd.lastDHash = h
		sd.hasDHash = true
		sd.lastCPProb = 0
		return 0, nil
	}
	dist := float64(sd.hasher.Distance(sd.lastDHash, h))
	sd.lastDHash = h
	sd.lastCPProb = sd.bocpd.Observe(dist)
	return sd.lastCPProb, nil
}

// LastChangeProbability returns the most recent per-frame change
// probability emitted by the BOCPD channel. Before any AddFrame call,
// returns 0.
func (sd *StagnationDetector) LastChangeProbability() float64 {
	return sd.lastCPProb
}

// IsChangePoint is a convenience predicate — true when the last change
// probability exceeds the conventional 0.5 threshold.
func (sd *StagnationDetector) IsChangePoint() bool {
	return sd.lastCPProb > 0.5
}

// BOCPD exposes the underlying change-point detector for diagnostics
// (most-likely run length, observation count) and for callers that want
// to Reset() the BOCPD channel independently of the history window.
func (sd *StagnationDetector) BOCPD() *BOCPD { return sd.bocpd }

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
