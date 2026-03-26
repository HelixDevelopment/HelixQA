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

// --- X11 Executor Tests ---

func TestX11Executor_Click(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Click(context.Background(), 100, 200)
	require.NoError(t, err)

	// Should have called mousemove then click.
	assert.GreaterOrEqual(t, runner.callCount(), 2)
}

func TestX11Executor_Type(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Type(context.Background(), "hello world")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Equal(t, "xdotool", c.name)
	assert.Contains(t, c.args, "type")
}

func TestX11Executor_Scroll_Up(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Scroll(context.Background(), "up", 3)
	require.NoError(t, err)
	// 3 scroll clicks.
	assert.Equal(t, 3, runner.callCount())
}

func TestX11Executor_Scroll_Down(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Scroll(context.Background(), "down", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, runner.callCount())
}

func TestX11Executor_Back(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Back(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "alt+Left")
}

func TestX11Executor_Home(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Home(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "super")
}

func TestX11Executor_Screenshot(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("X11-SCREENSHOT")
	exec := NewX11Executor(":0", runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("X11-SCREENSHOT"), data)
}

func TestX11Executor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["import"] = fmt.Errorf("no display")
	exec := NewX11Executor(":0", runner)

	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "x11 screenshot")
}

func TestX11Executor_LongPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.LongPress(context.Background(), 100, 200)
	require.NoError(t, err)
	// Should call mousemove, mousedown, mouseup.
	assert.GreaterOrEqual(t, runner.callCount(), 3)
}

func TestX11Executor_Swipe(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.Swipe(context.Background(), 10, 20, 30, 40)
	require.NoError(t, err)
	// Should call mousemove, mousedown, mousemove, mouseup.
	assert.GreaterOrEqual(t, runner.callCount(), 4)
}

func TestX11Executor_KeyPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewX11Executor(":0", runner)
	err := exec.KeyPress(context.Background(), "Return")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "Return")
}

func TestX11Executor_Interface(t *testing.T) {
	// Verify X11Executor satisfies ActionExecutor.
	var _ ActionExecutor = &X11Executor{}
}
