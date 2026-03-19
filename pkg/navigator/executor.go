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
	_, err := a.cmdRunner.Run(ctx,
		"adb", "-s", a.device, "shell", "input", "text", text,
	)
	return err
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

// PlaywrightExecutor implements ActionExecutor for web browsers
// via the Playwright CLI.
type PlaywrightExecutor struct {
	browserURL string
	cmdRunner  detector.CommandRunner
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

// Click dispatches a click at coordinates via the Playwright CLI.
func (p *PlaywrightExecutor) Click(
	ctx context.Context, x, y int,
) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "click",
		fmt.Sprintf("%d,%d", x, y),
	)
	return err
}

// Type enters text via Playwright keyboard type.
func (p *PlaywrightExecutor) Type(
	ctx context.Context, text string,
) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "type", text,
	)
	return err
}

// Scroll scrolls the page.
func (p *PlaywrightExecutor) Scroll(
	ctx context.Context, direction string, amount int,
) error {
	dy := amount
	if direction == "up" || direction == "left" {
		dy = -amount
	}
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "scroll",
		fmt.Sprintf("%d", dy),
	)
	return err
}

// LongPress is not natively supported in web — simulated
// via mousedown/delay/mouseup.
func (p *PlaywrightExecutor) LongPress(
	ctx context.Context, x, y int,
) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "longpress",
		fmt.Sprintf("%d,%d", x, y),
	)
	return err
}

// Swipe simulates a drag gesture.
func (p *PlaywrightExecutor) Swipe(
	ctx context.Context, fromX, fromY, toX, toY int,
) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "drag",
		fmt.Sprintf("%d,%d", fromX, fromY),
		fmt.Sprintf("%d,%d", toX, toY),
	)
	return err
}

// KeyPress simulates a key press.
func (p *PlaywrightExecutor) KeyPress(
	ctx context.Context, key string,
) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "press", key,
	)
	return err
}

// Back navigates back in the browser.
func (p *PlaywrightExecutor) Back(ctx context.Context) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "back",
	)
	return err
}

// Home navigates to the browser URL.
func (p *PlaywrightExecutor) Home(ctx context.Context) error {
	_, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "navigate", p.browserURL,
	)
	return err
}

// Screenshot captures the page.
func (p *PlaywrightExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	data, err := p.cmdRunner.Run(ctx,
		"npx", "playwright", "screenshot", "--full-page",
	)
	if err != nil {
		return nil, fmt.Errorf("playwright screenshot: %w", err)
	}
	return data, nil
}

// X11Executor implements ActionExecutor for desktop via
// xdotool and import (ImageMagick).
type X11Executor struct {
	display   string
	cmdRunner detector.CommandRunner
}

// NewX11Executor creates an X11Executor.
func NewX11Executor(
	display string,
	runner detector.CommandRunner,
) *X11Executor {
	return &X11Executor{
		display:   display,
		cmdRunner: runner,
	}
}

// Click moves the mouse and clicks via xdotool.
func (x *X11Executor) Click(
	ctx context.Context, px, py int,
) error {
	_, err := x.cmdRunner.Run(ctx,
		"xdotool", "mousemove", "--screen", "0",
		fmt.Sprintf("%d", px), fmt.Sprintf("%d", py),
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx, "xdotool", "click", "1")
	return err
}

// Type types text via xdotool.
func (x *X11Executor) Type(
	ctx context.Context, text string,
) error {
	_, err := x.cmdRunner.Run(ctx,
		"xdotool", "type", "--clearmodifiers", text,
	)
	return err
}

// Scroll uses xdotool to scroll.
func (x *X11Executor) Scroll(
	ctx context.Context, direction string, amount int,
) error {
	button := "5" // down
	if direction == "up" {
		button = "4"
	}
	for i := 0; i < amount; i++ {
		_, err := x.cmdRunner.Run(ctx,
			"xdotool", "click", button,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// LongPress holds the mouse button down.
func (x *X11Executor) LongPress(
	ctx context.Context, px, py int,
) error {
	_, err := x.cmdRunner.Run(ctx,
		"xdotool", "mousemove",
		fmt.Sprintf("%d", px), fmt.Sprintf("%d", py),
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx,
		"xdotool", "mousedown", "1",
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx,
		"xdotool", "mouseup", "1",
	)
	return err
}

// Swipe simulates a drag via xdotool.
func (x *X11Executor) Swipe(
	ctx context.Context, fromX, fromY, toX, toY int,
) error {
	_, err := x.cmdRunner.Run(ctx,
		"xdotool", "mousemove",
		fmt.Sprintf("%d", fromX), fmt.Sprintf("%d", fromY),
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx,
		"xdotool", "mousedown", "1",
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx,
		"xdotool", "mousemove",
		fmt.Sprintf("%d", toX), fmt.Sprintf("%d", toY),
	)
	if err != nil {
		return err
	}
	_, err = x.cmdRunner.Run(ctx,
		"xdotool", "mouseup", "1",
	)
	return err
}

// KeyPress sends a key via xdotool.
func (x *X11Executor) KeyPress(
	ctx context.Context, key string,
) error {
	_, err := x.cmdRunner.Run(ctx,
		"xdotool", "key", key,
	)
	return err
}

// Back sends Alt+Left (browser back).
func (x *X11Executor) Back(ctx context.Context) error {
	return x.KeyPress(ctx, "alt+Left")
}

// Home sends Super key.
func (x *X11Executor) Home(ctx context.Context) error {
	return x.KeyPress(ctx, "super")
}

// Screenshot captures via import (ImageMagick).
func (x *X11Executor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	data, err := x.cmdRunner.Run(ctx,
		"import", "-window", "root", "png:-",
	)
	if err != nil {
		return nil, fmt.Errorf("x11 screenshot: %w", err)
	}
	return data, nil
}
