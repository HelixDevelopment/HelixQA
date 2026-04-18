// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package linux implements the OCU P1/P1.5 CaptureSource backend for Linux.
// P1.5 wires a real screenshot pipeline: xwd+convert (X11, preferred) →
// gnome-screenshot (X11 fallback) → grim (Wayland fallback).
//
// Kill-switches (any one disables the real backend and returns ErrNotWired):
//   - env HELIXQA_CAPTURE_LINUX_STUB=1
//   - neither DISPLAY nor WAYLAND_DISPLAY is set
//   - none of xwd, gnome-screenshot, or grim is found on PATH
package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by Open when the production backend is disabled
// (HELIXQA_CAPTURE_LINUX_STUB=1, no display server, or no screenshot tool).
var ErrNotWired = errors.New("capture/linux: production xwd/gnomeshot producer not wired (stub, no display, or no tool on PATH)")

// backend names returned by detectBackend.
const (
	backendXwd   = "xwd"
	backendGnome = "gnome-screenshot"
	backendGrim  = "grim"
)

// detectBackend returns the name of the best available screenshot tool.
// Priority: xwd (requires xwd + convert) → gnome-screenshot → grim.
// Returns ErrNotWired when no tool is found.
func detectBackend() (string, error) {
	if _, err := exec.LookPath("xwd"); err == nil {
		if _, err2 := exec.LookPath("convert"); err2 == nil {
			return backendXwd, nil
		}
	}
	if _, err := exec.LookPath("gnome-screenshot"); err == nil {
		return backendGnome, nil
	}
	if _, err := exec.LookPath("grim"); err == nil {
		return backendGrim, nil
	}
	return "", ErrNotWired
}

// frameProducer is the injectable backend. Tests swap newFrameProducer
// for a fake; production keeps it as productionFrameProducer.
type frameProducer func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error

// productionFrameProducer dispatches to the best available screenshot tool:
// xwd+convert → gnome-screenshot → grim.  It honours all kill-switches.
func productionFrameProducer(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
	backend, err := detectBackend()
	if err != nil {
		return err
	}
	switch backend {
	case backendXwd:
		return xwdProducer(ctx, cfg, out, stopCh)
	default:
		return gnomeShotProducer(ctx, cfg, out, stopCh)
	}
}

// productionPC is the program-counter address of productionFrameProducer,
// captured once at package init so isProduction() never calls the function.
var productionPC = reflect.ValueOf(productionFrameProducer).Pointer()

// isProduction returns true when fp is the production (non-mock) producer.
func isProduction(fp frameProducer) bool {
	return reflect.ValueOf(fp).Pointer() == productionPC
}

// newFrameProducer is the package-level injectable; tests replace it.
var newFrameProducer frameProducer = productionFrameProducer

func init() {
	capture.Register("linux-x11", Open)
}

// linuxStubEnabled returns true when HELIXQA_CAPTURE_LINUX_STUB=1 is set.
func linuxStubEnabled() bool {
	return os.Getenv("HELIXQA_CAPTURE_LINUX_STUB") == "1"
}

// displayAvailable returns true when DISPLAY or WAYLAND_DISPLAY is set.
func displayAvailable() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// Open constructs a Source. When the production producer is selected (the
// default), Open enforces kill-switches in order:
//
//  1. HELIXQA_CAPTURE_LINUX_STUB=1 → ErrNotWired
//  2. No DISPLAY and no WAYLAND_DISPLAY → ErrNotWired
//  3. No screenshot tool on PATH → ErrNotWired
//
// Tests inject a mock producer via newFrameProducer before calling Open.
// On success the caller is responsible for Close().
func Open(_ context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
	producer := newFrameProducer
	if isProduction(producer) {
		if linuxStubEnabled() {
			return nil, ErrNotWired
		}
		if !displayAvailable() {
			return nil, fmt.Errorf("%w: DISPLAY and WAYLAND_DISPLAY are both unset", ErrNotWired)
		}
		if _, err := detectBackend(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNotWired, err)
		}
	}
	s := &Source{
		cfg:      cfg,
		frames:   make(chan contracts.Frame, 16),
		stopCh:   make(chan struct{}),
		producer: producer,
	}
	return s, nil
}

// Source is the Linux screenshot-based CaptureSource.
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
