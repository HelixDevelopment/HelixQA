// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package android implements the OCU P3/P3.5 Interactor backend for Android
// phones and Android TV via `adb shell input` commands. P3.5 wires real ADB
// input tap/swipe/text/keyevent actions.
//
// Kill-switches (either disables the real backend; action methods return
// ErrNotWired):
//   - env HELIXQA_INTERACT_ANDROID_STUB=1
//   - "adb" not found on PATH
//
// Device serial is read from env HELIXQA_ADB_SERIAL; if empty the default
// adb device (single connected device) is used.
package android

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by action methods when adb is absent or
// HELIXQA_INTERACT_ANDROID_STUB=1 is set.
var ErrNotWired = errors.New("interact/android: production ADB injector not wired (adb absent or HELIXQA_INTERACT_ANDROID_STUB=1)")

// injector is the injectable backend. Tests swap newInjector for a
// fake; production keeps it as productionInjector.
type injector interface {
	Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error
	Type(ctx context.Context, text string, opts contracts.TypeOptions) error
	Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error
	Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error
	Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error
}

// androidKeycode maps a contracts.KeyCode to the Android keyevent integer code.
func androidKeycode(kc contracts.KeyCode) int {
	switch kc {
	case contracts.KeyEnter:
		return 66
	case contracts.KeyEscape:
		return 111
	case contracts.KeyTab:
		return 61
	case contracts.KeyBackspace:
		return 67
	case contracts.KeySpace:
		return 62
	case contracts.KeyArrowUp:
		return 19
	case contracts.KeyArrowDown:
		return 20
	case contracts.KeyArrowLeft:
		return 21
	case contracts.KeyArrowRight:
		return 22
	case contracts.KeyDPadCenter:
		return 23
	default:
		return 0
	}
}

// escapeADBText escapes a string for `adb shell input text`. Spaces must be
// escaped as %s; single-quotes, double-quotes, and backslashes are escaped
// with a backslash so the shell does not interpret them.
func escapeADBText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, " ", "%s")
	return s
}

// adbInjector is the real ADB-backed injector.
type adbInjector struct {
	adbPath string
	serial  string // may be empty (single-device mode)
}

// adbArgs prepends -s <serial> when a serial is configured.
func (a *adbInjector) adbArgs(sub ...string) []string {
	if a.serial != "" {
		return append([]string{"-s", a.serial}, sub...)
	}
	return sub
}

// run executes an adb subcommand and returns any error.
func (a *adbInjector) run(args ...string) error {
	cmd := exec.Command(a.adbPath, a.adbArgs(args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb %v: %w: %s", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *adbInjector) Click(_ context.Context, at contracts.Point, opts contracts.ClickOptions) error {
	clicks := opts.Clicks
	if clicks <= 0 {
		clicks = 1
	}
	for range clicks {
		if err := a.run("shell", "input", "tap",
			fmt.Sprintf("%d", at.X),
			fmt.Sprintf("%d", at.Y),
		); err != nil {
			return fmt.Errorf("interact/android: click: %w", err)
		}
	}
	return nil
}

func (a *adbInjector) Type(_ context.Context, text string, opts contracts.TypeOptions) error {
	if opts.ClearFirst {
		// Select-all then delete.
		if err := a.run("shell", "input", "keyevent", "--longpress", "112"); err != nil {
			_ = err // best-effort
		}
		if err := a.run("shell", "input", "keyevent", "67"); err != nil {
			_ = err
		}
	}
	escaped := escapeADBText(text)
	if escaped == "" {
		return nil
	}
	return a.run("shell", "input", "text", escaped)
}

func (a *adbInjector) Scroll(_ context.Context, at contracts.Point, _, dy float64) error {
	// Map dy to a swipe: positive dy → swipe up (scroll down), negative → swipe down.
	const defaultDelta = 300
	delta := int(math.Round(dy))
	if delta == 0 {
		delta = defaultDelta
	}
	toY := at.Y - delta // swipe direction is opposite to scroll direction
	return a.run("shell", "input", "swipe",
		fmt.Sprintf("%d", at.X),
		fmt.Sprintf("%d", at.Y),
		fmt.Sprintf("%d", at.X),
		fmt.Sprintf("%d", toY),
		"200",
	)
}

func (a *adbInjector) Key(_ context.Context, code contracts.KeyCode, _ contracts.KeyOptions) error {
	kc := androidKeycode(code)
	if kc == 0 {
		return fmt.Errorf("interact/android: unknown KeyCode %q", code)
	}
	return a.run("shell", "input", "keyevent", fmt.Sprintf("%d", kc))
}

func (a *adbInjector) Drag(_ context.Context, from, to contracts.Point, opts contracts.DragOptions) error {
	dur := int(opts.Duration.Milliseconds())
	if dur <= 0 {
		dur = 300
	}
	return a.run("shell", "input", "swipe",
		fmt.Sprintf("%d", from.X),
		fmt.Sprintf("%d", from.Y),
		fmt.Sprintf("%d", to.X),
		fmt.Sprintf("%d", to.Y),
		fmt.Sprintf("%d", dur),
	)
}

// productionInjector is the not-yet-wired sentinel. Kept so isProduction()
// can detect it without calling any action method.
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

// androidStubEnabled returns true when HELIXQA_INTERACT_ANDROID_STUB=1 is set.
func androidStubEnabled() bool {
	return os.Getenv("HELIXQA_INTERACT_ANDROID_STUB") == "1"
}

// resolveInjector returns a real adbInjector when adb is available and stub
// mode is off, otherwise returns the productionInjector sentinel.
func resolveInjector() injector {
	if androidStubEnabled() {
		return productionInjector{}
	}
	p, err := exec.LookPath("adb")
	if err != nil {
		return productionInjector{}
	}
	return &adbInjector{
		adbPath: p,
		serial:  os.Getenv("HELIXQA_ADB_SERIAL"),
	}
}

// openWithKind returns a Factory that builds an Interactor with the given kind.
func openWithKind(kind string) interact.Factory {
	return func(_ context.Context, cfg interact.Config) (contracts.Interactor, error) {
		inj := newInjector
		if isProduction(inj) {
			inj = resolveInjector()
		}
		return &Interactor{
			kind: kind,
			cfg:  cfg,
			inj:  inj,
		}, nil
	}
}

func init() {
	interact.Register("android", openWithKind("android"))
	interact.Register("androidtv", openWithKind("androidtv"))
}

// Open constructs an Interactor using the "android" kind. Tests inject a
// mock via newInjector before calling Open.
func Open(ctx context.Context, cfg interact.Config) (contracts.Interactor, error) {
	return openWithKind("android")(ctx, cfg)
}

// Interactor is the ADB input-based injector. The kind field
// distinguishes phone ("android") from TV ("androidtv") so the
// P3.5 wiring can route DPAD key codes differently.
type Interactor struct {
	kind string
	cfg  interact.Config
	inj  injector
}

// Kind returns the backend kind ("android" or "androidtv").
func (i *Interactor) Kind() string { return i.kind }

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
