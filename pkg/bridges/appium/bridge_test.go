// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package appium

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

func TestNewBridge_DefaultBinaryPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("", mock)
	assert.Equal(t, "appium", b.binaryPath)
}

func TestNewBridge_CustomBinaryPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/usr/local/bin/appium", mock)
	assert.Equal(t, "/usr/local/bin/appium", b.binaryPath)
}

// --- Available ---

func TestBridge_Available_NotInPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/nonexistent/appium-binary", mock)
	assert.False(t, b.Available())
}

// --- StopServer: no-op when not running ---

func TestBridge_StopServer_NoProcess(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("appium", mock)
	err := b.StopServer()
	require.NoError(t, err)
}

// --- StartServer: binary not found ---

func TestBridge_StartServer_BinaryNotFound(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/nonexistent/appium", mock)

	err := b.StartServer(context.Background(), "4723")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appium: start server: start")
}

// --- StartServer: already running guard ---

func TestBridge_StartServer_AlreadyRunning(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("appium", mock)
	b.process = &fakeCmd{}

	err := b.StartServer(context.Background(), "4723")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appium: start server: already running")
}

// --- Status ---

func TestBridge_Status_ServerReady(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		`curl -sf http://localhost:4723/status`,
		[]byte(`{"status":0,"value":{"ready":true}}`),
		nil,
	)
	b := NewBridge("appium", mock)

	ready, err := b.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestBridge_Status_ServerReadyText(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		`curl -sf http://localhost:4723/status`,
		[]byte(`{"ready":true,"message":"Appium is ready"}`),
		nil,
	)
	b := NewBridge("appium", mock)

	ready, err := b.Status(context.Background())
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestBridge_Status_ServerNotReachable(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		`curl -sf http://localhost:4723/status`,
		nil,
		fmt.Errorf("connection refused"),
	)
	b := NewBridge("appium", mock)

	ready, err := b.Status(context.Background())
	require.NoError(t, err)
	assert.False(t, ready)
}

func TestBridge_Status_UnexpectedResponse(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		`curl -sf http://localhost:4723/status`,
		[]byte(`{"error":"not initialised"}`),
		nil,
	)
	b := NewBridge("appium", mock)

	ready, err := b.Status(context.Background())
	require.Error(t, err)
	assert.False(t, ready)
	assert.Contains(t, err.Error(), "appium: status: unexpected response")
}
