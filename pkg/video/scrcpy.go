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
// INCREASED: 16Mbps for high-quality frame extraction
const defaultBitRate = 16_000_000

// defaultMaxSecs is the default maximum recording duration
// in seconds (3 minutes).
const defaultMaxSecs = 180

// ScrcpyRecorder records the screen of an Android device
// using scrcpy or adb screenrecord.
//
// FIX-QA-2026-04-21-012: Android's `screenrecord` has a hard 180-second
// time-limit (enforced by the platform). A 2-hour autonomous QA
// session used to produce a single 3-minute segment and 1h57m of
// nothing. Start now spawns a goroutine that loops screenrecord with
// numbered segments; Stop concatenates all segments into outputPath
// via ffmpeg. Segments live under <outputPath>.segments/ and are
// cleaned up after the concat succeeds.
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

	// Loop-mode state for MethodADBScreenrecord. The goroutine
	// restarts screenrecord every `maxSecs` seconds until loopCancel
	// is signalled.
	loopCtx       context.Context
	loopCancel    context.CancelFunc
	loopDone      chan struct{}
	segments      []string
	segmentsDir   string
	segmentNumber int
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
// (resolving MethodAuto), builds the command, and starts the
// subprocess. For MethodADBScreenrecord it spawns a loop goroutine
// that keeps re-invoking screenrecord past Android's 180-second
// per-invocation cap (see FIX-QA-2026-04-21-012). Returns an error
// if already recording or if the command cannot be started.
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

	switch method {
	case MethodScrcpy:
		args := r.buildScrcpyArgs()
		cmd := exec.CommandContext(ctx, "scrcpy", args...)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start scrcpy recording: %w", err)
		}
		r.cmd = cmd

	case MethodADBScreenrecord:
		// FIX-QA-2026-04-21-012: screenrecord caps at 180s on the
		// device. Run it in a loop so a 2-hour session actually
		// records 2 hours of video (as a sequence of segments that
		// Stop concatenates at the end).
		//
		// FIX-QA-2026-04-21-014: loop lifetime MUST be bound to
		// Stop(), not to the caller's ctx. A bug in an earlier cut
		// chained loopCtx to the phase ctx passed into Start(); when
		// Execute phase ended its ctx was cancelled and every new
		// CommandContext returned exit=1 instantly, causing a
		// 20,000+-iteration spin in 4 minutes. Bind to Background so
		// the loop survives phase transitions and only exits on Stop.
		r.segmentsDir = r.outputPath + ".segments"
		if err := os.MkdirAll(r.segmentsDir, 0o755); err != nil {
			return fmt.Errorf("mkdir segments dir: %w", err)
		}
		r.loopCtx, r.loopCancel = context.WithCancel(context.Background())
		r.loopDone = make(chan struct{})
		r.segments = nil
		r.segmentNumber = 0
		go r.runSegmentLoop()

	default:
		return fmt.Errorf(
			"unsupported record method: %d", method,
		)
	}

	r.recording = true
	r.startedAt = time.Now()
	return nil
}

// runSegmentLoop re-invokes `adb shell screenrecord` repeatedly,
// pulling each segment to the host and appending it to r.segments.
// Exits when r.loopCtx is cancelled or after too many consecutive
// failures.
//
// Safety fuses added after the 2026-04-21 X-cycle incident where an
// earlier version spun 20,000+ failed segments in 4 minutes because
// each exec.Run returned exit 1 instantly (adb error) and there was
// no backoff. Now: minimum 2-second floor per iteration, exponential
// backoff on failure, hard ceiling of 5 consecutive failures.
func (r *ScrcpyRecorder) runSegmentLoop() {
	defer close(r.loopDone)

	consecutiveFailures := 0
	const maxConsecutiveFailures = 5

	for {
		if r.loopCtx.Err() != nil {
			return
		}
		if consecutiveFailures >= maxConsecutiveFailures {
			fmt.Printf(
				"  [video] segment loop: %d consecutive failures, giving up\n",
				consecutiveFailures,
			)
			return
		}

		iterStart := time.Now()

		r.mu.Lock()
		r.segmentNumber++
		segN := r.segmentNumber
		r.mu.Unlock()

		devicePath := fmt.Sprintf(
			"/sdcard/helixqa_record_%03d.mp4", segN,
		)
		hostPath := fmt.Sprintf(
			"%s/%03d.mp4", r.segmentsDir, segN,
		)

		args := []string{
			"-s", r.device, "shell", "screenrecord",
			"--bit-rate", fmt.Sprintf("%d", r.bitRate),
			"--size", "1920x1080",
			"--time-limit", fmt.Sprintf("%d", r.maxSecs),
			devicePath,
		}
		cmd := exec.CommandContext(r.loopCtx, "adb", args...)
		r.mu.Lock()
		r.cmd = cmd
		r.mu.Unlock()

		out, runErr := cmd.CombinedOutput()
		iterElapsed := time.Since(iterStart)

		// An immediate (sub-second) exit means screenrecord never
		// ran — almost certainly adb error (device offline, file
		// perms, encoder issue). Back off exponentially and bump
		// the consecutive-failure counter.
		if runErr != nil && iterElapsed < 2*time.Second {
			if r.loopCtx.Err() != nil {
				return
			}
			consecutiveFailures++
			backoff := time.Duration(consecutiveFailures) * 2 * time.Second
			fmt.Printf(
				"  [video] segment %d failed in %v: %v — %s (backoff %v, fail %d/%d)\n",
				segN, iterElapsed, runErr,
				strings.TrimSpace(string(out)),
				backoff, consecutiveFailures, maxConsecutiveFailures,
			)
			select {
			case <-r.loopCtx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		// Any real run (successful or interrupted after some time
		// on the device) resets the failure counter.
		consecutiveFailures = 0

		// Pull this segment to the host, even on context cancel,
		// because the device wrote at least a partial MP4.
		pullCtx, pullCancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		pullCmd := exec.CommandContext(pullCtx, "adb",
			"-s", r.device, "pull", devicePath, hostPath)
		pullErr := pullCmd.Run()
		pullCancel()

		if pullErr == nil {
			if info, err := os.Stat(hostPath); err == nil &&
				info.Size() > 20*1024 {
				r.mu.Lock()
				r.segments = append(r.segments, hostPath)
				r.mu.Unlock()
			}
		}
		_ = exec.Command("adb", "-s", r.device,
			"shell", "rm", "-f", devicePath).Run()
	}
}

// Stop terminates an active recording. For MethodScrcpy it SIGINTs
// scrcpy and waits. For MethodADBScreenrecord it signals the segment
// loop goroutine to exit, waits for the in-flight segment to finalize,
// and concatenates all pulled segments into outputPath via ffmpeg.
// Returns an error if no recording is active.
func (r *ScrcpyRecorder) Stop() error {
	r.mu.Lock()

	if !r.recording {
		r.mu.Unlock()
		return fmt.Errorf("no recording in progress")
	}

	method := r.method
	if method == MethodAuto {
		method = r.detectMethod()
	}

	switch method {
	case MethodScrcpy:
		if r.cmd != nil && r.cmd.Process != nil {
			if err := r.cmd.Process.Signal(sigINT()); err != nil {
				_ = r.cmd.Process.Kill()
			}
		}
		cmd := r.cmd
		r.mu.Unlock()
		if cmd != nil {
			_ = cmd.Wait()
		}
		r.mu.Lock()

	case MethodADBScreenrecord:
		// Cancel the segment loop's context first (stops the
		// next iteration) and also kill the current adb command
		// so the in-flight segment exits promptly.
		if r.loopCancel != nil {
			r.loopCancel()
		}
		if r.cmd != nil && r.cmd.Process != nil {
			_ = r.cmd.Process.Signal(sigINT())
		}
		// Make sure screenrecord on the device flushes the
		// moov atom before we pull the last segment.
		_ = exec.Command("adb", "-s", r.device,
			"shell", "killall", "-INT", "screenrecord").Run()
		done := r.loopDone
		segments := append([]string(nil), r.segments...)
		outPath := r.outputPath
		segDir := r.segmentsDir
		r.mu.Unlock()

		// Wait for the loop goroutine to finish pulling the
		// final segment.
		if done != nil {
			<-done
		}

		r.mu.Lock()
		// Refresh segment list after the loop exited.
		segments = append([]string(nil), r.segments...)
		r.mu.Unlock()

		if len(segments) > 0 {
			if err := concatSegmentsFFmpeg(
				segments, outPath,
			); err != nil {
				fmt.Printf(
					"  [video] concat %d segments failed: %v\n",
					len(segments), err,
				)
			} else {
				if info, err := os.Stat(outPath); err == nil {
					fmt.Printf(
						"  [video] concatenated %d segments → %s (%d bytes)\n",
						len(segments), outPath, info.Size(),
					)
				}
				// Clean segments dir on success.
				_ = os.RemoveAll(segDir)
			}
		} else {
			fmt.Printf(
				"  [video] WARNING: no segments pulled for %s\n",
				outPath,
			)
		}
		r.mu.Lock()
	}

	r.recording = false
	r.cmd = nil
	r.mu.Unlock()
	return nil
}

// concatSegmentsFFmpeg uses ffmpeg's `concat` demuxer to stitch all
// MP4 segments into a single output without re-encoding. The concat
// list is written to a temp file and removed on exit.
func concatSegmentsFFmpeg(segments []string, outputPath string) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to concatenate")
	}
	// Single segment: just rename.
	if len(segments) == 1 {
		return os.Rename(segments[0], outputPath)
	}
	listFile, err := os.CreateTemp("", "helixqa-concat-*.txt")
	if err != nil {
		return fmt.Errorf("create concat list: %w", err)
	}
	defer os.Remove(listFile.Name())
	for _, seg := range segments {
		abs, absErr := os.Readlink(seg)
		if absErr != nil {
			abs = seg
		}
		fmt.Fprintf(listFile, "file '%s'\n", abs)
	}
	listFile.Close()

	cmd := exec.Command("ffmpeg",
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "concat", "-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy", outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg concat: %w: %s",
			err, strings.TrimSpace(string(out)))
	}
	return nil
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
// INCREASED QUALITY: Using 16Mbps bitrate for high-quality frame extraction.
func (r *ScrcpyRecorder) buildADBArgs() []string {
	return []string{
		"-s", r.device,
		"shell", "screenrecord",
		"--bit-rate", fmt.Sprintf("%d", r.bitRate),
		"--size", "1920x1080", // Full HD for better frame extraction
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
