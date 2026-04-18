// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// screenshotTool identifies a screenshot binary available on the host.
type screenshotTool struct {
	name string // "gnome-screenshot" or "grim"
	path string
}

// resolveScreenshotTool returns the first available screenshot binary.
// Priority: gnome-screenshot (X11) → grim (Wayland).
// Returns an error when neither is on PATH.
func resolveScreenshotTool() (*screenshotTool, error) {
	for _, name := range []string{"gnome-screenshot", "grim"} {
		if p, err := exec.LookPath(name); err == nil {
			return &screenshotTool{name: name, path: p}, nil
		}
	}
	return nil, fmt.Errorf("capture/linux/gnomeshot: neither gnome-screenshot nor grim found on PATH")
}

// captureScreenshot invokes the screenshot tool and writes a PNG to dstPath.
func captureScreenshot(ctx context.Context, tool *screenshotTool, dstPath string) error {
	var cmd *exec.Cmd
	switch tool.name {
	case "gnome-screenshot":
		cmd = exec.CommandContext(ctx, tool.path, "-f", dstPath)
	case "grim":
		cmd = exec.CommandContext(ctx, tool.path, dstPath)
	default:
		return fmt.Errorf("capture/linux/gnomeshot: unknown tool %q", tool.name)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("capture/linux/gnomeshot: %s: %w: %s", tool.name, err, string(out))
	}
	return nil
}

// gnomeShotProducer is the fallback frame producer for when xwd+convert are
// not available.  It calls gnome-screenshot or grim periodically, reads the
// resulting PNG, decodes it to BGRA8, and emits frames.
//
// Kill-switches are checked by detectBackend before this function is called;
// the function itself only handles production tool execution.
func gnomeShotProducer(
	ctx context.Context,
	cfg contracts.CaptureConfig,
	out chan<- contracts.Frame,
	stopCh <-chan struct{},
) error {
	tool, err := resolveScreenshotTool()
	if err != nil {
		return err
	}

	fps := cfg.FrameRate
	if fps <= 0 {
		fps = 10
	}
	interval := time.Second / time.Duration(fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	tmpDir, err := os.MkdirTemp("", "helixqa-linux-cap-*")
	if err != nil {
		return fmt.Errorf("capture/linux/gnomeshot: MkdirTemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var seq uint64
	for {
		select {
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		pngPath := filepath.Join(tmpDir, fmt.Sprintf("frame-%d.png", seq))
		if ferr := captureScreenshot(ctx, tool, pngPath); ferr != nil {
			// Non-fatal: skip this frame, keep the loop alive.
			continue
		}

		raw, rerr := os.ReadFile(pngPath)
		_ = os.Remove(pngPath)
		if rerr != nil {
			continue
		}

		w, h, pixels, decErr := pngToBGRA8(raw)
		if decErr != nil {
			continue
		}

		f := contracts.Frame{
			Seq:       seq,
			Timestamp: time.Now(),
			Width:     w,
			Height:    h,
			Format:    contracts.PixelFormatBGRA8,
			Data:      &bytesFrameData{data: pixels},
		}
		seq++
		select {
		case out <- f:
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
