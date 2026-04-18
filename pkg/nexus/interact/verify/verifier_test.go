// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// fakeInteractor records which methods were called and returns configurable errors.
type fakeInteractor struct {
	err   error
	calls []string
}

func (f *fakeInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	f.calls = append(f.calls, "click")
	return f.err
}
func (f *fakeInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	f.calls = append(f.calls, "type")
	return f.err
}
func (f *fakeInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	f.calls = append(f.calls, "scroll")
	return f.err
}
func (f *fakeInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	f.calls = append(f.calls, "key")
	return f.err
}
func (f *fakeInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	f.calls = append(f.calls, "drag")
	return f.err
}

// recordingVerifier records which action names After() was called with.
type recordingVerifier struct {
	err     error
	actions []string
}

func (r *recordingVerifier) After(_ context.Context, action string) error {
	r.actions = append(r.actions, action)
	return r.err
}

func TestNoOp_AlwaysNil(t *testing.T) {
	var v NoOp
	require.NoError(t, v.After(context.Background(), "click"))
	require.NoError(t, v.After(context.Background(), "drag"))
	require.NoError(t, v.After(context.Background(), "anything"))
}

func TestWrap_CallsAfterWithCorrectAction(t *testing.T) {
	inner := &fakeInteractor{}
	rec := &recordingVerifier{}
	w := Wrap(inner, rec)
	ctx := context.Background()

	require.NoError(t, w.Click(ctx, contracts.Point{}, contracts.ClickOptions{}))
	require.NoError(t, w.Type(ctx, "hello", contracts.TypeOptions{}))
	require.NoError(t, w.Scroll(ctx, contracts.Point{}, 0, 5))
	require.NoError(t, w.Key(ctx, contracts.KeyEnter, contracts.KeyOptions{}))
	require.NoError(t, w.Drag(ctx, contracts.Point{}, contracts.Point{X: 10, Y: 10}, contracts.DragOptions{}))

	require.Equal(t, []string{"click", "type", "scroll", "key", "drag"}, rec.actions)
}

func TestWrap_InnerErrorShortCircuits_AfterNotCalled(t *testing.T) {
	innerErr := errors.New("inner-fail")
	inner := &fakeInteractor{err: innerErr}
	rec := &recordingVerifier{}
	w := Wrap(inner, rec)

	err := w.Click(context.Background(), contracts.Point{}, contracts.ClickOptions{})
	require.ErrorIs(t, err, innerErr)
	require.Empty(t, rec.actions, "After must not be called when inner returns an error")
}

func TestWrap_VerifierErrorPropagates(t *testing.T) {
	verifyErr := errors.New("verify-fail")
	inner := &fakeInteractor{}
	rec := &recordingVerifier{err: verifyErr}
	w := Wrap(inner, rec)

	err := w.Type(context.Background(), "text", contracts.TypeOptions{})
	require.ErrorIs(t, err, verifyErr)
	// inner was called successfully
	require.Equal(t, []string{"type"}, inner.calls)
}

func TestWrap_NoOpVerifierPassesThrough(t *testing.T) {
	inner := &fakeInteractor{}
	w := Wrap(inner, NoOp{})
	ctx := context.Background()

	require.NoError(t, w.Key(ctx, contracts.KeyArrowDown, contracts.KeyOptions{}))
	require.Equal(t, []string{"key"}, inner.calls)
}

func TestWrap_AllMethodsForwardToInner(t *testing.T) {
	inner := &fakeInteractor{}
	w := Wrap(inner, NoOp{})
	ctx := context.Background()

	require.NoError(t, w.Click(ctx, contracts.Point{X: 1, Y: 2}, contracts.ClickOptions{Clicks: 2}))
	require.NoError(t, w.Type(ctx, "abc", contracts.TypeOptions{ClearFirst: true}))
	require.NoError(t, w.Scroll(ctx, contracts.Point{X: 5, Y: 5}, 1, -1))
	require.NoError(t, w.Key(ctx, contracts.KeyBackspace, contracts.KeyOptions{}))
	require.NoError(t, w.Drag(ctx, contracts.Point{X: 0, Y: 0}, contracts.Point{X: 100, Y: 0}, contracts.DragOptions{Steps: 10}))

	require.Equal(t, []string{"click", "type", "scroll", "key", "drag"}, inner.calls)
}
