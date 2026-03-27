// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package allure provides a Bridge that wraps the Allure CLI tool
// for generating and opening HTML test reports via subprocess.
package allure

import (
	"context"
	"fmt"
	"os/exec"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Bridge wraps the Allure CLI tool for report generation and
// display.
type Bridge struct {
	binaryPath string
	runner     CommandRunner
}

// NewBridge returns a Bridge using the given binary path and runner.
// If binaryPath is empty, "allure" (resolved via PATH) is used.
func NewBridge(binaryPath string, runner CommandRunner) *Bridge {
	if binaryPath == "" {
		binaryPath = "allure"
	}
	return &Bridge{
		binaryPath: binaryPath,
		runner:     runner,
	}
}

// Available reports whether the allure binary can be found.
func (b *Bridge) Available() bool {
	path := b.binaryPath
	if path == "allure" {
		_, err := exec.LookPath("allure")
		return err == nil
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// GenerateReport generates a static HTML Allure report from the
// results in inputDir and writes the report to outputDir.
func (b *Bridge) GenerateReport(
	ctx context.Context,
	inputDir string,
	outputDir string,
) error {
	output, err := b.runner.Run(
		ctx, b.binaryPath,
		"generate", inputDir,
		"--output", outputDir,
		"--clean",
	)
	if err != nil {
		return fmt.Errorf(
			"allure: generate report: %w: %s",
			err, string(output),
		)
	}
	return nil
}

// OpenReport opens the Allure report in reportDir in the default
// web browser using the allure open command.
func (b *Bridge) OpenReport(
	ctx context.Context,
	reportDir string,
) error {
	output, err := b.runner.Run(
		ctx, b.binaryPath,
		"open", reportDir,
	)
	if err != nil {
		return fmt.Errorf(
			"allure: open report: %w: %s",
			err, string(output),
		)
	}
	return nil
}
