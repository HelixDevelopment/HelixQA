// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package appium provides a Bridge that wraps the Appium server
// tool for mobile test automation via subprocess.
package appium

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Bridge wraps the Appium server tool, managing a single running
// Appium server subprocess.
type Bridge struct {
	binaryPath string
	runner     CommandRunner

	mu      sync.Mutex
	process *exec.Cmd
}

// NewBridge returns a Bridge using the given binary path and runner.
// If binaryPath is empty, "appium" (resolved via PATH) is used.
func NewBridge(binaryPath string, runner CommandRunner) *Bridge {
	if binaryPath == "" {
		binaryPath = "appium"
	}
	return &Bridge{
		binaryPath: binaryPath,
		runner:     runner,
	}
}

// Available reports whether the appium binary can be found.
func (b *Bridge) Available() bool {
	path := b.binaryPath
	if path == "appium" {
		_, err := exec.LookPath("appium")
		return err == nil
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// StartServer starts the Appium server on the specified port.
// The server runs until StopServer() is called or the context is
// cancelled.
func (b *Bridge) StartServer(
	ctx context.Context,
	port string,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.process != nil {
		return fmt.Errorf("appium: start server: already running")
	}

	args := []string{"--port", port}
	cmd := exec.CommandContext(ctx, b.binaryPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(
			"appium: start server: start: %w", err,
		)
	}
	b.process = cmd
	return nil
}

// StopServer terminates the running Appium server subprocess.
// It is a no-op if no server is running.
func (b *Bridge) StopServer() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.process == nil {
		return nil
	}

	proc := b.process
	b.process = nil

	if proc.Process != nil {
		if err := proc.Process.Kill(); err != nil {
			return fmt.Errorf(
				"appium: stop server: kill: %w", err,
			)
		}
	}
	// Wait to reap the process; ignore intentional-kill error.
	_ = proc.Wait()
	return nil
}

// Status checks whether the Appium server is responding by querying
// the /status endpoint via the runner. It returns (true, nil) when
// the server responds with output containing "ready".
func (b *Bridge) Status(ctx context.Context) (bool, error) {
	output, err := b.runner.Run(
		ctx, "curl", "-sf",
		"http://localhost:4723/status",
	)
	if err != nil {
		// Server not reachable.
		return false, nil
	}
	ready := strings.Contains(string(output), "ready") ||
		strings.Contains(string(output), "\"status\":0")
	if !ready {
		return false, fmt.Errorf(
			"appium: status: unexpected response: %s",
			strings.TrimSpace(string(output)),
		)
	}
	return true, nil
}
