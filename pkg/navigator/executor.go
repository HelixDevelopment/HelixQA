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
	// Clear existing text before typing to prevent accumulation.
	// ADB `input text` always APPENDS at cursor position.
	// NOTE: We do NOT send KEYCODE_BACK here because it would
	// navigate away from the current screen if no keyboard is
	// open (e.g., settings screen with toggles). The LLM is
	// responsible for ensuring a text field is focused before
	// calling type.
	_ = a.Clear(ctx)
	time.Sleep(300 * time.Millisecond)

	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "input", "text", text,
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
	// Root cause: Android TV (Mi Box, Android 9) does not
	// reliably support Ctrl+A select-all via ADB. The meta
	// key combos are ignored on hardware-keyboard (DPAD)
	// input mode. Additionally, `adb shell input text`
	// always APPENDS at cursor position — never replaces.
	//
	// Fix: Move cursor to end, then batch-delete all chars
	// in a SINGLE adb shell command (avoids 100 round-trips).
	// Then verify with a second pass if needed.

	// Step 1: Move cursor to end of text.
	_, _ = a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell",
		"input", "keyevent", "KEYCODE_MOVE_END",
	)
	time.Sleep(100 * time.Millisecond)

	// Step 2: Delete chars in small batches (10 per call).
	// Large batches (50+) cause ADB timeouts on Android 9.
	// KEYCODE_DEL = 67.
	for batch := 0; batch < 3; batch++ {
		_, _ = a.cmdRunner.Run(ctx,
			"adb", "-s", a.device, "shell",
			"input", "keyevent",
			"67", "67", "67", "67", "67",
			"67", "67", "67", "67", "67",
		)
		time.Sleep(100 * time.Millisecond)
	}

	// Final single DEL for any remaining char.
	_, delErr := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell",
		"input", "keyevent", "KEYCODE_DEL",
	)
	if delErr != nil {
		return fmt.Errorf("adb clear delete: %w", delErr)
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

// Screenshot captures via adb shell screencap and returns
// the raw PNG data.
func (a *ADBExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	data, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "screencap", "-p",
	)
	if err != nil {
		return nil, fmt.Errorf("adb screenshot: %w", err)
	}
	return data, nil
}
