// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Process is the minimal contract a scrcpy-server child process must satisfy.
// The host implementation wraps exec.Cmd; tests inject a fake.
type Process interface {
	// Wait blocks until the process exits. Safe to call from any goroutine.
	Wait() error
	// Signal delivers a signal. Kill is the common case.
	Signal(sig os.Signal) error
	// Stderr returns a reader for the process stderr (scrcpy-server logs).
	Stderr() io.Reader
}

// ProcessLauncher starts a child process and returns it. Tests can supply a
// fake launcher that writes scripted output into the returned Process's
// Stderr and waits to be Signal'd.
type ProcessLauncher interface {
	Launch(ctx context.Context, name string, args ...string) (Process, error)
}

// ServerConfig describes a scrcpy-server launch.
type ServerConfig struct {
	// Serial is the ADB device serial. Empty means "the single connected
	// device"; production callers should always pass one.
	Serial string
	// JarLocalPath is the filesystem path to the scrcpy-server.jar bundled
	// with HelixQA (see package doc).
	JarLocalPath string
	// JarRemotePath is the destination on-device; defaults to
	// "/data/local/tmp/scrcpy-server.jar" when empty.
	JarRemotePath string
	// LocalTCPPort is the host-side port the server connects back to. When 0,
	// a free port is chosen.
	LocalTCPPort int
	// DevIgnorePath is the .devignore file to consult before opening the
	// control socket. Empty means: don't consult (testing only).
	DevIgnorePath string
	// Runner executes one-shot adb commands (push, reverse, getprop).
	Runner CommandRunner
	// Launcher starts the long-running "adb shell app_process …" child.
	Launcher ProcessLauncher
	// ServerVersion is the pinned scrcpy-server version string (matches the
	// bundled JAR). A mismatch crashes the server; callers SHOULD keep this
	// in sync with the JAR file under testdata/.
	ServerVersion string
	// VideoBitRate in bits/sec. Default 8_000_000 (8 Mbps) when 0.
	VideoBitRate int
	// MaxSize caps the longest screen edge; 0 means no cap.
	MaxSize int
	// AcceptTimeout caps how long Start waits for all three sockets to land.
	// Defaults to 30s when zero.
	AcceptTimeout time.Duration
	// EnableAudio controls whether the server opens the audio socket.
	EnableAudio bool
	// EnableControl controls whether the server opens the control socket.
	// Production always true; leave false only for video-only record tools.
	EnableControl bool
}

// Server is a running scrcpy-server.jar child plus its reverse-forward and
// inbound sockets. Use StartServer to construct one.
type Server struct {
	cfg       ServerConfig
	proc      Process
	listener  net.Listener
	localPort int
	session   *Session
	stopOnce  sync.Once
}

// ErrAcceptTimeout is returned when the server does not open all expected
// sockets within the configured timeout.
var ErrAcceptTimeout = errors.New("scrcpy: server failed to connect within timeout")

// ErrServerConfig is returned for invalid ServerConfig values.
var ErrServerConfig = errors.New("scrcpy: invalid server config")

// StartServer performs the full bring-up:
//
//  1. EnforceDevIgnore — abort if the device is deny-listed
//  2. adb push <jar> <remote>
//  3. adb reverse localabstract:scrcpy tcp:<port>
//  4. Open a loopback listener on <port>
//  5. adb shell CLASSPATH=<remote> app_process / com.genymobile.scrcpy.Server <args>
//  6. Accept video + optional audio + optional control sockets within AcceptTimeout
//  7. Return a *Server ready for Session() access
//
// On any error, all partially-acquired resources are released before returning.
func StartServer(ctx context.Context, cfg ServerConfig) (*Server, error) {
	if cfg.Runner == nil || cfg.Launcher == nil {
		return nil, fmt.Errorf("%w: Runner and Launcher are required", ErrServerConfig)
	}
	if cfg.JarLocalPath == "" {
		return nil, fmt.Errorf("%w: JarLocalPath empty", ErrServerConfig)
	}
	if cfg.ServerVersion == "" {
		return nil, fmt.Errorf("%w: ServerVersion empty", ErrServerConfig)
	}
	if cfg.JarRemotePath == "" {
		cfg.JarRemotePath = "/data/local/tmp/scrcpy-server.jar"
	}
	if cfg.AcceptTimeout == 0 {
		cfg.AcceptTimeout = 30 * time.Second
	}
	if cfg.VideoBitRate == 0 {
		cfg.VideoBitRate = 8_000_000
	}
	if cfg.DevIgnorePath != "" {
		if err := EnforceDevIgnore(ctx, cfg.Runner, cfg.Serial, cfg.DevIgnorePath); err != nil {
			return nil, err
		}
	}

	// Push the JAR. Harmless if already present; keeps the operation idempotent.
	pushArgs := append(adbArgs(cfg.Serial), "push", cfg.JarLocalPath, cfg.JarRemotePath)
	if _, err := cfg.Runner.Run(ctx, "adb", pushArgs...); err != nil {
		return nil, fmt.Errorf("scrcpy: adb push: %w", err)
	}

	// Set up the reverse forward.
	port := cfg.LocalTCPPort
	if port == 0 {
		var err error
		port, err = freeLoopbackPort()
		if err != nil {
			return nil, fmt.Errorf("scrcpy: pick port: %w", err)
		}
	}
	revArgs := append(adbArgs(cfg.Serial), "reverse", "localabstract:scrcpy", fmt.Sprintf("tcp:%d", port))
	if _, err := cfg.Runner.Run(ctx, "adb", revArgs...); err != nil {
		return nil, fmt.Errorf("scrcpy: adb reverse: %w", err)
	}

	// Open the loopback listener BEFORE launching the server.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		// Best-effort: remove the forward we just created.
		_, _ = cfg.Runner.Run(ctx, "adb", append(adbArgs(cfg.Serial), "reverse", "--remove", "localabstract:scrcpy")...)
		return nil, fmt.Errorf("scrcpy: listen %d: %w", port, err)
	}

	// Launch the server.
	appArgs := append(adbArgs(cfg.Serial),
		"shell",
		fmt.Sprintf("CLASSPATH=%s", cfg.JarRemotePath),
		"app_process", "/", "com.genymobile.scrcpy.Server",
		cfg.ServerVersion,
		fmt.Sprintf("bit_rate=%d", cfg.VideoBitRate),
	)
	if cfg.MaxSize > 0 {
		appArgs = append(appArgs, fmt.Sprintf("max_size=%d", cfg.MaxSize))
	}
	if cfg.EnableControl {
		appArgs = append(appArgs, "control=true")
	} else {
		appArgs = append(appArgs, "control=false")
	}
	if cfg.EnableAudio {
		appArgs = append(appArgs, "audio=true")
	} else {
		appArgs = append(appArgs, "audio=false")
	}
	proc, err := cfg.Launcher.Launch(ctx, "adb", appArgs...)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("scrcpy: launch app_process: %w", err)
	}

	// Accept sockets: always video; audio + control gated by config.
	wantSockets := 1
	if cfg.EnableAudio {
		wantSockets++
	}
	if cfg.EnableControl {
		wantSockets++
	}
	conns, err := acceptAll(listener, wantSockets, cfg.AcceptTimeout)
	if err != nil {
		_ = proc.Signal(os.Interrupt)
		_ = listener.Close()
		return nil, err
	}

	// scrcpy v3 opens video first, then audio (if enabled), then control (if enabled).
	sess := &Session{video: conns[0]}
	idx := 1
	if cfg.EnableAudio {
		sess.audio = conns[idx]
		idx++
	}
	if cfg.EnableControl {
		sess.control = conns[idx]
	}

	srv := &Server{
		cfg:       cfg,
		proc:      proc,
		listener:  listener,
		localPort: port,
		session:   sess,
	}
	return srv, nil
}

// Session returns the attached Session. Only valid between StartServer and Stop.
func (s *Server) Session() *Session { return s.session }

// LocalPort reports the actual loopback port selected (useful when the caller
// passed LocalTCPPort=0 and let the OS pick).
func (s *Server) LocalPort() int { return s.localPort }

// Stop terminates the server child, closes all sockets, and removes the ADB
// reverse forward. Idempotent and safe to call from multiple goroutines.
func (s *Server) Stop() error {
	var firstErr error
	s.stopOnce.Do(func() {
		if s.session != nil {
			if err := s.session.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if s.proc != nil {
			if err := s.proc.Signal(os.Interrupt); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if s.listener != nil {
			if err := s.listener.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if s.cfg.Runner != nil {
			revRemove := append(adbArgs(s.cfg.Serial), "reverse", "--remove", "localabstract:scrcpy")
			if _, err := s.cfg.Runner.Run(context.Background(), "adb", revRemove...); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("scrcpy: adb reverse --remove: %w", err)
			}
		}
	})
	return firstErr
}

// --- helpers ---

func adbArgs(serial string) []string {
	if serial == "" {
		return nil
	}
	return []string{"-s", serial}
}

func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	_, ps, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(ps)
	if err != nil {
		return 0, err
	}
	return port, nil
}

func acceptAll(l net.Listener, n int, timeout time.Duration) ([]net.Conn, error) {
	deadline := time.Now().Add(timeout)
	conns := make([]net.Conn, 0, n)
	// Use SetDeadline on the listener to bound Accept.
	type deadlineSetter interface {
		SetDeadline(time.Time) error
	}
	if ds, ok := l.(deadlineSetter); ok {
		if err := ds.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}
	for i := 0; i < n; i++ {
		c, err := l.Accept()
		if err != nil {
			for _, prev := range conns {
				_ = prev.Close()
			}
			if isTimeout(err) {
				return nil, fmt.Errorf("%w (got %d of %d)", ErrAcceptTimeout, len(conns), n)
			}
			return nil, fmt.Errorf("scrcpy: accept: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, nil
}

func isTimeout(err error) bool {
	var ne interface{ Timeout() bool }
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "i/o timeout")
}
