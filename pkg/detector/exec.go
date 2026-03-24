// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"os/exec"
)

// execRunner implements CommandRunner using os/exec.
type execRunner struct{}

// NewExecRunner returns a CommandRunner that executes commands
// via os/exec. This is the default runner used by the detector
// and is suitable for production use with real system commands.
func NewExecRunner() CommandRunner {
	return &execRunner{}
}

// Run executes a command and returns its combined output.
func (r *execRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
