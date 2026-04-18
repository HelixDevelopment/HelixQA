// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ld_preload

import "syscall"

// mkfifo creates a named pipe at path with mode 0600.
func mkfifo(path string) error {
	return syscall.Mkfifo(path, 0600)
}
