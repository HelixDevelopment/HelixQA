// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Session is the runtime handle for an active scrcpy-server connection. It
// wraps the three sockets (video, audio?, control?) the server opens after
// handshake and exposes Go-native channels for reading packets plus a Send
// method for writing control messages.
//
// A Session is single-reader per stream. Callers wire the Go channels into
// their pipeline (pkg/capture/android_capture delegates to these channels
// when HELIX_SCRCPY_DIRECT=1).
type Session struct {
	video   net.Conn
	audio   net.Conn
	control net.Conn

	// control-side write lock — scrcpy control socket is single-writer but
	// HelixQA pipelines may write from multiple goroutines.
	ctrlMu sync.Mutex

	// channel buffers; sized generously to absorb short stalls.
	videoCh chan VideoPacket
	audioCh chan AudioPacket
	devCh   chan DeviceMessage

	// shutdown plumbing
	closeOnce sync.Once
	closed    chan struct{}
}

// StartPumps launches the read goroutines that push VideoPacket /
// AudioPacket / DeviceMessage values onto the Session's channels. Must be
// called exactly once per Session. The returned channels are closed when
// the Session is Close()d or an underlying socket returns an error.
func (s *Session) StartPumps(ctx context.Context) (video <-chan VideoPacket, audio <-chan AudioPacket, devices <-chan DeviceMessage) {
	s.videoCh = make(chan VideoPacket, 64)
	if s.audio != nil {
		s.audioCh = make(chan AudioPacket, 64)
	}
	if s.control != nil {
		s.devCh = make(chan DeviceMessage, 16)
	}
	s.closed = make(chan struct{})

	go s.pumpVideo(ctx)
	if s.audio != nil {
		go s.pumpAudio(ctx)
	}
	if s.control != nil {
		go s.pumpDevices(ctx)
	}
	return s.videoCh, s.audioCh, s.devCh
}

func (s *Session) pumpVideo(ctx context.Context) {
	defer close(s.videoCh)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}
		pkt, err := ReadVideoPacket(s.video)
		if err != nil {
			return
		}
		select {
		case s.videoCh <- pkt:
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		}
	}
}

func (s *Session) pumpAudio(ctx context.Context) {
	defer close(s.audioCh)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}
		pkt, err := ReadAudioPacket(s.audio)
		if err != nil {
			return
		}
		select {
		case s.audioCh <- pkt:
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		}
	}
}

func (s *Session) pumpDevices(ctx context.Context) {
	defer close(s.devCh)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		default:
		}
		msg, err := ReadDeviceMessage(s.control)
		if err != nil {
			return
		}
		select {
		case s.devCh <- msg:
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		}
	}
}

// ErrNoControlSocket is returned by Send when the Session was opened with
// EnableControl=false.
var ErrNoControlSocket = errors.New("scrcpy: session has no control socket")

// Send serialises and writes a control message to the server. Safe for
// concurrent use — internal mutex protects against interleaved writes.
// Each Send uses a short write deadline (5s) so a stalled socket surfaces
// quickly rather than hanging the caller.
func (s *Session) Send(msg ControlMessage) error {
	if s == nil || s.control == nil {
		return ErrNoControlSocket
	}
	s.ctrlMu.Lock()
	defer s.ctrlMu.Unlock()
	if err := setDeadline(s.control, time.Now().Add(5*time.Second)); err != nil {
		return err
	}
	return WriteControlMessage(s.control, msg)
}

// Close terminates the Session: closes all three sockets and lets the
// pumping goroutines return. Safe to call multiple times from any goroutine.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	var firstErr error
	s.closeOnce.Do(func() {
		if s.closed == nil {
			s.closed = make(chan struct{})
		}
		close(s.closed)
		for _, c := range []net.Conn{s.video, s.audio, s.control} {
			if c == nil {
				continue
			}
			if err := c.Close(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("scrcpy: close conn: %w", err)
			}
		}
	})
	return firstErr
}

// Video, Audio, Control return the raw underlying net.Conn for callers that
// need to wire their own reader (e.g. integration tests asserting on the
// exact bytes). Normal callers should use StartPumps.
func (s *Session) Video() net.Conn   { return s.video }
func (s *Session) Audio() net.Conn   { return s.audio }
func (s *Session) Control() net.Conn { return s.control }

// --- helpers ---

type deadlineSetter interface {
	SetDeadline(time.Time) error
}

func setDeadline(c net.Conn, t time.Time) error {
	if ds, ok := c.(deadlineSetter); ok {
		return ds.SetDeadline(t)
	}
	// Net.Conn without SetDeadline (rare; only happens with exotic fakes) —
	// silently skip. Tests using net.Pipe() fall into this branch.
	return nil
}
