// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package linux implements the OCU P3/P3.5 Interactor backend for Linux via
// xdotool (X11) or ydotool (Wayland). P3.5 wires real input injection:
// click, type, scroll, key, and drag actions are dispatched to whichever
// dotool binary is available on PATH. Raw /dev/uinput writes are deferred to
// a future phase — xdotool covers 95% of QA interactions without sudo.
//
// Kill-switches (either disables the real backend; action methods return
// ErrNotWired):
//   - env HELIXQA_INTERACT_LINUX_STUB=1
//   - neither "xdotool" nor "ydotool" found on PATH
//
// Operator note: /dev/uinput access for a future raw-uinput path requires
// membership in the 'input' group. See docs/ocu-udev-setup.md.
package linux

import (
	"context"
	"errors"
	"os"
	"reflect"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by action methods when no xdotool/ydotool binary is
// available or HELIXQA_INTERACT_LINUX_STUB=1 is set.
var ErrNotWired = errors.New("interact/linux: production xdotool/ydotool injector not wired (binary absent or HELIXQA_INTERACT_LINUX_STUB=1)")

// injector is the injectable backend. Tests swap newInjector for a
// fake; production keeps it as productionInjector.
type injector interface {
	Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error
	Type(ctx context.Context, text string, opts contracts.TypeOptions) error
	Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error
	Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error
	Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error
}

// productionInjector is the not-yet-resolved sentinel.  It is only used when
// neither xdotool nor ydotool is available or the stub env is set.
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

// linuxStubEnabled returns true when HELIXQA_INTERACT_LINUX_STUB=1 is set.
func linuxStubEnabled() bool {
	return os.Getenv("HELIXQA_INTERACT_LINUX_STUB") == "1"
}

// resolveInjector returns a real xdotoolInjector when a dotool binary is
// available and stub mode is off, otherwise returns the productionInjector
// sentinel (action methods will return ErrNotWired).
func resolveInjector() injector {
	if linuxStubEnabled() {
		return productionInjector{}
	}
	tool, err := resolveXdotool()
	if err != nil {
		return productionInjector{}
	}
	return &xdotoolInjector{tool: tool}
}

func init() {
	interact.Register("linux", Open)
}

// Open constructs an Interactor. The production path always succeeds at Open
// time; ErrNotWired surfaces on the first action call when no dotool binary
// is found or the stub env is set — consistent with the P3 contract.
// Tests inject a mock via newInjector before calling Open.
func Open(_ context.Context, cfg interact.Config) (contracts.Interactor, error) {
	inj := newInjector
	if isProduction(inj) {
		inj = resolveInjector()
	}
	return &Interactor{
		cfg: cfg,
		inj: inj,
	}, nil
}

// Interactor is the Linux xdotool/ydotool-based input injector.
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
