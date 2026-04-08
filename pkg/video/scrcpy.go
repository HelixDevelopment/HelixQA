// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package video provides screen recording capabilities for
// Android devices. It supports scrcpy-based recording,
// ADB screenrecord, and screenshot-assembly methods.
package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// sigINT returns os.Signal for SIGINT. Separated for clarity.
func sigINT() os.Signal { return syscall.SIGINT }

// RecordMethod identifies the recording strategy.
type RecordMethod int

const (
	// MethodAuto selects the best available method
	// automatically (prefers scrcpy, falls back to adb).
	MethodAuto RecordMethod = iota

	// MethodScrcpy uses scrcpy to record to a file.
	MethodScrcpy

	// MethodADBScreenrecord uses `adb shell screenrecord`.
	MethodADBScreenrecord

	// MethodScreenshotAssembly assembles a video from
	// individual screenshots captured over time.
	MethodScreenshotAssembly
)

// defaultBitRate is the default recording bit rate in bps.
const defaultBitRate = 4_000_000

// defaultMaxSecs is the default maximum recording duration
// in seconds (3 minutes).
const defaultMaxSecs = 180

// ScrcpyRecorder records the screen of an Android device
// using scrcpy or adb screenrecord.
type ScrcpyRecorder struct {
	device     string
	outputPath string
	method     RecordMethod
	bitRate    int
	maxSecs    int
	cmd        *exec.Cmd
	recording  bool
	startedAt  time.Time
	mu         sync.Mutex
}

// RecorderOption configures a ScrcpyRecorder.
type RecorderOption func(*ScrcpyRecorder)

// WithMethod sets the recording method.
func WithMethod(m RecordMethod) RecorderOption {
	return func(r *ScrcpyRecorder) {
		r.method = m
	}
}

// WithBitRate sets the video bit rate in bits per second.
// Default is 4 Mbps.
func WithBitRate(bps int) RecorderOption {
	return func(r *ScrcpyRecorder) {
		if bps > 0 {
			r.bitRate = bps
		}
	}
}

// WithMaxDuration sets the maximum recording duration in
// seconds. Default is 180 seconds.
func WithMaxDuration(secs int) RecorderOption {
	return func(r *ScrcpyRecorder) {
		if secs > 0 {
			r.maxSecs = secs
		}
	}
}

// NewScrcpyRecorder creates a recorder for the given device
// that writes output to outputPath.
func NewScrcpyRecorder(
	device, outputPath string,
	opts ...RecorderOption,
) *ScrcpyRecorder {
	r := &ScrcpyRecorder{
		device:     device,
		outputPath: outputPath,
		method:     MethodAuto,
		bitRate:    defaultBitRate,
		maxSecs:    defaultMaxSecs,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Device returns the target device serial.
func (r *ScrcpyRecorder) Device() string {
	return r.device
}

// OutputPath returns the file path for the recorded video.
func (r *ScrcpyRecorder) OutputPath() string {
	return r.outputPath
}

// IsRecording reports whether a recording is in progress.
func (r *ScrcpyRecorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// Start begins recording. It selects the recording method
// (resolving MethodAuto), builds the command, and starts
// the subprocess. Returns an error if already recording
// or if the command cannot be started.
func (r *ScrcpyRecorder) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("recording already in progress")
	}

	method := r.method
	if method == MethodAuto {
		method = r.detectMethod()
	}

	var cmd *exec.Cmd
	switch method {
	case MethodScrcpy:
		args := r.buildScrcpyArgs()
		cmd = exec.CommandContext(ctx, "scrcpy", args...)
	case MethodADBScreenrecord:
		args := r.buildADBArgs()
		cmd = exec.CommandContext(ctx, "adb", args...)
	default:
		return fmt.Errorf(
			"unsupported record method: %d", method,
		)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start recording: %w", err)
	}

	r.cmd = cmd
	r.recording = true
	r.startedAt = time.Now()
	return nil
}

// Stop terminates an active recording. For ADB screenrecord,
// it sends SIGINT to produce a valid MP4 file, then pulls the
// recording from the device to the local output path. Returns
// an error if no recording is active.
func (r *ScrcpyRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return fmt.Errorf("no recording in progress")
	}

	var stopErr error
	if r.cmd != nil && r.cmd.Process != nil {
		// Send SIGINT (not SIGKILL) so screenrecord
		// flushes the moov atom and produces a valid
		// MP4 file.
		if err := r.cmd.Process.Signal(
			sigINT(),
		); err != nil {
			// Fallback to Kill if Signal fails.
			_ = r.cmd.Process.Kill()
		}
		// Wait to reap the process; ignore exit error.
		_ = r.cmd.Wait()
	}

	// For ADB screenrecord, pull the file from device.
	if r.method == MethodADBScreenrecord ||
		(r.method == MethodAuto &&
			r.detectMethod() == MethodADBScreenrecord) {
		r.pullFromDevice()
	}

	r.recording = false
	r.cmd = nil
	return stopErr
}

// pullFromDevice copies the recording from the Android
// device to the local output path, then removes the
// device-side file.
func (r *ScrcpyRecorder) pullFromDevice() {
	devicePath := "/sdcard/helixqa_record.mp4"

	// CRITICAL: Kill the remote screenrecord process on the
	// device. Sending SIGINT to the local adb process only
	// disconnects the adb session — it does NOT stop the
	// screenrecord process running on the device. Without
	// this kill, the file is still being written when we
	// try to pull it, resulting in truncated/empty MP4.
	killCmd := exec.Command(
		"adb", "-s", r.device,
		"shell", "killall", "-INT", "screenrecord",
	)
	if out, err := killCmd.CombinedOutput(); err != nil {
		// Fallback: try SIGTERM if SIGINT fails.
		killFallback := exec.Command(
			"adb", "-s", r.device,
			"shell", "killall", "screenrecord",
		)
		_ = killFallback.Run()
		fmt.Printf(
			"  [video] killall -INT failed (%v: %s), used SIGTERM fallback\n",
			err, strings.TrimSpace(string(out)),
		)
	}

	// Wait for screenrecord to flush the moov atom and
	// finalize the MP4 file on the device.
	// REDUCED for FLASHING FAST performance (was 3s).
	time.Sleep(1 * time.Second)

	pull := exec.Command(
		"adb", "-s", r.device,
		"pull", devicePath, r.outputPath,
	)
	if out, err := pull.CombinedOutput(); err != nil {
		fmt.Printf(
			"  [video] pull failed: %v: %s\n",
			err, string(out),
		)
		return
	}

	// Verify the pulled file is not trivially small.
	if info, err := os.Stat(r.outputPath); err == nil {
		if info.Size() < 50*1024 { // < 50KB is suspicious
			fmt.Printf(
				"  [video] WARNING: recording is only %d bytes — may be incomplete\n",
				info.Size(),
			)
		} else {
			fmt.Printf(
				"  [video] pulled recording to %s (%d bytes)\n",
				r.outputPath, info.Size(),
			)
		}
	}

	// Clean up device-side file.
	rm := exec.Command(
		"adb", "-s", r.device,
		"shell", "rm", "-f", devicePath,
	)
	_ = rm.Run()
}

// Duration returns the elapsed recording time. Returns
// zero if no recording has been started.
func (r *ScrcpyRecorder) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.startedAt.IsZero() {
		return 0
	}
	return time.Since(r.startedAt)
}

// buildScrcpyArgs constructs the argument slice for the
// scrcpy command.
func (r *ScrcpyRecorder) buildScrcpyArgs() []string {
	return []string{
		"--serial", r.device,
		"--record", r.outputPath,
		"--video-bit-rate", fmt.Sprintf("%d", r.bitRate),
		"--max-size", "0",
		"--no-audio",
	}
}

// buildADBArgs constructs the argument slice for the
// `adb shell screenrecord` command.
func (r *ScrcpyRecorder) buildADBArgs() []string {
	return []string{
		"-s", r.device,
		"shell", "screenrecord",
		"--bit-rate", fmt.Sprintf("%d", r.bitRate),
		"--time-limit", fmt.Sprintf("%d", r.maxSecs),
		"/sdcard/helixqa_record.mp4",
	}
}

// detectMethod probes the host PATH for scrcpy. If found,
// returns MethodScrcpy; otherwise MethodADBScreenrecord.
func (r *ScrcpyRecorder) detectMethod() RecordMethod {
	if _, err := exec.LookPath("scrcpy"); err == nil {
		return MethodScrcpy
	}
	return MethodADBScreenrecord
}
