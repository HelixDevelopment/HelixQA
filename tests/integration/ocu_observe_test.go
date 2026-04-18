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

	"digital.vasic.helixqa/pkg/nexus/observe"
	// blank imports so each backend's init() registers its factory kind
	_ "digital.vasic.helixqa/pkg/nexus/observe/ax_tree"
	_ "digital.vasic.helixqa/pkg/nexus/observe/cdp"
	_ "digital.vasic.helixqa/pkg/nexus/observe/dbus"
	_ "digital.vasic.helixqa/pkg/nexus/observe/ld_preload"
	_ "digital.vasic.helixqa/pkg/nexus/observe/plthook"
)

// TestOCU_Observe_FactoryKindsRegistered asserts all five P4 kinds are
// discoverable through the factory after sub-packages load.
func TestOCU_Observe_FactoryKindsRegistered(t *testing.T) {
	kinds := observe.Kinds()
	for _, want := range []string{"ld_preload", "plthook", "dbus", "cdp", "ax_tree"} {
		require.Contains(t, kinds, want, "kind %q must be registered via init()", want)
	}
}

// TestOCU_Observe_OpenEachKind asserts each kind can be opened via the
// factory. All five backends return a non-nil Observer from Open() in P4
// (ErrNotWired surfaces at Start time, not at Open time).
func TestOCU_Observe_OpenEachKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, k := range []string{"ld_preload", "plthook", "dbus", "cdp", "ax_tree"} {
		obs, err := observe.Open(ctx, k, observe.Config{})
		require.NoError(t, err, "kind %s: Open() must succeed in P4", k)
		require.NotNil(t, obs, "kind %s: Open() must return non-nil Observer", k)
	}
}
