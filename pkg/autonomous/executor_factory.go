// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"fmt"

	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/navigator"
)

// ExecutorFactory creates platform-specific ActionExecutor
// instances. Implementations select the appropriate executor
// (ADB, Playwright, X11) based on the platform string and
// configuration.
type ExecutorFactory interface {
	// Create returns an ActionExecutor for the given platform.
	// Returns an error if the platform is unsupported or
	// required configuration is missing.
	Create(platform string) (navigator.ActionExecutor, error)
}

// ExecutorConfig holds platform-specific configuration used
// by DefaultExecutorFactory to create executors.
type ExecutorConfig struct {
	// AndroidDevice is the ADB device/emulator serial.
	AndroidDevice string

	// BrowserURL is the URL for web platform testing.
	BrowserURL string

	// DesktopDisplay is the X11 display (e.g. ":0").
	DesktopDisplay string

	// CommandRunner overrides the default os/exec runner.
	// If nil, detector.NewExecRunner() is used.
	CommandRunner detector.CommandRunner
}

// DefaultExecutorFactory creates real platform executors using
// the navigator package's ADBExecutor, PlaywrightExecutor, and
// X11Executor. It uses os/exec for command execution by default.
type DefaultExecutorFactory struct {
	config ExecutorConfig
	runner detector.CommandRunner
}

// NewDefaultExecutorFactory creates a DefaultExecutorFactory
// with the given configuration. If cfg.CommandRunner is nil,
// the factory uses detector.NewExecRunner() for real command
// execution.
func NewDefaultExecutorFactory(
	cfg ExecutorConfig,
) *DefaultExecutorFactory {
	runner := cfg.CommandRunner
	if runner == nil {
		runner = detector.NewExecRunner()
	}
	return &DefaultExecutorFactory{
		config: cfg,
		runner: runner,
	}
}

// Create returns the appropriate ActionExecutor for the
// platform. Supported platforms: "android", "desktop", "web".
func (f *DefaultExecutorFactory) Create(
	platform string,
) (navigator.ActionExecutor, error) {
	switch platform {
	case "android":
		if f.config.AndroidDevice == "" {
			return nil, fmt.Errorf(
				"android device ID is required; "+
					"set --device or ANDROID_DEVICE",
			)
		}
		return navigator.NewADBExecutor(
			f.config.AndroidDevice, f.runner,
		), nil

	case "web":
		if f.config.BrowserURL == "" {
			return nil, fmt.Errorf(
				"browser URL is required; "+
					"set --browser-url or WEB_URL",
			)
		}
		return navigator.NewPlaywrightExecutor(
			f.config.BrowserURL, f.runner,
		), nil

	case "desktop":
		display := f.config.DesktopDisplay
		if display == "" {
			display = ":0"
		}
		return navigator.NewX11Executor(
			display, f.runner,
		), nil

	default:
		return nil, fmt.Errorf(
			"unsupported platform: %q", platform,
		)
	}
}

// NoopExecutorFactory always returns a noopExecutor. Useful
// for testing or when no real platform interaction is needed.
type NoopExecutorFactory struct{}

// Create returns a noopExecutor regardless of platform.
func (f *NoopExecutorFactory) Create(
	_ string,
) (navigator.ActionExecutor, error) {
	return &noopExecutor{}, nil
}
