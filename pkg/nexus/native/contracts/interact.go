// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"time"
)

// Point is a 2-D screen coordinate in pixels.
type Point struct {
	X, Y int
}

// Translate returns a new Point offset by (dx, dy).
func (p Point) Translate(dx, dy int) Point {
	return Point{X: p.X + dx, Y: p.Y + dy}
}

// MouseButton identifies which physical mouse button an action targets.
type MouseButton int

const (
	ClickLeft   MouseButton = iota // primary / left button
	ClickRight                     // secondary / right button
	ClickMiddle                    // middle / scroll-wheel button
)

// ClickOptions parameterises a mouse-click action.
type ClickOptions struct {
	Button    MouseButton
	Clicks    int
	Modifiers []string
	HoldFor   time.Duration
}

// TypeOptions parameterises a keyboard-typing action.
type TypeOptions struct {
	DelayPerChar time.Duration
	ClearFirst   bool
}

// KeyCode is a portable key identifier string.
type KeyCode string

const (
	KeyEnter      KeyCode = "enter"
	KeyEscape     KeyCode = "escape"
	KeyTab        KeyCode = "tab"
	KeyBackspace  KeyCode = "backspace"
	KeySpace      KeyCode = "space"
	KeyArrowUp    KeyCode = "arrow_up"
	KeyArrowDown  KeyCode = "arrow_down"
	KeyArrowLeft  KeyCode = "arrow_left"
	KeyArrowRight KeyCode = "arrow_right"
	KeyDPadCenter KeyCode = "dpad_center"
)

// KeyOptions parameterises a key-press action.
type KeyOptions struct {
	Modifiers []string
	HoldFor   time.Duration
}

// DragOptions parameterises a mouse-drag action.
type DragOptions struct {
	Button    MouseButton
	Steps     int
	Duration  time.Duration
	Modifiers []string
}

// Interactor is the interface that all input-injection backends must implement.
// Coordinates are in logical screen pixels unless otherwise stated.
type Interactor interface {
	// Click performs one or more mouse clicks at the given point.
	Click(ctx context.Context, at Point, opts ClickOptions) error

	// Type injects the given text as keyboard events at the current focus.
	Type(ctx context.Context, text string, opts TypeOptions) error

	// Scroll sends a scroll event at the given point.
	// deltaX and deltaY are in logical scroll units (positive = down/right).
	Scroll(ctx context.Context, at Point, deltaX, deltaY float64) error

	// Key presses and releases a single key, optionally with modifiers.
	Key(ctx context.Context, code KeyCode, opts KeyOptions) error

	// Drag moves the mouse from src to dst while holding a button.
	Drag(ctx context.Context, src, dst Point, opts DragOptions) error
}
