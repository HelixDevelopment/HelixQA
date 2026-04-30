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
	require.NotNil(t, c)
	assert.Equal(t, "adb", c.name)
	assert.Contains(t, c.args, "shell")
	// The atomic command puts clear+type in a single shell
	// script argument, so check the last arg as a string.
	script := c.args[len(c.args)-1]
	assert.Contains(t, script, "input text")
	assert.Contains(t, script, "hello")
}

// TestADBExecutor_Type_OpensImeBeforeTyping is the Article XI §11.5
// regression guard for the Compose-TV form-fill failure documented
// in qa-results/session-20260429_164618 and reproduced manually in
// docs/audits/androidtv-realdevice-2026-04-29.md.
//
// On Compose-TV (Jetpack Compose for TV) a focused EditText has no
// IME open until DPAD_CENTER is pressed. Without that, every
// `adb shell input text` is silently dropped — login banks fail
// with a stagnant frame and HelixQA reports stagnation as crash.
//
// The Type() implementation MUST emit DPAD_CENTER before typing
// and BACK after typing. If a future refactor drops either, this
// test fails — exposing the bluff before it ships.
func TestADBExecutor_Type_OpensImeBeforeTyping(t *testing.T) {
	runner := newMockRunner()
	exec := NewADBExecutor("emulator-5554", runner)
	err := exec.Type(context.Background(), "admin")
	require.NoError(t, err)

	c := runner.lastCall()
	require.NotNil(t, c)
	script := c.args[len(c.args)-1]

	imeOpen := strings.Index(script, "KEYCODE_DPAD_CENTER")
	textIdx := strings.Index(script, "input text")
	require.NotEqual(t, -1, imeOpen, "expected KEYCODE_DPAD_CENTER in script; got: %s", script)
	require.NotEqual(t, -1, textIdx, "expected `input text` in script; got: %s", script)
	require.Less(t, imeOpen, textIdx,
		"KEYCODE_DPAD_CENTER must appear BEFORE `input text` "+
			"so the IME is open before keystrokes are sent. Got: %s",
		script,
	)

	backIdx := strings.Index(script, "KEYCODE_BACK")
	require.NotEqual(t, -1, backIdx, "expected KEYCODE_BACK postlude in script; got: %s", script)
	require.Greater(t, backIdx, textIdx,
		"KEYCODE_BACK must appear AFTER `input text` so the "+
			"keyboard is dismissed. Got: %s",
		script,
	)
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
	// Return a larger response with varied content to pass validation
	// (>=5000 bytes, non-uniform — matches Screenshot threshold)
	var response []byte
	for i := 0; i < 6000; i++ {
		response = append(response, byte(i%256))
	}
	runner.response = response
	exec := NewADBExecutor("emulator-5554", runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, response, data)
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
