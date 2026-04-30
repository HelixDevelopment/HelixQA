// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecRunner_Echo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/echo not available")  // SKIP-OK: #legacy-untriaged
	}
	r := ExecRunner{}
	out, err := r.Run(context.Background(), "/bin/echo", "hello", "world")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello world" {
		t.Errorf("stdout = %q, want %q", got, "hello world")
	}
}

func TestExecRunner_NonexistentBinary(t *testing.T) {
	r := ExecRunner{}
	_, err := r.Run(context.Background(), "/nonexistent-helixqa-scrcpy-runtime", "arg")
	if err == nil {
		t.Fatal("expected error for non-existent binary")
	}
	if !strings.Contains(err.Error(), "exec") {
		t.Errorf("error should mention 'exec': %v", err)
	}
}

func TestExecRunner_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/false not available")  // SKIP-OK: #legacy-untriaged
	}
	r := ExecRunner{}
	_, err := r.Run(context.Background(), "/bin/false")
	if err == nil {
		t.Fatal("expected error from /bin/false")
	}
}

func TestExecRunner_NonZeroExit_IncludesStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell not available")  // SKIP-OK: #legacy-untriaged
	}
	r := ExecRunner{}
	// `sh -c "echo oops >&2; exit 2"` — stderr content + non-zero exit.
	_, err := r.Run(context.Background(), "/bin/sh", "-c", "echo helixqa-test-oops >&2; exit 2")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "helixqa-test-oops") {
		t.Errorf("stderr not included in error: %v", err)
	}
}

func TestExecRunner_ContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/sleep not available")  // SKIP-OK: #legacy-untriaged
	}
	r := ExecRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := r.Run(ctx, "/bin/sleep", "10")
	if err == nil {
		t.Fatal("expected error from cancelled sleep")
	}
}

// --- OSProcessLauncher ---

func TestOSProcessLauncher_SpawnAndSignal(t *testing.T) {
	// bluff-scan: nil-only-ok (process lifecycle — spawn → SIGTERM → wait must complete without error)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/sleep not available")  // SKIP-OK: #legacy-untriaged
	}
	l := OSProcessLauncher{}
	p, err := l.Launch(context.Background(), "/bin/sleep", "10")
	if err != nil {
		t.Fatal(err)
	}
	// Drain stderr in background so the process doesn't deadlock waiting on
	// the buffer (sleep produces nothing, but it's good hygiene).
	go func() { _, _ = io.ReadAll(p.Stderr()) }()
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal: %v", err)
	}
	// Wait returns a "signal: terminated" non-nil err, which is fine.
	_ = p.Wait()
}

func TestOSProcessLauncher_Kill(t *testing.T) {
	// bluff-scan: nil-only-ok (process lifecycle — spawn → kill must complete without error)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/sleep not available")  // SKIP-OK: #legacy-untriaged
	}
	l := OSProcessLauncher{}
	p, err := l.Launch(context.Background(), "/bin/sleep", "10")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.ReadAll(p.Stderr()) }()
	ep := p.(*osExecProcess)
	if err := ep.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	// Second Kill via sync.Once is a no-op; still nil.
	if err := ep.Kill(); err != nil {
		t.Fatalf("second Kill: %v", err)
	}
	_ = p.Wait()
}

func TestOSProcessLauncher_NonexistentBinary(t *testing.T) {
	l := OSProcessLauncher{}
	_, err := l.Launch(context.Background(), "/nonexistent-helixqa-scrcpy-runtime-launch", "arg")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOSProcessLauncher_SignalNoProcess(t *testing.T) {
	// Build an osExecProcess manually with a nil Process to exercise the
	// guard in Signal().
	p := &osExecProcess{}
	if err := p.Signal(os.Interrupt); err == nil {
		t.Error("signal on nil process should error")
	}
}

func TestOSProcessLauncher_StderrIsNonNil(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX /bin/sh not available")  // SKIP-OK: #legacy-untriaged
	}
	l := OSProcessLauncher{}
	// sh -c emits a line to stderr immediately so we can observe it.
	p, err := l.Launch(context.Background(), "/bin/sh", "-c", "echo helixqa-stderr-hello >&2; sleep 10")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = p.(*osExecProcess).Kill()
		_ = p.Wait()
	}()
	buf := make([]byte, 64)
	n, _ := p.Stderr().Read(buf)
	if !strings.Contains(string(buf[:n]), "helixqa-stderr-hello") {
		t.Errorf("stderr pipe did not deliver expected bytes: got %q", string(buf[:n]))
	}
}

// --- Default constructors ---

func TestDefaultRunner_ReturnsExecRunner(t *testing.T) {
	r := DefaultRunner()
	if _, ok := r.(ExecRunner); !ok {
		t.Errorf("DefaultRunner returned %T, want ExecRunner", r)
	}
}

func TestDefaultLauncher_ReturnsOSProcessLauncher(t *testing.T) {
	l := DefaultLauncher()
	if _, ok := l.(OSProcessLauncher); !ok {
		t.Errorf("DefaultLauncher returned %T, want OSProcessLauncher", l)
	}
}
