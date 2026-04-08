// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package detector

import (
	"os/exec"
	"syscall"
)

// setProcessGroup sets the process group ID for the command
// to allow killing the entire process tree on timeout.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup kills the entire process group.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Kill the process group (negative PID)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
