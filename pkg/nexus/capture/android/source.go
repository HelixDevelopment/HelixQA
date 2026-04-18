// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package android implements the OCU P1/P1.5 CaptureSource backend for
// Android + Android TV via `adb shell screenrecord` piping H.264 NAL units
// over stdout. Factory registers both 'android' and 'androidtv' kinds.
//
// Kill-switches (either disables the real backend and returns ErrNotWired):
//   - env HELIXQA_CAPTURE_ANDROID_STUB=1
//   - "adb" not found on PATH
//
// Device serial is read from env HELIXQA_ADB_SERIAL; if empty the default
// adb device (single connected device) is used.
package android

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by Start (via the production producer) when adb is
// absent or HELIXQA_CAPTURE_ANDROID_STUB=1.
var ErrNotWired = errors.New("capture/android: production adb screenrecord producer not wired (adb absent or HELIXQA_CAPTURE_ANDROID_STUB=1)")

// frameProducer is the injectable backend. Tests swap newFrameProducer
// for a fake; production keeps it as productionFrameProducer.
type frameProducer func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error

// productionFrameProducer launches `adb shell screenrecord --output-format=h264
// --size WxH -` with stdout piped, reads H.264 NAL units by splitting on
// start-code prefixes, and emits each NAL as a contracts.Frame on out.
// It honours stopCh for early termination and sends SIGINT to the subprocess
// on exit for a clean stop.
func productionFrameProducer(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotWired, err)
	}

	serial := os.Getenv("HELIXQA_ADB_SERIAL")

	// Build resolution flag.
	w, h := cfg.Width, cfg.Height
	if w <= 0 {
		w = 1280
	}
	if h <= 0 {
		h = 720
	}
	size := fmt.Sprintf("%dx%d", w, h)

	var args []string
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "screenrecord",
		"--output-format=h264",
		"--size", size,
		"-",
	)

	cmd := exec.CommandContext(ctx, adbPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("capture/android: StdoutPipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("capture/android: cmd.Start: %w", err)
	}

	// Goroutine: stop the subprocess when stopCh fires or ctx is done.
	go func() {
		select {
		case <-stopCh:
		case <-ctx.Done():
		}
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}()

	// Read the H.264 byte stream in chunks and split into NAL units.
	const chunkSize = 64 * 1024
	buf := make([]byte, 0, chunkSize)
	tmp := make([]byte, chunkSize)
	var seq uint64

	for {
		n, readErr := stdout.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			nals := splitH264NALUnits(buf)
			// Keep any trailing incomplete NAL in buf.
			if len(nals) > 0 {
				last := nals[len(nals)-1]
				consumed := 0
				for _, nal := range nals {
					consumed += len(nal)
				}
				// Retain data after the last complete NAL boundary only when
				// there is remaining data after consumed bytes.
				if consumed < len(buf) {
					buf = append(buf[:0], buf[consumed:]...)
				} else {
					buf = buf[:0]
				}
				// Emit all but the last NAL (which may be incomplete).
				emit := nals
				if len(nals) > 1 {
					emit = nals[:len(nals)-1]
					buf = append(last, buf...) // prepend incomplete last NAL
				}
				for _, nal := range emit {
					cp := make([]byte, len(nal))
					copy(cp, nal)
					f := contracts.Frame{
						Seq:       seq,
						Timestamp: time.Now(),
						Width:     w,
						Height:    h,
						Format:    contracts.PixelFormatH264,
						Data:      &bytesFrameData{data: cp},
					}
					seq++
					select {
					case out <- f:
					case <-stopCh:
						_ = cmd.Wait()
						return nil
					case <-ctx.Done():
						_ = cmd.Wait()
						return ctx.Err()
					}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			_ = cmd.Wait()
			return fmt.Errorf("capture/android: read stdout: %w", readErr)
		}
	}

	return cmd.Wait()
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

// adbAvailable returns true when the adb binary is on PATH.
func adbAvailable() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

// androidStubEnabled returns true when HELIXQA_CAPTURE_ANDROID_STUB=1 is set.
func androidStubEnabled() bool {
	return os.Getenv("HELIXQA_CAPTURE_ANDROID_STUB") == "1"
}

// openWithKind returns a Factory that builds a Source with the given kind.
func openWithKind(kind string) capture.Factory {
	return func(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
		s := &Source{
			kind:     kind,
			cfg:      cfg,
			frames:   make(chan contracts.Frame, 16),
			stopCh:   make(chan struct{}),
			producer: newFrameProducer,
		}
		return s, nil
	}
}

func init() {
	capture.Register("android", openWithKind("android"))
	capture.Register("androidtv", openWithKind("androidtv"))
}

// Open constructs a Source using the "android" kind. Tests inject a mock
// producer via newFrameProducer before calling Open. On success the caller
// is responsible for Close().
func Open(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
	return openWithKind("android")(ctx, cfg)
}

// Source is the ADB screenrecord H.264-based CaptureSource.
type Source struct {
	kind     string
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

// Name implements contracts.CaptureSource. Returns the kind the Source was
// constructed with ("android" or "androidtv").
func (s *Source) Name() string { return s.kind }

// Start begins producing frames on Frames(). Returns immediately;
// frames flow until Stop() or the producer exits.
// If adb is absent or the stub env is set, ErrNotWired is returned immediately.
func (s *Source) Start(ctx context.Context, cfg contracts.CaptureConfig) error {
	if s.closed.Load() {
		return fmt.Errorf("capture/android: Source already closed")
	}
	// Snapshot the producer at Start() time so tests can swap newFrameProducer
	// between Open() and Start() without races.
	producer := s.producer
	if isProduction(producer) {
		if androidStubEnabled() || !adbAvailable() {
			return ErrNotWired
		}
	}
	s.cfg = cfg
	go s.run(ctx, producer)
	// Surface immediate errors to the caller.
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
