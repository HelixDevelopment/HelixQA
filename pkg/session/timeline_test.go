// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTimeline(t *testing.T) {
	tl := NewTimeline()
	assert.NotNil(t, tl)
	assert.Equal(t, 0, tl.Count())
	assert.Empty(t, tl.Events())
}

func TestTimeline_RecordEvent_AutoID(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{
		Type:        EventAction,
		Platform:    "android",
		Description: "clicked button",
	})

	events := tl.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "evt-000001", events[0].ID)
	assert.Equal(t, EventAction, events[0].Type)
	assert.Equal(t, "android", events[0].Platform)
	assert.False(t, events[0].Timestamp.IsZero())
}

func TestTimeline_RecordEvent_CustomID(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{
		ID:          "custom-1",
		Type:        EventIssue,
		Description: "found bug",
	})

	events := tl.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "custom-1", events[0].ID)
}

func TestTimeline_RecordEvent_CustomTimestamp(t *testing.T) {
	tl := NewTimeline()
	ts := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	tl.RecordEvent(TimelineEvent{
		Type:      EventNavigation,
		Timestamp: ts,
	})

	events := tl.Events()
	require.Len(t, events, 1)
	assert.Equal(t, ts, events[0].Timestamp)
}

func TestTimeline_Count(t *testing.T) {
	tl := NewTimeline()
	assert.Equal(t, 0, tl.Count())

	tl.RecordEvent(TimelineEvent{Type: EventAction})
	assert.Equal(t, 1, tl.Count())

	tl.RecordEvent(TimelineEvent{Type: EventScreenshot})
	tl.RecordEvent(TimelineEvent{Type: EventCrash})
	assert.Equal(t, 3, tl.Count())
}

func TestTimeline_Events_ReturnsCopy(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{Type: EventAction})

	events := tl.Events()
	events[0].Type = EventCrash // modify the copy

	original := tl.Events()
	assert.Equal(t, EventAction, original[0].Type)
}

func TestTimeline_EventsByType(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{Type: EventAction, Platform: "android"})
	tl.RecordEvent(TimelineEvent{Type: EventScreenshot, Platform: "android"})
	tl.RecordEvent(TimelineEvent{Type: EventAction, Platform: "desktop"})
	tl.RecordEvent(TimelineEvent{Type: EventIssue, Platform: "web"})
	tl.RecordEvent(TimelineEvent{Type: EventAction, Platform: "web"})

	actions := tl.EventsByType(EventAction)
	assert.Len(t, actions, 3)
	for _, a := range actions {
		assert.Equal(t, EventAction, a.Type)
	}

	screenshots := tl.EventsByType(EventScreenshot)
	assert.Len(t, screenshots, 1)

	crashes := tl.EventsByType(EventCrash)
	assert.Empty(t, crashes)
}

func TestTimeline_EventsByPlatform(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{Type: EventAction, Platform: "android"})
	tl.RecordEvent(TimelineEvent{Type: EventAction, Platform: "desktop"})
	tl.RecordEvent(TimelineEvent{Type: EventIssue, Platform: "android"})
	tl.RecordEvent(TimelineEvent{Type: EventCrash, Platform: "web"})

	android := tl.EventsByPlatform("android")
	assert.Len(t, android, 2)

	desktop := tl.EventsByPlatform("desktop")
	assert.Len(t, desktop, 1)

	ios := tl.EventsByPlatform("ios")
	assert.Empty(t, ios)
}

func TestTimeline_Reset(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{Type: EventAction})
	tl.RecordEvent(TimelineEvent{Type: EventAction})
	assert.Equal(t, 2, tl.Count())

	tl.Reset()
	assert.Equal(t, 0, tl.Count())
	assert.Empty(t, tl.Events())

	// Counter also resets; new event should start at 1.
	tl.RecordEvent(TimelineEvent{Type: EventAction})
	events := tl.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "evt-000001", events[0].ID)
}

func TestTimeline_IDSequence(t *testing.T) {
	tl := NewTimeline()
	for i := 0; i < 5; i++ {
		tl.RecordEvent(TimelineEvent{Type: EventAction})
	}

	events := tl.Events()
	for i, e := range events {
		expected := fmt.Sprintf("evt-%06d", i+1)
		assert.Equal(t, expected, e.ID)
	}
}

func TestTimeline_MetadataPreserved(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{
		Type: EventAction,
		Metadata: map[string]string{
			"key": "value",
			"foo": "bar",
		},
	})

	events := tl.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "value", events[0].Metadata["key"])
	assert.Equal(t, "bar", events[0].Metadata["foo"])
}

func TestTimeline_AllEventTypes(t *testing.T) {
	types := []EventType{
		EventAction, EventScreenshot, EventIssue,
		EventPhaseChange, EventCrash, EventNavigation,
	}

	tl := NewTimeline()
	for _, et := range types {
		tl.RecordEvent(TimelineEvent{Type: et})
	}
	assert.Equal(t, len(types), tl.Count())

	for _, et := range types {
		filtered := tl.EventsByType(et)
		assert.Len(t, filtered, 1, "type %s", et)
	}
}

func TestTimeline_FieldsPreserved(t *testing.T) {
	tl := NewTimeline()
	tl.RecordEvent(TimelineEvent{
		Type:           EventIssue,
		Platform:       "android",
		VideoOffset:    15 * time.Second,
		ScreenID:       "screen-settings",
		Description:    "Button truncated",
		ScreenshotPath: "/tmp/ss.png",
		IssueID:        "HQA-0042",
		FeatureID:      "feat-settings",
	})

	e := tl.Events()[0]
	assert.Equal(t, "android", e.Platform)
	assert.Equal(t, 15*time.Second, e.VideoOffset)
	assert.Equal(t, "screen-settings", e.ScreenID)
	assert.Equal(t, "Button truncated", e.Description)
	assert.Equal(t, "/tmp/ss.png", e.ScreenshotPath)
	assert.Equal(t, "HQA-0042", e.IssueID)
	assert.Equal(t, "feat-settings", e.FeatureID)
}

func TestEventType_Constants(t *testing.T) {
	assert.Equal(t, EventType("action"), EventAction)
	assert.Equal(t, EventType("screenshot"), EventScreenshot)
	assert.Equal(t, EventType("issue"), EventIssue)
	assert.Equal(t, EventType("phase_change"), EventPhaseChange)
	assert.Equal(t, EventType("crash"), EventCrash)
	assert.Equal(t, EventType("navigation"), EventNavigation)
}

// Stress test: concurrent event recording.
func TestTimeline_Stress_ConcurrentRecording(t *testing.T) {
	tl := NewTimeline()
	const goroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				tl.RecordEvent(TimelineEvent{
					Type:     EventAction,
					Platform: fmt.Sprintf("platform-%d", gID),
					Description: fmt.Sprintf(
						"event %d from goroutine %d", i, gID,
					),
				})
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, goroutines*eventsPerGoroutine, tl.Count())

	// All IDs should be unique.
	ids := make(map[string]bool)
	for _, e := range tl.Events() {
		assert.False(t, ids[e.ID], "duplicate ID: %s", e.ID)
		ids[e.ID] = true
	}
}
