// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/detector"
)

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
