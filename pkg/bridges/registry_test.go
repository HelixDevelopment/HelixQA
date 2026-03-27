// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package bridges

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

// --- ToolStatus struct ---

func TestToolStatus_Fields(t *testing.T) {
	ts := ToolStatus{
		Name:      "scrcpy",
		Available: true,
		Path:      "/usr/bin/scrcpy",
		Version:   "scrcpy 2.3.1",
	}
	assert.Equal(t, "scrcpy", ts.Name)
	assert.True(t, ts.Available)
	assert.Equal(t, "/usr/bin/scrcpy", ts.Path)
	assert.Equal(t, "scrcpy 2.3.1", ts.Version)
}

func TestToolStatus_Unavailable(t *testing.T) {
	ts := ToolStatus{Name: "nonexistent"}
	assert.False(t, ts.Available)
	assert.Empty(t, ts.Path)
	assert.Empty(t, ts.Version)
}

// --- toolProbes ---

func TestToolProbes_ContainsExpectedTools(t *testing.T) {
	names := make([]string, 0, len(toolProbes))
	for _, p := range toolProbes {
		names = append(names, p.name)
	}

	expected := []string{
		"scrcpy", "appium", "allure", "perfetto",
		"maestro", "ffmpeg", "adb", "npx", "xdotool",
	}
	for _, want := range expected {
		assert.Contains(t, names, want,
			"toolProbes should contain %q", want,
		)
	}
}

func TestToolProbes_Count(t *testing.T) {
	assert.Equal(t, 9, len(toolProbes))
}

// --- probeVersion ---

func TestProbeVersion_ReturnsFirstNonEmptyLine(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"ffmpeg -version",
		[]byte("ffmpeg version 6.1\nbuilt with gcc\n"),
		nil,
	)
	v := probeVersion(
		context.Background(), mock, "ffmpeg", []string{"-version"},
	)
	assert.Equal(t, "ffmpeg version 6.1", v)
}

func TestProbeVersion_ErrorReturnsEmpty(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"notfound --version",
		nil,
		fmt.Errorf("exit status 127"),
	)
	v := probeVersion(
		context.Background(), mock,
		"notfound", []string{"--version"},
	)
	assert.Empty(t, v)
}

func TestProbeVersion_EmptyOutputReturnsEmpty(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"mytool --version",
		[]byte("   \n\n"),
		nil,
	)
	v := probeVersion(
		context.Background(), mock,
		"mytool", []string{"--version"},
	)
	assert.Empty(t, v)
}

// --- DiscoverTools ---

func TestDiscoverTools_ReturnsAllProbes(t *testing.T) {
	mock := newMockRunner()
	// Allow any version call to return empty (tools not on PATH
	// in CI). We only care about the slice length and names.
	results := DiscoverTools(mock)

	require.Equal(t, len(toolProbes), len(results))

	names := make([]string, 0, len(results))
	for _, r := range results {
		names = append(names, r.Name)
	}
	assert.Contains(t, names, "scrcpy")
	assert.Contains(t, names, "appium")
	assert.Contains(t, names, "allure")
	assert.Contains(t, names, "perfetto")
	assert.Contains(t, names, "maestro")
	assert.Contains(t, names, "ffmpeg")
	assert.Contains(t, names, "adb")
	assert.Contains(t, names, "npx")
	assert.Contains(t, names, "xdotool")
}

func TestDiscoverTools_UnavailableToolHasNoPath(t *testing.T) {
	mock := newMockRunner()
	results := DiscoverTools(mock)

	// Any tool not installed on the test host should have
	// Available=false and an empty Path.
	for _, r := range results {
		if !r.Available {
			assert.Empty(t, r.Path,
				"unavailable tool %q should have empty path",
				r.Name,
			)
		}
	}
}

func TestDiscoverTools_AvailableToolHasPath(t *testing.T) {
	mock := newMockRunner()
	results := DiscoverTools(mock)

	for _, r := range results {
		if r.Available {
			assert.NotEmpty(t, r.Path,
				"available tool %q should have a path", r.Name,
			)
		}
	}
}

func TestDiscoverTools_OrderMatchesProbes(t *testing.T) {
	mock := newMockRunner()
	results := DiscoverTools(mock)

	require.Equal(t, len(toolProbes), len(results))
	for i, probe := range toolProbes {
		assert.Equal(t, probe.name, results[i].Name,
			"result[%d] should be %q", i, probe.name,
		)
	}
}
