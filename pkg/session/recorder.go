// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// Screenshot represents a captured screenshot with metadata.
type Screenshot struct {
	// Path is the file path to the screenshot.
	Path string `json:"path"`

	// Platform identifies the platform.
	Platform string `json:"platform"`

	// Name is a descriptive name.
	Name string `json:"name"`

	// Index is the sequential screenshot number.
	Index int `json:"index"`

	// Timestamp is when the screenshot was captured.
	Timestamp time.Time `json:"timestamp"`

	// VideoOffset is the offset into the platform video.
	VideoOffset time.Duration `json:"video_offset"`
}

// SessionRecorder coordinates video recording, screenshot
// capture, and timeline event tracking across multiple
// platforms during an autonomous QA session.
type SessionRecorder struct {
	sessionID     string
	outputDir     string
	videos        map[string]*VideoManager
	timeline      *Timeline
	screenshotIdx int
	mu            sync.Mutex
}

// NewSessionRecorder creates a recorder for the given session
// and output directory.
func NewSessionRecorder(sessionID, outputDir string) *SessionRecorder {
	return &SessionRecorder{
		sessionID: sessionID,
		outputDir: outputDir,
		videos:    make(map[string]*VideoManager),
		timeline:  NewTimeline(),
	}
}

// SessionID returns the session identifier.
func (sr *SessionRecorder) SessionID() string {
	return sr.sessionID
}

// OutputDir returns the output directory.
func (sr *SessionRecorder) OutputDir() string {
	return sr.outputDir
}

// StartRecording begins video recording for the given platform.
func (sr *SessionRecorder) StartRecording(
	platform string,
) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if _, exists := sr.videos[platform]; exists {
		return fmt.Errorf(
			"session: recording already initialized for %s",
			platform,
		)
	}

	videoPath := filepath.Join(
		sr.outputDir,
		"videos",
		fmt.Sprintf("%s-%s.mp4", platform, sr.sessionID),
	)
	vm := NewVideoManager(platform, videoPath)
	if err := vm.Start(); err != nil {
		return err
	}
	sr.videos[platform] = vm

	sr.timeline.RecordEvent(TimelineEvent{
		Type:        EventPhaseChange,
		Platform:    platform,
		Description: fmt.Sprintf("Video recording started for %s", platform),
	})

	return nil
}

// StopRecording stops video recording for the given platform
// and returns the video output path.
func (sr *SessionRecorder) StopRecording(
	platform string,
) (string, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	vm, exists := sr.videos[platform]
	if !exists {
		return "", fmt.Errorf(
			"session: no recording found for platform %s",
			platform,
		)
	}

	path, err := vm.Stop()
	if err != nil {
		return "", err
	}

	sr.timeline.RecordEvent(TimelineEvent{
		Type:        EventPhaseChange,
		Platform:    platform,
		Description: fmt.Sprintf("Video recording stopped for %s", platform),
	})

	return path, nil
}

// CaptureScreenshot records a screenshot event and returns
// Screenshot metadata. The caller is responsible for actually
// capturing the screenshot bytes and writing them to the
// returned path.
func (sr *SessionRecorder) CaptureScreenshot(
	platform, name string,
) Screenshot {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.screenshotIdx++
	idx := sr.screenshotIdx

	path := filepath.Join(
		sr.outputDir,
		"screenshots",
		platform,
		fmt.Sprintf("%04d-%s.png", idx, name),
	)

	var offset time.Duration
	if vm, ok := sr.videos[platform]; ok {
		offset = vm.Offset()
	}

	now := time.Now()
	ss := Screenshot{
		Path:        path,
		Platform:    platform,
		Name:        name,
		Index:       idx,
		Timestamp:   now,
		VideoOffset: offset,
	}

	sr.timeline.RecordEvent(TimelineEvent{
		Type:           EventScreenshot,
		Platform:       platform,
		Timestamp:      now,
		VideoOffset:    offset,
		Description:    fmt.Sprintf("Screenshot: %s", name),
		ScreenshotPath: path,
	})

	return ss
}

// RecordEvent records a timeline event. This is a pass-through
// to the underlying Timeline, with automatic video offset
// calculation if the platform is being recorded.
func (sr *SessionRecorder) RecordEvent(event TimelineEvent) {
	sr.mu.Lock()
	if vm, ok := sr.videos[event.Platform]; ok {
		event.VideoOffset = vm.Offset()
	}
	sr.mu.Unlock()

	sr.timeline.RecordEvent(event)
}

// VideoTimestamp returns the current video offset for the
// given platform. Returns zero if no recording is active.
func (sr *SessionRecorder) VideoTimestamp(
	platform string,
) time.Duration {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if vm, ok := sr.videos[platform]; ok {
		return vm.Offset()
	}
	return 0
}

// ExportTimeline returns all recorded timeline events.
func (sr *SessionRecorder) ExportTimeline() []TimelineEvent {
	return sr.timeline.Events()
}

// TimelineCount returns the number of timeline events.
func (sr *SessionRecorder) TimelineCount() int {
	return sr.timeline.Count()
}

// ScreenshotCount returns the total number of screenshots
// captured so far.
func (sr *SessionRecorder) ScreenshotCount() int {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.screenshotIdx
}

// VideoPlatforms returns the list of platforms with
// initialized video managers.
func (sr *SessionRecorder) VideoPlatforms() []string {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	platforms := make([]string, 0, len(sr.videos))
	for p := range sr.videos {
		platforms = append(platforms, p)
	}
	return platforms
}

// IsRecording returns whether video recording is active for
// the given platform.
func (sr *SessionRecorder) IsRecording(platform string) bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if vm, ok := sr.videos[platform]; ok {
		return vm.IsRecording()
	}
	return false
}
