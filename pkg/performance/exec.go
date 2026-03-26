// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"context"
	"os/exec"
)

// execRunner implements CommandRunner using os/exec.
type execRunner struct{}

// Run executes a command and returns its combined output.
func (r *execRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
