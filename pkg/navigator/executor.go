// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package navigator provides a navigation engine that drives
// platform-specific UI interactions during autonomous QA sessions.
// It bridges LLM agent decisions with physical UI actions via
// ActionExecutor implementations for ADB (Android), Playwright
// (Web), and X11 (Desktop).
package navigator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/detector"
)

// ActionExecutor is the platform-specific interface for
// performing UI interactions. Implementations for Android
// (ADB), Web (Playwright), and Desktop (X11) use the
// CommandRunner pattern for testability.
type ActionExecutor interface {
	// Click taps or clicks at the given coordinates.
	Click(ctx context.Context, x, y int) error
	// Type enters text into the currently focused element.
	Type(ctx context.Context, text string) error
	// Clear selects all text in the focused field and deletes
	// it. This prevents text accumulation when typing into
	// fields that already contain content (e.g., ADB input
	// text appends rather than replaces).
	Clear(ctx context.Context) error
	// Scroll scrolls in the given direction by the given amount.
	Scroll(ctx context.Context, direction string, amount int) error
	// LongPress performs a long press at the given coordinates.
	LongPress(ctx context.Context, x, y int) error
	// Swipe performs a swipe gesture.
	Swipe(ctx context.Context, fromX, fromY, toX, toY int) error
	// KeyPress simulates a key press.
	KeyPress(ctx context.Context, key string) error
	// Back presses the back button.
	Back(ctx context.Context) error
	// Home presses the home button.
	Home(ctx context.Context) error
	// Screenshot captures the current screen.
	Screenshot(ctx context.Context) ([]byte, error)
}

// ActionResult describes the outcome of a performed action.
type ActionResult struct {
	// Action is the action that was performed.
	Action string `json:"action"`

	// Success indicates whether the action completed.
	Success bool `json:"success"`

	// Error contains any error message.
	Error string `json:"error,omitempty"`

	// Duration is how long the action took.
	Duration time.Duration `json:"duration"`

	// ScreenChanged indicates if the screen changed after the action.
	ScreenChanged bool `json:"screen_changed"`

	// NewScreenID is the screen ID after the action.
	NewScreenID string `json:"new_screen_id,omitempty"`
}

// ExploreResult describes the outcome of an exploration step.
type ExploreResult struct {
	// ActionsPerformed is the number of actions taken.
	ActionsPerformed int `json:"actions_performed"`

	// ScreensDiscovered is the number of new screens found.
	ScreensDiscovered int `json:"screens_discovered"`

	// IssuesFound is the number of issues detected.
	IssuesFound int `json:"issues_found"`

	// Duration is how long the exploration took.
	Duration time.Duration `json:"duration"`
}

// ADBExecutor implements ActionExecutor for Android via ADB.
type ADBExecutor struct {
	device    string
	cmdRunner detector.CommandRunner
}

// NewADBExecutor creates an ADBExecutor for the given device.
func NewADBExecutor(
	device string,
	runner detector.CommandRunner,
) *ADBExecutor {
	return &ADBExecutor{
		device:    device,
		cmdRunner: runner,
	}
}

// Click taps at coordinates via adb shell input tap.
func (a *ADBExecutor) Click(
	ctx context.Context, x, y int,
) error {
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "input", "tap",
		fmt.Sprintf("%d", x), fmt.Sprintf("%d", y),
	)
	return err
}

// Type enters text via adb shell input text.
func (a *ADBExecutor) Type(
	ctx context.Context, text string,
) error {
	// Atomically clear and type in a SINGLE adb shell call.
	// This avoids the multi-command round-trip issue where
	// batch DEL keycodes hang on Android TV virtual keyboards.
	//
	// The shell script: move to end, delete 20 chars (enough
	// for any previous search query), then type the new text.
	// All in one `adb shell` invocation = no inter-command lag.
	script := fmt.Sprintf(
		"input keyevent KEYCODE_MOVE_END && "+
			"input keyevent 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 && "+
			"input text '%s'",
		strings.ReplaceAll(text, "'", "'\\''"),
	)
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", script,
	)
	return err
}

// Clear selects all text in the focused field and deletes it.
// Uses Ctrl+A (select all) followed by Delete to reliably
// clear the entire field content regardless of cursor position
// or field length. This is far more reliable than the previous
// approach of MOVE_END + looping KEYCODE_DEL which only
// deleted a fixed number of characters.
func (a *ADBExecutor) Clear(ctx context.Context) error {
	// Single shell command: move to end + 20 DEL keycodes.
	// All in one `adb shell` call to avoid round-trip hangs
	// on Android TV (Mi Box Android 9) virtual keyboards.
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell",
		"input keyevent KEYCODE_MOVE_END && "+
			"input keyevent 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67 67",
	)
	if err != nil {
		return fmt.Errorf("adb clear: %w", err)
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

// Scroll swipes in the given direction.
func (a *ADBExecutor) Scroll(
	ctx context.Context, direction string, amount int,
) error {
	var fromX, fromY, toX, toY int
	switch direction {
	case "up":
		fromX, fromY, toX, toY = 540, 1200, 540, 1200-amount
	case "down":
		fromX, fromY, toX, toY = 540, 600, 540, 600+amount
	case "left":
		fromX, fromY, toX, toY = 800, 960, 800-amount, 960
	case "right":
		fromX, fromY, toX, toY = 200, 960, 200+amount, 960
	default:
		return fmt.Errorf("unknown scroll direction: %s", direction)
	}
	return a.Swipe(ctx, fromX, fromY, toX, toY)
}

// LongPress performs a long press via adb shell input swipe
// (swipe to same coordinate with duration).
func (a *ADBExecutor) LongPress(
	ctx context.Context, x, y int,
) error {
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "input", "swipe",
		fmt.Sprintf("%d", x), fmt.Sprintf("%d", y),
		fmt.Sprintf("%d", x), fmt.Sprintf("%d", y),
		"1000",
	)
	return err
}

// Swipe performs a swipe gesture via adb shell input swipe.
func (a *ADBExecutor) Swipe(
	ctx context.Context, fromX, fromY, toX, toY int,
) error {
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "input", "swipe",
		fmt.Sprintf("%d", fromX), fmt.Sprintf("%d", fromY),
		fmt.Sprintf("%d", toX), fmt.Sprintf("%d", toY),
	)
	return err
}

// KeyPress sends a key event via adb shell input keyevent.
func (a *ADBExecutor) KeyPress(
	ctx context.Context, key string,
) error {
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell",
		"input", "keyevent", key,
	)
	return err
}

// Back sends the BACK key event.
func (a *ADBExecutor) Back(ctx context.Context) error {
	return a.KeyPress(ctx, "KEYCODE_BACK")
}

// Home sends the HOME key event.
func (a *ADBExecutor) Home(ctx context.Context) error {
	return a.KeyPress(ctx, "KEYCODE_HOME")
}

// Shell executes an arbitrary adb shell command and returns its
// stdout. Used by playback_check and frame_diff actions that need
// to run dumpsys/screencap pipelines beyond the fixed input
// helpers above. The command string is passed verbatim as a single
// shell argument to `adb -s <device> shell`, so callers can chain
// with && / | just like an interactive shell.
func (a *ADBExecutor) Shell(
	ctx context.Context, cmd string,
) ([]byte, error) {
	return a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", cmd,
	)
}

// Screenshot captures via adb shell screencap and returns
// the raw PNG data. It validates the screenshot is not blank
// and retries up to 5 times if necessary.
// CRITICAL: Increased retry delay to 500ms for apps that need time to render
// after cold start (ANR prevention and splash screen handling).
func (a *ADBExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		// Use exec-out for faster direct output (bypasses /sdcard)
		data, err := a.cmdRunner.Run(ctx,
			"adb", "-s", a.device, "exec-out", "screencap", "-p",
		)
		if err != nil {
			lastErr = err
			// FALLBACK: Try shell method if exec-out fails (some devices don't support it)
			data, err = a.cmdRunner.Run(ctx,
				"adb", "-s", a.device, "shell", "screencap", "-p",
			)
			if err != nil {
				lastErr = err
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		// Validate screenshot has content (not blank)
		if len(data) < 5000 {
			// Too small to be a valid screenshot (increased threshold for Android TV)
			lastErr = fmt.Errorf("screenshot too small (%d bytes), likely blank", len(data))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check if all bytes are the same (blank/uniform color)
		if isUniformImage(data) {
			lastErr = fmt.Errorf("screenshot appears to be uniform/blank")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		return data, nil
	}
	return nil, fmt.Errorf("adb screenshot failed after 5 attempts: %w", lastErr)
}

// isUniformImage checks if image data is uniform (all same color)
// by sampling bytes from the PNG data.
func isUniformImage(data []byte) bool {
	if len(data) < 100 {
		return true
	}
	// Sample pixels from different parts of the image
	// Skip PNG header (first 33 bytes)
	sampleStart := 33
	if len(data) <= sampleStart+100 {
		return false // Can't determine, assume valid
	}

	// Compare samples - if all same, likely blank
	sample1 := data[sampleStart]
	sample2 := data[sampleStart+len(data)/4]
	sample3 := data[sampleStart+len(data)/2]
	sample4 := data[sampleStart+3*len(data)/4]

	// Allow some variance for compression
	threshold := byte(10)
	if absDiff(sample1, sample2) < threshold &&
		absDiff(sample2, sample3) < threshold &&
		absDiff(sample3, sample4) < threshold {
		return true
	}
	return false
}

// absDiff returns absolute difference between two bytes
func absDiff(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}
