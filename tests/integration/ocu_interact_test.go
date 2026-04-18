//go:build integration
// +build integration

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/interact"
	// blank imports so each backend's init() registers its factory kinds
	_ "digital.vasic.helixqa/pkg/nexus/interact/android"
	_ "digital.vasic.helixqa/pkg/nexus/interact/linux"
	_ "digital.vasic.helixqa/pkg/nexus/interact/web"

	"digital.vasic.helixqa/pkg/nexus/interact/verify"
)

// TestOCU_Interact_FactoryKindsRegistered asserts all four P3 kinds
// are discoverable through the factory after sub-packages load.
func TestOCU_Interact_FactoryKindsRegistered(t *testing.T) {
	kinds := interact.Kinds()
	require.Contains(t, kinds, "linux")
	require.Contains(t, kinds, "web")
	require.Contains(t, kinds, "android")
	require.Contains(t, kinds, "androidtv")
}

// TestOCU_Interact_OpenEachKind asserts each kind can be opened via
// the factory. All four backends return a non-nil Interactor from Open()
// in P3 (ErrNotWired surfaces at action-call time, not at Open time).
func TestOCU_Interact_OpenEachKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, k := range []string{"linux", "web", "android", "androidtv"} {
		intr, err := interact.Open(ctx, k, interact.Config{})
		require.NoError(t, err, "kind %s: Open() must succeed in P3", k)
		require.NotNil(t, intr, "kind %s: Open() must return non-nil Interactor", k)
	}
}

// TestOCU_Interact_WrapWithNoOp asserts that verify.Wrap + NoOp composes
// correctly with any backend Interactor returned by the factory.
func TestOCU_Interact_WrapWithNoOp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, k := range []string{"linux", "web", "android", "androidtv"} {
		inner, err := interact.Open(ctx, k, interact.Config{VerifyAfterAction: true})
		require.NoError(t, err, "kind %s: Open() must succeed", k)
		wrapped := verify.Wrap(inner, verify.NoOp{})
		require.NotNil(t, wrapped, "kind %s: Wrap must return non-nil Interactor", k)
	}
}
