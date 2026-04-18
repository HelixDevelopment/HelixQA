// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package android implements the OCU P3 Interactor backend for Android
// phones and Android TV via `adb shell input` commands. P3 scope ships
// the plumbing (Interactor struct, injectable injector, factory
// registration for both "android" and "androidtv" kinds). Real ADB
// input command wiring arrives in P3.5.
package android

import (
	"context"
	"errors"
	"reflect"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by every action method while the real ADB
// input wiring is still pending (P3.5 scope).
var ErrNotWired = errors.New("interact/android: production ADB injector not wired yet (P3.5)")

// injector is the injectable backend. Tests swap newInjector for a
// fake; production keeps it as productionInjector.
type injector interface {
	Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error
	Type(ctx context.Context, text string, opts contracts.TypeOptions) error
	Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error
	Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error
	Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error
}

// productionInjector is the not-yet-wired stub used in production.
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

// openWithKind returns a Factory that builds an Interactor with the given kind.
func openWithKind(kind string) interact.Factory {
	return func(_ context.Context, cfg interact.Config) (contracts.Interactor, error) {
		return &Interactor{
			kind: kind,
			cfg:  cfg,
			inj:  newInjector,
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
