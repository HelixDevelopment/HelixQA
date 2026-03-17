// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// mockRunner is a test implementation of CommandRunner.
type mockRunner struct {
	responses map[string]mockResponse
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
	// Build key from name + first few args.
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}

	// Try exact match first.
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}

	// Try matching by command name only.
	if resp, ok := m.responses[name]; ok {
		return resp.output, resp.err
	}

	// Try matching by prefix.
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) {
			return resp.output, resp.err
		}
	}

	return nil, fmt.Errorf("no mock for: %s", key)
}

// --- Detector constructor tests ---

func TestNew_DefaultPlatform(t *testing.T) {
	d := New(config.PlatformAndroid)
	assert.Equal(t, config.PlatformAndroid, d.Platform())
	assert.Equal(t, "evidence", d.evidenceDir)
}

func TestNew_WithOptions(t *testing.T) {
	d := New(
		config.PlatformAndroid,
		WithDevice("emulator-5554"),
		WithPackageName("com.test.app"),
		WithEvidenceDir("/tmp/evidence"),
	)
	assert.Equal(t, "emulator-5554", d.device)
	assert.Equal(t, "com.test.app", d.packageName)
	assert.Equal(t, "/tmp/evidence", d.evidenceDir)
}

func TestNew_WebOptions(t *testing.T) {
	d := New(
		config.PlatformWeb,
		WithBrowserURL("http://localhost:3000"),
	)
	assert.Equal(t, config.PlatformWeb, d.Platform())
	assert.Equal(t, "http://localhost:3000", d.browserURL)
}

func TestNew_DesktopOptions(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithProcessName("java"),
		WithProcessPID(12345),
	)
	assert.Equal(t, config.PlatformDesktop, d.Platform())
	assert.Equal(t, "java", d.processName)
	assert.Equal(t, 12345, d.processPID)
}

func TestNew_WithCommandRunner(t *testing.T) {
	mock := newMockRunner()
	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
	)
	assert.NotNil(t, d.cmdRunner)
}

// --- Check dispatch tests ---

func TestCheck_UnsupportedPlatform(t *testing.T) {
	d := New(Platform("unknown"))
	_, err := d.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestCheck_DispatchesAndroid(t *testing.T) {
	mock := newMockRunner()
	mock.On("adb", []byte("12345"), nil)
	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
		WithPackageName("com.test"),
	)
	result, err := d.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformAndroid, result.Platform)
}

func TestCheck_DispatchesWeb(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)
	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)
	result, err := d.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformWeb, result.Platform)
}

func TestCheck_DispatchesDesktop(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)
	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("java"),
	)
	result, err := d.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
}

// --- CheckApp tests ---

func TestCheckApp_OverridesPlatform(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)
	d := New(
		config.PlatformAndroid,
		WithCommandRunner(mock),
	)
	result, err := d.CheckApp(
		context.Background(), config.PlatformDesktop,
	)
	require.NoError(t, err)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
	// Platform restored after call.
	assert.Equal(t, config.PlatformAndroid, d.Platform())
}

// --- Platform type alias ---

type Platform = config.Platform
