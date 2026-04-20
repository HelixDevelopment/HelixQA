// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/bridge/scrcpy"
	"digital.vasic.helixqa/pkg/capture/frames"
)

// --- Fake server that exposes a scrcpy.Session ---

// fakeServer implements ScrcpyRunner. Its Session() returns a real
// *scrcpy.Session whose video socket is a net.Pipe() end we feed from the
// test. Other sockets are nil so StartPumps only launches the video pump.
type fakeServer struct {
	sess *scrcpy.Session

	mu      sync.Mutex
	stopped bool
	videoClient net.Conn
}

func (f *fakeServer) Session() *scrcpy.Session { return f.sess }

func (f *fakeServer) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return nil
	}
	f.stopped = true
	// Close the video socket from the server side so the pump exits.
	if f.videoClient != nil {
		_ = f.videoClient.Close()
	}
	return nil
}

// newFakeServerWithVideoBytes returns a fakeServer whose video socket will
// deliver exactly `bytes` when read, then EOF.
//
// Internally it uses a net.Pipe: the scrcpy.Session reads from client,
// the helper goroutine writes `bytes` to server and closes.
func newFakeServerWithVideoBytes(t *testing.T, bytesIn []byte) *fakeServer {
	t.Helper()
	client, server := net.Pipe()
	// Write the canned video stream in a goroutine so the pipe write doesn't
	// block the test.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		if len(bytesIn) > 0 {
			_, _ = server.Write(bytesIn)
		}
		_ = server.Close()
	}()
	t.Cleanup(func() { <-writerDone })

	sess := scrcpy.NewSession(client, nil, nil)
	return &fakeServer{sess: sess, videoClient: client}
}

// buildVideoPacketBytes produces the scrcpy v3 video-packet wire format:
// 12-byte header { pts_us uint64, length+flags uint32 } then payload.
func buildVideoPacketBytes(ptsMicros int64, flags uint32, payload []byte) []byte {
	var hdr [12]byte
	if ptsMicros < 0 {
		binary.BigEndian.PutUint64(hdr[0:8], ^uint64(0))
	} else {
		binary.BigEndian.PutUint64(hdr[0:8], uint64(ptsMicros))
	}
	size := uint32(len(payload))
	binary.BigEndian.PutUint32(hdr[8:12], size|flags)
	out := make([]byte, 12+len(payload))
	copy(out, hdr[:])
	copy(out[12:], payload)
	return out
}

const (
	videoFlagConfig = 1 << 31
	videoFlagKey    = 1 << 30
)

// --- Tests ---

func TestNewDirectSource_ValidationRejects(t *testing.T) {
	cases := []struct {
		name string
		cfg  DirectConfig
		sub  string
	}{
		{"nil-server", DirectConfig{Width: 1, Height: 1}, "Server"},
		{"zero-width", DirectConfig{Server: &fakeServer{}, Height: 1}, "dimensions"},
		{"zero-height", DirectConfig{Server: &fakeServer{}, Width: 1}, "dimensions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDirectSource(tc.cfg)
			if err == nil || !errors.Is(err, ErrDirectConfig) {
				t.Errorf("want ErrDirectConfig, got %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), tc.sub) {
				t.Errorf("error missing %q: %v", tc.sub, err)
			}
		})
	}
}

func TestDirectSource_PumpsFrames(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildVideoPacketBytes(100_000, videoFlagKey, []byte{0x67, 0x42, 0xAA}))
	buf.Write(buildVideoPacketBytes(200_000, 0, []byte{0x88, 0x99}))

	server := newFakeServerWithVideoBytes(t, buf.Bytes())
	src, err := NewDirectSource(DirectConfig{
		Server: server, Width: 1920, Height: 1080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	// Read up to 2 frames with a timeout.
	got := collect(t, src.Frames(), 2, 2*time.Second)
	if len(got) != 2 {
		t.Fatalf("got %d frames, want 2", len(got))
	}
	if got[0].PTS != 100*time.Millisecond {
		t.Errorf("frame 0 pts = %v, want 100ms", got[0].PTS)
	}
	if got[1].PTS != 200*time.Millisecond {
		t.Errorf("frame 1 pts = %v, want 200ms", got[1].PTS)
	}
	if got[0].Source != "scrcpy-direct" || got[0].Width != 1920 || got[0].Height != 1080 {
		t.Errorf("metadata: %+v", got[0])
	}
	if !bytes.Equal(got[0].Data, []byte{0x67, 0x42, 0xAA}) {
		t.Errorf("frame 0 payload wrong: %v", got[0].Data)
	}
}

func TestDirectSource_SkipsConfigByDefault(t *testing.T) {
	var buf bytes.Buffer
	// One config packet (SPS/PPS) + one real IDR.
	buf.Write(buildVideoPacketBytes(-1, videoFlagConfig, []byte{0x67})) // config
	buf.Write(buildVideoPacketBytes(50_000, videoFlagKey, []byte{0x41, 0x42}))

	server := newFakeServerWithVideoBytes(t, buf.Bytes())
	fixed := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	src, err := NewDirectSource(DirectConfig{
		Server: server, Width: 1, Height: 1,
		Clock: func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	got := collect(t, src.Frames(), 1, time.Second)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1 (config skipped)", len(got))
	}
	if got[0].PTS != 50*time.Millisecond {
		t.Errorf("frame 0 pts = %v", got[0].PTS)
	}
}

func TestDirectSource_IncludesConfigWhenRequested(t *testing.T) {
	var buf bytes.Buffer
	// Config packet (PTS=-1) then a keyframe.
	buf.Write(buildVideoPacketBytes(-1, videoFlagConfig, []byte{0x67}))
	buf.Write(buildVideoPacketBytes(10_000, videoFlagKey, []byte{0x41}))

	server := newFakeServerWithVideoBytes(t, buf.Bytes())
	fixed := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	src, err := NewDirectSource(DirectConfig{
		Server: server, Width: 1, Height: 1,
		Clock:         func() time.Time { return fixed },
		IncludeConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	got := collect(t, src.Frames(), 2, time.Second)
	if len(got) != 2 {
		t.Fatalf("got %d frames, want 2 (config included)", len(got))
	}
	// Config packet: PTS=-1 -> replaced by Clock()-StartedAt = 0.
	if got[0].PTS != 0 {
		t.Errorf("config frame PTS = %v, want 0", got[0].PTS)
	}
}

func TestDirectSource_DoubleStart(t *testing.T) {
	server := newFakeServerWithVideoBytes(t, nil)
	src, _ := NewDirectSource(DirectConfig{Server: server, Width: 1, Height: 1})
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if err := src.Start(context.Background()); err == nil {
		t.Error("second Start should error")
	}
}

func TestDirectSource_StopIdempotent(t *testing.T) {
	server := newFakeServerWithVideoBytes(t, nil)
	src, _ := NewDirectSource(DirectConfig{Server: server, Width: 1, Height: 1})
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Errorf("first stop: %v", err)
	}
	if err := src.Stop(); err != nil {
		t.Errorf("second stop: %v", err)
	}
	if !server.stopped {
		t.Error("Server.Stop not called")
	}
}

func TestDirectSource_StartedAt(t *testing.T) {
	server := newFakeServerWithVideoBytes(t, nil)
	fixed := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	src, _ := NewDirectSource(DirectConfig{
		Server: server, Width: 1, Height: 1,
		Clock: func() time.Time { return fixed },
	})
	if !src.StartedAt().IsZero() {
		t.Error("StartedAt should be zero before Start")
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if !src.StartedAt().Equal(fixed) {
		t.Errorf("StartedAt = %v, want %v", src.StartedAt(), fixed)
	}
}

func TestIsDirectEnabled(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(string) (string, bool)
		want   bool
	}{
		{"nil-lookup", nil, false},
		{"unset", func(string) (string, bool) { return "", false }, false},
		{"set-zero", func(k string) (string, bool) { return "0", k == "HELIX_SCRCPY_DIRECT" }, false},
		{"set-one", func(k string) (string, bool) { return "1", k == "HELIX_SCRCPY_DIRECT" }, true},
		{"set-empty", func(k string) (string, bool) { return "", k == "HELIX_SCRCPY_DIRECT" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDirectEnabled(tc.lookup); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

// --- small utilities ---

func collect(t *testing.T, ch <-chan frames.Frame, n int, timeout time.Duration) []frames.Frame {
	t.Helper()
	out := make([]frames.Frame, 0, n)
	dl := time.After(timeout)
	for len(out) < n {
		select {
		case f, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, f)
		case <-dl:
			return out
		}
	}
	return out
}

// Ensure we don't accidentally break if someone runs the tests from a
// different cwd — just a sanity check.
func TestCwdSanity(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Skip(err)
	}
	if filepath.Base(wd) != "android" {
		t.Logf("cwd = %s (informational)", wd)
	}
}

// Silence unused-import linter when some refs only appear in certain
// configurations (keeps `io` import lint-clean).
var _ = io.EOF
