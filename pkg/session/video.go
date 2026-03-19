// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"sync"
	"time"
)

// VideoManager tracks video recording state for a single
// platform. It does not execute ffmpeg/adb directly; the
// actual command execution is delegated to the caller or to
// an ActionExecutor. VideoManager only tracks timing and
// state so that timeline events can reference video offsets.
type VideoManager struct {
	// platform identifies which platform this video covers.
	platform string

	// outputPath is the path where the video will be saved.
	outputPath string

	// startedAt is when recording began.
	startedAt time.Time

	// recording indicates whether recording is active.
	recording bool

	mu sync.Mutex
}

// NewVideoManager creates a VideoManager for the given platform
// and output path.
func NewVideoManager(platform, outputPath string) *VideoManager {
	return &VideoManager{
		platform:   platform,
		outputPath: outputPath,
	}
}

// Start marks recording as started. Returns an error if
// already recording.
func (vm *VideoManager) Start() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.recording {
		return fmt.Errorf(
			"video: already recording for platform %s",
			vm.platform,
		)
	}
	vm.recording = true
	vm.startedAt = time.Now()
	return nil
}

// Stop marks recording as stopped and returns the output path.
// Returns an error if not currently recording.
func (vm *VideoManager) Stop() (string, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.recording {
		return "", fmt.Errorf(
			"video: not recording for platform %s",
			vm.platform,
		)
	}
	vm.recording = false
	return vm.outputPath, nil
}

// IsRecording returns whether recording is active.
func (vm *VideoManager) IsRecording() bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.recording
}

// Offset returns the current offset from when recording
// started. Returns zero if not recording.
func (vm *VideoManager) Offset() time.Duration {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.recording {
		return 0
	}
	return time.Since(vm.startedAt)
}

// Platform returns the platform identifier.
func (vm *VideoManager) Platform() string {
	return vm.platform
}

// OutputPath returns the video output path.
func (vm *VideoManager) OutputPath() string {
	return vm.outputPath
}

// StartedAt returns when recording started. Returns zero
// time if never started.
func (vm *VideoManager) StartedAt() time.Time {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.startedAt
}
