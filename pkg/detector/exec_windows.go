// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package detector

import (
	"os"
	"os/exec"
)

// isPIDAlive reports whether a process with the given PID is
// currently alive on Windows.
//
// Windows has no syscall.Kill / signal-0 probe. os.FindProcess on
// Windows actually opens a handle to the process and fails when no
// such process exists, so a non-nil error from FindProcess (or a
// nil process) means the PID is dead. This avoids shelling out to
// any external binary, matching the LVA-8 fix rationale on POSIX.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc == nil {
		return false
	}
	return true
}

// setProcessGroup is a no-op on Windows.
func setProcessGroup(cmd *exec.Cmd) {
	_ = cmd
	// Windows process group handling would require job objects.
	// For now, rely on context cancellation.
}

// killProcessGroup kills the process on Windows.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
