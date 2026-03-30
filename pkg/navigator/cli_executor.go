// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/detector"
)

// CLIExecutor implements ActionExecutor for CLI/TUI applications
// by sending stdin input and reading stdout output via a
// CommandRunner. Type and KeyPress drive input; Screenshot
// captures terminal output. Pointer-based UI methods (Click,
// Scroll, LongPress, Swipe, Back, Home) are no-ops.
type CLIExecutor struct {
	command string
	args    []string
	runner  detector.CommandRunner
}

// NewCLIExecutor creates a CLIExecutor for the given command.
// args are fixed arguments passed to every invocation.
func NewCLIExecutor(
	command string,
	args []string,
	runner detector.CommandRunner,
) *CLIExecutor {
	return &CLIExecutor{
		command: command,
		args:    args,
		runner:  runner,
	}
}

// cliKeyMap maps named keys to their control-character
// or ANSI escape sequence equivalents.
var cliKeyMap = map[string]string{
	"enter":  "\n",
	"tab":    "\t",
	"escape": "\x1b",
	"esc":    "\x1b",
	"up":     "\x1b[A",
	"down":   "\x1b[B",
	"right":  "\x1b[C",
	"left":   "\x1b[D",
	"ctrl-c": "\x03",
	"ctrl-d": "\x04",
	"ctrl-z": "\x1a",
}

// Type sends text as stdin input by running the command with
// the text appended as an argument representing the input.
func (c *CLIExecutor) Type(
	ctx context.Context, text string,
) error {
	runArgs := append(append([]string{}, c.args...), text)
	_, err := c.runner.Run(ctx, c.command, runArgs...)
	return err
}

// KeyPress maps the named key to a control character or ANSI
// escape sequence and sends it as stdin input. Unrecognised
// keys are sent verbatim.
func (c *CLIExecutor) KeyPress(
	ctx context.Context, key string,
) error {
	char, ok := cliKeyMap[key]
	if !ok {
		char = key
	}
	runArgs := append(append([]string{}, c.args...), char)
	_, err := c.runner.Run(ctx, c.command, runArgs...)
	return err
}

// Clear sends Ctrl-U (kill line) to clear the current input.
func (c *CLIExecutor) Clear(ctx context.Context) error {
	runArgs := append(append([]string{}, c.args...), "\x15")
	_, err := c.runner.Run(ctx, c.command, runArgs...)
	return err
}

// Screenshot captures terminal output by running the command
// and returning its stdout as raw bytes.
func (c *CLIExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	data, err := c.runner.Run(ctx, c.command, c.args...)
	if err != nil {
		return nil, fmt.Errorf("cli screenshot: %w", err)
	}
	return data, nil
}

// Click is not applicable for CLI — returns nil.
func (c *CLIExecutor) Click(
	_ context.Context, _, _ int,
) error {
	return nil
}

// Scroll is not applicable for CLI — returns nil.
func (c *CLIExecutor) Scroll(
	_ context.Context, _ string, _ int,
) error {
	return nil
}

// LongPress is not applicable for CLI — returns nil.
func (c *CLIExecutor) LongPress(
	_ context.Context, _, _ int,
) error {
	return nil
}

// Swipe is not applicable for CLI — returns nil.
func (c *CLIExecutor) Swipe(
	_ context.Context, _, _, _, _ int,
) error {
	return nil
}

// Back is not applicable for CLI — returns nil.
func (c *CLIExecutor) Back(_ context.Context) error {
	return nil
}

// Home is not applicable for CLI — returns nil.
func (c *CLIExecutor) Home(_ context.Context) error {
	return nil
}
