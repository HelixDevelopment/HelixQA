// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package detector provides real-time crash and ANR detection
// during test execution. It supports Android (ADB), web
// (browser process), and desktop (JVM process) platforms.
package detector

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// DetectionResult captures the outcome of a crash/ANR check.
type DetectionResult struct {
	// Platform is the platform that was checked.
	Platform config.Platform `json:"platform"`

	// HasCrash indicates a crash was detected.
	HasCrash bool `json:"has_crash"`

	// HasANR indicates an ANR was detected (Android only).
	HasANR bool `json:"has_anr"`

	// ProcessAlive indicates whether the target process is
	// still running.
	ProcessAlive bool `json:"process_alive"`

	// StackTrace contains the crash stack trace if available.
	StackTrace string `json:"stack_trace,omitempty"`

	// LogEntries contains relevant log lines leading to the
	// detection.
	LogEntries []string `json:"log_entries,omitempty"`

	// ScreenshotPath is the path to a screenshot taken at
	// detection time.
	ScreenshotPath string `json:"screenshot_path,omitempty"`

	// Timestamp is when the detection was performed.
	Timestamp time.Time `json:"timestamp"`

	// Error contains any error encountered during detection.
	Error string `json:"error,omitempty"`
}

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Detector performs crash and ANR detection for a specific
// platform configuration.
type Detector struct {
	platform    config.Platform
	device      string
	packageName string
	browserURL  string
	processName string
	processPID  int
	evidenceDir string
	cmdRunner   CommandRunner
}

// Option configures a Detector.
type Option func(*Detector)

// WithDevice sets the Android device/emulator ID.
func WithDevice(device string) Option {
	return func(d *Detector) {
		d.device = device
	}
}

// WithPackageName sets the Android package name.
func WithPackageName(pkg string) Option {
	return func(d *Detector) {
		d.packageName = pkg
	}
}

// WithBrowserURL sets the web browser URL to monitor.
func WithBrowserURL(url string) Option {
	return func(d *Detector) {
		d.browserURL = url
	}
}

// WithProcessName sets the desktop process name to monitor.
func WithProcessName(name string) Option {
	return func(d *Detector) {
		d.processName = name
	}
}

// WithProcessPID sets the desktop process PID to monitor.
func WithProcessPID(pid int) Option {
	return func(d *Detector) {
		d.processPID = pid
	}
}

// WithEvidenceDir sets the directory for saving evidence.
func WithEvidenceDir(dir string) Option {
	return func(d *Detector) {
		d.evidenceDir = dir
	}
}

// WithCommandRunner sets a custom command runner (for testing).
func WithCommandRunner(runner CommandRunner) Option {
	return func(d *Detector) {
		d.cmdRunner = runner
	}
}

// New creates a Detector for the specified platform.
func New(platform config.Platform, opts ...Option) *Detector {
	d := &Detector{
		platform:    platform,
		evidenceDir: "evidence",
		cmdRunner:   &execRunner{},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Check performs a crash/ANR detection check for the
// configured platform.
func (d *Detector) Check(ctx context.Context) (*DetectionResult, error) {
	switch d.platform {
	case config.PlatformAndroid:
		return d.checkAndroid(ctx)
	case config.PlatformWeb:
		return d.checkWeb(ctx)
	case config.PlatformDesktop:
		return d.checkDesktop(ctx)
	default:
		return nil, fmt.Errorf(
			"unsupported platform for detection: %s",
			d.platform,
		)
	}
}

// CheckApp dispatches to the platform-specific check based on
// the platform argument, allowing runtime platform selection.
// Unlike Check(), this method does not mutate the detector's
// configured platform, making it safe for concurrent use.
func (d *Detector) CheckApp(
	ctx context.Context,
	platform config.Platform,
) (*DetectionResult, error) {
	switch platform {
	case config.PlatformAndroid:
		return d.checkAndroid(ctx)
	case config.PlatformWeb:
		return d.checkWeb(ctx)
	case config.PlatformDesktop:
		return d.checkDesktop(ctx)
	default:
		return nil, fmt.Errorf(
			"unsupported platform for detection: %s",
			platform,
		)
	}
}

// Platform returns the configured platform.
func (d *Detector) Platform() config.Platform {
	return d.platform
}
