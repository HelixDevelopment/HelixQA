// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CLI Executor Tests ---

func TestCLIExecutor_Type_SendsTextAsInput(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Type(context.Background(), "hello world")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	assert.Equal(t, "bash", c.name)
	assert.Contains(t, c.args, "hello world")
}

func TestCLIExecutor_Type_CustomCommand(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("python3", []string{"-c", "import sys; print(sys.stdin.read())"}, runner)
	err := exec.Type(context.Background(), "test input")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	assert.Equal(t, "python3", c.name)
}

func TestCLIExecutor_KeyPress_Enter(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.KeyPress(context.Background(), "enter")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	// Enter maps to \n — passed as input
	assert.Equal(t, "bash", c.name)
	found := false
	for _, a := range c.args {
		if a == "\n" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected \\n in args for enter key")
}

func TestCLIExecutor_KeyPress_Tab(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.KeyPress(context.Background(), "tab")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	found := false
	for _, a := range c.args {
		if a == "\t" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected \\t in args for tab key")
}

func TestCLIExecutor_KeyPress_Escape(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.KeyPress(context.Background(), "escape")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	found := false
	for _, a := range c.args {
		if a == "\x1b" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ESC in args for escape key")
}

func TestCLIExecutor_KeyPress_Up(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.KeyPress(context.Background(), "up")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	found := false
	for _, a := range c.args {
		if a == "\x1b[A" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ANSI up arrow in args")
}

func TestCLIExecutor_KeyPress_Down(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.KeyPress(context.Background(), "down")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	found := false
	for _, a := range c.args {
		if a == "\x1b[B" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ANSI down arrow in args")
}

func TestCLIExecutor_Screenshot_ReturnsStdout(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("terminal output here")
	exec := NewCLIExecutor("bash", nil, runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("terminal output here"), data)
}

func TestCLIExecutor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["bash"] = fmt.Errorf("command failed")
	exec := NewCLIExecutor("bash", nil, runner)

	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cli screenshot")
}

func TestCLIExecutor_Click_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Click(context.Background(), 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Scroll_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Scroll(context.Background(), "up", 3)
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_LongPress_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.LongPress(context.Background(), 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Swipe_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Swipe(context.Background(), 0, 0, 10, 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Back_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Back(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Home_ReturnsNil(t *testing.T) {
	runner := newMockRunner()
	exec := NewCLIExecutor("bash", nil, runner)
	err := exec.Home(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, runner.callCount())
}

func TestCLIExecutor_Interface(t *testing.T) {
	// Verify CLIExecutor satisfies ActionExecutor.
	var _ ActionExecutor = &CLIExecutor{}
}
