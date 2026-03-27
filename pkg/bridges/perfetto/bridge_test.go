// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perfetto

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner is a test implementation of CommandRunner.
type mockRunner struct {
	responses map[string]mockResponse
	calls     []string
}

type mockResponse struct {
	output []byte
	err    error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string]mockResponse),
	}
}

func (m *mockRunner) On(
	cmdKey string, output []byte, err error,
) {
	m.responses[cmdKey] = mockResponse{output: output, err: err}
}

func (m *mockRunner) Run(
	ctx context.Context, name string, args ...string,
) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}
	m.calls = append(m.calls, key)

	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	if resp, ok := m.responses[name]; ok {
		return resp.output, resp.err
	}
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) {
			return resp.output, resp.err
		}
	}
	return nil, fmt.Errorf("no mock for: %s", key)
}

// --- NewBridge ---

func TestNewBridge_NotNil(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge(mock)
	assert.NotNil(t, b)
}

// --- Available ---

func TestBridge_Available_Found(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell which perfetto",
		[]byte("/usr/bin/perfetto\n"),
		nil,
	)
	b := NewBridge(mock)
	assert.True(t, b.Available())
}

func TestBridge_Available_NotFound(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell which perfetto",
		[]byte(""),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge(mock)
	assert.False(t, b.Available())
}

func TestBridge_Available_EmptyOutput(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell which perfetto",
		[]byte("   \n"),
		nil,
	)
	b := NewBridge(mock)
	assert.False(t, b.Available())
}

// --- adbArgs helper ---

func TestAdbArgs_WithDevice(t *testing.T) {
	args := adbArgs("emulator-5554", "shell", "ls")
	assert.Equal(t, []string{
		"-s", "emulator-5554", "shell", "ls",
	}, args)
}

func TestAdbArgs_WithoutDevice(t *testing.T) {
	args := adbArgs("", "shell", "ls")
	assert.Equal(t, []string{"shell", "ls"}, args)
}

// --- StartTrace ---

func TestBridge_StartTrace_Success(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s emulator-5554 shell perfetto"+
			" --config /data/local/tmp/trace.cfg"+
			" --out /data/local/tmp/trace.perfetto"+
			" --background",
		[]byte(""),
		nil,
	)
	b := NewBridge(mock)

	err := b.StartTrace(
		context.Background(),
		"emulator-5554",
		"/data/local/tmp/trace.cfg",
		"/data/local/tmp/trace.perfetto",
	)
	require.NoError(t, err)
}

func TestBridge_StartTrace_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell perfetto",
		[]byte("perfetto: config not found"),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge(mock)

	err := b.StartTrace(
		context.Background(), "",
		"/missing/trace.cfg", "/tmp/trace.perfetto",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perfetto: start trace")
}

func TestBridge_StartTrace_WithDevice(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s device123 shell perfetto",
		[]byte(""),
		nil,
	)
	b := NewBridge(mock)

	err := b.StartTrace(
		context.Background(),
		"device123",
		"/data/cfg",
		"/data/out",
	)
	require.NoError(t, err)
	assert.Contains(t, mock.calls[0], "-s device123")
}

// --- StopTrace ---

func TestBridge_StopTrace_Success(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof perfetto",
		[]byte("1234\n"),
		nil,
	)
	mock.On(
		"adb shell kill -SIGINT 1234",
		[]byte(""),
		nil,
	)
	b := NewBridge(mock)

	err := b.StopTrace(context.Background(), "")
	require.NoError(t, err)
}

func TestBridge_StopTrace_WithDevice(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s emulator-5554 shell pidof perfetto",
		[]byte("5678\n"),
		nil,
	)
	mock.On(
		"adb -s emulator-5554 shell kill -SIGINT 5678",
		[]byte(""),
		nil,
	)
	b := NewBridge(mock)

	err := b.StopTrace(context.Background(), "emulator-5554")
	require.NoError(t, err)
}

func TestBridge_StopTrace_PidofError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof perfetto",
		nil,
		fmt.Errorf("adb not found"),
	)
	b := NewBridge(mock)

	err := b.StopTrace(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perfetto: stop trace: pidof")
}

func TestBridge_StopTrace_NotRunning(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof perfetto",
		[]byte(""),
		nil,
	)
	b := NewBridge(mock)

	err := b.StopTrace(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perfetto not running")
}

func TestBridge_StopTrace_KillError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell pidof perfetto",
		[]byte("9999\n"),
		nil,
	)
	mock.On(
		"adb shell kill -SIGINT 9999",
		[]byte("permission denied"),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge(mock)

	err := b.StopTrace(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perfetto: stop trace: kill")
}

// --- PullTrace ---

func TestBridge_PullTrace_Success(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb pull /data/local/tmp/trace.perfetto /tmp/trace.perfetto",
		[]byte("1 file pulled"),
		nil,
	)
	b := NewBridge(mock)

	err := b.PullTrace(
		context.Background(), "",
		"/data/local/tmp/trace.perfetto",
		"/tmp/trace.perfetto",
	)
	require.NoError(t, err)
}

func TestBridge_PullTrace_WithDevice(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s emulator-5554 pull",
		[]byte("1 file pulled"),
		nil,
	)
	b := NewBridge(mock)

	err := b.PullTrace(
		context.Background(), "emulator-5554",
		"/data/trace.perfetto",
		"/tmp/trace.perfetto",
	)
	require.NoError(t, err)
	assert.Contains(t, mock.calls[0], "-s emulator-5554")
}

func TestBridge_PullTrace_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb pull",
		[]byte("error: remote object does not exist"),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge(mock)

	err := b.PullTrace(
		context.Background(), "",
		"/missing/trace.perfetto",
		"/tmp/trace.perfetto",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perfetto: pull trace")
}
