// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package detector

import (
	"os/exec"
)

// setProcessGroup is a no-op on Windows.
func setProcessGroup(cmd *exec.Cmd) {
	// Windows process group handling would require job objects
	// For now, rely on context cancellation
}

// killProcessGroup kills the process on Windows.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
