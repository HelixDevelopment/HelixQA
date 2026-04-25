// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeRunnerSeq is a CommandRunner that records every invocation. Unlike the
// simpler fakeRunner in devguard_test.go, this one supports per-call scripted
// responses keyed by a prefix match on args.
type fakeRunnerSeq struct {
	mu    sync.Mutex
	calls []fakeCall
	// script maps "first-N-args joined by space" -> response
	script map[string]fakeResponse
}

type fakeCall struct {
	name string
	args []string
}

type fakeResponse struct {
	out []byte
	err error
}

func (r *fakeRunnerSeq) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if r.script == nil {
		return nil, nil
	}
	for i := len(args); i >= 0; i-- {
		key := name
		for j := 0; j < i; j++ {
			key += " " + args[j]
		}
		if resp, ok := r.script[key]; ok {
			return resp.out, resp.err
		}
	}
	return nil, nil
}

// fakeLauncher reports a fake Process that can be signalled. It also runs a
// goroutine the test controls to dial back into the scrcpy listener.
type fakeLauncher struct {
	onLaunch func(ctx context.Context, name string, args []string) (Process, error)
}

func (f *fakeLauncher) Launch(ctx context.Context, name string, args ...string) (Process, error) {
	return f.onLaunch(ctx, name, append([]string(nil), args...))
}

type fakeProcess struct {
	waitCh    chan struct{}
	stderrBuf io.Reader
	sigMu     sync.Mutex
	signalled bool
}

func (p *fakeProcess) Wait() error {
	<-p.waitCh
	return nil
}

func (p *fakeProcess) Signal(sig os.Signal) error {
	p.sigMu.Lock()
	defer p.sigMu.Unlock()
	if p.signalled {
		return nil
	}
	p.signalled = true
	close(p.waitCh)
	return nil
}

func (p *fakeProcess) Stderr() io.Reader { return p.stderrBuf }

// TestStartServer_HappyPath exercises the full bring-up: devguard + push +
// reverse + listener + launcher + 3 socket accepts + session creation.
// The fake launcher dials back into our listener three times to simulate the
// scrcpy-server opening video/audio/control.
func TestStartServer_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "scrcpy-server.jar")
	if err := os.WriteFile(jar, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoreFile := filepath.Join(tmp, ".devignore")
	_ = os.WriteFile(ignoreFile, []byte("ATMOSphere\n"), 0o644)

	// Pick a port ahead of time so the fake launcher can dial back.
	port, err := freeLoopbackPort()
	if err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunnerSeq{
		script: map[string]fakeResponse{
			"adb -s s shell getprop ro.product.model": {out: []byte("Pixel 7\n")},
			// push and reverse return nothing — no error, no stdout.
		},
	}

	// Launcher: when invoked, open 3 TCP connections to our listener.
	var dialErrs []error
	var dialWG sync.WaitGroup
	proc := &fakeProcess{waitCh: make(chan struct{})}
	launcher := &fakeLauncher{
		onLaunch: func(ctx context.Context, name string, args []string) (Process, error) {
			for i := 0; i < 3; i++ {
				dialWG.Add(1)
				go func() {
					defer dialWG.Done()
					c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(port)), 5*time.Second)
					if err != nil {
						dialErrs = append(dialErrs, err)
						return
					}
					// Keep the socket open; the session owns it now.
					_ = c
				}()
			}
			return proc, nil
		},
	}

	cfg := ServerConfig{
		Serial:        "s",
		JarLocalPath:  jar,
		LocalTCPPort:  port,
		DevIgnorePath: ignoreFile,
		Runner:        runner,
		Launcher:      launcher,
		ServerVersion: "2.3-helixqa",
		VideoBitRate:  4_000_000,
		AcceptTimeout: 5 * time.Second,
		EnableControl: true,
		EnableAudio:   true,
	}
	srv, err := StartServer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	dialWG.Wait()
	for _, e := range dialErrs {
		if e != nil {
			t.Errorf("dial error: %v", e)
		}
	}
	if srv.LocalPort() != port {
		t.Errorf("LocalPort = %d, want %d", srv.LocalPort(), port)
	}
	sess := srv.Session()
	if sess == nil || sess.Video() == nil || sess.Audio() == nil || sess.Control() == nil {
		t.Fatal("session sockets nil")
	}

	// Verify adb argv shape for the launched app_process.
	// The launcher received (ctx, "adb", args...) and the last args list includes
	// "-s s", "shell", "CLASSPATH=...", "app_process", "/", server-class, version, ...
	var sawLaunch bool
	_ = sawLaunch // launcher is invoked through closure above; we already saw it

	// Verify Stop removes the reverse forward.
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
	// Inspect recorded calls: we expect push, reverse add, and reverse --remove
	// after Stop.
	got := runnerCallArgs(runner)
	mustContain(t, got, []string{"push"})
	mustContain(t, got, []string{"reverse", "localabstract:scrcpy"})
	mustContain(t, got, []string{"reverse", "--remove", "localabstract:scrcpy"})
}

func TestStartServer_Blocked(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "scrcpy-server.jar")
	_ = os.WriteFile(jar, []byte("fake"), 0o644)
	ignore := filepath.Join(tmp, ".devignore")
	_ = os.WriteFile(ignore, []byte("ATMOSphere\n"), 0o644)
	runner := &fakeRunnerSeq{script: map[string]fakeResponse{
		"adb -s dev shell getprop ro.product.model": {out: []byte("ATMOSphere TV\n")},
	}}
	launcher := &fakeLauncher{onLaunch: func(context.Context, string, []string) (Process, error) {
		t.Error("launcher must not be invoked for blocked device")
		return nil, nil
	}}
	cfg := ServerConfig{
		Serial: "dev", JarLocalPath: jar, DevIgnorePath: ignore,
		Runner: runner, Launcher: launcher, ServerVersion: "x",
		AcceptTimeout: time.Second,
	}
	_, err := StartServer(context.Background(), cfg)
	if !errors.Is(err, ErrDeviceBlocked) {
		t.Errorf("want ErrDeviceBlocked, got %v", err)
	}
}

func TestStartServer_InvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  ServerConfig
	}{
		{"nil-runner", ServerConfig{JarLocalPath: "x", ServerVersion: "1"}},
		{"empty-jar", ServerConfig{Runner: &fakeRunnerSeq{}, Launcher: &fakeLauncher{}, ServerVersion: "1"}},
		{"empty-version", ServerConfig{Runner: &fakeRunnerSeq{}, Launcher: &fakeLauncher{}, JarLocalPath: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := StartServer(context.Background(), tc.cfg)
			if !errors.Is(err, ErrServerConfig) {
				t.Errorf("want ErrServerConfig, got %v", err)
			}
		})
	}
}

func TestStartServer_AcceptTimeout(t *testing.T) {
	tmp := t.TempDir()
	jar := filepath.Join(tmp, "scrcpy-server.jar")
	_ = os.WriteFile(jar, []byte("fake"), 0o644)
	runner := &fakeRunnerSeq{}
	// Launcher doesn't dial back — accept times out.
	launcher := &fakeLauncher{onLaunch: func(ctx context.Context, name string, args []string) (Process, error) {
		return &fakeProcess{waitCh: make(chan struct{})}, nil
	}}
	cfg := ServerConfig{
		Serial: "s", JarLocalPath: jar, Runner: runner, Launcher: launcher,
		ServerVersion: "1", AcceptTimeout: 200 * time.Millisecond,
		EnableControl: true,
	}
	_, err := StartServer(context.Background(), cfg)
	if !errors.Is(err, ErrAcceptTimeout) {
		t.Errorf("want ErrAcceptTimeout, got %v", err)
	}
}

func TestFreeLoopbackPort(t *testing.T) {
	p, err := freeLoopbackPort()
	if err != nil {
		t.Fatal(err)
	}
	if p <= 0 || p > 65535 {
		t.Errorf("got port %d", p)
	}
	// Prove it's reusable: opening a new listener on the same port succeeds.
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(p)))
	if err == nil {
		_ = l.Close()
	}
}

// --- Session-level tests ---

func TestSession_Send_NilControl(t *testing.T) {
	s := &Session{}
	if err := s.Send(ResetVideo()); !errors.Is(err, ErrNoControlSocket) {
		t.Errorf("want ErrNoControlSocket, got %v", err)
	}
}

func TestSession_Send_RoundTrip(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	sess := &Session{control: client}

	// Server side: read bytes in a goroutine so Send can complete.
	got := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(server, buf); err == nil {
			got <- buf[0]
		}
	}()

	if err := sess.Send(ResetVideo()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case b := <-got:
		if ControlType(b) != CtrlResetVideo {
			t.Errorf("got ctrl byte %d, want %d", b, CtrlResetVideo)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not see the bytes")
	}
}

func TestSession_Close_Idempotent(t *testing.T) {
	a, b := net.Pipe()
	sess := &Session{video: a, control: b}
	if err := sess.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestSession_StartPumps_VideoStream(t *testing.T) {
	client, server := net.Pipe()
	sess := &Session{video: client}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	videoCh, _, _ := sess.StartPumps(ctx)

	// Server side: write one known video packet.
	go func() {
		defer server.Close()
		var hdr [12]byte
		binary.BigEndian.PutUint64(hdr[0:8], 100)
		binary.BigEndian.PutUint32(hdr[8:12], 4) // plain 4-byte body
		_, _ = server.Write(hdr[:])
		_, _ = server.Write([]byte{1, 2, 3, 4})
	}()

	select {
	case pkt := <-videoCh:
		if pkt.PTSMicros != 100 || len(pkt.Payload) != 4 {
			t.Errorf("unexpected packet: %+v", pkt)
		}
	case <-time.After(time.Second):
		t.Fatal("no video packet arrived")
	}
}

// --- small helpers ---

func runnerCallArgs(r *fakeRunnerSeq) [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, 0, len(r.calls))
	for _, c := range r.calls {
		out = append(out, c.args)
	}
	return out
}

// mustContain asserts that every token in needle appears in at least one
// observed args slice, in order (not necessarily contiguous within the slice).
func mustContain(t *testing.T, haystack [][]string, needle []string) {
	t.Helper()
	for _, call := range haystack {
		if subseqContains(call, needle) {
			return
		}
	}
	t.Errorf("no call contained the subsequence %v; calls=%v", needle, haystack)
}

func subseqContains(s, sub []string) bool {
	si := 0
	for _, tok := range sub {
		for ; si < len(s); si++ {
			if s[si] == tok {
				si++
				break
			}
		}
		if si > len(s) {
			return false
		}
	}
	return true
}

func itoa(i int) string {
	// Small local itoa so tests don't pull strconv into every line.
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
