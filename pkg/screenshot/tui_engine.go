package screenshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// TUIEngine captures screenshots of TUI (terminal UI) applications.
type TUIEngine struct{}

// NewTUIEngine creates a new TUI screenshot engine.
func NewTUIEngine() *TUIEngine { return &TUIEngine{} }

// Name returns the engine name.
func (e *TUIEngine) Name() string { return "tui-terminal" }

// Supported returns true when running in a terminal.
func (e *TUIEngine) Supported(ctx context.Context) bool {
	return os.Getenv("TERM") != "" || os.Getenv("SHELL") != ""
}

// Capture takes a screenshot of the current TUI state.
// Uses the same approach as CLIEngine but marks the platform as TUI.
func (e *TUIEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	for _, tool := range []string{"gnome-screenshot", "scrot", "maim"} {
		if _, err := exec.LookPath(tool); err == nil {
			tmpFile := "/tmp/helixqa-tui-screenshot.png"
			var args []string
			switch tool {
			case "gnome-screenshot":
				args = []string{"-f", tmpFile}
			case "scrot", "maim":
				args = []string{tmpFile}
			}
			if err := exec.CommandContext(ctx, tool, args...).Run(); err == nil {
				data, _ := os.ReadFile(tmpFile)
				return &Result{
					Data:      data,
					Format:    "png",
					Platform:  config.PlatformTUI,
					Timestamp: time.Now(),
					Duration:  time.Since(start),
				}, nil
			}
		}
	}
	return &Result{
		Data:      []byte("placeholder-tui-buffer"),
		Format:    "txt",
		Platform:  config.PlatformTUI,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, fmt.Errorf("no TUI screenshot tool available")
}
