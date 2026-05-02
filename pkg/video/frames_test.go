// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package video

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameExtractor_BuildFFmpegArgs(t *testing.T) {
	fe := NewFrameExtractor("ffmpeg")
	args := fe.buildArgs("/input/video.mp4", "/out", 1)

	assert.Contains(t, args, "-i",
		"ffmpeg args must contain -i flag")
	assert.Contains(t, args, "/input/video.mp4",
		"ffmpeg args must contain the input video path")

	// Find the fps filter value.
	found := false
	for _, a := range args {
		if strings.Contains(a, "fps=1") {
			found = true
			break
		}
	}
	assert.True(t, found,
		"ffmpeg args must contain fps=1 filter")
}

func TestFrameExtractor_BuildSceneArgs(t *testing.T) {
	fe := NewFrameExtractor("ffmpeg")
	args := fe.buildSceneArgs(
		"/input/video.mp4", "/out", 0.4,
	)

	assert.Contains(t, args, "-i",
		"scene args must contain -i flag")
	assert.Contains(t, args, "/input/video.mp4",
		"scene args must contain the input video path")

	// The select filter must reference "select" and the
	// threshold value.
	found := false
	for _, a := range args {
		if strings.Contains(a, "select") &&
			strings.Contains(a, "0.4") {
			found = true
			break
		}
	}
	assert.True(t, found,
		"scene args must contain select filter with threshold")
}

func TestFrameExtractor_OutputPattern(t *testing.T) {
	fe := NewFrameExtractor("ffmpeg")
	pattern := fe.outputPattern("/recordings/session1")

	assert.Contains(t, pattern, "frame_%04d.png",
		"output pattern must contain frame_%04d.png")
	assert.Contains(t, pattern, "/recordings/session1",
		"output pattern must contain the output directory")
}

func TestNewFrameExtractor_CustomPath(t *testing.T) {
	fe := NewFrameExtractor("/usr/local/bin/ffmpeg")
	require.NotNil(t, fe)
	assert.Equal(t, "/usr/local/bin/ffmpeg", fe.ffmpegPath)
}

func TestFrameExtractor_BuildArgs_FPSValues(t *testing.T) {
	tests := []struct {
		fps     int
		wantFPS string
	}{
		{fps: 1, wantFPS: "fps=1"},
		{fps: 5, wantFPS: "fps=5"},
		{fps: 30, wantFPS: "fps=30"},
	}

	fe := NewFrameExtractor("ffmpeg")
	for _, tc := range tests {
		t.Run(tc.wantFPS, func(t *testing.T) {
			args := fe.buildArgs("/v.mp4", "/out", tc.fps)
			found := false
			for _, a := range args {
				if strings.Contains(a, tc.wantFPS) {
					found = true
					break
				}
			}
			assert.True(t, found,
				"args must contain %s", tc.wantFPS)
		})
	}
}
