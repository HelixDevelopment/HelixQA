// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/interact"
)

type mockInjector struct {
	err   error
	calls []string
}

func (m *mockInjector) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	m.calls = append(m.calls, "click")
	return m.err
}
func (m *mockInjector) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	m.calls = append(m.calls, "type")
	return m.err
}
func (m *mockInjector) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	m.calls = append(m.calls, "scroll")
	return m.err
}
func (m *mockInjector) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	m.calls = append(m.calls, "key")
	return m.err
}
func (m *mockInjector) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	m.calls = append(m.calls, "drag")
	return m.err
}

func withMock(t *testing.T, mock injector) func() {
	t.Helper()
	orig := newInjector
	newInjector = mock
	return func() { newInjector = orig }
}

func TestInteractor_ProductionReturnsErrNotWired(t *testing.T) {
	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.NotNil(t, i)
	require.ErrorIs(t, i.Click(context.Background(), contracts.Point{}, contracts.ClickOptions{}), ErrNotWired)
}

func TestInteractor_MockClick(t *testing.T) {
	mock := &mockInjector{}
	defer withMock(t, mock)()

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.NoError(t, i.Click(context.Background(), contracts.Point{X: 100, Y: 200}, contracts.ClickOptions{}))
	require.Equal(t, []string{"click"}, mock.calls)
}

func TestInteractor_AllMethodsDelegate(t *testing.T) {
	mock := &mockInjector{}
	defer withMock(t, mock)()

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, i.Click(ctx, contracts.Point{}, contracts.ClickOptions{}))
	require.NoError(t, i.Type(ctx, "hello world", contracts.TypeOptions{}))
	require.NoError(t, i.Scroll(ctx, contracts.Point{}, 0, -3))
	require.NoError(t, i.Key(ctx, contracts.KeyTab, contracts.KeyOptions{}))
	require.NoError(t, i.Drag(ctx, contracts.Point{}, contracts.Point{X: 50, Y: 50}, contracts.DragOptions{}))
	require.Equal(t, []string{"click", "type", "scroll", "key", "drag"}, mock.calls)
}

func TestInteractor_MockError(t *testing.T) {
	sentinel := errors.New("cdp-error")
	mock := &mockInjector{err: sentinel}
	defer withMock(t, mock)()

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.ErrorIs(t, i.Key(context.Background(), contracts.KeyEscape, contracts.KeyOptions{}), sentinel)
}

func TestInteractor_RegisteredAsWeb(t *testing.T) {
	kinds := interact.Kinds()
	found := false
	for _, k := range kinds {
		if k == "web" {
			found = true
			break
		}
	}
	require.True(t, found, "web kind must be registered via init()")
}
