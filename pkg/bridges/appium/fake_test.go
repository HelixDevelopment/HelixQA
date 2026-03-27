// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package appium

import "os/exec"

// fakeCmd is a zero-value exec.Cmd used in tests to simulate a
// running process without actually starting one.
type fakeCmd = exec.Cmd
