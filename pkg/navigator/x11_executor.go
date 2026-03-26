// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/detector"
)

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
