// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// FFmpegRecorder records the screen using ffmpeg for web and desktop platforms.
// It captures the X11 display (Linux) and saves to an MP4 file.
type FFmpegRecorder struct {
	outputPath string
	cmd        *exec.Cmd
	recording  bool
	startedAt  time.Time
	mu         sync.Mutex
}

// NewFFmpegRecorder creates a new ffmpeg-based screen recorder.
func NewFFmpegRecorder(outputPath string) *FFmpegRecorder {
	return &FFmpegRecorder{
		outputPath: outputPath,
	}
}

// Start begins screen recording using ffmpeg.
// It captures the primary X11 display at 30fps with H.264 encoding.
func (r *FFmpegRecorder) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("recording already in progress")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(r.outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Build ffmpeg command for X11 capture
	// -f x11grab: X11 screen capture
	// -i :0.0: Display :0, screen 0
	// -r 30: 30fps
	// -c:v libx264: H.264 codec
	// -preset ultrafast: Fast encoding
	// -crf 23: Quality (lower = better)
	// -pix_fmt yuv420p: Compatibility
	args := []string{
		"-f", "x11grab",
		"-video_size", "1920x1080",
		"-i", ":0.0",
		"-r", "30",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-y", // Overwrite output
		r.outputPath,
	}

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	r.cmd = exec.CommandContext(ctx, "ffmpeg", args...)

	// Redirect output to avoid cluttering console
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	r.recording = true
	r.startedAt = time.Now()
	fmt.Printf("  [video] ffmpeg recording started: %s\n", r.outputPath)
	return nil
}

// Stop terminates the ffmpeg recording.
// It sends SIGINT to gracefully stop the recording.
func (r *FFmpegRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("no recording in progress")
	}

	if r.cmd != nil && r.cmd.Process != nil {
		// Send SIGINT to gracefully stop ffmpeg
		if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
			// Fallback to kill
			_ = r.cmd.Process.Kill()
		}

		// Wait for ffmpeg to finish
		done := make(chan error, 1)
		go func() {
			done <- r.cmd.Wait()
		}()

		select {
		case <-done:
			// Process finished
		case <-time.After(5 * time.Second):
			// Timeout, force kill
			_ = r.cmd.Process.Kill()
		}
	}

	r.recording = false
	r.cmd = nil

	// Verify the output file
	if info, err := os.Stat(r.outputPath); err == nil {
		if info.Size() < 10*1024 { // < 10KB is suspicious
			fmt.Printf("  [video] WARNING: recording is only %d bytes - may be incomplete\n", info.Size())
		} else {
			fmt.Printf("  [video] recording saved: %s (%d KB, %v)\n",
				r.outputPath, info.Size()/1024, time.Since(r.startedAt).Round(time.Second))
		}
	}

	return nil
}

// IsRecording reports whether recording is in progress.
func (r *FFmpegRecorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// Duration returns the elapsed recording time.
func (r *FFmpegRecorder) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.startedAt.IsZero() {
		return 0
	}
	return time.Since(r.startedAt)
}
