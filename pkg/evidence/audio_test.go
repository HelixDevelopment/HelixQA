// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

func TestCollector_StartAudioRecording_Success(t *testing.T) {
	dir := t.TempDir()
	runner := newMockRunner()

	c := New(
		WithOutputDir(dir),
		WithPlatform(config.PlatformAndroid),
		WithCommandRunner(runner),
	)

	ctx := context.Background()

	// StartAudioRecording will try to exec ffmpeg, which won't
	// be available in test. We verify the state management by
	// checking that it returns an error about starting ffmpeg
	// (not a logic error about already recording).
	err := c.StartAudioRecording(
		ctx, "test-audio", "high", "wav", "default",
	)
	// ffmpeg likely not available in test env, so we accept
	// either success or a start error (not "already in progress").
	if err != nil {
		assert.Contains(t, err.Error(), "start audio recording")
		assert.False(t, c.IsAudioRecording())
	} else {
		assert.True(t, c.IsAudioRecording())
		// Clean up.
		_, _ = c.StopAudioRecording(ctx)
	}
}

func TestCollector_StartAudioRecording_AlreadyRecording(
	t *testing.T,
) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	// Force audio recording state directly.
	c.mu.Lock()
	c.audioRecording = true
	c.audioRecordingID = "fake-recording"
	c.mu.Unlock()

	ctx := context.Background()
	err := c.StartAudioRecording(
		ctx, "second", "high", "wav", "default",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestCollector_StopAudioRecording_NotRecording(
	t *testing.T,
) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	ctx := context.Background()
	_, err := c.StopAudioRecording(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no audio recording")
}

func TestCollector_StopAudioRecording_Success(t *testing.T) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	// Simulate an active audio recording (no real process).
	c.mu.Lock()
	c.audioRecording = true
	c.audioRecordingID = "test-audio-123"
	c.audioCmd = nil // No real process.
	c.mu.Unlock()

	ctx := context.Background()
	item, err := c.StopAudioRecording(ctx)
	require.NoError(t, err)
	assert.Equal(t, TypeAudio, item.Type)
	assert.Contains(t, item.Path, "test-audio-123")
	assert.Contains(t, item.Path, ".wav")
	assert.Equal(t, config.PlatformAndroid, item.Platform)
	assert.False(t, c.IsAudioRecording())
	assert.Equal(t, 1, c.Count())
}

func TestCollector_IsAudioRecording(t *testing.T) {
	c := New()

	assert.False(t, c.IsAudioRecording())

	c.mu.Lock()
	c.audioRecording = true
	c.mu.Unlock()

	assert.True(t, c.IsAudioRecording())

	c.mu.Lock()
	c.audioRecording = false
	c.mu.Unlock()

	assert.False(t, c.IsAudioRecording())
}

func TestCollector_AudioRecordingQualityMapping(t *testing.T) {
	tests := []struct {
		name       string
		quality    string
		wantRate   string
		wantFmt    string
	}{
		{
			name:     "standard quality",
			quality:  "standard",
			wantRate: "44100",
			wantFmt:  "s16",
		},
		{
			name:     "high quality",
			quality:  "high",
			wantRate: "48000",
			wantFmt:  "s32",
		},
		{
			name:     "ultra quality",
			quality:  "ultra",
			wantRate: "96000",
			wantFmt:  "s32",
		},
		{
			name:     "unknown defaults to high",
			quality:  "garbage",
			wantRate: "48000",
			wantFmt:  "s32",
		},
		{
			name:     "empty defaults to high",
			quality:  "",
			wantRate: "48000",
			wantFmt:  "s32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := AudioQualityMap(tt.quality)
			assert.Equal(t, tt.wantRate, params.SampleRate)
			assert.Equal(t, tt.wantFmt, params.SampleFmt)
		})
	}
}

func TestCollector_AudioItemsByType(t *testing.T) {
	dir := t.TempDir()
	c := New(WithOutputDir(dir))

	// Simulate completed audio recording.
	c.mu.Lock()
	c.audioRecording = true
	c.audioRecordingID = "audio-test"
	c.audioCmd = nil
	c.mu.Unlock()

	ctx := context.Background()
	_, err := c.StopAudioRecording(ctx)
	require.NoError(t, err)

	audioItems := c.ItemsByType(TypeAudio)
	assert.Len(t, audioItems, 1)
	assert.Equal(t, TypeAudio, audioItems[0].Type)

	videoItems := c.ItemsByType(TypeVideo)
	assert.Empty(t, videoItems)
}

func TestTypeAudio_Constant(t *testing.T) {
	assert.Equal(t, Type("audio"), TypeAudio)
}
