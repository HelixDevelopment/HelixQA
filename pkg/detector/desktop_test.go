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

func TestCheckDesktop_ProcessAlive(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f java",
		[]byte("12345"),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
	assert.False(t, result.HasCrash)
}

func TestCheckDesktop_ProcessDead(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f java",
		[]byte(""),
		fmt.Errorf("no process"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
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

func TestCheckDesktop_DefaultJava(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f java",
		[]byte("1111"),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
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
