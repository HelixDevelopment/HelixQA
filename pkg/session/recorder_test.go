// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionRecorder(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	assert.NotNil(t, sr)
	assert.Equal(t, "sess-001", sr.SessionID())
	assert.Equal(t, "/tmp/qa", sr.OutputDir())
	assert.Equal(t, 0, sr.ScreenshotCount())
	assert.Equal(t, 0, sr.TimelineCount())
	assert.Empty(t, sr.VideoPlatforms())
}

func TestSessionRecorder_StartRecording(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	err := sr.StartRecording("android")
	require.NoError(t, err)

	assert.True(t, sr.IsRecording("android"))
	assert.False(t, sr.IsRecording("desktop"))
	assert.Contains(t, sr.VideoPlatforms(), "android")
}

func TestSessionRecorder_StartRecording_Duplicate(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))

	err := sr.StartRecording("android")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")
}

func TestSessionRecorder_StopRecording(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))

	path, err := sr.StopRecording("android")
	require.NoError(t, err)
	assert.Contains(t, path, "android-sess-001.mp4")
	assert.False(t, sr.IsRecording("android"))
}

func TestSessionRecorder_StopRecording_NoPlatform(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	_, err := sr.StopRecording("android")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recording found")
}

func TestSessionRecorder_CaptureScreenshot(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")

	ss := sr.CaptureScreenshot("android", "login-page")
	assert.Equal(t, 1, ss.Index)
	assert.Equal(t, "android", ss.Platform)
	assert.Equal(t, "login-page", ss.Name)
	assert.Contains(t, ss.Path, "0001-login-page.png")
	assert.Contains(t, ss.Path, "android")
	assert.False(t, ss.Timestamp.IsZero())
}

func TestSessionRecorder_CaptureScreenshot_Sequence(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")

	ss1 := sr.CaptureScreenshot("android", "first")
	ss2 := sr.CaptureScreenshot("android", "second")
	ss3 := sr.CaptureScreenshot("desktop", "third")

	assert.Equal(t, 1, ss1.Index)
	assert.Equal(t, 2, ss2.Index)
	assert.Equal(t, 3, ss3.Index)
	assert.Equal(t, 3, sr.ScreenshotCount())
}

func TestSessionRecorder_CaptureScreenshot_WithVideo(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))

	time.Sleep(5 * time.Millisecond)
	ss := sr.CaptureScreenshot("android", "after-delay")
	assert.Greater(t, ss.VideoOffset, time.Duration(0))
}

func TestSessionRecorder_CaptureScreenshot_WithoutVideo(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	ss := sr.CaptureScreenshot("android", "no-video")
	assert.Equal(t, time.Duration(0), ss.VideoOffset)
}

func TestSessionRecorder_RecordEvent(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	sr.RecordEvent(TimelineEvent{
		Type:        EventAction,
		Platform:    "android",
		Description: "clicked button",
	})

	events := sr.ExportTimeline()
	// Includes StartRecording-generated events + our event.
	found := false
	for _, e := range events {
		if e.Description == "clicked button" {
			found = true
			break
		}
	}
	assert.True(t, found, "event should be in timeline")
}

func TestSessionRecorder_RecordEvent_WithVideoOffset(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))

	time.Sleep(5 * time.Millisecond)
	sr.RecordEvent(TimelineEvent{
		Type:        EventAction,
		Platform:    "android",
		Description: "offset event",
	})

	events := sr.ExportTimeline()
	var found *TimelineEvent
	for i := range events {
		if events[i].Description == "offset event" {
			found = &events[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.Greater(t, found.VideoOffset, time.Duration(0))
}

func TestSessionRecorder_VideoTimestamp(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")

	// No recording — should be zero.
	assert.Equal(t, time.Duration(0), sr.VideoTimestamp("android"))

	require.NoError(t, sr.StartRecording("android"))
	time.Sleep(5 * time.Millisecond)
	ts := sr.VideoTimestamp("android")
	assert.Greater(t, ts, time.Duration(0))
}

func TestSessionRecorder_IsRecording_NoVideo(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	assert.False(t, sr.IsRecording("android"))
}

func TestSessionRecorder_MultiplePlatforms(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))
	require.NoError(t, sr.StartRecording("desktop"))
	require.NoError(t, sr.StartRecording("web"))

	platforms := sr.VideoPlatforms()
	assert.Len(t, platforms, 3)

	assert.True(t, sr.IsRecording("android"))
	assert.True(t, sr.IsRecording("desktop"))
	assert.True(t, sr.IsRecording("web"))

	// Stop one.
	_, err := sr.StopRecording("desktop")
	require.NoError(t, err)
	assert.False(t, sr.IsRecording("desktop"))
	assert.True(t, sr.IsRecording("android"))
}

func TestSessionRecorder_TimelineIncludesStartStop(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")
	require.NoError(t, sr.StartRecording("android"))
	_, err := sr.StopRecording("android")
	require.NoError(t, err)

	events := sr.ExportTimeline()
	assert.GreaterOrEqual(t, len(events), 2)

	descriptions := make([]string, len(events))
	for i, e := range events {
		descriptions[i] = e.Description
	}

	hasStart := false
	hasStop := false
	for _, d := range descriptions {
		if strings.Contains(d, "started") {
			hasStart = true
		}
		if strings.Contains(d, "stopped") {
			hasStop = true
		}
	}
	assert.True(t, hasStart, "should have start event")
	assert.True(t, hasStop, "should have stop event")
}

func TestSessionRecorder_ScreenshotPathFormat(t *testing.T) {
	sr := NewSessionRecorder("sess-001", "/tmp/qa")

	ss := sr.CaptureScreenshot("android", "settings")
	assert.True(t, strings.HasPrefix(ss.Path, "/tmp/qa/screenshots/android/"))
	assert.True(t, strings.HasSuffix(ss.Path, ".png"))
}

// Stress test: concurrent captures and events.
func TestSessionRecorder_Stress_ConcurrentOperations(t *testing.T) {
	sr := NewSessionRecorder("stress-001", "/tmp/stress")
	require.NoError(t, sr.StartRecording("android"))
	require.NoError(t, sr.StartRecording("desktop"))

	const goroutines = 20
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			platform := "android"
			if gID%2 == 0 {
				platform = "desktop"
			}
			for i := 0; i < opsPerGoroutine; i++ {
				name := fmt.Sprintf("ss-%d-%d", gID, i)
				sr.CaptureScreenshot(platform, name)
				sr.RecordEvent(TimelineEvent{
					Type:        EventAction,
					Platform:    platform,
					Description: name,
				})
				_ = sr.VideoTimestamp(platform)
				_ = sr.IsRecording(platform)
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, goroutines*opsPerGoroutine, sr.ScreenshotCount())
	// Timeline has: 2 start events + goroutines * opsPerGoroutine screenshots
	// + goroutines * opsPerGoroutine action events.
	total := sr.TimelineCount()
	assert.GreaterOrEqual(t, total, goroutines*opsPerGoroutine*2)
}
