package screenshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// MacOSEngine captures screenshots using screencapture.
type MacOSEngine struct{}

// NewMacOSEngine creates a new macOS screenshot engine.
func NewMacOSEngine() *MacOSEngine { return &MacOSEngine{} }

// Name returns the engine name.
func (e *MacOSEngine) Name() string { return "macos-screencapture" }

// Supported returns true if screencapture is available.
func (e *MacOSEngine) Supported(ctx context.Context) bool {
	_, err := exec.LookPath("screencapture")
	return err == nil
}

// Capture takes a screenshot via screencapture.
func (e *MacOSEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	tmpFile := "/tmp/helixqa-macos-screenshot.png"
	args := []string{"-x", tmpFile}
	if opts.DisplayID != "" {
		args = []string{"-x", "-D", opts.DisplayID, tmpFile}
	}
	if err := exec.CommandContext(ctx, "screencapture", args...).Run(); err != nil {
		return nil, fmt.Errorf("screencapture failed: %w", err)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, err
	}
	return &Result{
		Data:      data,
		Format:    "png",
		Platform:  config.PlatformDesktop,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, nil
}
