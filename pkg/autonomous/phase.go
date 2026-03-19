// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package autonomous provides the session coordinator for
// autonomous QA sessions. It orchestrates LLM agents, vision
// analysis, navigation, and issue detection across multiple
// platforms in parallel.
package autonomous

import (
	"fmt"
	"sync"
	"time"
)

// PhaseStatus represents the current state of a session phase.
type PhaseStatus string

const (
	PhasePending   PhaseStatus = "pending"
	PhaseRunning   PhaseStatus = "running"
	PhaseCompleted PhaseStatus = "completed"
	PhaseFailed    PhaseStatus = "failed"
	PhaseSkipped   PhaseStatus = "skipped"
)

// Phase describes a session phase with its state and timing.
type Phase struct {
	Name     string      `json:"name"`
	Status   PhaseStatus `json:"status"`
	StartAt  time.Time   `json:"start_at,omitempty"`
	EndAt    time.Time   `json:"end_at,omitempty"`
	Progress float64     `json:"progress"`
	Error    error       `json:"-"`
}

// Duration returns the phase duration. Returns zero if not
// started or still running.
func (p Phase) Duration() time.Duration {
	if p.StartAt.IsZero() {
		return 0
	}
	if p.EndAt.IsZero() {
		return time.Since(p.StartAt)
	}
	return p.EndAt.Sub(p.StartAt)
}

// PhaseListener receives notifications on phase transitions.
type PhaseListener interface {
	OnPhaseStart(phase Phase)
	OnPhaseComplete(phase Phase)
	OnPhaseError(phase Phase, err error)
}

// PhaseManager tracks phase transitions with listener
// notifications. Thread-safe.
type PhaseManager struct {
	phases    []Phase
	current   int
	listeners []PhaseListener
	mu        sync.Mutex
}

// NewPhaseManager creates a PhaseManager with the standard
// session phases.
func NewPhaseManager() *PhaseManager {
	return &PhaseManager{
		phases: []Phase{
			{Name: "setup", Status: PhasePending},
			{Name: "doc-driven", Status: PhasePending},
			{Name: "curiosity", Status: PhasePending},
			{Name: "report", Status: PhasePending},
		},
		current: -1,
	}
}

// AddListener registers a listener for phase events.
func (pm *PhaseManager) AddListener(l PhaseListener) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.listeners = append(pm.listeners, l)
}

// Start transitions a phase from pending to running.
func (pm *PhaseManager) Start(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := pm.findPhase(name)
	if idx < 0 {
		return fmt.Errorf("phase %q not found", name)
	}

	if pm.phases[idx].Status != PhasePending {
		return fmt.Errorf(
			"phase %q is %s, expected pending",
			name, pm.phases[idx].Status,
		)
	}

	pm.phases[idx].Status = PhaseRunning
	pm.phases[idx].StartAt = time.Now()
	pm.current = idx

	phase := pm.phases[idx]
	for _, l := range pm.listeners {
		l.OnPhaseStart(phase)
	}
	return nil
}

// Complete transitions a phase from running to completed.
func (pm *PhaseManager) Complete(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := pm.findPhase(name)
	if idx < 0 {
		return fmt.Errorf("phase %q not found", name)
	}

	if pm.phases[idx].Status != PhaseRunning {
		return fmt.Errorf(
			"phase %q is %s, expected running",
			name, pm.phases[idx].Status,
		)
	}

	pm.phases[idx].Status = PhaseCompleted
	pm.phases[idx].EndAt = time.Now()
	pm.phases[idx].Progress = 1.0

	phase := pm.phases[idx]
	for _, l := range pm.listeners {
		l.OnPhaseComplete(phase)
	}
	return nil
}

// Fail transitions a phase from running to failed.
func (pm *PhaseManager) Fail(name string, err error) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := pm.findPhase(name)
	if idx < 0 {
		return fmt.Errorf("phase %q not found", name)
	}

	if pm.phases[idx].Status != PhaseRunning {
		return fmt.Errorf(
			"phase %q is %s, expected running",
			name, pm.phases[idx].Status,
		)
	}

	pm.phases[idx].Status = PhaseFailed
	pm.phases[idx].EndAt = time.Now()
	pm.phases[idx].Error = err

	phase := pm.phases[idx]
	for _, l := range pm.listeners {
		l.OnPhaseError(phase, err)
	}
	return nil
}

// Skip transitions a phase from pending to skipped.
func (pm *PhaseManager) Skip(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := pm.findPhase(name)
	if idx < 0 {
		return fmt.Errorf("phase %q not found", name)
	}

	if pm.phases[idx].Status != PhasePending {
		return fmt.Errorf(
			"phase %q is %s, expected pending",
			name, pm.phases[idx].Status,
		)
	}

	pm.phases[idx].Status = PhaseSkipped
	return nil
}

// Current returns the currently running phase. Returns a
// zero Phase if no phase is running.
func (pm *PhaseManager) Current() Phase {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.current < 0 || pm.current >= len(pm.phases) {
		return Phase{}
	}
	return pm.phases[pm.current]
}

// All returns a copy of all phases.
func (pm *PhaseManager) All() []Phase {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := make([]Phase, len(pm.phases))
	copy(result, pm.phases)
	return result
}

// UpdateProgress sets the progress for the named phase.
func (pm *PhaseManager) UpdateProgress(
	name string, progress float64,
) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := pm.findPhase(name)
	if idx < 0 {
		return fmt.Errorf("phase %q not found", name)
	}

	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	pm.phases[idx].Progress = progress
	return nil
}

// findPhase returns the index of the named phase, or -1.
// Must be called with mu held.
func (pm *PhaseManager) findPhase(name string) int {
	for i, p := range pm.phases {
		if p.Name == name {
			return i
		}
	}
	return -1
}
