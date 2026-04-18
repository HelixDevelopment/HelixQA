// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestStress_Wrap_100Concurrent wraps 100 independent fakeInteractors
// concurrently, calls Click on each wrapped instance, and verifies no
// data races under -race.
func TestStress_Wrap_100Concurrent(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			// Each goroutine owns its own fakeInteractor — no sharing.
			inner := &fakeInteractor{}
			w := Wrap(inner, NoOp{})
			err := w.Click(context.Background(), contracts.Point{X: 5, Y: 5}, contracts.ClickOptions{})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}
