// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestStress_Web_100Concurrent constructs 100 Interactors concurrently,
// each using its own private injector instance, and verifies no data
// races under -race.
func TestStress_Web_100Concurrent(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			// Construct the Interactor directly with a per-goroutine injector
			// to avoid any contention on the package-level newInjector var.
			intr := &Interactor{
				cfg: interact.Config{},
				inj: &mockInjector{},
			}
			err := intr.Click(context.Background(), contracts.Point{X: 10, Y: 20}, contracts.ClickOptions{})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}
