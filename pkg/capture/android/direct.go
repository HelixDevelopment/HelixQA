// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package android hosts HelixQA's Android capture paths alongside the
// pre-existing pkg/capture.AndroidCapture type. The subpackage exists so that
// the scrcpy-direct delegation (OpenClawing4.md §5.1.3) can emit the unified
// frames.Frame value without colliding with the legacy pkg/capture.Frame.
//
// Usage:
//
//	srv, err := scrcpy.StartServer(ctx, serverCfg)
//	if err != nil { return err }
//	src, err := android.NewDirectSource(android.DirectConfig{
//	    Server:       srv,
//	    Width:        1920,
//	    Height:       1080,
//	})
//	if err != nil { srv.Stop(); return err }
//	if err := src.Start(ctx); err != nil { srv.Stop(); return err }
//	for f := range src.Frames() { /* … */ }
//
// The direct path is opt-in, typically gated by an env var by higher layers:
//
//	if os.Getenv("HELIX_SCRCPY_DIRECT") == "1" { /* use NewDirectSource */ }
package android

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/bridge/scrcpy"
	"digital.vasic.helixqa/pkg/capture/frames"
)

// ScrcpyRunner is the minimal Server-like surface DirectSource needs. The
// production type is *scrcpy.Server; tests pass a fake that exposes a
// Session with pre-loaded video packets.
type ScrcpyRunner interface {
	Session() *scrcpy.Session
	Stop() error
}

// DirectConfig drives NewDirectSource.
type DirectConfig struct {
	// Server is a running scrcpy-server (typically *scrcpy.Server from
	// scrcpy.StartServer). DirectSource.Stop will forward to Server.Stop.
	Server ScrcpyRunner

	// Width / Height are attached to every emitted frames.Frame; they are
	// the encoded resolution scrcpy-server is sending.
	Width  int
	Height int

	// ChannelBuffer sizes DirectSource's output channel (default 64).
	ChannelBuffer int

	// Clock is used to fill in timestamps when the server emits a config
	// packet (PTSMicros == -1). Defaults to time.Now.
	Clock func() time.Time

	// IncludeConfig, when true, forwards SPS/PPS config packets as Frames
	// whose time is the wall-clock delta since Start. Most consumers leave
	// this false — config packets are metadata, not visible frames.
	IncludeConfig bool
}

// ErrDirectConfig is returned for malformed DirectConfig.
var ErrDirectConfig = errors.New("android/direct: invalid DirectConfig")

// DirectSource adapts a scrcpy.Session's VideoPacket channel into a
// frames.Frame channel so callers implementing the pkg/capture/linux.Source
// contract can treat Android and Linux capture uniformly.
//
// Audio + control messages remain accessible via cfg.Server.Session() — this
// type is explicitly video-only.
type DirectSource struct {
	cfg DirectConfig

	startedAt time.Time
	frameCh   chan frames.Frame
	pumpDone  chan struct{}

	startMu  sync.Mutex
	started  bool
	stopOnce sync.Once
}

// NewDirectSource validates cfg and returns a DirectSource ready for Start.
func NewDirectSource(cfg DirectConfig) (*DirectSource, error) {
	if cfg.Server == nil {
		return nil, fmt.Errorf("%w: Server required", ErrDirectConfig)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: bad dimensions %dx%d", ErrDirectConfig, cfg.Width, cfg.Height)
	}
	if cfg.ChannelBuffer <= 0 {
		cfg.ChannelBuffer = 64
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &DirectSource{
		cfg:      cfg,
		frameCh:  make(chan frames.Frame, cfg.ChannelBuffer),
		pumpDone: make(chan struct{}),
	}, nil
}

// Start launches the pump goroutine that reads from the scrcpy Session's
// video channel and publishes frames.Frame. Safe to call once.
func (d *DirectSource) Start(ctx context.Context) error {
	d.startMu.Lock()
	defer d.startMu.Unlock()
	if d.started {
		return errors.New("android/direct: Start already called")
	}
	sess := d.cfg.Server.Session()
	if sess == nil {
		return errors.New("android/direct: Server.Session() returned nil")
	}
	videoCh, _, _ := sess.StartPumps(ctx)
	d.startedAt = d.cfg.Clock()
	d.started = true
	go d.pump(ctx, videoCh)
	return nil
}

// Frames returns the read-only frame channel. Closed when the pump exits
// (scrcpy video channel closed, ctx cancelled, or Stop called).
func (d *DirectSource) Frames() <-chan frames.Frame { return d.frameCh }

// StartedAt reports the timestamp Start captured. Zero before Start.
func (d *DirectSource) StartedAt() time.Time { return d.startedAt }

// Stop forwards to Server.Stop (which closes the scrcpy sockets and so
// terminates the pump), then waits for the pump goroutine to exit.
// Idempotent.
func (d *DirectSource) Stop() error {
	var firstErr error
	d.stopOnce.Do(func() {
		if d.cfg.Server != nil {
			if err := d.cfg.Server.Stop(); err != nil {
				firstErr = err
			}
		}
		<-d.pumpDone
	})
	return firstErr
}

func (d *DirectSource) pump(ctx context.Context, in <-chan scrcpy.VideoPacket) {
	defer close(d.frameCh)
	defer close(d.pumpDone)
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-in:
			if !ok {
				return
			}
			if pkt.IsConfig && !d.cfg.IncludeConfig {
				continue
			}
			f, err := d.packetToFrame(pkt)
			if err != nil {
				// One bad packet does not terminate the session; skip + continue.
				continue
			}
			select {
			case d.frameCh <- f:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (d *DirectSource) packetToFrame(pkt scrcpy.VideoPacket) (frames.Frame, error) {
	pts := time.Duration(pkt.PTSMicros) * time.Microsecond
	if pkt.PTSMicros < 0 {
		pts = d.cfg.Clock().Sub(d.startedAt)
	}
	if len(pkt.Payload) == 0 {
		return frames.Frame{}, fmt.Errorf("android/direct: empty payload")
	}
	return frames.New(pts, d.cfg.Width, d.cfg.Height, frames.FormatH264AnnexB, "scrcpy-direct", pkt.Payload)
}

// IsDirectEnabled reports whether HELIX_SCRCPY_DIRECT is set to "1" in the
// supplied lookup. Higher-level code uses this to route new Android captures
// through DirectSource while leaving the legacy pkg/capture.AndroidCapture
// path untouched for existing callers.
func IsDirectEnabled(lookup func(string) (string, bool)) bool {
	if lookup == nil {
		return false
	}
	v, ok := lookup("HELIX_SCRCPY_DIRECT")
	return ok && v == "1"
}
