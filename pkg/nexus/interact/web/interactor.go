// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package web implements the OCU P3/P3.5 Interactor backend for Chromium /
// Firefox via the Chrome DevTools Protocol. P3.5 wires real CDP Input domain
// actions (click, type, scroll, key, drag) via chromedp.
//
// Kill-switches (either disables the real backend; action methods return
// ErrNotWired):
//   - env HELIXQA_INTERACT_WEB_STUB=1
//   - neither "chromium" nor "google-chrome" found on PATH
package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by action methods when the production CDP injector
// is disabled (no chromium on PATH or HELIXQA_INTERACT_WEB_STUB=1).
var ErrNotWired = errors.New("interact/web: production CDP injector not wired (chromium absent or HELIXQA_INTERACT_WEB_STUB=1)")

// injector is the injectable backend. Tests swap newInjector for a
// fake; production keeps it as productionInjector.
type injector interface {
	Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error
	Type(ctx context.Context, text string, opts contracts.TypeOptions) error
	Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error
	Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error
	Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error
}

// cdpInjector is the real chromedp-backed injector. It holds an allocator
// context and a browser context that are created lazily on first action.
// Both contexts are cancelled when Close() is called on the parent Interactor.
type cdpInjector struct {
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	chromeCtx    context.Context
	chromeCancel context.CancelFunc
}

func newCDPInjector(parent context.Context) (*cdpInjector, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(parent,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	return &cdpInjector{
		allocCtx:     allocCtx,
		allocCancel:  allocCancel,
		chromeCtx:    chromeCtx,
		chromeCancel: chromeCancel,
	}, nil
}

func (c *cdpInjector) close() {
	if c.chromeCancel != nil {
		c.chromeCancel()
	}
	if c.allocCancel != nil {
		c.allocCancel()
	}
}

// mapButton maps contracts.MouseButton to a chromedp MouseOption.
func mapButton(b contracts.MouseButton) chromedp.MouseOption {
	switch b {
	case contracts.ClickRight:
		return chromedp.ButtonRight
	case contracts.ClickMiddle:
		return chromedp.ButtonMiddle
	default:
		return chromedp.ButtonLeft
	}
}

// mapKeyCode maps a contracts.KeyCode to a key string accepted by
// chromedp.KeyEvent (which wraps the CDP dispatchKeyEvent domain).
func mapKeyCode(code contracts.KeyCode) string {
	switch code {
	case contracts.KeyEnter:
		return "\r"
	case contracts.KeyEscape:
		return "\x1b"
	case contracts.KeyTab:
		return "\t"
	case contracts.KeyBackspace:
		return "\b"
	case contracts.KeySpace:
		return " "
	case contracts.KeyArrowUp:
		return "\ue013" // DOM_VK_UP (chromedp uses \ue0XX for special keys)
	case contracts.KeyArrowDown:
		return "\ue015"
	case contracts.KeyArrowLeft:
		return "\ue012"
	case contracts.KeyArrowRight:
		return "\ue014"
	case contracts.KeyDPadCenter:
		return "\r" // DPad centre = Enter in web context
	default:
		return string(code)
	}
}

func (c *cdpInjector) Click(_ context.Context, at contracts.Point, opts contracts.ClickOptions) error {
	clicks := opts.Clicks
	if clicks <= 0 {
		clicks = 1
	}
	actions := make([]chromedp.Action, 0, clicks)
	for range clicks {
		actions = append(actions, chromedp.MouseClickXY(
			float64(at.X), float64(at.Y),
			mapButton(opts.Button),
		))
	}
	return chromedp.Run(c.chromeCtx, actions...)
}

func (c *cdpInjector) Type(_ context.Context, text string, opts contracts.TypeOptions) error {
	if opts.ClearFirst {
		// Select all then overwrite with typed text.
		if err := chromedp.Run(c.chromeCtx, chromedp.KeyEvent("a\x01")); err != nil {
			return fmt.Errorf("interact/web: select-all: %w", err)
		}
	}
	// Type each rune via KeyEvent.
	if err := chromedp.Run(c.chromeCtx, chromedp.KeyEvent(text)); err != nil {
		return fmt.Errorf("interact/web: KeyEvent type: %w", err)
	}
	return nil
}

func (c *cdpInjector) Scroll(_ context.Context, at contracts.Point, dx, dy float64) error {
	// CDP dispatchMouseEvent with type "mouseWheel".
	return chromedp.Run(c.chromeCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseWheel, float64(at.X), float64(at.Y)).
			WithDeltaX(dx).
			WithDeltaY(dy).
			Do(ctx)
	}))
}

func (c *cdpInjector) Key(_ context.Context, code contracts.KeyCode, _ contracts.KeyOptions) error {
	k := mapKeyCode(code)
	return chromedp.Run(c.chromeCtx, chromedp.KeyEvent(k))
}

func (c *cdpInjector) Drag(_ context.Context, from, to contracts.Point, opts contracts.DragOptions) error {
	// Implement drag as: mousePressed at from → mouseMoved to to → mouseReleased at to.
	btn := input.Left
	switch opts.Button {
	case contracts.ClickRight:
		btn = input.Right
	case contracts.ClickMiddle:
		btn = input.Middle
	}
	return chromedp.Run(c.chromeCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		if err := input.DispatchMouseEvent(input.MousePressed, float64(from.X), float64(from.Y)).
			WithButton(btn).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MouseMoved, float64(to.X), float64(to.Y)).
			WithButton(btn).Do(ctx); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, float64(to.X), float64(to.Y)).
			WithButton(btn).WithClickCount(1).Do(ctx)
	}))
}

// productionInjector is the not-yet-wired sentinel used before P3.5 wiring.
// After P3.5 it is only kept so isProduction() can detect the sentinel type.
type productionInjector struct{}

func (productionInjector) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	return ErrNotWired
}
func (productionInjector) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	return ErrNotWired
}
func (productionInjector) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	return ErrNotWired
}
func (productionInjector) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	return ErrNotWired
}
func (productionInjector) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	return ErrNotWired
}

// productionInjectorType is the reflect.Type of productionInjector,
// used by isProduction to detect the un-wired sentinel without calling it.
var productionInjectorType = reflect.TypeOf(productionInjector{})

// isProduction returns true when inj is the un-wired production stub.
func isProduction(inj injector) bool {
	return reflect.TypeOf(inj) == productionInjectorType
}

// newInjector is the package-level injectable; tests replace it.
var newInjector injector = productionInjector{}

// webStubEnabled returns true when HELIXQA_INTERACT_WEB_STUB=1 is set.
func webStubEnabled() bool {
	return os.Getenv("HELIXQA_INTERACT_WEB_STUB") == "1"
}

// webChromiumAvailable returns true when a chromium binary exists on PATH.
func webChromiumAvailable() bool {
	for _, name := range []string{"chromium", "google-chrome", "chromium-browser"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

func init() {
	interact.Register("web", Open)
}

// Open constructs an Interactor. When the production injector is selected and
// no chromium binary is found (or HELIXQA_INTERACT_WEB_STUB=1 is set), Open
// succeeds but action methods return ErrNotWired — consistent with P3 behaviour
// so callers can inspect the backend before use.
// Tests inject a mock via newInjector before calling Open.
func Open(ctx context.Context, cfg interact.Config) (contracts.Interactor, error) {
	inj := newInjector
	if isProduction(inj) {
		if !webStubEnabled() && webChromiumAvailable() {
			cdp, err := newCDPInjector(ctx)
			if err != nil {
				return nil, fmt.Errorf("interact/web: newCDPInjector: %w", err)
			}
			inj = cdp
		}
		// else: keep productionInjector (stub/no-browser) — action methods
		// return ErrNotWired, matching the P3 contract.
	}
	return &Interactor{
		cfg: cfg,
		inj: inj,
	}, nil
}

// Interactor is the CDP-based input injector.
type Interactor struct {
	cfg interact.Config
	inj injector
}

// Click implements contracts.Interactor.
func (i *Interactor) Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error {
	return i.inj.Click(ctx, at, opts)
}

// Type implements contracts.Interactor.
func (i *Interactor) Type(ctx context.Context, text string, opts contracts.TypeOptions) error {
	return i.inj.Type(ctx, text, opts)
}

// Scroll implements contracts.Interactor.
func (i *Interactor) Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error {
	return i.inj.Scroll(ctx, at, dx, dy)
}

// Key implements contracts.Interactor.
func (i *Interactor) Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error {
	return i.inj.Key(ctx, code, opts)
}

// Drag implements contracts.Interactor.
func (i *Interactor) Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error {
	return i.inj.Drag(ctx, from, to, opts)
}
