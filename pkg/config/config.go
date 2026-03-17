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
