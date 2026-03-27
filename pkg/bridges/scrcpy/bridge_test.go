// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

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
	assert.Equal(t, "scrcpy", b.binaryPath)
}

func TestNewBridge_CustomBinaryPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/usr/local/bin/scrcpy", mock)
	assert.Equal(t, "/usr/local/bin/scrcpy", b.binaryPath)
}

// --- Available ---

func TestBridge_Available_NotInPath(t *testing.T) {
	mock := newMockRunner()
	// Use a path that definitely does not exist.
	b := NewBridge("/nonexistent/scrcpy-binary", mock)
	assert.False(t, b.Available())
}

// --- buildArgs ---

func TestBridge_BuildArgs_WithDevice(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	args := b.buildArgs("emulator-5554", "--record", "/tmp/out.mp4")
	assert.Equal(t, []string{
		"--serial", "emulator-5554",
		"--record", "/tmp/out.mp4",
	}, args)
}

func TestBridge_BuildArgs_NoDevice(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	args := b.buildArgs("", "--record", "/tmp/out.mp4")
	assert.Equal(t, []string{"--record", "/tmp/out.mp4"}, args)
}

func TestBridge_BuildArgs_MirrorNoDevice(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	args := b.buildArgs("")
	assert.Empty(t, args)
}

// --- Stop ---

func TestBridge_Stop_NoProcess(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	// Should be a no-op and return nil.
	err := b.Stop()
	require.NoError(t, err)
}

// --- Record / Mirror: already running guard ---

func TestBridge_Record_AlreadyRunning(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	// Simulate an existing process by setting the field directly.
	b.process = &fakeCmd{}

	err := b.Record(
		context.Background(), "emulator-5554", "/tmp/out.mp4",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scrcpy: record: already running")
}

func TestBridge_Mirror_AlreadyRunning(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("scrcpy", mock)
	b.process = &fakeCmd{}

	err := b.Mirror(context.Background(), "emulator-5554")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scrcpy: mirror: already running")
}

// --- Record: binary not found error ---

func TestBridge_Record_BinaryNotFound(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/nonexistent/scrcpy", mock)

	err := b.Record(
		context.Background(), "emulator-5554", "/tmp/out.mp4",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scrcpy: record: start")
}

// --- Mirror: binary not found error ---

func TestBridge_Mirror_BinaryNotFound(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/nonexistent/scrcpy", mock)

	err := b.Mirror(context.Background(), "emulator-5554")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scrcpy: mirror: start")
}
