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

// --- Playwright Executor Tests ---

func TestPlaywrightExecutor_Click(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Click(context.Background(), 50, 100)
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Equal(t, "npx", c.name)
	assert.Contains(t, c.args, "click")
}

func TestPlaywrightExecutor_Type(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Type(context.Background(), "test input")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "type")
	assert.Contains(t, c.args, "test input")
}

func TestPlaywrightExecutor_Scroll(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Scroll(context.Background(), "down", 300)
	require.NoError(t, err)
	assert.Greater(t, runner.callCount(), 0)
}

func TestPlaywrightExecutor_Back(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Back(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "back")
}

func TestPlaywrightExecutor_Home(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Home(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "http://localhost:8080")
}

func TestPlaywrightExecutor_Screenshot(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("WEB-SCREENSHOT")
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("WEB-SCREENSHOT"), data)
}

func TestPlaywrightExecutor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["npx"] = fmt.Errorf("browser closed")
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)

	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "playwright screenshot")
}

func TestPlaywrightExecutor_LongPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.LongPress(context.Background(), 50, 100)
	require.NoError(t, err)
}

func TestPlaywrightExecutor_Swipe(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.Swipe(context.Background(), 10, 20, 30, 40)
	require.NoError(t, err)
}

func TestPlaywrightExecutor_KeyPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor("http://localhost:8080", runner)
	err := exec.KeyPress(context.Background(), "Enter")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "press")
	assert.Contains(t, c.args, "Enter")
}

func TestPlaywrightExecutor_Interface(t *testing.T) {
	// Verify PlaywrightExecutor satisfies ActionExecutor.
	var _ ActionExecutor = &PlaywrightExecutor{}
}
