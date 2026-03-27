// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package perfetto provides a Bridge that wraps the Perfetto tracing
// tool for Android performance tracing via ADB subprocess commands.
package perfetto

import (
	"context"
	"fmt"
	"strings"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Bridge wraps the Perfetto tracing tool, dispatching ADB commands
// to start, stop, and retrieve traces from an Android device.
type Bridge struct {
	runner CommandRunner
}

// NewBridge returns a Bridge using the given runner.
func NewBridge(runner CommandRunner) *Bridge {
	return &Bridge{runner: runner}
}

// Available reports whether the perfetto binary is present on the
// connected Android device by querying its path via ADB shell.
func (b *Bridge) Available() bool {
	output, err := b.runner.Run(
		context.Background(),
		"adb", "shell", "which", "perfetto",
	)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// StartTrace starts a Perfetto trace session on the given device
// using the provided config file, writing the trace to outputPath
// on the device. The trace runs in the background until StopTrace
// is called.
func (b *Bridge) StartTrace(
	ctx context.Context,
	device string,
	configPath string,
	outputPath string,
) error {
	args := adbArgs(device,
		"shell", "perfetto",
		"--config", configPath,
		"--out", outputPath,
		"--background",
	)
	output, err := b.runner.Run(ctx, "adb", args...)
	if err != nil {
		return fmt.Errorf(
			"perfetto: start trace: %w: %s",
			err, strings.TrimSpace(string(output)),
		)
	}
	return nil
}

// StopTrace stops the running Perfetto trace session on the given
// device by sending SIGINT to the perfetto process.
func (b *Bridge) StopTrace(
	ctx context.Context,
	device string,
) error {
	// Locate the perfetto PID and send SIGINT to flush and stop.
	pidArgs := adbArgs(device, "shell", "pidof", "perfetto")
	pidOut, err := b.runner.Run(ctx, "adb", pidArgs...)
	if err != nil {
		return fmt.Errorf(
			"perfetto: stop trace: pidof: %w", err,
		)
	}
	pid := strings.TrimSpace(string(pidOut))
	if pid == "" {
		return fmt.Errorf(
			"perfetto: stop trace: perfetto not running",
		)
	}

	killArgs := adbArgs(device, "shell", "kill", "-SIGINT", pid)
	output, err := b.runner.Run(ctx, "adb", killArgs...)
	if err != nil {
		return fmt.Errorf(
			"perfetto: stop trace: kill: %w: %s",
			err, strings.TrimSpace(string(output)),
		)
	}
	return nil
}

// PullTrace copies a trace file from remotePath on the device to
// localPath on the host using ADB pull.
func (b *Bridge) PullTrace(
	ctx context.Context,
	device string,
	remotePath string,
	localPath string,
) error {
	args := adbArgs(device, "pull", remotePath, localPath)
	output, err := b.runner.Run(ctx, "adb", args...)
	if err != nil {
		return fmt.Errorf(
			"perfetto: pull trace: %w: %s",
			err, strings.TrimSpace(string(output)),
		)
	}
	return nil
}

// adbArgs prepends -s <device> when device is non-empty.
func adbArgs(device string, args ...string) []string {
	if device != "" {
		return append([]string{"-s", device}, args...)
	}
	return args
}
