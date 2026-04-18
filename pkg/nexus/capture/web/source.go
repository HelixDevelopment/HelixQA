// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package web implements the OCU P1/P1.5 CaptureSource backend for
// Chromium / Firefox via the Chrome DevTools Protocol. P1.5 wires a real
// headless Chromium via chromedp: screenshots are taken at cfg.FrameRate fps,
// decoded from PNG to BGRA8, and emitted on the Frames() channel.
//
// Kill-switches (either disables the real backend and returns ErrNotWired):
//   - env HELIXQA_CAPTURE_WEB_STUB=1
//   - neither "chromium" nor "google-chrome" found on PATH
package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by Open when the production CDP backend is
// disabled (no chromium/google-chrome on PATH, or HELIXQA_CAPTURE_WEB_STUB=1).
var ErrNotWired = errors.New("capture/web: production CDP producer not wired (chromium absent or HELIXQA_CAPTURE_WEB_STUB=1)")

// ErrChromeNotFound is returned when neither chromium nor google-chrome is
// found on PATH.
var ErrChromeNotFound = errors.New("capture/web: chromium/google-chrome not found on PATH")

// frameProducer is the injectable backend. Tests swap newFrameProducer
// for a fake; production keeps it as productionFrameProducer.
type frameProducer func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error

// productionFrameProducer launches a headless Chromium via chromedp, takes
// PNG screenshots at cfg.FrameRate fps, decodes them to BGRA8, and emits
// each decoded frame on out. It honours stopCh for early termination.
func productionFrameProducer(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
	// Allocate a headless Chrome instance.
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	defer allocCancel()

	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	defer chromeCancel()

	// Determine frame interval.
	fps := cfg.FrameRate
	if fps <= 0 {
		fps = 10
	}
	interval := time.Second / time.Duration(fps)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var seq uint64
	for {
		select {
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		var buf []byte
		if err := chromedp.Run(chromeCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
			// Browser may have exited — surface error to caller.
			return fmt.Errorf("capture/web: CaptureScreenshot: %w", err)
		}

		w, h, raw, err := pngToBGRA8(buf)
		if err != nil {
			return fmt.Errorf("capture/web: pngToBGRA8: %w", err)
		}

		f := contracts.Frame{
			Seq:       seq,
			Timestamp: time.Now(),
			Width:     w,
			Height:    h,
			Format:    contracts.PixelFormatBGRA8,
			Data:      &bytesFrameData{data: raw},
		}
		seq++

		select {
		case out <- f:
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// productionPC is the program-counter address of productionFrameProducer,
// captured once at package init so isProduction() never calls the function.
var productionPC = reflect.ValueOf(productionFrameProducer).Pointer()

// isProduction returns true when fp is the production (non-mock) producer.
// Used by tests that explicitly set newFrameProducer = productionFrameProducer
// to verify graceful fallback when browser is absent.
func isProduction(fp frameProducer) bool {
	return reflect.ValueOf(fp).Pointer() == productionPC
}

// newFrameProducer is the package-level injectable; tests replace it.
var newFrameProducer frameProducer = productionFrameProducer

func init() {
	capture.Register("web", Open)
}

// chromiumAvailable returns the path to the first available chromium binary,
// or ("", false) when neither chromium nor google-chrome is on PATH.
func chromiumAvailable() (string, bool) {
	for _, name := range []string{"chromium", "google-chrome", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}
	return "", false
}

// stubEnabled returns true when HELIXQA_CAPTURE_WEB_STUB=1 is set.
func stubEnabled() bool {
	return os.Getenv("HELIXQA_CAPTURE_WEB_STUB") == "1"
}

// Open constructs a Source. When the production producer is selected (the
// default) and no chromium binary is found on PATH (or
// HELIXQA_CAPTURE_WEB_STUB=1 is set), Open returns ErrNotWired so that CI
// machines without a browser stay green. Tests inject a mock producer via
// newFrameProducer before calling Open. On success the caller is responsible
// for Close().
func Open(_ context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
	producer := newFrameProducer
	if isProduction(producer) {
		if stubEnabled() {
			return nil, ErrNotWired
		}
		if _, ok := chromiumAvailable(); !ok {
			return nil, fmt.Errorf("%w: %w", ErrNotWired, ErrChromeNotFound)
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

// Source is the Chromium/Firefox CDP-based CaptureSource.
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
func (s *Source) Name() string { return "web" }

// Start begins producing frames on Frames(). Returns immediately;
// frames flow until Stop() or the producer exits.
func (s *Source) Start(ctx context.Context, cfg contracts.CaptureConfig) error {
	if s.closed.Load() {
		return fmt.Errorf("capture/web: Source already closed")
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

// pngToBGRA8 decodes a PNG-encoded byte slice and returns (width, height,
// BGRA8 raw pixels, error). The raw slice is width*height*4 bytes long.
func pngToBGRA8(buf []byte) (int, int, []byte, error) {
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("pngToBGRA8: decode: %w", err)
	}
	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y
	raw := make([]byte, w*h*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			idx := ((y-bounds.Min.Y)*w + (x - bounds.Min.X)) * 4
			raw[idx+0] = byte(b >> 8) // B
			raw[idx+1] = byte(g >> 8) // G
			raw[idx+2] = byte(r >> 8) // R
			raw[idx+3] = byte(a >> 8) // A
		}
	}
	return w, h, raw, nil
}

// bytesFrameData wraps a []byte and satisfies contracts.FrameData.
type bytesFrameData struct{ data []byte }

func (d *bytesFrameData) AsBytes() ([]byte, error)                  { return d.data, nil }
func (d *bytesFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (d *bytesFrameData) Release() error                            { return nil }
