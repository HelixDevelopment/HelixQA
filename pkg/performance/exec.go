// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// commandTimeout is the maximum time a single ADB/dumpsys
// command is allowed to run before being killed.
// REDUCED for aggressive performance - fail fast, recover fast.
const commandTimeout = 5 * time.Second

// execRunner implements CommandRunner using os/exec with
// a per-command timeout to prevent hung ADB sessions.
type execRunner struct{}

// Run executes a command and returns its combined output.
// A per-command timeout prevents individual commands from
// blocking indefinitely (e.g. dumpsys on unresponsive
// devices).
func (r *execRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	cmdCtx, cancel := context.WithTimeout(
		ctx, commandTimeout,
	)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)

	// Performance optimization: Set process group for faster cleanup on timeout
	setProcessGroup(cmd)

	out, err := cmd.CombinedOutput()
	if cmdCtx.Err() == context.DeadlineExceeded &&
		ctx.Err() == nil {
		// Kill the process group aggressively
		killProcessGroup(cmd)
		return out, fmt.Errorf(
			"command timed out after %v: %s",
			commandTimeout, name,
		)
	}
	return out, err
}
