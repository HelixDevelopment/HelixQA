// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/detector"
)

// PlaywrightExecutor implements ActionExecutor for web browsers
// via a headless Chromium instance controlled by a Node.js
// Playwright bridge script. The bridge manages the browser
// lifecycle and accepts JSON commands over stdin/stdout.
type PlaywrightExecutor struct {
	browserURL string
	cmdRunner  detector.CommandRunner
	bridgePath string
	launched   bool
	mu         sync.Mutex
}

// NewPlaywrightExecutor creates a PlaywrightExecutor.
func NewPlaywrightExecutor(
	browserURL string,
	runner detector.CommandRunner,
) *PlaywrightExecutor {
	return &PlaywrightExecutor{
		browserURL: browserURL,
		cmdRunner:  runner,
	}
}

// findBridge locates the playwright-bridge.js script relative
// to the HelixQA binary or project root.
func (p *PlaywrightExecutor) findBridge() string {
	if p.bridgePath != "" {
		return p.bridgePath
	}
	// Check common locations.
	candidates := []string{
		"HelixQA/scripts/playwright-bridge.js",
		"scripts/playwright-bridge.js",
	}
	// Try relative to executable.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "..", "scripts",
				"playwright-bridge.js"),
		)
	}
	// Try relative to working directory.
	if wd, err := os.Getwd(); err == nil {
		for _, c := range candidates {
			full := filepath.Join(wd, c)
			if _, err := os.Stat(full); err == nil {
				p.bridgePath = full
				return full
			}
		}
	}
	// Absolute fallback.
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			p.bridgePath = c
			return c
		}
	}
	return "scripts/playwright-bridge.js"
}

// ensureLaunched starts the browser if not already running.
// The bridge script launches headless Chromium as a detached
// process, writes the CDP endpoint to a state file, then
// exits. Subsequent action calls reconnect via CDP.
func (p *PlaywrightExecutor) ensureLaunched(
	ctx context.Context,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.launched {
		return nil
	}

	bridge := p.findBridge()
	cmd := map[string]interface{}{
		"action": "launch",
		"url":    p.browserURL,
	}
	cmdJSON, _ := json.Marshal(cmd)

	launchCtx, cancel := context.WithTimeout(
		ctx, 45*time.Second,
	)
	defer cancel()

	proc := osexec.CommandContext(
		launchCtx, "node", bridge,
	)
	proc.Stdin = bytes.NewReader(cmdJSON)
	// Use /tmp for browser state file — always exists.
	proc.Env = append(os.Environ(),
		"HELIX_OUTPUT_DIR=/tmp",
		"NODE_PATH="+nodePath(),
	)

	out, err := proc.Output()
	if err != nil {
		if exitErr, ok := err.(*osexec.ExitError); ok {
			return fmt.Errorf(
				"launch browser: %w: %s",
				err,
				strings.TrimSpace(
					string(exitErr.Stderr),
				),
			)
		}
		return fmt.Errorf("launch browser: %w", err)
	}
	fmt.Printf(
		"  [playwright] browser launched: %s\n",
		strings.TrimSpace(string(out)),
	)
	p.launched = true
	return nil
}

// runBridgeCmd sends a JSON command to the bridge and returns
// the raw stdout output.
func (p *PlaywrightExecutor) runBridgeCmd(
	ctx context.Context,
	cmd map[string]interface{},
) ([]byte, error) {
	if err := p.ensureLaunched(ctx); err != nil {
		return nil, err
	}

	bridge := p.findBridge()
	cmdJSON, _ := json.Marshal(cmd)

	execCtx, cancel := context.WithTimeout(
		ctx, 30*time.Second,
	)
	defer cancel()

	proc := osexec.CommandContext(execCtx, "node", bridge)
	proc.Stdin = bytes.NewReader(cmdJSON)
	proc.Env = append(os.Environ(),
		"HELIX_OUTPUT_DIR=/tmp",
		"NODE_PATH="+nodePath(),
	)

	out, err := proc.Output()
	if err != nil {
		// Include stderr in the error for debugging.
		if exitErr, ok := err.(*osexec.ExitError); ok {
			return nil, fmt.Errorf(
				"bridge: %w: %s",
				err, strings.TrimSpace(
					string(exitErr.Stderr),
				),
			)
		}
		return nil, fmt.Errorf("bridge: %w", err)
	}
	return out, nil
}

// nodePath returns the best NODE_PATH for finding Playwright.
// It searches catalog-web/node_modules relative to the working
// directory, since Playwright is installed there.
func nodePath() string {
	if v := os.Getenv("NODE_PATH"); v != "" {
		return v
	}
	if wd, err := os.Getwd(); err == nil {
		candidates := []string{
			filepath.Join(wd, "catalog-web", "node_modules"),
			filepath.Join(wd, "..", "catalog-web", "node_modules"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(
				filepath.Join(c, "playwright"),
			); err == nil {
				return c
			}
		}
	}
	return ""
}

// Click dispatches a click at coordinates.
func (p *PlaywrightExecutor) Click(
	ctx context.Context, x, y int,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "click",
		"x":      x,
		"y":      y,
	})
	return err
}

// Type enters text via Playwright keyboard.
func (p *PlaywrightExecutor) Type(
	ctx context.Context, text string,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "type",
		"text":   text,
	})
	return err
}

// Clear selects all text and deletes it via Playwright
// keyboard shortcuts.
func (p *PlaywrightExecutor) Clear(
	ctx context.Context,
) error {
	// Ctrl+A selects all, then Backspace deletes.
	if err := p.KeyPress(ctx, "Control+a"); err != nil {
		return err
	}
	return p.KeyPress(ctx, "Backspace")
}

// Scroll scrolls the page in the given direction.
func (p *PlaywrightExecutor) Scroll(
	ctx context.Context, direction string, amount int,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action":    "scroll",
		"direction": direction,
		"amount":    amount,
	})
	return err
}

// LongPress performs a long press at the given coordinates.
func (p *PlaywrightExecutor) LongPress(
	ctx context.Context, x, y int,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "longpress",
		"x":      x,
		"y":      y,
	})
	return err
}

// Swipe performs a drag gesture.
func (p *PlaywrightExecutor) Swipe(
	ctx context.Context, fromX, fromY, toX, toY int,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "swipe",
		"fromX":  fromX,
		"fromY":  fromY,
		"toX":    toX,
		"toY":    toY,
	})
	return err
}

// KeyPress simulates a key press.
func (p *PlaywrightExecutor) KeyPress(
	ctx context.Context, key string,
) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "key",
		"key":    key,
	})
	return err
}

// Back navigates back in the browser.
func (p *PlaywrightExecutor) Back(ctx context.Context) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "back",
	})
	return err
}

// Home navigates to the browser URL.
func (p *PlaywrightExecutor) Home(ctx context.Context) error {
	_, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "navigate",
		"url":    p.browserURL,
	})
	return err
}

// Screenshot captures the page as PNG.
func (p *PlaywrightExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	data, err := p.runBridgeCmd(ctx, map[string]interface{}{
		"action": "screenshot",
	})
	if err != nil {
		return nil, fmt.Errorf("playwright screenshot: %w", err)
	}
	return data, nil
}
