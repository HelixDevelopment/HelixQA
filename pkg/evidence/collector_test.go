// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// mockRunner simulates command execution for testing.
type mockRunner struct {
	mu        sync.Mutex
	responses map[string][]byte
	errors    map[string]error
	calls     []string
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string][]byte),
		errors:    make(map[string]error),
	}
}

func (m *mockRunner) Run(
	_ context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := name
	for _, a := range args {
		key += " " + a
	}
	m.calls = append(m.calls, key)

	if err, ok := m.errors[name]; ok {
		return nil, err
	}
	if resp, ok := m.responses[name]; ok {
		return resp, nil
	}
	return []byte("ok"), nil
}

func TestCollector_New_Defaults(t *testing.T) {
	c := New()
	assert.Equal(t, "evidence", c.outputDir)
	assert.Equal(t, config.PlatformAndroid, c.platform)
	assert.Equal(t, 0, c.Count())
}

func TestCollector_New_WithOptions(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformWeb),
		WithCommandRunner(runner),
	)

	assert.Equal(t, dir, c.outputDir)
	assert.Equal(t, config.PlatformWeb, c.platform)
}

func TestCollector_CaptureScreenshot_Android(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	// Mock ADB pull: create the output file.
	runner.responses["adb"] = []byte("ok")

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	item, err := c.CaptureScreenshot(ctx, "test-step")
	require.NoError(t, err)
	assert.Equal(t, TypeScreenshot, item.Type)
	assert.Equal(t, config.PlatformAndroid, item.Platform)
	assert.NotEmpty(t, item.Path)
	assert.Equal(t, 1, c.Count())
}

func TestCollector_CaptureScreenshot_Web(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformWeb),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	item, err := c.CaptureScreenshot(ctx, "web-shot")
	require.NoError(t, err)
	assert.Equal(t, TypeScreenshot, item.Type)
	assert.Contains(t, item.Path, "web-shot")
}

func TestCollector_CaptureScreenshot_Desktop(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformDesktop),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	item, err := c.CaptureScreenshot(ctx, "desktop-shot")
	require.NoError(t, err)
	assert.Equal(t, TypeScreenshot, item.Type)
}

func TestCollector_CaptureScreenshot_UnsupportedPlatform(t *testing.T) {
	c := New(WithPlatform(config.PlatformAll))

	ctx := context.Background()
	_, err := c.CaptureScreenshot(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestCollector_CaptureScreenshot_CommandError(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()
	runner.errors["adb"] = fmt.Errorf("device not found")

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	_, err := c.CaptureScreenshot(ctx, "fail")
	assert.Error(t, err)
}

func TestCollector_CaptureLogcat(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()
	runner.responses["adb"] = []byte(
		"E/ActivityManager: ANR in com.example\n" +
			"W/System: Low memory\n",
	)

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	item, err := c.CaptureLogcat(ctx, "crash-logs", 100)
	require.NoError(t, err)
	assert.Equal(t, TypeLogcat, item.Type)
	assert.Contains(t, item.Path, "logcat")

	// Verify file was written.
	data, err := os.ReadFile(item.Path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ANR")
}

func TestCollector_CaptureLogcat_NonAndroid(t *testing.T) {
	c := New(WithPlatform(config.PlatformWeb))

	ctx := context.Background()
	_, err := c.CaptureLogcat(ctx, "test", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only available on Android")
}

func TestCollector_Recording(t *testing.T) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	ctx := context.Background()

	assert.False(t, c.IsRecording())

	// Start recording.
	err := c.StartRecording(ctx, "test-video")
	require.NoError(t, err)
	assert.True(t, c.IsRecording())

	// Can't start another while recording.
	err = c.StartRecording(ctx, "another")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	// Stop recording.
	item, err := c.StopRecording(ctx)
	require.NoError(t, err)
	assert.Equal(t, TypeVideo, item.Type)
	assert.Contains(t, item.Path, ".mp4")
	assert.False(t, c.IsRecording())

	// Can't stop when not recording.
	_, err = c.StopRecording(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recording")
}

func TestCollector_Items(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()

	// Capture multiple items.
	_, err := c.CaptureScreenshot(ctx, "shot1")
	require.NoError(t, err)
	_, err = c.CaptureScreenshot(ctx, "shot2")
	require.NoError(t, err)

	items := c.Items()
	assert.Len(t, items, 2)
	assert.Equal(t, 2, c.Count())
}

func TestCollector_ItemsByType(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()
	runner.responses["adb"] = []byte("log output")

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()

	_, err := c.CaptureScreenshot(ctx, "shot")
	require.NoError(t, err)
	_, err = c.CaptureLogcat(ctx, "logs", 50)
	require.NoError(t, err)

	screenshots := c.ItemsByType(TypeScreenshot)
	assert.Len(t, screenshots, 1)

	logcats := c.ItemsByType(TypeLogcat)
	assert.Len(t, logcats, 1)

	videos := c.ItemsByType(TypeVideo)
	assert.Empty(t, videos)
}

func TestCollector_Reset(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	_, _ = c.CaptureScreenshot(ctx, "shot")
	assert.Equal(t, 1, c.Count())

	c.Reset()
	assert.Equal(t, 0, c.Count())
	assert.Empty(t, c.Items())
}

func TestCollector_EnsureOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested")
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	_, err := c.CaptureScreenshot(ctx, "test")
	require.NoError(t, err)

	// Verify directory was created.
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestItem_Types(t *testing.T) {
	assert.Equal(t, Type("screenshot"), TypeScreenshot)
	assert.Equal(t, Type("video"), TypeVideo)
	assert.Equal(t, Type("logcat"), TypeLogcat)
	assert.Equal(t, Type("stacktrace"), TypeStackTrace)
	assert.Equal(t, Type("console_log"), TypeConsoleLog)
}
