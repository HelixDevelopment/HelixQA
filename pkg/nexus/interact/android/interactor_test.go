// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
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
	// Force stub mode for determinism — the real adb path is guarded by
	// resolveInjector; stub mode always falls back to productionInjector.
	t.Setenv("HELIXQA_INTERACT_ANDROID_STUB", "1")
	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.NotNil(t, i)
	require.ErrorIs(t, i.Click(context.Background(), contracts.Point{}, contracts.ClickOptions{}), ErrNotWired)
}

func TestInteractor_MockAllMethods(t *testing.T) {
	mock := &mockInjector{}
	defer withMock(t, mock)()

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, i.Click(ctx, contracts.Point{X: 540, Y: 960}, contracts.ClickOptions{}))
	require.NoError(t, i.Type(ctx, "catalogizer", contracts.TypeOptions{}))
	require.NoError(t, i.Scroll(ctx, contracts.Point{X: 540, Y: 500}, 0, 200))
	require.NoError(t, i.Key(ctx, contracts.KeyDPadCenter, contracts.KeyOptions{}))
	require.NoError(t, i.Drag(ctx, contracts.Point{X: 100, Y: 100}, contracts.Point{X: 400, Y: 100}, contracts.DragOptions{}))
	require.Equal(t, []string{"click", "type", "scroll", "key", "drag"}, mock.calls)
}

func TestInteractor_MockError(t *testing.T) {
	sentinel := errors.New("adb-error")
	mock := &mockInjector{err: sentinel}
	defer withMock(t, mock)()

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.ErrorIs(t, i.Scroll(context.Background(), contracts.Point{}, 0, 10), sentinel)
}

func TestFactory_RegistersBothKinds(t *testing.T) {
	kinds := interact.Kinds()
	haveAndroid, haveTV := false, false
	for _, k := range kinds {
		switch k {
		case "android":
			haveAndroid = true
		case "androidtv":
			haveTV = true
		}
	}
	require.True(t, haveAndroid, "android kind must be registered")
	require.True(t, haveTV, "androidtv kind must be registered")
}

func TestInteractor_KindDistinguishesPhoneAndTV(t *testing.T) {
	mock := &mockInjector{}
	defer withMock(t, mock)()

	phone, err := interact.Open(context.Background(), "android", interact.Config{})
	require.NoError(t, err)
	tv, err := interact.Open(context.Background(), "androidtv", interact.Config{})
	require.NoError(t, err)

	phoneI, ok := phone.(*Interactor)
	require.True(t, ok)
	require.Equal(t, "android", phoneI.Kind())

	tvI, ok := tv.(*Interactor)
	require.True(t, ok)
	require.Equal(t, "androidtv", tvI.Kind())
}

// TestAndroidKeycode_Mappings verifies that the 8 most common keys map to the
// correct Android keyevent integer codes.
func TestAndroidKeycode_Mappings(t *testing.T) {
	cases := []struct {
		code contracts.KeyCode
		want int
	}{
		{contracts.KeyEnter, 66},
		{contracts.KeyEscape, 111},
		{contracts.KeyTab, 61},
		{contracts.KeyBackspace, 67},
		{contracts.KeySpace, 62},
		{contracts.KeyArrowUp, 19},
		{contracts.KeyArrowDown, 20},
		{contracts.KeyDPadCenter, 23},
	}
	for _, tc := range cases {
		got := androidKeycode(tc.code)
		require.Equal(t, tc.want, got, "KeyCode %q", tc.code)
	}
}

// TestProduction_MissingAdbErrors verifies graceful fallback when stub mode is
// on — Open must succeed but Click must return ErrNotWired, never panic.
func TestProduction_MissingAdbErrors(t *testing.T) {
	t.Setenv("HELIXQA_INTERACT_ANDROID_STUB", "1")
	orig := newInjector
	defer func() { newInjector = orig }()
	newInjector = productionInjector{}

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err, "Open must always succeed")
	require.NotNil(t, i)

	err = i.Click(context.Background(), contracts.Point{X: 100, Y: 200}, contracts.ClickOptions{})
	require.Error(t, err, "Click must return an error in stub/no-adb mode")
	require.ErrorIs(t, err, ErrNotWired)
}
