// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/bridge/scrcpy"
)

// --- fakes for scrcpy.StartServer ---

type fakeCtxRunner struct {
	responses map[string][]byte
	err       error
}

func (f *fakeCtxRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

type fakeLauncher struct {
	onLaunch func(ctx context.Context, name string, args []string) (scrcpy.Process, error)
}

func (f *fakeLauncher) Launch(ctx context.Context, name string, args ...string) (scrcpy.Process, error) {
	return f.onLaunch(ctx, name, append([]string(nil), args...))
}

type fakeProc struct {
	waitCh chan struct{}
	once   sync.Once
}

func (p *fakeProc) Wait() error                  { <-p.waitCh; return nil }
func (p *fakeProc) Signal(os.Signal) error       { p.once.Do(func() { close(p.waitCh) }); return nil }
func (p *fakeProc) Stderr() io.Reader            { return bytes.NewReader(nil) }

func buildVideoBytes(ptsMicros int64, flags uint32, body []byte) []byte {
	var hdr [12]byte
	if ptsMicros < 0 {
		binary.BigEndian.PutUint64(hdr[0:8], ^uint64(0))
	} else {
		binary.BigEndian.PutUint64(hdr[0:8], uint64(ptsMicros))
	}
	binary.BigEndian.PutUint32(hdr[8:12], uint32(len(body))|flags)
	out := append([]byte(nil), hdr[:]...)
	out = append(out, body...)
	return out
}

const videoFlagKeyframe = 1 << 30

// --- Tests ---

func TestNewDirectFromServerConfig_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "scrcpy-server.jar")
	if err := os.WriteFile(jar, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(tmp, ".devignore")
	_ = os.WriteFile(ignore, []byte("ATMOSphere\n"), 0o644)

	// Pre-pick a port so the launcher can dial back.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_, portStr, _ := net.SplitHostPort(addr)
	_ = l.Close()
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatal(err)
	}

	runner := &fakeCtxRunner{}

	// Serialize the dials so the first connection (scrcpy v3 video socket)
	// is also the one that receives our canned frame bytes. Parallel dials
	// race; we want determinism.
	dialDone := make(chan struct{})
	var dialErr error
	proc := &fakeProc{waitCh: make(chan struct{})}
	launcher := &fakeLauncher{
		onLaunch: func(ctx context.Context, name string, args []string) (scrcpy.Process, error) {
			go func() {
				defer close(dialDone)
				// Dial #1 — video socket.
				c1, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", portStr))
				if err != nil {
					dialErr = err
					return
				}
				// Push one keyframe packet on the first connection (video).
				if _, err := c1.Write(buildVideoBytes(100_000, videoFlagKeyframe, []byte{0x67, 0x42, 0xAA})); err != nil {
					dialErr = err
					return
				}
				// Dial #2 — audio, #3 — control.
				c2, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", portStr))
				if err != nil {
					dialErr = err
					return
				}
				c3, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", portStr))
				if err != nil {
					dialErr = err
					return
				}
				// Keep the sockets alive; DirectSource.Stop closes them via
				// scrcpy.Session.Close.
				_, _, _ = c1, c2, c3
			}()
			return proc, nil
		},
	}

	cfg := DirectServiceConfig{
		Server: scrcpy.ServerConfig{
			Serial:         "dev1",
			JarLocalPath:   jar,
			LocalTCPPort:   port,
			DevIgnorePath:  ignore,
			Runner:         runner,
			Launcher:       launcher,
			ServerVersion:  "3.x",
			AcceptTimeout:  3 * time.Second,
			EnableAudio:    true,
			EnableControl:  true,
		},
		Width: 1920, Height: 1080,
	}
	src, err := NewDirectFromServerConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewDirectFromServerConfig: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	<-dialDone
	if dialErr != nil {
		t.Errorf("dial: %v", dialErr)
	}
	// We pushed one keyframe packet; verify it arrives as a frame.
	got := collect(t, src.Frames(), 1, 2*time.Second)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1", len(got))
	}
	if got[0].Source != "scrcpy-direct" || got[0].Width != 1920 {
		t.Errorf("frame metadata wrong: %+v", got[0])
	}
}

func TestNewDirectFromServerConfig_BadDimensions(t *testing.T) {
	_, err := NewDirectFromServerConfig(context.Background(), DirectServiceConfig{
		Width: 0, Height: 1,
	})
	if !errors.Is(err, ErrDirectConfig) {
		t.Errorf("want ErrDirectConfig, got %v", err)
	}
}

func TestNewDirectFromServerConfig_StartServerError(t *testing.T) {
	// Missing JarLocalPath in ServerConfig -> StartServer returns
	// ErrServerConfig; our wrapper wraps it.
	_, err := NewDirectFromServerConfig(context.Background(), DirectServiceConfig{
		Server: scrcpy.ServerConfig{}, // everything empty
		Width:  1, Height: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewDirectFromServerConfig_ForcesEnableControl(t *testing.T) {
	// Leave EnableControl=false in the ServerConfig; the service should
	// flip it true so input injection is always available.
	var launched bool
	launcher := &fakeLauncher{
		onLaunch: func(ctx context.Context, name string, args []string) (scrcpy.Process, error) {
			launched = true
			// Verify the launched argv contains control=true.
			var seenControl bool
			for _, a := range args {
				if a == "control=true" {
					seenControl = true
				}
			}
			if !seenControl {
				t.Errorf("launcher invoked without control=true; argv=%v", args)
			}
			// We don't dial back; let AcceptTimeout fire.
			return &fakeProc{waitCh: make(chan struct{})}, nil
		},
	}
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "jar")
	_ = os.WriteFile(jar, []byte("fake"), 0o644)
	cfg := DirectServiceConfig{
		Server: scrcpy.ServerConfig{
			JarLocalPath:  jar,
			ServerVersion: "3.x",
			Runner:        &fakeCtxRunner{},
			Launcher:      launcher,
			AcceptTimeout: 100 * time.Millisecond,
			EnableControl: false, // should be flipped to true
		},
		Width: 1, Height: 1,
	}
	// We EXPECT this to fail at accept-timeout (since launcher doesn't
	// dial), but the control=true check happens before the timeout.
	_, err := NewDirectFromServerConfig(context.Background(), cfg)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !launched {
		t.Error("launcher never invoked")
	}
}

func TestMustBeEnabled(t *testing.T) {
	err := MustBeEnabled(func(k string) (string, bool) { return "", false })
	if err == nil {
		t.Error("unset env should error")
	}
	err = MustBeEnabled(func(k string) (string, bool) {
		if k == "HELIX_SCRCPY_DIRECT" {
			return "1", true
		}
		return "", false
	})
	if err != nil {
		t.Errorf("set env should be nil, got %v", err)
	}
}

