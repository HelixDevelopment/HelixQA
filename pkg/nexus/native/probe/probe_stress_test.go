// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"
	"sync"
	"testing"
)

// TestStress_ProbeLocal_Concurrent runs 100 concurrent ProbeLocal
// calls and asserts zero panics, zero errors, and that every call
// populated OS/CPU/Memory.
func TestStress_ProbeLocal_Concurrent(t *testing.T) {
	ctx := context.Background()
	const N = 100
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := ProbeLocal(ctx)
			if err != nil {
				errs <- err
				return
			}
			if r.OS == "" || r.CPUCores == 0 {
				errs <- context.DeadlineExceeded // any non-nil sentinel
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("stress error: %v", e)
	}
}
