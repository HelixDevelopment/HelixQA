// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"sync"
	"time"
)

// StateTracker tracks navigation state including history,
// current screen, and action counts.
type StateTracker struct {
	currentScreenID string
	history         []HistoryEntry
	actionCount     int
	errorCount      int
	startedAt       time.Time
	mu              sync.Mutex
}

// HistoryEntry records a single navigation step.
type HistoryEntry struct {
	// FromScreen is the screen before the action.
	FromScreen string `json:"from_screen"`

	// ToScreen is the screen after the action.
	ToScreen string `json:"to_screen"`

	// Action describes what was done.
	Action string `json:"action"`

	// Timestamp is when the action occurred.
	Timestamp time.Time `json:"timestamp"`

	// Success indicates if the action succeeded.
	Success bool `json:"success"`
}

// NewStateTracker creates a StateTracker.
func NewStateTracker() *StateTracker {
	return &StateTracker{
		history:   make([]HistoryEntry, 0, 64),
		startedAt: time.Now(),
	}
}

// CurrentScreen returns the current screen ID.
func (st *StateTracker) CurrentScreen() string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.currentScreenID
}

// SetCurrentScreen updates the current screen.
func (st *StateTracker) SetCurrentScreen(screenID string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.currentScreenID = screenID
}

// RecordAction records a navigation action in history.
func (st *StateTracker) RecordAction(
	from, to, action string, success bool,
) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.actionCount++
	if !success {
		st.errorCount++
	}

	st.history = append(st.history, HistoryEntry{
		FromScreen: from,
		ToScreen:   to,
		Action:     action,
		Timestamp:  time.Now(),
		Success:    success,
	})
	if success && to != "" {
		st.currentScreenID = to
	}
}

// History returns a copy of the navigation history.
func (st *StateTracker) History() []HistoryEntry {
	st.mu.Lock()
	defer st.mu.Unlock()

	result := make([]HistoryEntry, len(st.history))
	copy(result, st.history)
	return result
}

// ActionCount returns the total number of actions taken.
func (st *StateTracker) ActionCount() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.actionCount
}

// ErrorCount returns the number of failed actions.
func (st *StateTracker) ErrorCount() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.errorCount
}

// Elapsed returns the time since the tracker was created.
func (st *StateTracker) Elapsed() time.Duration {
	return time.Since(st.startedAt)
}

// Reset clears all history and counters.
func (st *StateTracker) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.history = st.history[:0]
	st.actionCount = 0
	st.errorCount = 0
	st.currentScreenID = ""
}
