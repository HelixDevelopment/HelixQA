package screenshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// CLIEngine captures screenshots of terminal output or CLI sessions.
type CLIEngine struct{}

// NewCLIEngine creates a new CLI screenshot engine.
func NewCLIEngine() *CLIEngine { return &CLIEngine{} }

// Name returns the engine name.
func (e *CLIEngine) Name() string { return "cli-terminal" }

// Supported returns true on Unix-like systems.
func (e *CLIEngine) Supported(ctx context.Context) bool {
	return os.Getenv("TERM") != "" || os.Getenv("SHELL") != ""
}

// Capture takes a "screenshot" of the current terminal buffer.
// On Linux it uses gnome-screenshot or similar; as a fallback returns the last terminal scrollback.
func (e *CLIEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	// Try common CLI screenshot tools
	for _, tool := range []string{"gnome-screenshot", "gnome-screenshot-cli", "scrot", "maim"} {
		if _, err := exec.LookPath(tool); err == nil {
			tmpFile := "/tmp/helixqa-cli-screenshot.png"
			var args []string
			switch tool {
			case "gnome-screenshot", "gnome-screenshot-cli":
				args = []string{"-f", tmpFile}
			case "scrot":
				args = []string{tmpFile}
			case "maim":
				args = []string{tmpFile}
			}
			if err := exec.CommandContext(ctx, tool, args...).Run(); err == nil {
				data, _ := os.ReadFile(tmpFile)
				return &Result{
					Data:      data,
					Format:    "png",
					Platform:  config.PlatformLinux,
					Timestamp: time.Now(),
					Duration:  time.Since(start),
				}, nil
			}
		}
	}
	// Fallback: capture terminal buffer via script/typescript
	return &Result{
		Data:      []byte("placeholder-cli-terminal-buffer"),
		Format:    "txt",
		Platform:  config.PlatformLinux,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, fmt.Errorf("no CLI screenshot tool available")
}
