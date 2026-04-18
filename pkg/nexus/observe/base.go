// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe

import (
	"context"
	"sync"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ProducerFunc is the backend-specific event source injected by each
// sub-package. The function must honour ctx and stopCh; it sends events
// on out and returns when observation ends. A non-nil return is logged
// by the loop but does not surface in P4 (error propagation arrives in
// P4.5).
type ProducerFunc func(
	ctx context.Context,
	target contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error

// BaseObserver factors the state and lifecycle shared across every P4
// Observer backend. Each sub-package embeds *BaseObserver and supplies
// a backend-specific ProducerFunc to StartLoop.
type BaseObserver struct {
	cfg    Config
	ring   *RingBuffer
	events chan contracts.Event

	stopCh  chan struct{}
	started sync.Once
	stopped sync.Once
	wg      sync.WaitGroup
}

// NewBase initialises a BaseObserver from cfg. BufferSize ≤ 0 defaults
// to 1024.
func NewBase(cfg Config) *BaseObserver {
	size := cfg.BufferSize
	if size <= 0 {
		size = 1024
	}
	return &BaseObserver{
		cfg:    cfg,
		ring:   NewRingBuffer(size),
		events: make(chan contracts.Event, size),
		stopCh: make(chan struct{}),
	}
}

// StartLoop launches the producer goroutine. It is idempotent (sync.Once).
// Sub-packages call this from their Start() implementation.
func (b *BaseObserver) StartLoop(ctx context.Context, target contracts.Target, prod ProducerFunc) {
	b.started.Do(func() {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			defer close(b.events)
			raw := make(chan contracts.Event, b.cfg.BufferSize)
			// Run the producer in its own goroutine; it closes raw when done.
			go func() {
				_ = prod(ctx, target, raw, b.stopCh)
				// Producer returned — close raw so the drain loop below exits.
				close(raw)
			}()
			// Drain raw to completion. stopCh only suppresses new starts,
			// not already-buffered events — we honour it only when raw is
			// already empty to avoid dropping events the producer buffered.
			for {
				select {
				case e, ok := <-raw:
					if !ok {
						// raw closed — producer finished, all events delivered.
						return
					}
					b.ring.Push(e)
					select {
					case b.events <- e:
					default:
						// events channel full — event already in ring; skip
					}
				case <-b.stopCh:
					// Drain any remaining buffered events before exiting.
					for {
						select {
						case e, ok := <-raw:
							if !ok {
								return
							}
							b.ring.Push(e)
							select {
							case b.events <- e:
							default:
							}
						default:
							// raw empty and stop requested — exit cleanly.
							return
						}
					}
				}
			}
		}()
	})
}

// Events implements contracts.Observer.
func (b *BaseObserver) Events() <-chan contracts.Event {
	return b.events
}

// Snapshot implements contracts.Observer.
func (b *BaseObserver) Snapshot(at time.Time, window time.Duration) ([]contracts.Event, error) {
	return b.ring.Snapshot(at, window), nil
}

// BaseStop signals the producer to halt and waits for the goroutine to
// exit. Idempotent. Sub-packages call this from their Stop() implementation.
func (b *BaseObserver) BaseStop() error {
	b.stopped.Do(func() {
		close(b.stopCh)
	})
	b.wg.Wait()
	return nil
}
