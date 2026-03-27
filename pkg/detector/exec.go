// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DefaultCommandTimeout is the maximum time any single
// external command (ADB, etc.) is allowed to run before
// being killed. This prevents hung ADB sessions (e.g.
// uiautomator dump returning "null root node") from
// blocking the entire pipeline.
const DefaultCommandTimeout = 15 * time.Second

// execRunner implements CommandRunner using os/exec.
// Every command is wrapped with a per-command timeout
// to prevent indefinite blocking.
type execRunner struct {
	timeout time.Duration
}

// NewExecRunner returns a CommandRunner that executes commands
// via os/exec. This is the default runner used by the detector
// and is suitable for production use with real system commands.
// Each command is enforced with a 15-second timeout.
func NewExecRunner() CommandRunner {
	return &execRunner{timeout: DefaultCommandTimeout}
}

// NewExecRunnerWithTimeout returns a CommandRunner with a
// custom per-command timeout.
func NewExecRunnerWithTimeout(
	timeout time.Duration,
) CommandRunner {
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	return &execRunner{timeout: timeout}
}

// Run executes a command and returns its combined output.
// A per-command timeout is applied on top of any existing
// context deadline to prevent individual commands from
// blocking indefinitely.
func (r *execRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	cmdCtx, cancel := context.WithTimeout(
		ctx, timeout,
	)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	out, err := cmd.CombinedOutput()
	if cmdCtx.Err() == context.DeadlineExceeded &&
		ctx.Err() == nil {
		// The per-command timeout fired, not the parent
		// context. Return a clear error so callers can
		// distinguish a hung command from a pipeline
		// cancellation.
		return out, fmt.Errorf(
			"command timed out after %v: %s %v",
			r.timeout, name, args,
		)
	}
	return out, err
}
