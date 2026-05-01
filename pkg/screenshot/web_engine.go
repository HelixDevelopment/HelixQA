package screenshot

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// WebEngine captures screenshots using Playwright or chromedp.
type WebEngine struct {
	browserURL string
}

// NewWebEngine creates a new web screenshot engine.
func NewWebEngine(browserURL string) *WebEngine {
	if browserURL == "" {
		browserURL = "http://localhost:8080"
	}
	return &WebEngine{browserURL: browserURL}
}

// Name returns the engine name.
func (e *WebEngine) Name() string { return "web-playwright" }

// Supported returns true if chromium or playwright is available.
func (e *WebEngine) Supported(ctx context.Context) bool {
	_, err := exec.LookPath("chromium")
	if err == nil {
		return true
	}
	_, err = exec.LookPath("chromium-browser")
	if err == nil {
		return true
	}
	_, err = exec.LookPath("google-chrome")
	return err == nil
}

// Capture takes a screenshot of the configured browser URL.
func (e *WebEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	start := time.Now()
	// Use headless chromium for capture
	cmd := exec.CommandContext(ctx, "chromium",
		"--headless", "--disable-gpu", "--no-sandbox",
		"--screenshot=/tmp/helixqa-web-screenshot.png",
		fmt.Sprintf("--window-size=%d,%d", opts.Width, opts.Height),
		e.browserURL,
	)
	if opts.DarkMode {
		cmd.Args = append(cmd.Args, "--force-dark-mode")
	}
	if err := cmd.Run(); err != nil {
		// Fallback to chromium-browser
		cmd = exec.CommandContext(ctx, "chromium-browser",
			"--headless", "--disable-gpu", "--no-sandbox",
			"--screenshot=/tmp/helixqa-web-screenshot.png",
			fmt.Sprintf("--window-size=%d,%d", opts.Width, opts.Height),
			e.browserURL,
		)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("web capture failed: %w", err)
		}
	}
	data, err := exec.CommandContext(ctx, "cat", "/tmp/helixqa-web-screenshot.png").Output()
	if err != nil {
		return nil, err
	}
	return &Result{
		Data:     data,
		Format:   "png",
		Width:    opts.Width,
		Height:   opts.Height,
		Platform: config.PlatformWeb,
		Timestamp: time.Now(),
		Duration: time.Since(start),
	}, nil
}
