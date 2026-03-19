// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVideoManager(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/video.mp4")
	assert.NotNil(t, vm)
	assert.Equal(t, "android", vm.Platform())
	assert.Equal(t, "/tmp/video.mp4", vm.OutputPath())
	assert.False(t, vm.IsRecording())
	assert.Equal(t, time.Duration(0), vm.Offset())
}

func TestVideoManager_Start(t *testing.T) {
	vm := NewVideoManager("desktop", "/tmp/desktop.mp4")
	err := vm.Start()
	require.NoError(t, err)
	assert.True(t, vm.IsRecording())
	assert.False(t, vm.StartedAt().IsZero())
}

func TestVideoManager_Start_AlreadyRecording(t *testing.T) {
	vm := NewVideoManager("web", "/tmp/web.mp4")
	require.NoError(t, vm.Start())
	err := vm.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already recording")
}

func TestVideoManager_Stop(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	require.NoError(t, vm.Start())

	path, err := vm.Stop()
	require.NoError(t, err)
	assert.Equal(t, "/tmp/android.mp4", path)
	assert.False(t, vm.IsRecording())
}

func TestVideoManager_Stop_NotRecording(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	_, err := vm.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not recording")
}

func TestVideoManager_Offset_NotRecording(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	assert.Equal(t, time.Duration(0), vm.Offset())
}

func TestVideoManager_Offset_Recording(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	require.NoError(t, vm.Start())

	// Offset should be positive after a brief wait.
	time.Sleep(5 * time.Millisecond)
	offset := vm.Offset()
	assert.Greater(t, offset, time.Duration(0))
}

func TestVideoManager_StartedAt_NeverStarted(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	assert.True(t, vm.StartedAt().IsZero())
}

func TestVideoManager_StartedAt_AfterStart(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	before := time.Now()
	require.NoError(t, vm.Start())
	after := time.Now()

	started := vm.StartedAt()
	assert.False(t, started.Before(before))
	assert.False(t, started.After(after))
}

func TestVideoManager_StartStopRestart(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")

	require.NoError(t, vm.Start())
	assert.True(t, vm.IsRecording())

	_, err := vm.Stop()
	require.NoError(t, err)
	assert.False(t, vm.IsRecording())

	// Cannot restart — Start checks recording flag not vm state.
	// After Stop, recording is false, so Start should work.
	err = vm.Start()
	require.NoError(t, err)
	assert.True(t, vm.IsRecording())
}

// Stress test: concurrent access to VideoManager.
func TestVideoManager_Stress_ConcurrentAccess(t *testing.T) {
	vm := NewVideoManager("android", "/tmp/android.mp4")
	require.NoError(t, vm.Start())

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = vm.IsRecording()
			_ = vm.Offset()
			_ = vm.Platform()
			_ = vm.OutputPath()
			_ = vm.StartedAt()
		}()
	}
	wg.Wait()

	assert.True(t, vm.IsRecording())
}
