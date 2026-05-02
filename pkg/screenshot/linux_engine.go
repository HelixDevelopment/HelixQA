package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// LinuxEngine captures screenshots on Linux using xwd, grim, or ImageMagick import.
type LinuxEngine struct {
	backend string
}

// NewLinuxEngine creates a new Linux screenshot engine, probing available backends.
func NewLinuxEngine() *LinuxEngine {
	e := &LinuxEngine{}
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xwd"); err == nil {
			e.backend = "xwd"
			return e
		}
		if _, err := exec.LookPath("import"); err == nil {
			e.backend = "import"
			return e
		}
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("grim"); err == nil {
			e.backend = "grim"
			return e
		}
	}
	return e
}

// Name returns the engine name.
func (e *LinuxEngine) Name() string {
	if e.backend == "" {
		return "linux-unavailable"
	}
	return "linux-" + e.backend
}

// Supported returns true if a backend is available.
func (e *LinuxEngine) Supported(ctx context.Context) bool {
	return e.backend != ""
}

// Capture takes a screenshot using the available backend.
func (e *LinuxEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	tmpFile := "/tmp/helixqa-linux-screenshot.png"
	var cmd *exec.Cmd
	switch e.backend {
	case "xwd":
		cmd = exec.CommandContext(ctx, "xwd", "-root", "-silent")
		// xwd outputs XWD format; convert to PNG
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("xwd capture failed: %w", err)
		}
		cmd = exec.CommandContext(ctx, "convert", "xwd:-", tmpFile)
		cmd.Stdin = bytes.NewReader(out)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("xwd convert failed: %w", err)
		}
	case "grim":
		cmd = exec.CommandContext(ctx, "grim", tmpFile)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("grim capture failed: %w", err)
		}
	case "import":
		cmd = exec.CommandContext(ctx, "import", "-window", "root", tmpFile)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("import capture failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("no linux backend available")
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
