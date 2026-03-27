// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package scrcpy provides a Bridge that wraps the scrcpy tool for
// Android screen mirroring and recording via subprocess.
package scrcpy

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Bridge wraps the scrcpy tool for Android screen mirroring and
// recording. It manages a single active scrcpy subprocess.
type Bridge struct {
	binaryPath string
	runner     CommandRunner

	mu      sync.Mutex
	process *exec.Cmd
}

// NewBridge returns a Bridge using the given binary path and runner.
// If binaryPath is empty, "scrcpy" (resolved via PATH) is used.
func NewBridge(binaryPath string, runner CommandRunner) *Bridge {
	if binaryPath == "" {
		binaryPath = "scrcpy"
	}
	return &Bridge{
		binaryPath: binaryPath,
		runner:     runner,
	}
}

// Available reports whether the scrcpy binary can be found.
func (b *Bridge) Available() bool {
	path := b.binaryPath
	if path == "scrcpy" {
		_, err := exec.LookPath("scrcpy")
		return err == nil
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// Record starts a screen recording session for the given Android
// device, writing the output to outputPath on the local machine.
// The recording runs until Stop() is called.
func (b *Bridge) Record(
	ctx context.Context,
	device string,
	outputPath string,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.process != nil {
		return fmt.Errorf("scrcpy: record: already running")
	}

	args := b.buildArgs(device, "--record", outputPath)
	cmd := exec.CommandContext(ctx, b.binaryPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("scrcpy: record: start: %w", err)
	}
	b.process = cmd
	return nil
}

// Stop terminates any running scrcpy subprocess (recording or
// mirroring). It is a no-op if no process is running.
func (b *Bridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.process == nil {
		return nil
	}

	proc := b.process
	b.process = nil

	if proc.Process != nil {
		if err := proc.Process.Kill(); err != nil {
			return fmt.Errorf("scrcpy: stop: kill: %w", err)
		}
	}
	// Wait to reap the process; ignore the exit error since we
	// killed it intentionally.
	_ = proc.Wait()
	return nil
}

// Mirror starts a live screen mirroring session for the given Android
// device. The session runs until Stop() is called or the context is
// cancelled.
func (b *Bridge) Mirror(
	ctx context.Context,
	device string,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.process != nil {
		return fmt.Errorf("scrcpy: mirror: already running")
	}

	args := b.buildArgs(device)
	cmd := exec.CommandContext(ctx, b.binaryPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("scrcpy: mirror: start: %w", err)
	}
	b.process = cmd
	return nil
}

// buildArgs constructs the scrcpy argument slice. It prepends
// --serial <device> when device is non-empty and appends any
// extra arguments provided.
func (b *Bridge) buildArgs(device string, extra ...string) []string {
	var args []string
	if device != "" {
		args = append(args, "--serial", device)
	}
	return append(args, extra...)
}
