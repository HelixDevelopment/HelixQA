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

func TestCheckWeb_BrowserAlive(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f chromium",
		[]byte("12345"),
		nil,
	)

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
	assert.False(t, result.HasCrash)
}

func TestCheckWeb_BrowserDead(t *testing.T) {
	mock := newMockRunner()
	// All browser checks return empty.
	mock.On(
		"pgrep",
		[]byte(""),
		fmt.Errorf("no process found"),
	)

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0],
		"browser process not found")
}

func TestCheckWeb_FirefoxAlive(t *testing.T) {
	mock := newMockRunner()
	// chromium not found, chrome not found, google-chrome not
	// found, firefox found.
	responses := map[string]mockResponse{
		"pgrep -f chromium":      {nil, fmt.Errorf("not found")},
		"pgrep -f chrome":        {nil, fmt.Errorf("not found")},
		"pgrep -f google-chrome": {nil, fmt.Errorf("not found")},
		"pgrep -f firefox":       {[]byte("6789"), nil},
	}
	mock.responses = responses

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
	assert.False(t, result.HasCrash)
}

func TestCheckWeb_PlatformIsWeb(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformWeb, result.Platform)
}

func TestCheckWeb_PlaywrightAlive(t *testing.T) {
	mock := newMockRunner()
	responses := map[string]mockResponse{
		"pgrep -f chromium":      {nil, fmt.Errorf("not found")},
		"pgrep -f chrome":        {nil, fmt.Errorf("not found")},
		"pgrep -f google-chrome": {nil, fmt.Errorf("not found")},
		"pgrep -f firefox":       {nil, fmt.Errorf("not found")},
		"pgrep -f playwright":    {[]byte("1234"), nil},
	}
	mock.responses = responses

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

func TestCheckWeb_TimestampSet(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	d := New(
		config.PlatformWeb,
		WithCommandRunner(mock),
	)

	result, err := d.checkWeb(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Timestamp.IsZero())
}
