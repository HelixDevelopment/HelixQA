// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package record hosts the OCU P5 recording layer. It implements
// contracts.Recorder by accepting frames from a CaptureSource, pushing them
// into a bounded FrameRing, and forwarding each frame to an injectable
// Encoder backend.
//
// Three codec stubs (x264, nvenc, vaapi) are registered via their init()
// functions. All stubs return ErrNotWired in production; real FFmpeg/NVENC
// CGO bindings arrive in P5.5. The WebRTC/WHIP publisher is off by default
// and also returns ErrNotWired until P5.5.
//
// No CGO is used in P5. All file paths are user-writable. NVENC remote
// dispatch reuses the ocuremote.Dispatcher SSH trust established by P2.
package record

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

// ErrNoSource is returned when Start is called before AttachSource.
var ErrNoSource = errors.New("record: no CaptureSource attached; call AttachSource first")

// ErrAlreadyStarted is returned when Start is called on an already-running
// Recorder.
var ErrAlreadyStarted = errors.New("record: already started")

// WebRTCPublisher is the injectable interface that LiveStream delegates to.
// The default implementation (in pkg/nexus/record/webrtc) returns
// ErrNotWired in P5. Tests can supply a mock.
type WebRTCPublisher interface {
	Publish(ctx context.Context) (whipURL string, err error)
}

// RecordConfig is an alias kept for callers who prefer the record package
// name-space. contracts.RecordConfig is the canonical type.
type RecordConfig = contracts.RecordConfig

// Recorder captures frames from a CaptureSource, stores them in a ring
// buffer, and forwards them to an Encoder. It implements contracts.Recorder.
type Recorder struct {
	mu        sync.Mutex
	src       contracts.CaptureSource
	enc       encoder.Encoder
	ring      *FrameRing
	publisher WebRTCPublisher

	stopCh  chan struct{}
	once    sync.Once // guards Start
	stopped sync.Once // guards Stop
	wg      sync.WaitGroup

	encErr error // first error from Encode, captured for Stop
}

// NewRecorder constructs a Recorder with the given ring capacity and encoder.
// Passing a nil encoder means Encode calls are skipped (frames still enter the
// ring). ringCap ≤ 0 defaults to 1024.
func NewRecorder(ringCap int, enc encoder.Encoder) *Recorder {
	if ringCap <= 0 {
		ringCap = 1024
	}
	return &Recorder{
		ring:   NewFrameRing(ringCap),
		enc:    enc,
		stopCh: make(chan struct{}),
	}
}

// WithPublisher attaches a WebRTCPublisher used by LiveStream. If not set,
// LiveStream returns ErrNotWired.
func (r *Recorder) WithPublisher(p WebRTCPublisher) *Recorder {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publisher = p
	return r
}

// AttachSource implements contracts.Recorder.
func (r *Recorder) AttachSource(src contracts.CaptureSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.src = src
	return nil
}

// Start implements contracts.Recorder. It begins draining frames from the
// attached CaptureSource, forwarding each to the Encoder and the ring.
func (r *Recorder) Start(ctx context.Context, cfg RecordConfig) error {
	r.mu.Lock()
	src := r.src
	r.mu.Unlock()

	if src == nil {
		return ErrNoSource
	}

	started := false
	r.once.Do(func() {
		started = true
	})
	if !started {
		return ErrAlreadyStarted
	}

	r.wg.Add(1)
	go r.drain(ctx, src)
	return nil
}

func (r *Recorder) processFrame(f contracts.Frame) {
	r.ring.Push(f)
	if r.enc != nil {
		if err := r.enc.Encode(f); err != nil {
			r.mu.Lock()
			if r.encErr == nil {
				r.encErr = err
			}
			r.mu.Unlock()
		}
	}
}

func (r *Recorder) drain(ctx context.Context, src contracts.CaptureSource) {
	defer r.wg.Done()
	frames := src.Frames()
	for {
		// Priority: always consume a pending frame before checking stop/ctx.
		// This ensures a pre-closed buffered channel is fully drained even
		// when Stop() is called immediately after Start().
		select {
		case f, ok := <-frames:
			if !ok {
				return
			}
			r.processFrame(f)
			continue
		default:
		}

		// No frame immediately available — block on all three signals.
		select {
		case f, ok := <-frames:
			if !ok {
				return
			}
			r.processFrame(f)
		case <-r.stopCh:
			// Drain any remaining buffered frames before exiting.
			for {
				select {
				case f, ok := <-frames:
					if !ok {
						return
					}
					r.processFrame(f)
				default:
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// Stop implements contracts.Recorder. It signals the drain goroutine to exit,
// closes the Encoder, and waits for clean shutdown.
func (r *Recorder) Stop() error {
	var closeErr error
	r.stopped.Do(func() {
		close(r.stopCh)
	})
	r.wg.Wait()

	if r.enc != nil {
		closeErr = r.enc.Close()
	}

	r.mu.Lock()
	encErr := r.encErr
	r.mu.Unlock()

	if encErr != nil {
		return encErr
	}
	return closeErr
}

// Clip implements contracts.Recorder. It is defined in clip.go.

// LiveStream implements contracts.Recorder. It delegates to the injected
// WebRTCPublisher; returns ErrNotWired when no publisher is wired.
func (r *Recorder) LiveStream(ctx context.Context) (string, error) {
	r.mu.Lock()
	pub := r.publisher
	r.mu.Unlock()

	if pub == nil {
		return "", errors.New("record: no WebRTC publisher not wired (opt-in required, real impl P5.5)")
	}
	return pub.Publish(ctx)
}

// Clip implements contracts.Recorder via clip.go (method defined there).
func (r *Recorder) Clip(around time.Time, window time.Duration, out io.Writer, opts contracts.ClipOptions) error {
	return clipWrite(r.ring, around, window, out, opts)
}

// Compile-time interface satisfaction check.
var _ contracts.Recorder = (*Recorder)(nil)
