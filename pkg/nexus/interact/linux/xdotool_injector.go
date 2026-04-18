// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// xdotoolTool identifies which dotool binary to use.
type xdotoolTool struct {
	name string // "xdotool" or "ydotool"
	path string
}

// resolveXdotool returns the first available interaction tool.
// Priority: xdotool (X11) → ydotool (Wayland).
// Returns ErrNotWired when neither is on PATH.
func resolveXdotool() (*xdotoolTool, error) {
	for _, name := range []string{"xdotool", "ydotool"} {
		if p, err := exec.LookPath(name); err == nil {
			return &xdotoolTool{name: name, path: p}, nil
		}
	}
	return nil, ErrNotWired
}

// xdotoolKeyName maps a contracts.KeyCode to the key name string accepted by
// xdotool's "key" subcommand (X11 keysym names).
func xdotoolKeyName(code contracts.KeyCode) string {
	switch code {
	case contracts.KeyEnter:
		return "Return"
	case contracts.KeyEscape:
		return "Escape"
	case contracts.KeyTab:
		return "Tab"
	case contracts.KeyBackspace:
		return "BackSpace"
	case contracts.KeySpace:
		return "space"
	case contracts.KeyArrowUp:
		return "Up"
	case contracts.KeyArrowDown:
		return "Down"
	case contracts.KeyArrowLeft:
		return "Left"
	case contracts.KeyArrowRight:
		return "Right"
	case contracts.KeyDPadCenter:
		return "Return" // DPad centre = Enter in X11 context
	default:
		return string(code)
	}
}

// xdotoolInjector is the real xdotool/ydotool-backed injector.
type xdotoolInjector struct {
	tool *xdotoolTool
}

// run executes the tool with the given arguments and returns any error.
func (x *xdotoolInjector) run(args ...string) error {
	cmd := exec.Command(x.tool.path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("interact/linux: %s %s: %w: %s",
			x.tool.name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (x *xdotoolInjector) Click(_ context.Context, at contracts.Point, opts contracts.ClickOptions) error {
	clicks := opts.Clicks
	if clicks <= 0 {
		clicks = 1
	}
	// Move mouse to position first, then click.
	if err := x.run("mousemove", fmt.Sprintf("%d", at.X), fmt.Sprintf("%d", at.Y)); err != nil {
		return fmt.Errorf("interact/linux: mousemove: %w", err)
	}
	button := "1"
	switch opts.Button {
	case contracts.ClickRight:
		button = "3"
	case contracts.ClickMiddle:
		button = "2"
	}
	for range clicks {
		if err := x.run("click", button); err != nil {
			return fmt.Errorf("interact/linux: click: %w", err)
		}
	}
	return nil
}

func (x *xdotoolInjector) Type(_ context.Context, text string, opts contracts.TypeOptions) error {
	if opts.ClearFirst {
		// Select-all then delete.
		if err := x.run("key", "--clearmodifiers", "ctrl+a"); err != nil {
			return fmt.Errorf("interact/linux: select-all: %w", err)
		}
		if err := x.run("key", "Delete"); err != nil {
			return fmt.Errorf("interact/linux: delete: %w", err)
		}
	}
	return x.run("type", "--clearmodifiers", "--", text)
}

func (x *xdotoolInjector) Scroll(_ context.Context, at contracts.Point, _, dy float64) error {
	// xdotool click: button 4 = scroll up, button 5 = scroll down.
	if err := x.run("mousemove", fmt.Sprintf("%d", at.X), fmt.Sprintf("%d", at.Y)); err != nil {
		return fmt.Errorf("interact/linux: mousemove: %w", err)
	}
	button := "5" // positive dy = scroll down
	if dy < 0 {
		button = "4" // negative dy = scroll up
	}
	clicks := int(dy)
	if clicks < 0 {
		clicks = -clicks
	}
	if clicks == 0 {
		clicks = 1
	}
	for range clicks {
		if err := x.run("click", button); err != nil {
			return fmt.Errorf("interact/linux: scroll click: %w", err)
		}
	}
	return nil
}

func (x *xdotoolInjector) Key(_ context.Context, code contracts.KeyCode, _ contracts.KeyOptions) error {
	keyName := xdotoolKeyName(code)
	return x.run("key", "--clearmodifiers", keyName)
}

func (x *xdotoolInjector) Drag(_ context.Context, from, to contracts.Point, _ contracts.DragOptions) error {
	// mousedown at from, mousemove to to, mouseup.
	if err := x.run("mousemove", fmt.Sprintf("%d", from.X), fmt.Sprintf("%d", from.Y)); err != nil {
		return fmt.Errorf("interact/linux: drag mousemove from: %w", err)
	}
	if err := x.run("mousedown", "1"); err != nil {
		return fmt.Errorf("interact/linux: mousedown: %w", err)
	}
	if err := x.run("mousemove", fmt.Sprintf("%d", to.X), fmt.Sprintf("%d", to.Y)); err != nil {
		_ = x.run("mouseup", "1") // best-effort release on error
		return fmt.Errorf("interact/linux: drag mousemove to: %w", err)
	}
	if err := x.run("mouseup", "1"); err != nil {
		return fmt.Errorf("interact/linux: mouseup: %w", err)
	}
	return nil
}
