// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADBExecutor_Clear_SendsSelectAllThenDelete(
	t *testing.T,
) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Clear(context.Background())
	require.NoError(t, err)

	// Should have at least 2 calls: select-all and delete.
	calls := runner.calls
	require.GreaterOrEqual(t, len(calls), 2,
		"Clear must send at least select-all + delete",
	)

	// First call should be the select-all via longpress
	// KEYCODE_A (keycode 29).
	first := calls[0]
	assert.Equal(t, "adb", first.name)
	assert.Contains(t, first.args, "keyevent")
	assert.Contains(t, first.args, "--longpress")
	assert.Contains(t, first.args, "29")

	// Last call should be KEYCODE_DEL to delete selection.
	last := calls[len(calls)-1]
	assert.Equal(t, "adb", last.name)
	assert.Contains(t, last.args, "keyevent")
	assert.Contains(t, last.args, "KEYCODE_DEL")
}

func TestADBExecutor_Clear_FallbackOnSelectAllFailure(
	t *testing.T,
) {
	runner := newMockRunner()
	// Make the longpress call fail to trigger the fallback
	// path (MOVE_HOME + SHIFT+MOVE_END).
	runner.failOn["adb -s emulator-5554 shell "+
		"input keyevent --longpress 29"] =
		assert.AnError

	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Clear(context.Background())
	require.NoError(t, err)

	// Should have fallback calls: MOVE_HOME, SHIFT+MOVE_END,
	// then KEYCODE_DEL.
	calls := runner.calls
	require.GreaterOrEqual(t, len(calls), 3,
		"Fallback path must send MOVE_HOME, "+
			"SHIFT+MOVE_END, DEL",
	)

	// Last call must still be DEL.
	last := calls[len(calls)-1]
	assert.Contains(t, last.args, "KEYCODE_DEL")
}

func TestADBExecutor_Clear_DeleteError(t *testing.T) {
	runner := newMockRunner()
	// Fail all adb calls — the delete will fail.
	runner.failOn["adb"] = assert.AnError

	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Clear(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adb clear delete")
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
