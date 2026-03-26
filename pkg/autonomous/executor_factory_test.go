// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"
	"time"

	"digital.vasic.docprocessor/pkg/coverage"
	"digital.vasic.docprocessor/pkg/feature"
	"digital.vasic.llmorchestrator/pkg/agent"

	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/navigator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommandRunner implements detector.CommandRunner for tests.
type mockCommandRunner struct {
	lastCmd  string
	lastArgs []string
}

func (m *mockCommandRunner) Run(
	_ context.Context, name string, args ...string,
) ([]byte, error) {
	m.lastCmd = name
	m.lastArgs = args
	return []byte("mock-output"), nil
}

func TestDefaultExecutorFactory_Create_Android(t *testing.T) {
	mock := &mockCommandRunner{}
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		AndroidDevice: "emulator-5554",
		CommandRunner: mock,
	})

	exec, err := factory.Create("android")
	require.NoError(t, err)
	require.NotNil(t, exec)

	// Verify it's an ADBExecutor by exercising it.
	err = exec.Click(context.Background(), 100, 200)
	assert.NoError(t, err)
	assert.Equal(t, "adb", mock.lastCmd)
}

func TestDefaultExecutorFactory_Create_Android_NoDevice(t *testing.T) {
	factory := NewDefaultExecutorFactory(ExecutorConfig{})

	_, err := factory.Create("android")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "android device ID")
}

func TestDefaultExecutorFactory_Create_Web(t *testing.T) {
	mock := &mockCommandRunner{}
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		BrowserURL:    "http://localhost:8080",
		CommandRunner: mock,
	})

	exec, err := factory.Create("web")
	require.NoError(t, err)
	require.NotNil(t, exec)

	// Verify it's a PlaywrightExecutor by exercising it.
	err = exec.Click(context.Background(), 50, 60)
	assert.NoError(t, err)
	assert.Equal(t, "npx", mock.lastCmd)
}

func TestDefaultExecutorFactory_Create_Web_NoURL(t *testing.T) {
	factory := NewDefaultExecutorFactory(ExecutorConfig{})

	_, err := factory.Create("web")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "browser URL")
}

func TestDefaultExecutorFactory_Create_Desktop(t *testing.T) {
	mock := &mockCommandRunner{}
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		DesktopDisplay: ":1",
		CommandRunner:  mock,
	})

	exec, err := factory.Create("desktop")
	require.NoError(t, err)
	require.NotNil(t, exec)

	// Verify it's an X11Executor by exercising it.
	err = exec.Click(context.Background(), 10, 20)
	assert.NoError(t, err)
	assert.Equal(t, "xdotool", mock.lastCmd)
}

func TestDefaultExecutorFactory_Create_Desktop_DefaultDisplay(
	t *testing.T,
) {
	mock := &mockCommandRunner{}
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		CommandRunner: mock,
	})

	exec, err := factory.Create("desktop")
	require.NoError(t, err)
	require.NotNil(t, exec)
	assert.NoError(t, exec.Click(context.Background(), 0, 0))
}

func TestDefaultExecutorFactory_Create_Unknown(t *testing.T) {
	factory := NewDefaultExecutorFactory(ExecutorConfig{})

	_, err := factory.Create("ios")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestDefaultExecutorFactory_DefaultRunner(t *testing.T) {
	// When no CommandRunner is provided, should use
	// detector.NewExecRunner (real os/exec).
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		AndroidDevice: "device-1",
	})
	assert.NotNil(t, factory.runner)

	// It should create an executor without panicking.
	exec, err := factory.Create("android")
	require.NoError(t, err)
	require.NotNil(t, exec)
}

func TestNoopExecutorFactory_Create(t *testing.T) {
	factory := &NoopExecutorFactory{}

	for _, platform := range []string{
		"android", "desktop", "web", "unknown",
	} {
		exec, err := factory.Create(platform)
		require.NoError(t, err, "platform: %s", platform)
		require.NotNil(t, exec, "platform: %s", platform)

		// Verify all methods work.
		ctx := context.Background()
		assert.NoError(t, exec.Click(ctx, 0, 0))
		assert.NoError(t, exec.Type(ctx, "test"))
		assert.NoError(t, exec.Scroll(ctx, "down", 100))
		assert.NoError(t, exec.LongPress(ctx, 0, 0))
		assert.NoError(t, exec.Swipe(ctx, 0, 0, 1, 1))
		assert.NoError(t, exec.KeyPress(ctx, "Enter"))
		assert.NoError(t, exec.Back(ctx))
		assert.NoError(t, exec.Home(ctx))

		data, err := exec.Screenshot(ctx)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
	}
}

func TestExecutorFactory_Interface(t *testing.T) {
	// Verify both factories satisfy the interface.
	var _ ExecutorFactory = &DefaultExecutorFactory{}
	var _ ExecutorFactory = &NoopExecutorFactory{}
}

func TestExecutorConfig_AllFields(t *testing.T) {
	mock := &mockCommandRunner{}
	cfg := ExecutorConfig{
		AndroidDevice:  "pixel-7",
		BrowserURL:     "https://example.com",
		DesktopDisplay: ":2",
		CommandRunner:  mock,
	}

	factory := NewDefaultExecutorFactory(cfg)

	// All three platforms should succeed.
	android, err := factory.Create("android")
	require.NoError(t, err)
	require.NotNil(t, android)

	web, err := factory.Create("web")
	require.NoError(t, err)
	require.NotNil(t, web)

	desktop, err := factory.Create("desktop")
	require.NoError(t, err)
	require.NotNil(t, desktop)
}

func TestSessionCoordinator_WithExecutorFactory(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.Platforms = []string{"desktop"}
	cfg.Timeout = 5 * time.Second
	cfg.CuriosityEnabled = false

	mock := &mockCommandRunner{}
	factory := NewDefaultExecutorFactory(ExecutorConfig{
		DesktopDisplay: ":0",
		CommandRunner:  mock,
	})

	pool := agent.NewPool()
	require.NoError(t, pool.Register(
		newTestAgent("a1", "claude"),
	))

	sc := NewSessionCoordinator(
		cfg, pool, &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
		WithExecutorFactory(factory),
	)

	result, err := sc.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, StatusComplete, result.Status)
}

func TestNewExecRunner(t *testing.T) {
	// Verify the exported factory creates a working runner.
	runner := detector.NewExecRunner()
	require.NotNil(t, runner)

	// Sanity: run a trivial command.
	out, err := runner.Run(
		context.Background(), "echo", "hello",
	)
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

// Ensure noopExecutor satisfies navigator.ActionExecutor.
var _ navigator.ActionExecutor = &noopExecutor{}
