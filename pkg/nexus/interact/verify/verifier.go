// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package verify provides the post-action verification hook that
// wraps an Interactor so every mutating call is followed by a
// capture → compare check. P3 scope: Verifier interface + a no-op
// default. Real pixel/AX-diff verification arrives in P6 when the
// automation engine composes capture + vision.
package verify

import (
	"context"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Verifier decides whether a just-completed action succeeded.
type Verifier interface {
	After(ctx context.Context, action string) error
}

// NoOp verifier accepts every action.
type NoOp struct{}

func (NoOp) After(context.Context, string) error { return nil }

// Wrap returns a contracts.Interactor that calls v.After(kind) after
// every successful method call.
func Wrap(inner contracts.Interactor, v Verifier) contracts.Interactor {
	return &wrappedInteractor{inner: inner, v: v}
}

type wrappedInteractor struct {
	inner contracts.Interactor
	v     Verifier
}

func (w *wrappedInteractor) Click(ctx context.Context, at contracts.Point, opts contracts.ClickOptions) error {
	if err := w.inner.Click(ctx, at, opts); err != nil {
		return err
	}
	return w.v.After(ctx, "click")
}

func (w *wrappedInteractor) Type(ctx context.Context, text string, opts contracts.TypeOptions) error {
	if err := w.inner.Type(ctx, text, opts); err != nil {
		return err
	}
	return w.v.After(ctx, "type")
}

func (w *wrappedInteractor) Scroll(ctx context.Context, at contracts.Point, dx, dy float64) error {
	if err := w.inner.Scroll(ctx, at, dx, dy); err != nil {
		return err
	}
	return w.v.After(ctx, "scroll")
}

func (w *wrappedInteractor) Key(ctx context.Context, code contracts.KeyCode, opts contracts.KeyOptions) error {
	if err := w.inner.Key(ctx, code, opts); err != nil {
		return err
	}
	return w.v.After(ctx, "key")
}

func (w *wrappedInteractor) Drag(ctx context.Context, from, to contracts.Point, opts contracts.DragOptions) error {
	if err := w.inner.Drag(ctx, from, to, opts); err != nil {
		return err
	}
	return w.v.After(ctx, "drag")
}
