// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/interact"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestStress_Android_100Concurrent constructs 100 Interactors concurrently
// (mix of android + androidtv kinds), each using its own private injector,
// and verifies no data races under -race.
func TestStress_Android_100Concurrent(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		kind := "android"
		if i%2 == 0 {
			kind = "androidtv"
		}
		go func(k string) {
			defer wg.Done()
			// Construct the Interactor directly with a per-goroutine injector
			// to avoid any contention on the package-level newInjector var.
			intr := &Interactor{
				kind: k,
				cfg:  interact.Config{},
				inj:  &mockInjector{},
			}
			err := intr.Click(context.Background(), contracts.Point{X: 540, Y: 960}, contracts.ClickOptions{})
			require.NoError(t, err)
		}(kind)
	}
	wg.Wait()
}
