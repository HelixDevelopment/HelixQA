// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration types for HelixQA
// test orchestration runs. It defines the structure for
// specifying platforms, devices, speed modes, and output
// preferences.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Platform identifies a target testing platform.
type Platform string

const (
	// PlatformAndroid targets Android devices and emulators.
	PlatformAndroid Platform = "android"
	// PlatformWeb targets web browsers.
	PlatformWeb Platform = "web"
	// PlatformDesktop targets desktop (JVM) applications.
	PlatformDesktop Platform = "desktop"
	// PlatformAll targets all supported platforms.
	PlatformAll Platform = "all"
)

// SpeedMode controls the pacing of test execution.
type SpeedMode string

const (
	// SpeedSlow adds delays between steps for debugging.
	SpeedSlow SpeedMode = "slow"
	// SpeedNormal is the default execution speed.
	SpeedNormal SpeedMode = "normal"
	// SpeedFast minimizes delays for CI pipelines.
	SpeedFast SpeedMode = "fast"
)

// ReportFormat specifies the output format for QA reports.
type ReportFormat string

const (
	// ReportMarkdown generates Markdown reports.
	ReportMarkdown ReportFormat = "markdown"
	// ReportHTML generates HTML reports.
	ReportHTML ReportFormat = "html"
	// ReportJSON generates JSON reports.
	ReportJSON ReportFormat = "json"
)

// Config holds the complete configuration for a HelixQA run.
type Config struct {
	// Banks lists the paths to test bank files or directories.
	Banks []string `yaml:"banks" json:"banks"`

	// Platforms specifies which platforms to test.
	Platforms []Platform `yaml:"platforms" json:"platforms"`

	// Device is the device or emulator identifier for Android.
	Device string `yaml:"device" json:"device"`

	// PackageName is the Android application package name.
	PackageName string `yaml:"package_name" json:"package_name"`

	// OutputDir is the directory for results and evidence.
	OutputDir string `yaml:"output_dir" json:"output_dir"`

	// Speed controls execution pacing.
	Speed SpeedMode `yaml:"speed" json:"speed"`

	// ReportFormat selects the output report format.
	ReportFormat ReportFormat `yaml:"report_format" json:"report_format"`

	// ValidateSteps enables step-by-step validation with crash
	// detection between steps.
	ValidateSteps bool `yaml:"validate" json:"validate"`

	// Record enables video recording of test execution.
	Record bool `yaml:"record" json:"record"`

	// Verbose enables detailed logging output.
	Verbose bool `yaml:"verbose" json:"verbose"`

	// Timeout is the maximum duration for the entire run.
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// StepTimeout is the maximum duration for a single step.
	StepTimeout time.Duration `yaml:"step_timeout" json:"step_timeout"`

	// BrowserURL is the URL for web platform testing.
	BrowserURL string `yaml:"browser_url" json:"browser_url"`

	// DesktopProcess is the process name for desktop testing.
	DesktopProcess string `yaml:"desktop_process" json:"desktop_process"`

	// DesktopPID is the process ID for desktop testing. If set,
	// it takes precedence over DesktopProcess.
	DesktopPID int `yaml:"desktop_pid" json:"desktop_pid"`

	// Autonomous holds configuration for autonomous QA sessions.
	Autonomous AutonomousConfig `yaml:"autonomous" json:"autonomous"`
}

// AutonomousConfig holds configuration for autonomous QA
// sessions driven by LLM agents and computer vision.
type AutonomousConfig struct {
	// Enabled activates autonomous QA mode.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// CoverageTarget is the desired feature coverage (0-1).
	CoverageTarget float64 `yaml:"coverage_target" json:"coverage_target"`

	// CuriosityEnabled enables the curiosity-driven phase.
	CuriosityEnabled bool `yaml:"curiosity_enabled" json:"curiosity_enabled"`

	// CuriosityTimeout limits the curiosity phase.
	CuriosityTimeout time.Duration `yaml:"curiosity_timeout" json:"curiosity_timeout"`

	// AgentsEnabled lists which CLI agents to use.
	AgentsEnabled []string `yaml:"agents_enabled" json:"agents_enabled"`

	// AgentPoolSize is the number of agents in the pool.
	AgentPoolSize int `yaml:"agent_pool_size" json:"agent_pool_size"`

	// AgentTimeout is the timeout for agent operations.
	AgentTimeout time.Duration `yaml:"agent_timeout" json:"agent_timeout"`

	// AgentMaxRetries is the max retries per LLM call.
	AgentMaxRetries int `yaml:"agent_max_retries" json:"agent_max_retries"`

	// VisionProvider selects the vision provider ("auto",
	// "openai", "anthropic", "gemini", "qwen").
	VisionProvider string `yaml:"vision_provider" json:"vision_provider"`

	// VisionOpenCVEnabled enables OpenCV-based analysis.
	VisionOpenCVEnabled bool `yaml:"vision_opencv_enabled" json:"vision_opencv_enabled"`

	// VisionSSIMThreshold is the SSIM similarity threshold.
	VisionSSIMThreshold float64 `yaml:"vision_ssim_threshold" json:"vision_ssim_threshold"`

	// DocsRoot is the path to project documentation.
	DocsRoot string `yaml:"docs_root" json:"docs_root"`

	// DocsAutoDiscover enables automatic doc discovery.
	DocsAutoDiscover bool `yaml:"docs_auto_discover" json:"docs_auto_discover"`

	// DocsFormats lists supported documentation formats.
	DocsFormats []string `yaml:"docs_formats" json:"docs_formats"`

	// RecordingVideo enables video recording.
	RecordingVideo bool `yaml:"recording_video" json:"recording_video"`

	// RecordingScreenshots enables screenshot capture.
	RecordingScreenshots bool `yaml:"recording_screenshots" json:"recording_screenshots"`

	// RecordingVideoQuality sets video quality (low/medium/high).
	RecordingVideoQuality string `yaml:"recording_video_quality" json:"recording_video_quality"`

	// RecordingScreenshotFormat sets screenshot format (png/jpg).
	RecordingScreenshotFormat string `yaml:"recording_screenshot_format" json:"recording_screenshot_format"`

	// RecordingAudio enables audio recording during test
	// execution. Audio is captured as a separate high-quality
	// track for analysis of audio reproduction problems
	// (glitches, dropouts, distortion).
	RecordingAudio bool `yaml:"recording_audio" json:"recording_audio"`

	// RecordingAudioQuality sets audio recording quality.
	// Supported: "standard" (44.1kHz/16bit), "high"
	// (48kHz/24bit), "ultra" (96kHz/32bit). Default: "high".
	RecordingAudioQuality string `yaml:"recording_audio_quality" json:"recording_audio_quality"`

	// RecordingAudioFormat sets audio file format.
	// Supported: "wav" (lossless), "flac" (lossless compressed).
	// Default: "wav".
	RecordingAudioFormat string `yaml:"recording_audio_format" json:"recording_audio_format"`

	// RecordingAudioDevice is the audio input device for
	// host-side recording. Use "default" for system default, or
	// specify a PulseAudio/ALSA device name. For Android device
	// recording, use "adb" to capture via adb shell.
	RecordingAudioDevice string `yaml:"recording_audio_device" json:"recording_audio_device"`

	// RecordingFFmpegPath is the path to ffmpeg binary.
	RecordingFFmpegPath string `yaml:"recording_ffmpeg_path" json:"recording_ffmpeg_path"`

	// AndroidDevice is the ADB device/emulator ID.
	AndroidDevice string `yaml:"android_device" json:"android_device"`

	// AndroidPackage is the Android app package name.
	AndroidPackage string `yaml:"android_package" json:"android_package"`

	// WebURL is the URL for web testing.
	WebURL string `yaml:"web_url" json:"web_url"`

	// WebBrowser selects the browser (chromium/chrome/firefox).
	WebBrowser string `yaml:"web_browser" json:"web_browser"`

	// DesktopProcess is the desktop process name.
	DesktopProcess string `yaml:"desktop_process" json:"desktop_process"`

	// DesktopDisplay is the X11 display.
	DesktopDisplay string `yaml:"desktop_display" json:"desktop_display"`

	// ReportFormats lists output report formats.
	ReportFormats []string `yaml:"report_formats" json:"report_formats"`

	// TicketsEnabled enables ticket generation.
	TicketsEnabled bool `yaml:"tickets_enabled" json:"tickets_enabled"`

	// TicketsMinSeverity is the minimum severity for tickets.
	TicketsMinSeverity string `yaml:"tickets_min_severity" json:"tickets_min_severity"`
}

// DefaultAutonomousConfig returns sensible defaults for
// autonomous QA configuration.
func DefaultAutonomousConfig() AutonomousConfig {
	return AutonomousConfig{
		Enabled:               true,
		CoverageTarget:        0.90,
		CuriosityEnabled:      true,
		CuriosityTimeout:      30 * time.Minute,
		AgentsEnabled:         []string{"opencode", "claude-code", "gemini"},
		AgentPoolSize:         3,
		AgentTimeout:          60 * time.Second,
		AgentMaxRetries:       3,
		VisionProvider:        "auto",
		VisionOpenCVEnabled:   true,
		VisionSSIMThreshold:   0.95,
		DocsRoot:              "./docs",
		DocsAutoDiscover:      true,
		DocsFormats:           []string{"md", "yaml", "html", "adoc", "rst"},
		RecordingVideo:        true,
		RecordingScreenshots:  true,
		RecordingVideoQuality: "medium",
		RecordingScreenshotFormat: "png",
		RecordingAudio:            false,
		RecordingAudioQuality:     "high",
		RecordingAudioFormat:      "wav",
		RecordingAudioDevice:      "default",
		RecordingFFmpegPath:       "/usr/bin/ffmpeg",
		WebBrowser:            "chromium",
		DesktopDisplay:        ":0",
		ReportFormats:         []string{"markdown", "html", "json"},
		TicketsEnabled:        true,
		TicketsMinSeverity:    "low",
	}
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Platforms:    []Platform{PlatformAll},
		OutputDir:    "qa-results",
		Speed:        SpeedNormal,
		ReportFormat: ReportMarkdown,
		ValidateSteps: true,
		Record:       true,
		Timeout:      30 * time.Minute,
		StepTimeout:  2 * time.Minute,
	}
}

// Validate checks that the configuration is valid and returns
// an error describing any problems.
func (c *Config) Validate() error {
	if len(c.Banks) == 0 {
		return fmt.Errorf("config: at least one test bank path is required")
	}
	if c.OutputDir == "" {
		return fmt.Errorf("config: output directory is required")
	}
	if !c.isValidSpeed() {
		return fmt.Errorf("config: invalid speed mode: %q", c.Speed)
	}
	if !c.isValidReportFormat() {
		return fmt.Errorf("config: invalid report format: %q", c.ReportFormat)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("config: timeout must be positive")
	}
	if c.StepTimeout <= 0 {
		return fmt.Errorf("config: step timeout must be positive")
	}
	for _, p := range c.Platforms {
		if !isValidPlatform(p) {
			return fmt.Errorf("config: invalid platform: %q", p)
		}
	}
	return nil
}

// ExpandedPlatforms returns the actual platforms to test,
// expanding PlatformAll into individual platforms.
func (c *Config) ExpandedPlatforms() []Platform {
	for _, p := range c.Platforms {
		if p == PlatformAll {
			return []Platform{
				PlatformAndroid,
				PlatformWeb,
				PlatformDesktop,
			}
		}
	}
	return c.Platforms
}

// StepDelay returns the delay between steps based on speed.
func (c *Config) StepDelay() time.Duration {
	switch c.Speed {
	case SpeedSlow:
		return 2 * time.Second
	case SpeedFast:
		return 0
	default:
		return 500 * time.Millisecond
	}
}

// ParsePlatforms parses a comma-separated platform string.
func ParsePlatforms(s string) ([]Platform, error) {
	if s == "" || s == "all" {
		return []Platform{PlatformAll}, nil
	}
	parts := strings.Split(s, ",")
	platforms := make([]Platform, 0, len(parts))
	for _, part := range parts {
		p := Platform(strings.TrimSpace(part))
		if !isValidPlatform(p) {
			return nil, fmt.Errorf(
				"invalid platform: %q", part,
			)
		}
		platforms = append(platforms, p)
	}
	return platforms, nil
}

// ParseBanks parses a comma-separated list of bank paths.
func ParseBanks(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	banks := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			banks = append(banks, trimmed)
		}
	}
	return banks
}

func (c *Config) isValidSpeed() bool {
	switch c.Speed {
	case SpeedSlow, SpeedNormal, SpeedFast:
		return true
	}
	return false
}

func (c *Config) isValidReportFormat() bool {
	switch c.ReportFormat {
	case ReportMarkdown, ReportHTML, ReportJSON:
		return true
	}
	return false
}

func isValidPlatform(p Platform) bool {
	switch p {
	case PlatformAndroid, PlatformWeb, PlatformDesktop,
		PlatformAll:
		return true
	}
	return false
}
