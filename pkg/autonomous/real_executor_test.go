// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealExecutorFactory_CreateAndroid(t *testing.T) {
	factory := NewRealExecutorFactory(RealExecutorConfig{
		AndroidDevice: "device123",
	})

	exec, err := factory.Create("android")
	require.NoError(t, err)
	require.NotNil(t, exec)
}

func TestRealExecutorFactory_CreateAndroidTV(t *testing.T) {
	factory := NewRealExecutorFactory(RealExecutorConfig{
		AndroidDevice: "device123",
	})

	exec, err := factory.Create("androidtv")
	require.NoError(t, err)
	require.NotNil(t, exec)
}

func TestRealExecutorFactory_CreateWeb(t *testing.T) {
	factory := NewRealExecutorFactory(RealExecutorConfig{
		WebURL:     "http://localhost:3000",
		WebBrowser: "chromium",
	})

	exec, err := factory.Create("web")
	require.NoError(t, err)
	require.NotNil(t, exec)
}

func TestRealExecutorFactory_CreateDesktop(t *testing.T) {
	factory := NewRealExecutorFactory(RealExecutorConfig{
		DesktopProcess: "catalogizer",
		DesktopDisplay: ":0",
	})

	exec, err := factory.Create("desktop")
	require.NoError(t, err)
	require.NotNil(t, exec)
}

func TestRealExecutorFactory_UnsupportedPlatform(t *testing.T) {
	factory := NewRealExecutorFactory(RealExecutorConfig{})

	exec, err := factory.Create("unknown")
	assert.Error(t, err)
	assert.Nil(t, exec)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestRealExecutorFactory_Interface(t *testing.T) {
	// Verify RealExecutorFactory satisfies ExecutorFactory.
	var _ ExecutorFactory = &RealExecutorFactory{}
}
