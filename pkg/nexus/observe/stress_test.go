// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"

	// blank imports so each backend's init() registers its factory kind
	_ "digital.vasic.helixqa/pkg/nexus/observe/ax_tree"
	_ "digital.vasic.helixqa/pkg/nexus/observe/cdp"
	_ "digital.vasic.helixqa/pkg/nexus/observe/dbus"
	_ "digital.vasic.helixqa/pkg/nexus/observe/ld_preload"
	_ "digital.vasic.helixqa/pkg/nexus/observe/plthook"
)

// syntheticProducer emits n events then closes.
type syntheticProducer struct {
	n int
}

func (s syntheticProducer) run(
	_ context.Context,
	_ contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error {
	for i := 0; i < s.n; i++ {
		select {
		case out <- contracts.Event{
			Kind:      contracts.EventKindHook,
			Timestamp: time.Now(),
			Payload:   map[string]any{"i": i},
		}:
		case <-stopCh:
			return nil
		}
	}
	return nil
}

// openWithSyntheticProducer opens any backend kind through the factory
// and injects a synthetic producer by wrapping the returned Observer's
// BaseObserver directly.
func runCycle(t *testing.T, kind string, n int) {
	t.Helper()
	// Build via factory (production path — Open always succeeds).
	obs, err := observe.Open(context.Background(), kind, observe.Config{BufferSize: 64})
	require.NoError(t, err)

	// For the stress test we bypass Start() (which returns ErrNotWired in
	// production) and exercise the BaseObserver machinery directly via a
	// wrapper that calls StartLoop with a synthetic producer.
	type starter interface {
		StartLoop(context.Context, contracts.Target, observe.ProducerFunc)
		Events() <-chan contracts.Event
		Snapshot(time.Time, time.Duration) ([]contracts.Event, error)
		BaseStop() error
	}
	// The Observer structs embed *BaseObserver so they expose these methods.
	s, ok := obs.(starter)
	require.True(t, ok, "kind %q: Observer must embed *BaseObserver", kind)

	prod := syntheticProducer{n: n}
	s.StartLoop(context.Background(), contracts.Target{ProcessName: "stress"}, prod.run)

	// Drain events channel.
	count := 0
	for range s.Events() {
		count++
	}
	require.Equal(t, n, count, "kind %q: expected %d events", kind, n)

	// Snapshot must return some events (ring persists them).
	snap, err := s.Snapshot(time.Now().Add(time.Hour), 0)
	require.NoError(t, err)
	require.NotEmpty(t, snap)

	require.NoError(t, s.BaseStop())
}

// TestStress_Observe_100Concurrent runs 100 goroutines each performing a
// full Start→drain→Snapshot→Stop cycle across all five P4 backends under
// the Go race detector (-race flag).
func TestStress_Observe_100Concurrent(t *testing.T) {
	kinds := []string{"ld_preload", "plthook", "dbus", "cdp", "ax_tree"}
	const goroutinesPerKind = 20 // 5 kinds × 20 = 100 total goroutines
	const eventsPerCycle = 8

	var wg sync.WaitGroup
	for _, k := range kinds {
		for i := 0; i < goroutinesPerKind; i++ {
			wg.Add(1)
			go func(kind string) {
				defer wg.Done()
				runCycle(t, kind, eventsPerCycle)
			}(k)
		}
	}
	wg.Wait()
}
