// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// TestCheckDesktop_NoTargetConfigured documents the close-out⁷⁵
// contract change: when neither processName nor PID is configured,
// isDesktopProcessAlive returns true unconditionally — the absence
// of a configured target means "no desktop app under test", not
// "crash detected". The prior behavior (fallback to `pgrep -f java`)
// produced 23-of-23 false-positive crash reports against helixqa
// runs that didn't supply -desktop-process.
func TestCheckDesktop_NoTargetConfigured(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithCommandRunner(newMockRunner()), // mock unused — no command should fire
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"no-target-configured must return ProcessAlive=true (no app under test ≠ crash)")
	assert.False(t, result.HasCrash,
		"no-target-configured must NOT flag crash — close-out⁷⁵ contract")
}

func TestCheckDesktop_ProcessDead_RequiresExplicitTarget(t *testing.T) {
	// close-out⁷⁵ update: a "dead process" reading now requires an
	// explicitly configured processName OR PID. Without either, there's
	// no target to be dead. This test exercises the explicit-name path.
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte(""),
		fmt.Errorf("no process"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0],
		"desktop process not alive")
}

func TestCheckDesktop_ByProcessName(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte("5678"),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

func TestCheckDesktop_ByPID_Alive(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"kill -0 12345",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessPID(12345),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

func TestCheckDesktop_ByPID_Dead(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"kill -0 12345",
		[]byte(""),
		fmt.Errorf("no such process"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessPID(12345),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
}

func TestCheckDesktop_PIDTakesPrecedence(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"kill -0 99",
		[]byte(""),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("should-not-use"),
		WithProcessPID(99),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

// TestCheckDesktop_DefaultBehavior documents that the close-out⁷⁵
// contract no longer falls back to `pgrep -f java`. Default = no
// target = ProcessAlive=true. Use WithProcessName("java") explicitly
// if you actually want java-process detection.
func TestCheckDesktop_DefaultBehavior(t *testing.T) {
	mock := newMockRunner()
	// Mock is intentionally NOT configured for "pgrep -f java" — the
	// detector must NOT invoke any command when no target is set.

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"default (no target) returns ProcessAlive=true per close-out⁷⁵")
}

func TestCheckDesktop_PlatformIsDesktop(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
}

func TestCheckDesktop_CrashMessageContainsPID(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"kill -0 42",
		[]byte(""),
		fmt.Errorf("no such process"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessPID(42),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0], "PID 42")
}

func TestCheckDesktop_CrashMessageContainsName(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte(""),
		fmt.Errorf("not found"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0], "myapp")
}
