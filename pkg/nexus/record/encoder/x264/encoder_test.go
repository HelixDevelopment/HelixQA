// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package x264_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	x264pkg "digital.vasic.helixqa/pkg/nexus/record/encoder/x264"
)

// ---------------------------------------------------------------------------
// Existing tests (must stay green)
// ---------------------------------------------------------------------------

// TestX264_FactoryRegistered verifies the init() registers "x264" in the
// parent encoder factory.
func TestX264_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "x264", "x264 must be registered via init()")
}

// TestX264_ProductionReturnsErrNotWired verifies that the production stub
// returns ErrNotWired from Encode().
func TestX264_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("x264")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestX264_Close_AlwaysSucceeds verifies Close() never errors on the stub.
func TestX264_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("x264")
	require.NoError(t, err)
	require.NoError(t, enc.Close())
}

// ---------------------------------------------------------------------------
// P5.5 new tests
// ---------------------------------------------------------------------------

// TestResolveFFmpeg_MissingPath_Errors — when the ffmpeg candidates slice
// contains only a name that does not exist, NewProductionEncoder must return
// ErrNotWired (not panic).
func TestResolveFFmpeg_MissingPath_Errors(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_X264_STUB", "")

	// Override candidates to a name that will never exist on PATH.
	orig := x264pkg.FFmpegCandidates
	x264pkg.FFmpegCandidates = []string{"__no_ffmpeg_here__"}
	t.Cleanup(func() { x264pkg.FFmpegCandidates = orig })

	_, err := x264pkg.NewProductionEncoder(x264pkg.RecordConfig{Width: 16, Height: 16, FrameRate: 1}, nil)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_RECORD_X264_STUB=1 must force
// ErrNotWired from NewProductionEncoder regardless of whether ffmpeg is installed.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_X264_STUB", "1")

	_, err := x264pkg.NewProductionEncoder(x264pkg.RecordConfig{Width: 16, Height: 16, FrameRate: 1}, nil)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestBuildFFmpegArgs_IncludesCodec — buildFFmpegArgs must produce an argv
// that contains -c:v libx264 -preset ultrafast, the correct size string, and
// the correct frame-rate string.
func TestBuildFFmpegArgs_IncludesCodec(t *testing.T) {
	cfg := x264pkg.RecordConfig{Width: 1920, Height: 1080, FrameRate: 25}
	args := x264pkg.BuildFFmpegArgs(cfg, "/usr/bin/ffmpeg")

	require.Equal(t, "/usr/bin/ffmpeg", args[0])

	assertContainsPair := func(key, val string) {
		t.Helper()
		for i := 0; i+1 < len(args); i++ {
			if args[i] == key && args[i+1] == val {
				return
			}
		}
		t.Errorf("args do not contain pair %q %q; args=%v", key, val, args)
	}

	assertContainsPair("-c:v", "libx264")
	assertContainsPair("-preset", "ultrafast")
	assertContainsPair("-s", "1920x1080")
	assertContainsPair("-r", "25")
	assertContainsPair("-pix_fmt", "bgra")
}
