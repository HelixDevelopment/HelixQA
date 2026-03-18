// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

func TestCollector_Stress_ConcurrentCapture(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = c.CaptureScreenshot(ctx, "concurrent")
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 50, c.Count())
}

func TestCollector_Stress_ConcurrentItemsRead(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()

	// Pre-populate.
	for i := 0; i < 20; i++ {
		_, err := c.CaptureScreenshot(ctx, "preload")
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items := c.Items()
			_ = len(items)
			_ = c.Count()
			_ = c.ItemsByType(TypeScreenshot)
			_ = c.ItemsByType(TypeLogcat)
		}()
	}
	wg.Wait()
}

func TestCollector_Stress_RecordingToggle(t *testing.T) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	ctx := context.Background()

	// Start/stop recording 50 times sequentially.
	for i := 0; i < 50; i++ {
		require.NoError(t, c.StartRecording(ctx, "toggle"))
		assert.True(t, c.IsRecording())

		item, err := c.StopRecording(ctx)
		require.NoError(t, err)
		assert.Equal(t, TypeVideo, item.Type)
		assert.False(t, c.IsRecording())
	}

	assert.Equal(t, 50, c.Count())
}

func BenchmarkCollector_CaptureScreenshot(b *testing.B) {
	dir := b.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CaptureScreenshot(ctx, "bench")
	}
}

func BenchmarkCollector_Items(b *testing.B) {
	dir := b.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		c.CaptureScreenshot(ctx, "preload")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Items()
	}
}
