// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package allure

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
	assert.Equal(t, "allure", b.binaryPath)
}

func TestNewBridge_CustomBinaryPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/usr/local/bin/allure", mock)
	assert.Equal(t, "/usr/local/bin/allure", b.binaryPath)
}

// --- Available ---

func TestBridge_Available_NotInPath(t *testing.T) {
	mock := newMockRunner()
	b := NewBridge("/nonexistent/allure-binary", mock)
	assert.False(t, b.Available())
}

// --- GenerateReport ---

func TestBridge_GenerateReport_Success(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"allure generate /tmp/results --output /tmp/report --clean",
		[]byte("Report successfully generated"),
		nil,
	)
	b := NewBridge("allure", mock)

	err := b.GenerateReport(
		context.Background(), "/tmp/results", "/tmp/report",
	)
	require.NoError(t, err)
	assert.Contains(t, mock.calls[0], "generate")
	assert.Contains(t, mock.calls[0], "/tmp/results")
	assert.Contains(t, mock.calls[0], "--output")
	assert.Contains(t, mock.calls[0], "/tmp/report")
	assert.Contains(t, mock.calls[0], "--clean")
}

func TestBridge_GenerateReport_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"allure generate",
		[]byte("error: input dir not found"),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge("allure", mock)

	err := b.GenerateReport(
		context.Background(), "/missing/results", "/tmp/report",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allure: generate report")
	assert.Contains(t, err.Error(), "exit status 1")
}

// --- OpenReport ---

func TestBridge_OpenReport_Success(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"allure open /tmp/report",
		[]byte("Server started at http://localhost:12345"),
		nil,
	)
	b := NewBridge("allure", mock)

	err := b.OpenReport(context.Background(), "/tmp/report")
	require.NoError(t, err)
	assert.Contains(t, mock.calls[0], "open")
	assert.Contains(t, mock.calls[0], "/tmp/report")
}

func TestBridge_OpenReport_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"allure open",
		[]byte("error: report dir not found"),
		fmt.Errorf("exit status 1"),
	)
	b := NewBridge("allure", mock)

	err := b.OpenReport(context.Background(), "/missing/report")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allure: open report")
	assert.Contains(t, err.Error(), "exit status 1")
}
