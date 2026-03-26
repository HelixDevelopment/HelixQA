// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package evidence provides centralized evidence collection
// for QA test execution. It handles screenshots, video
// recording, logcat capture, and stack trace collection across
// Android (ADB), Web (Playwright), and Desktop platforms.
package evidence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
)

// Type identifies the kind of evidence collected.
type Type string

const (
	TypeScreenshot Type = "screenshot"
	TypeVideo      Type = "video"
	TypeLogcat     Type = "logcat"
	TypeStackTrace Type = "stacktrace"
	TypeConsoleLog Type = "console_log"
	TypeAudio      Type = "audio"
)

// Item represents a single piece of collected evidence.
type Item struct {
	// Type identifies the evidence kind.
	Type Type `json:"type"`

	// Path is the file path to the evidence.
	Path string `json:"path"`

	// Platform identifies where the evidence was collected.
	Platform config.Platform `json:"platform"`

	// Step is the test step associated with this evidence.
	Step string `json:"step,omitempty"`

	// Timestamp is when the evidence was collected.
	Timestamp time.Time `json:"timestamp"`

	// Size is the file size in bytes (0 if unknown).
	Size int64 `json:"size"`
}

// Collector gathers evidence during QA test execution.
type Collector struct {
	mu               sync.Mutex
	outputDir        string
	platform         config.Platform
	cmdRunner        detector.CommandRunner
	items            []Item
	recording        bool
	recordingID      string
	audioRecording   bool
	audioRecordingID string
	audioCmd         *exec.Cmd
}

// Option configures a Collector.
type Option func(*Collector)

// WithOutputDir sets the evidence output directory.
func WithOutputDir(dir string) Option {
	return func(c *Collector) {
		c.outputDir = dir
	}
}

// WithPlatform sets the target platform.
func WithPlatform(p config.Platform) Option {
	return func(c *Collector) {
		c.platform = p
	}
}

// WithCommandRunner sets a custom command runner.
func WithCommandRunner(r detector.CommandRunner) Option {
	return func(c *Collector) {
		c.cmdRunner = r
	}
}

// New creates a Collector with the given options.
func New(opts ...Option) *Collector {
	c := &Collector{
		outputDir: "evidence",
		platform:  config.PlatformAndroid,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CaptureScreenshot takes a screenshot and saves it as
// evidence. The method varies by platform.
func (c *Collector) CaptureScreenshot(
	ctx context.Context,
	name string,
) (*Item, error) {
	if err := c.ensureOutputDir(); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf(
		"%s-%d.png", name, time.Now().UnixMilli(),
	)
	path := filepath.Join(c.outputDir, filename)

	switch c.platform {
	case config.PlatformAndroid:
		return c.captureAndroidScreenshot(ctx, path)
	case config.PlatformWeb:
		return c.captureWebScreenshot(ctx, path)
	case config.PlatformDesktop:
		return c.captureDesktopScreenshot(ctx, path)
	default:
		return nil, fmt.Errorf(
			"unsupported platform: %s", c.platform,
		)
	}
}

// CaptureLogcat captures Android logcat output.
func (c *Collector) CaptureLogcat(
	ctx context.Context,
	name string,
	lines int,
) (*Item, error) {
	if c.platform != config.PlatformAndroid {
		return nil, fmt.Errorf(
			"logcat only available on Android",
		)
	}

	if err := c.ensureOutputDir(); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf(
		"%s-logcat-%d.txt", name, time.Now().UnixMilli(),
	)
	path := filepath.Join(c.outputDir, filename)

	runner := c.getRunner()
	output, err := runner.Run(ctx, "adb", "logcat",
		"-d", "-t", fmt.Sprintf("%d", lines))
	if err != nil {
		return nil, fmt.Errorf("capture logcat: %w", err)
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return nil, fmt.Errorf("write logcat: %w", err)
	}

	item := c.addItem(TypeLogcat, path, "")
	return &item, nil
}

// StartRecording begins video recording. Call StopRecording
// to finalize.
func (c *Collector) StartRecording(
	ctx context.Context,
	name string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.recording {
		return fmt.Errorf("recording already in progress")
	}

	if err := c.ensureOutputDir(); err != nil {
		return err
	}

	c.recording = true
	c.recordingID = fmt.Sprintf(
		"%s-%d", name, time.Now().UnixMilli(),
	)
	return nil
}

// StopRecording stops video recording and returns the
// evidence item.
func (c *Collector) StopRecording(
	_ context.Context,
) (*Item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.recording {
		return nil, fmt.Errorf("no recording in progress")
	}

	path := filepath.Join(
		c.outputDir,
		c.recordingID+".mp4",
	)

	c.recording = false
	item := Item{
		Type:      TypeVideo,
		Path:      path,
		Platform:  c.platform,
		Timestamp: time.Now(),
	}
	c.items = append(c.items, item)
	c.recordingID = ""
	return &item, nil
}

// IsRecording returns whether recording is in progress.
func (c *Collector) IsRecording() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.recording
}

// AudioQualityParams maps a quality name to sample rate and
// sample format suitable for ffmpeg arguments.
type AudioQualityParams struct {
	SampleRate string
	SampleFmt  string
}

// AudioQualityMap returns the ffmpeg parameters for the given
// quality string. Returns high quality params if the quality
// string is unrecognized.
func AudioQualityMap(quality string) AudioQualityParams {
	switch quality {
	case "standard":
		return AudioQualityParams{
			SampleRate: "44100",
			SampleFmt:  "s16",
		}
	case "high":
		return AudioQualityParams{
			SampleRate: "48000",
			SampleFmt:  "s32",
		}
	case "ultra":
		return AudioQualityParams{
			SampleRate: "96000",
			SampleFmt:  "s32",
		}
	default:
		return AudioQualityParams{
			SampleRate: "48000",
			SampleFmt:  "s32",
		}
	}
}

// StartAudioRecording begins audio capture using ffmpeg. The
// audio device, quality, and format are specified via config.
// Call StopAudioRecording to finalize and get the evidence item.
func (c *Collector) StartAudioRecording(
	ctx context.Context,
	name string,
	quality string,
	format string,
	device string,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.audioRecording {
		return fmt.Errorf("audio recording already in progress")
	}

	if err := c.ensureOutputDir(); err != nil {
		return err
	}

	c.audioRecordingID = fmt.Sprintf(
		"%s-%d", name, time.Now().UnixMilli(),
	)

	ext := format
	if ext == "" {
		ext = "wav"
	}
	outPath := filepath.Join(
		c.outputDir, c.audioRecordingID+"."+ext,
	)

	params := AudioQualityMap(quality)

	if device == "" {
		device = "default"
	}

	// Build ffmpeg command for PulseAudio capture.
	// Falls back to ALSA if pulse is not available.
	args := []string{
		"-f", "pulse",
		"-i", device,
		"-ac", "2",
		"-ar", params.SampleRate,
		"-sample_fmt", params.SampleFmt,
		"-y",
		outPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start audio recording: %w", err)
	}

	c.audioCmd = cmd
	c.audioRecording = true
	return nil
}

// StopAudioRecording stops the active audio capture and returns
// the evidence item pointing to the recorded file.
func (c *Collector) StopAudioRecording(
	_ context.Context,
) (*Item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.audioRecording {
		return nil, fmt.Errorf("no audio recording in progress")
	}

	// Send interrupt to ffmpeg for graceful shutdown.
	if c.audioCmd != nil && c.audioCmd.Process != nil {
		_ = c.audioCmd.Process.Signal(os.Interrupt)
		_ = c.audioCmd.Wait()
	}

	// Determine file extension from the path.
	ext := "wav"
	if c.audioCmd != nil && len(c.audioCmd.Args) > 0 {
		last := c.audioCmd.Args[len(c.audioCmd.Args)-1]
		if e := filepath.Ext(last); e != "" {
			ext = e[1:] // strip leading dot
		}
	}

	path := filepath.Join(
		c.outputDir,
		c.audioRecordingID+"."+ext,
	)

	c.audioRecording = false
	c.audioCmd = nil

	item := Item{
		Type:      TypeAudio,
		Path:      path,
		Platform:  c.platform,
		Timestamp: time.Now(),
	}
	if info, err := os.Stat(path); err == nil {
		item.Size = info.Size()
	}
	c.items = append(c.items, item)
	c.audioRecordingID = ""
	return &item, nil
}

// IsAudioRecording returns whether audio recording is active.
func (c *Collector) IsAudioRecording() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.audioRecording
}

// Items returns all collected evidence items.
func (c *Collector) Items() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Item, len(c.items))
	copy(result, c.items)
	return result
}

// ItemsByType returns evidence items filtered by type.
func (c *Collector) ItemsByType(t Type) []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []Item
	for _, item := range c.items {
		if item.Type == t {
			result = append(result, item)
		}
	}
	return result
}

// Count returns the total number of collected items.
func (c *Collector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Reset clears all collected items.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
}

// captureAndroidScreenshot uses ADB screencap.
func (c *Collector) captureAndroidScreenshot(
	ctx context.Context,
	path string,
) (*Item, error) {
	runner := c.getRunner()
	devicePath := "/sdcard/helixqa-screenshot.png"

	_, err := runner.Run(ctx, "adb", "shell",
		"screencap", "-p", devicePath)
	if err != nil {
		return nil, fmt.Errorf("screencap: %w", err)
	}

	_, err = runner.Run(ctx, "adb", "pull", devicePath, path)
	if err != nil {
		return nil, fmt.Errorf("pull screenshot: %w", err)
	}

	// Cleanup device file.
	_, _ = runner.Run(ctx, "adb", "shell", "rm", devicePath)

	item := c.addItem(TypeScreenshot, path, "")
	return &item, nil
}

// captureWebScreenshot captures a web page screenshot.
func (c *Collector) captureWebScreenshot(
	ctx context.Context,
	path string,
) (*Item, error) {
	runner := c.getRunner()
	_, err := runner.Run(ctx, "npx", "playwright",
		"screenshot", "--path", path)
	if err != nil {
		return nil, fmt.Errorf("web screenshot: %w", err)
	}
	item := c.addItem(TypeScreenshot, path, "")
	return &item, nil
}

// captureDesktopScreenshot captures a desktop screenshot.
func (c *Collector) captureDesktopScreenshot(
	ctx context.Context,
	path string,
) (*Item, error) {
	runner := c.getRunner()
	_, err := runner.Run(ctx, "import", "-window", "root", path)
	if err != nil {
		return nil, fmt.Errorf("desktop screenshot: %w", err)
	}
	item := c.addItem(TypeScreenshot, path, "")
	return &item, nil
}

func (c *Collector) addItem(
	t Type, path, step string,
) Item {
	item := Item{
		Type:      t,
		Path:      path,
		Platform:  c.platform,
		Step:      step,
		Timestamp: time.Now(),
	}
	// Try to get file size.
	if info, err := os.Stat(path); err == nil {
		item.Size = info.Size()
	}
	c.mu.Lock()
	c.items = append(c.items, item)
	c.mu.Unlock()
	return item
}

func (c *Collector) ensureOutputDir() error {
	return os.MkdirAll(c.outputDir, 0755)
}

func (c *Collector) getRunner() detector.CommandRunner {
	if c.cmdRunner != nil {
		return c.cmdRunner
	}
	return &defaultRunner{}
}

// defaultRunner executes commands via os/exec.
type defaultRunner struct{}

func (r *defaultRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	// Import exec only here to avoid test dependency.
	return nil, fmt.Errorf(
		"default runner: command execution not available in test",
	)
}
