// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package web implements the OCU P3 Interactor backend for Chromium /
// Firefox via the Chrome DevTools Protocol. P3 scope ships the plumbing
// (Interactor struct, injectable injector, factory registration). Real
// CDP Input domain wiring arrives in P3.5.
package web

import (
	"context"
	"errors"
	"reflect"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by every action method while the real CDP
// wiring is still pending (P3.5 scope).
var ErrNotWired = errors.New("interact/web: production CDP injector not wired yet (P3.5)")

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

func init() {
	interact.Register("web", Open)
}

// Open constructs an Interactor. The production path always succeeds at
// Open time; ErrNotWired surfaces when an action method is called.
// Tests inject a mock via newInjector before calling Open.
func Open(_ context.Context, cfg interact.Config) (contracts.Interactor, error) {
	return &Interactor{
		cfg: cfg,
		inj: newInjector,
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
