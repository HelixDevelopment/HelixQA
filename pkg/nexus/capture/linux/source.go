// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package linux implements the OCU P1 CaptureSource backend for
// linux X11 SHM source via xwd subprocess (P1 scope); PipeWire /
// cgo bindings land later. P1 scope ships the plumbing (Source
// struct, lifecycle, factory registration, injectable frame
// producer). Production xwd subprocess wiring that parses real
// XWD bytes arrives in P1.5.
package linux

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by Open (and by the production producer)
// while the real xwd subprocess wiring is still pending (P1.5 scope).
var ErrNotWired = errors.New("capture/linux: production xwd producer not wired yet (P1.5)")

// frameProducer is the injectable backend. Tests swap newFrameProducer
// for a fake; production keeps it as productionFrameProducer.
type frameProducer func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error

// productionFrameProducer is the not-yet-wired stub used in production.
func productionFrameProducer(_ context.Context, _ contracts.CaptureConfig, _ chan<- contracts.Frame, _ <-chan struct{}) error {
	return ErrNotWired
}

// productionPC is the program-counter address of productionFrameProducer,
// captured once at package init so isProduction() never calls the function.
var productionPC = reflect.ValueOf(productionFrameProducer).Pointer()

// isProduction returns true when fp is the unimplemented production stub.
// Uses reflect.Value.Pointer() to compare function PCs — the idiomatic
// Go pattern for func-value identity without unsafe.
func isProduction(fp frameProducer) bool {
	return reflect.ValueOf(fp).Pointer() == productionPC
}

// newFrameProducer is the package-level injectable; tests replace it.
var newFrameProducer frameProducer = productionFrameProducer

func init() {
	capture.Register("linux-x11", Open)
}

// Open constructs a Source. Returns ErrNotWired immediately when the
// production xwd backend has not yet been wired (P1.5). Tests inject
// a mock producer via newFrameProducer before calling Open.
// On success the caller is responsible for Close().
func Open(_ context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
	producer := newFrameProducer
	if isProduction(producer) {
		return nil, ErrNotWired
	}
	s := &Source{
		cfg:      cfg,
		frames:   make(chan contracts.Frame, 16),
		stopCh:   make(chan struct{}),
		producer: producer,
	}
	return s, nil
}

// Source is the Linux X11 xwd-based CaptureSource.
type Source struct {
	cfg      contracts.CaptureConfig
	frames   chan contracts.Frame
	stopCh   chan struct{}
	producer frameProducer

	framesProduced atomic.Uint64
	framesDropped  atomic.Uint64
	lastFrameAt    atomic.Int64 // unix nanos

	runOnce  sync.Once
	closed   atomic.Bool
	runErr   error
	runErrMu sync.RWMutex
}

// Name implements contracts.CaptureSource.
func (s *Source) Name() string { return "linux-x11" }

// Start begins producing frames on Frames(). Returns immediately;
// frames flow until Stop() or the producer exits.
func (s *Source) Start(ctx context.Context, cfg contracts.CaptureConfig) error {
	if s.closed.Load() {
		return fmt.Errorf("capture/linux: Source already closed")
	}
	s.cfg = cfg
	go s.run(ctx, s.producer)
	// Surface immediate errors (e.g. ErrNotWired) to the caller.
	time.Sleep(10 * time.Millisecond)
	s.runErrMu.RLock()
	defer s.runErrMu.RUnlock()
	return s.runErr
}

func (s *Source) run(ctx context.Context, producer frameProducer) {
	pipe := make(chan contracts.Frame, cap(s.frames))
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := producer(ctx, s.cfg, pipe, s.stopCh)
		close(pipe)
		if err != nil {
			s.runErrMu.Lock()
			s.runErr = err
			s.runErrMu.Unlock()
		}
	}()
	for f := range pipe {
		select {
		case s.frames <- f:
			s.framesProduced.Add(1)
			s.lastFrameAt.Store(time.Now().UnixNano())
		default:
			s.framesDropped.Add(1)
		}
	}
	<-done
}

// Stop signals the producer to exit. Idempotent.
func (s *Source) Stop() error {
	s.runOnce.Do(func() { close(s.stopCh) })
	return nil
}

// Frames returns the push channel; drain or frames will be dropped.
func (s *Source) Frames() <-chan contracts.Frame { return s.frames }

// Stats returns a point-in-time snapshot.
func (s *Source) Stats() contracts.CaptureStats {
	ts := time.Unix(0, s.lastFrameAt.Load())
	return contracts.CaptureStats{
		FramesProduced: s.framesProduced.Load(),
		FramesDropped:  s.framesDropped.Load(),
		LastFrameAt:    ts,
	}
}

// Close stops and releases resources. Idempotent.
func (s *Source) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	_ = s.Stop()
	return nil
}
