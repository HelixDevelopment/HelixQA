package screenshot

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
)

// AndroidEngine captures screenshots using adb exec-out screencap.
type AndroidEngine struct {
	device string
	runner detector.CommandRunner
}

// NewAndroidEngine creates a new Android screenshot engine.
func NewAndroidEngine(device string) *AndroidEngine {
	if device == "" {
		device = ""
	}
	return &AndroidEngine{
		device: device,
		runner: detector.NewExecRunner(),
	}
}

// Name returns the engine name.
func (e *AndroidEngine) Name() string { return "android-adb" }

// Supported returns true if adb is available.
func (e *AndroidEngine) Supported(ctx context.Context) bool {
	_, err := e.runner.Run(ctx, "adb", "version")
	return err == nil
}

// Capture takes a screenshot via adb.
func (e *AndroidEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	maxRetries := opts.MaxRetries
	if maxRetries < 1 {
		maxRetries = 3
	}
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		args := []string{"exec-out", "screencap", "-p"}
		if e.device != "" {
			args = append([]string{"-s", e.device}, args...)
		}
		data, err := e.runner.Run(ctx, "adb", args...)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if len(data) < 5000 {
			lastErr = fmt.Errorf("screenshot too small (%d bytes)", len(data))
			continue
		}
		return &Result{
			Data:      data,
			Format:    "png",
			Platform:  config.PlatformAndroid,
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}
	return nil, fmt.Errorf("android capture failed after %d attempts: %w", maxRetries, lastErr)
}
