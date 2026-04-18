// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestXdotoolKeyName_Mappings verifies that 8 common KeyCodes map to the
// correct xdotool key names (X11 keysym strings).
func TestXdotoolKeyName_Mappings(t *testing.T) {
	cases := []struct {
		code contracts.KeyCode
		want string
	}{
		{contracts.KeyEnter, "Return"},
		{contracts.KeyEscape, "Escape"},
		{contracts.KeyTab, "Tab"},
		{contracts.KeyBackspace, "BackSpace"},
		{contracts.KeySpace, "space"},
		{contracts.KeyArrowUp, "Up"},
		{contracts.KeyArrowDown, "Down"},
		{contracts.KeyArrowLeft, "Left"},
	}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			require.Equal(t, tc.want, xdotoolKeyName(tc.code))
		})
	}
}

// TestXdotoolKeyName_ArrowRightAndDPadCenter verifies the two remaining
// common key codes that map to distinct strings.
func TestXdotoolKeyName_ArrowRightAndDPadCenter(t *testing.T) {
	require.Equal(t, "Right", xdotoolKeyName(contracts.KeyArrowRight))
	require.Equal(t, "Return", xdotoolKeyName(contracts.KeyDPadCenter))
}

// TestStubEnv_ForcesErrNotWired verifies that HELIXQA_INTERACT_LINUX_STUB=1
// causes action methods to return ErrNotWired regardless of installed binaries.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_INTERACT_LINUX_STUB", "1")

	original := newInjector
	defer func() { newInjector = original }()
	newInjector = productionInjector{} // force production sentinel

	i, err := Open(context.Background(), interact.Config{})
	require.NoError(t, err)
	require.NotNil(t, i)

	err = i.Click(context.Background(), contracts.Point{X: 1, Y: 1}, contracts.ClickOptions{})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotWired), "expected ErrNotWired, got: %v", err)
}

// TestProduction_NoBinary_Errors verifies that when neither xdotool nor
// ydotool is on PATH, resolveInjector returns the productionInjector sentinel
// whose action methods return ErrNotWired.
func TestProduction_NoBinary_Errors(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", t.TempDir()) // empty dir — no binaries on PATH

	inj := resolveInjector()
	require.True(t, isProduction(inj),
		"resolveInjector must return productionInjector sentinel when no binary found")

	err := inj.Click(context.Background(), contracts.Point{}, contracts.ClickOptions{})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestResolveInjector_StubOverridesPath verifies that the stub env variable
// beats a populated PATH — even if a dotool binary were present, stub wins.
func TestResolveInjector_StubOverridesPath(t *testing.T) {
	t.Setenv("HELIXQA_INTERACT_LINUX_STUB", "1")

	inj := resolveInjector()
	require.True(t, isProduction(inj),
		"stub env must force productionInjector sentinel")

	err := inj.Type(context.Background(), "hello", contracts.TypeOptions{})
	require.ErrorIs(t, err, ErrNotWired)
}
