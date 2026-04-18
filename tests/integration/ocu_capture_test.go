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

	"digital.vasic.helixqa/pkg/nexus/capture"
	// import sub-packages so their init() registers factories
	_ "digital.vasic.helixqa/pkg/nexus/capture/android"
	_ "digital.vasic.helixqa/pkg/nexus/capture/linux"
	_ "digital.vasic.helixqa/pkg/nexus/capture/web"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestOCU_Capture_FactoryKindsRegistered asserts all four P1 kinds
// are discoverable through the factory after sub-packages load.
func TestOCU_Capture_FactoryKindsRegistered(t *testing.T) {
	kinds := capture.Kinds()
	require.Contains(t, kinds, "web")
	require.Contains(t, kinds, "linux-x11")
	require.Contains(t, kinds, "android")
	require.Contains(t, kinds, "androidtv")
}

// TestOCU_Capture_OpenEachKind asserts each kind can be opened via
// the factory. web and linux-x11 return ErrNotWired from Open() in
// the production path (no subprocess wired until P1.5); android and
// androidtv always open successfully (ErrNotWired surfaces at Start
// time instead). The test accepts both outcomes and, on success,
// verifies Name() matches the requested kind.
func TestOCU_Capture_OpenEachKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, k := range []string{"web", "linux-x11", "android", "androidtv"} {
		src, err := capture.Open(ctx, k, contracts.CaptureConfig{})
		if err != nil {
			// Production path: web and linux-x11 return ErrNotWired from Open.
			// Verify the error mentions the kind or "not wired" so we don't
			// accidentally swallow a real factory-lookup failure.
			require.Contains(t, err.Error(), "not wired",
				"kind %s: unexpected error %v", k, err)
			t.Logf("kind %s: Open returned ErrNotWired (P1.5 not yet landed) — expected", k)
			continue
		}
		require.NotNil(t, src, "kind %s: Open returned nil source without error", k)
		require.Equal(t, k, src.Name(), "kind %s: Name() should match requested kind", k)
		_ = src.Close()
	}
}
