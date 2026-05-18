package screenshot

import (
	"context"
	"fmt"
	"os"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
)

// IOSEngine captures screenshots from iOS simulators or physical devices via xcrun simctl.
type IOSEngine struct {
	device string
	runner detector.CommandRunner
}

// NewIOSEngine creates a new iOS screenshot engine.
func NewIOSEngine(device string) *IOSEngine {
	return &IOSEngine{
		device: device,
		runner: detector.NewExecRunner(),
	}
}

// Name returns the engine name.
func (e *IOSEngine) Name() string { return "ios-xcrun" }

// Supported returns true if xcrun is available.
func (e *IOSEngine) Supported(ctx context.Context) bool {
	_, err := e.runner.Run(ctx, "xcrun", "--version")
	return err == nil
}

// Capture takes a screenshot via xcrun simctl io booted screenshot.
func (e *IOSEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	device := e.device
	if device == "" {
		device = "booted"
	}
	path := "/tmp/helixqa-ios-screenshot.png"
	args := []string{"simctl", "io", device, "screenshot", path}
	if _, err := e.runner.Run(ctx, "xcrun", args...); err != nil {
		return nil, fmt.Errorf("xcrun simctl screenshot failed: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read captured iOS screenshot %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("captured iOS screenshot %s is empty (0 bytes) — xcrun reported success but produced no bytes", path)
	}
	return &Result{
		Data:      data,
		Format:    "png",
		Platform:  config.PlatformIOS,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, nil
}
