// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADBExecutor_Clear_AtomicShellCommand(
	t *testing.T,
) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Clear(context.Background())
	require.NoError(t, err)

	// Single atomic shell call: MOVE_END + 20 DEL keycodes.
	calls := runner.calls
	require.Equal(t, 1, len(calls),
		"Clear must use a single atomic shell command",
	)

	c := calls[0]
	assert.Equal(t, "adb", c.name)
	assert.Contains(t, c.args, "shell")
	// The script is the last arg.
	script := c.args[len(c.args)-1]
	assert.Contains(t, script, "KEYCODE_MOVE_END")
	assert.Contains(t, script, "67")
}

func TestADBExecutor_Clear_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["adb"] = assert.AnError

	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Clear(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adb clear")
}

func TestPlaywrightExecutor_Clear_Interface(t *testing.T) {
	// Verify PlaywrightExecutor satisfies ActionExecutor
	// (including the Clear method).
	var _ ActionExecutor = &PlaywrightExecutor{}
}

func TestX11Executor_Clear_Interface(t *testing.T) {
	// Verify X11Executor satisfies ActionExecutor
	// (including the Clear method).
	var _ ActionExecutor = &X11Executor{}
}

func TestCLIExecutor_Clear_Interface(t *testing.T) {
	// Verify CLIExecutor satisfies ActionExecutor
	// (including the Clear method).
	var _ ActionExecutor = &CLIExecutor{}
}

func TestAPIExecutor_Clear_Noop(t *testing.T) {
	runner := newMockRunner()
	exec := NewAPIExecutor("http://localhost:8080", runner)

	err := exec.Clear(context.Background())
	require.NoError(t, err)

	// API executor Clear is a no-op — no commands sent.
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Clear_SendsCtrlU(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)

	err := exec.Clear(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	assert.Equal(t, "bash", c.name)
	// Ctrl-U is \x15.
	assert.Contains(t, c.args, "\x15")
}
