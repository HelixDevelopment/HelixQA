// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// ExecRunner is the production CommandRunner — shells out to named programs
// via exec.CommandContext and returns stdout. On non-zero exit it wraps the
// underlying ExitError with stderr content so callers surface a meaningful
// diagnostic rather than a bare "exit status 1".
//
// ExecRunner is safe for concurrent use (exec.CommandContext is) and holds
// no state.
type ExecRunner struct{}

// Run implements CommandRunner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return out, fmt.Errorf("scrcpy: exec %s: %w (stderr: %s)", name, err, string(ee.Stderr))
	}
	return out, fmt.Errorf("scrcpy: exec %s: %w", name, err)
}

// OSProcessLauncher is the production ProcessLauncher wrapping
// exec.CommandContext for long-running child processes (typically
// `adb shell app_process …` spawning scrcpy-server.jar).
//
// The child's stdout is discarded — scrcpy-server's video bytes flow through
// the adb reverse-forwarded TCP socket, not stdout. Stderr is exposed via
// Process.Stderr so HelixQA can tee server diagnostics into the session
// archive.
type OSProcessLauncher struct{}

// Launch implements ProcessLauncher.
func (OSProcessLauncher) Launch(ctx context.Context, name string, args ...string) (Process, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("scrcpy: StderrPipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stderr.Close()
		return nil, fmt.Errorf("scrcpy: exec %s: %w", name, err)
	}
	return &osExecProcess{cmd: cmd, stderr: stderr}, nil
}

type osExecProcess struct {
	cmd      *exec.Cmd
	stderr   io.ReadCloser
	killOnce sync.Once
	killErr  error
}

// Wait blocks until the process exits.
func (p *osExecProcess) Wait() error { return p.cmd.Wait() }

// Signal delivers a signal to the running process.
func (p *osExecProcess) Signal(sig os.Signal) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return errors.New("scrcpy: no process to signal")
	}
	return p.cmd.Process.Signal(sig)
}

// Stderr returns the stderr pipe. Callers drain it to avoid deadlocking the
// child once the OS pipe buffer fills.
func (p *osExecProcess) Stderr() io.Reader { return p.stderr }

// Kill forcibly terminates the process. Idempotent via sync.Once — Signal
// path uses SIGTERM; tests that want a hard kill call this.
func (p *osExecProcess) Kill() error {
	p.killOnce.Do(func() {
		if p.cmd.Process != nil {
			p.killErr = p.cmd.Process.Kill()
		}
	})
	return p.killErr
}

// DefaultRunner returns the production CommandRunner. Exposed as a function
// rather than a package-level var so callers can't monkey-patch it.
func DefaultRunner() CommandRunner { return ExecRunner{} }

// DefaultLauncher returns the production ProcessLauncher.
func DefaultLauncher() ProcessLauncher { return OSProcessLauncher{} }
