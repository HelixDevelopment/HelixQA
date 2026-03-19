// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner records commands for testing.
type mockRunner struct {
	mu       sync.Mutex
	calls    []mockCall
	failOn   map[string]error
	response []byte
}

type mockCall struct {
	name string
	args []string
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		failOn: make(map[string]error),
	}
}

func (m *mockRunner) Run(
	_ context.Context, name string, args ...string,
) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, mockCall{name: name, args: args})

	key := name + " " + strings.Join(args, " ")
	if err, ok := m.failOn[key]; ok {
		return nil, err
	}
	if err, ok := m.failOn[name]; ok {
		return nil, err
	}

	if m.response != nil {
		return m.response, nil
	}
	return []byte("ok"), nil
}

func (m *mockRunner) lastCall() *mockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

func (m *mockRunner) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// --- ADB Executor Tests ---

func TestADBExecutor_Click(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Click(context.Background(), 100, 200)
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	assert.Equal(t, "adb", c.name)
	assert.Contains(t, c.args, "tap")
	assert.Contains(t, c.args, "100")
	assert.Contains(t, c.args, "200")
}

func TestADBExecutor_Type(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Type(context.Background(), "hello")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "text")
	assert.Contains(t, c.args, "hello")
}

func TestADBExecutor_Scroll_Directions(t *testing.T) {
	directions := []string{"up", "down", "left", "right"}
	for _, dir := range directions {
		runner := newMockRunner()
		exec := NewADBExecutor("emulator-5554", runner)
		err := exec.Scroll(context.Background(), dir, 200)
		assert.NoError(t, err, "direction %s", dir)
		assert.Greater(t, runner.callCount(), 0)
	}
}

func TestADBExecutor_Scroll_InvalidDirection(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Scroll(context.Background(), "diagonal", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown scroll direction")
}

func TestADBExecutor_LongPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.LongPress(context.Background(), 300, 400)
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "swipe")
	assert.Contains(t, c.args, "1000")
}

func TestADBExecutor_Swipe(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Swipe(context.Background(), 100, 200, 300, 400)
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "swipe")
}

func TestADBExecutor_KeyPress(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.KeyPress(context.Background(), "KEYCODE_ENTER")
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "keyevent")
	assert.Contains(t, c.args, "KEYCODE_ENTER")
}

func TestADBExecutor_Back(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Back(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "KEYCODE_BACK")
}

func TestADBExecutor_Home(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Home(context.Background())
	require.NoError(t, err)

	c := runner.lastCall()
	assert.Contains(t, c.args, "KEYCODE_HOME")
}

func TestADBExecutor_Screenshot(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("PNG-DATA")
	exec := NewADBExecutor("emulator-5554", runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("PNG-DATA"), data)
}

func TestADBExecutor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["adb"] = fmt.Errorf("device offline")
	exec := NewADBExecutor("emulator-5554", runner)

	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adb screenshot")
}

func TestADBExecutor_Click_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["adb"] = fmt.Errorf("device offline")
	exec := NewADBExecutor("emulator-5554", runner)

	err := exec.Click(context.Background(), 100, 200)
	assert.Error(t, err)
}

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
