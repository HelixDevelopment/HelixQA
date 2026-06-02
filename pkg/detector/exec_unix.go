// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package detector

import (
	"errors"
	"os/exec"
	"syscall"
)

// isPIDAlive reports whether a process with the given PID is
// currently alive, using the POSIX "signal 0" liveness probe
// directly via syscall.Kill rather than shelling out to the
// platform `kill` binary.
//
// Why not exec `kill -0 <pid>`: the platform `/bin/kill` BINARY
// (which os/exec resolves — NOT the shell builtin) has
// inconsistent exit-code semantics across platforms. On macOS,
// `/bin/kill -0 <out-of-range-pid>` exits 0 for an absent PID
// instead of failing, so a dead process reads as alive. That is
// the LVA-8 crash-detector bluff: a dead PID reported alive →
// HasCrash=false → the validator reports a crashed step as
// passed. syscall.Kill(pid, 0) avoids the binary entirely and
// returns reliable cross-platform errno semantics:
//   - nil          → process exists and we may signal it (alive)
//   - syscall.EPERM → process exists but we lack permission (alive)
//   - syscall.ESRCH → no such process (dead)
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but is owned by another user;
	// it is alive for liveness-detection purposes.
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	// ESRCH (and any other error such as EINVAL on a bad pid) means
	// the process is not reachable as a live target → treat as dead.
	return false
}

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
