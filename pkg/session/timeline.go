// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package session provides recording and timeline management for
// autonomous QA sessions. It handles video recording coordination,
// screenshot capture indexing, and chronological event tracking
// across multiple platforms.
package session

import (
	"fmt"
	"sync"
	"time"
)

// EventType identifies the kind of timeline event.
type EventType string

const (
	// EventAction records a user interaction.
	EventAction EventType = "action"
	// EventScreenshot records a screenshot capture.
	EventScreenshot EventType = "screenshot"
	// EventIssue records an issue detection.
	EventIssue EventType = "issue"
	// EventPhaseChange records a session phase transition.
	EventPhaseChange EventType = "phase_change"
	// EventCrash records a crash detection.
	EventCrash EventType = "crash"
	// EventNavigation records a screen navigation.
	EventNavigation EventType = "navigation"
)

// TimelineEvent records a single event in the session timeline.
type TimelineEvent struct {
	// ID is a unique identifier for this event.
	ID string `json:"id"`

	// Type identifies the event kind.
	Type EventType `json:"type"`

	// Platform identifies which platform this event is for.
	Platform string `json:"platform"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// VideoOffset is the offset into the platform's video.
	VideoOffset time.Duration `json:"video_offset"`

	// ScreenID identifies the screen where the event occurred.
	ScreenID string `json:"screen_id,omitempty"`

	// Description is a human-readable event description.
	Description string `json:"description"`

	// ScreenshotPath is the path to a related screenshot.
	ScreenshotPath string `json:"screenshot_path,omitempty"`

	// IssueID links to an issue if applicable.
	IssueID string `json:"issue_id,omitempty"`

	// FeatureID links to a feature if applicable.
	FeatureID string `json:"feature_id,omitempty"`

	// Metadata holds additional event-specific data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Timeline records chronological events during a QA session.
type Timeline struct {
	events  []TimelineEvent
	counter int
	mu      sync.Mutex
}

// NewTimeline creates an empty Timeline.
func NewTimeline() *Timeline {
	return &Timeline{
		events: make([]TimelineEvent, 0, 64),
	}
}

// RecordEvent adds an event to the timeline. The event ID is
// auto-assigned if empty, and timestamp defaults to now.
func (t *Timeline) RecordEvent(event TimelineEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.counter++
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%06d", t.counter)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	t.events = append(t.events, event)
}

// Events returns a copy of all recorded events.
func (t *Timeline) Events() []TimelineEvent {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]TimelineEvent, len(t.events))
	copy(result, t.events)
	return result
}

// Count returns the number of recorded events.
func (t *Timeline) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.events)
}

// EventsByType returns events filtered by the given type.
func (t *Timeline) EventsByType(et EventType) []TimelineEvent {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []TimelineEvent
	for _, e := range t.events {
		if e.Type == et {
			result = append(result, e)
		}
	}
	return result
}

// EventsByPlatform returns events filtered by platform.
func (t *Timeline) EventsByPlatform(platform string) []TimelineEvent {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []TimelineEvent
	for _, e := range t.events {
		if e.Platform == platform {
			result = append(result, e)
		}
	}
	return result
}

// Reset clears all events and resets the counter.
func (t *Timeline) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = t.events[:0]
	t.counter = 0
}
