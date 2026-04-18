// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package vaapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	vaapipkg "digital.vasic.helixqa/pkg/nexus/record/encoder/vaapi"
)

// ---------------------------------------------------------------------------
// Existing tests (must stay green)
// ---------------------------------------------------------------------------

// TestVAAPI_FactoryRegistered verifies the init() registers "vaapi" in the
// parent encoder factory.
func TestVAAPI_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "vaapi", "vaapi must be registered via init()")
}

// TestVAAPI_ProductionReturnsErrNotWired verifies the production stub returns
// ErrNotWired from Encode() in P5.
func TestVAAPI_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("vaapi")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestVAAPI_Close_AlwaysSucceeds verifies Close() never errors on the stub.
func TestVAAPI_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("vaapi")
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
	t.Setenv("HELIXQA_RECORD_VAAPI_STUB", "")

	orig := vaapipkg.FFmpegCandidates
	vaapipkg.FFmpegCandidates = []string{"__no_ffmpeg_here__"}
	t.Cleanup(func() { vaapipkg.FFmpegCandidates = orig })

	_, err := vaapipkg.NewProductionEncoder(
		vaapipkg.RecordConfig{Width: 16, Height: 16, FrameRate: 1, DeviceNode: "/dev/null"},
		nil,
	)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestResolveVAAPIDevice_MissingNode_Errors — when the device node does not
// exist on disk, NewProductionEncoder must return ErrNotWired.
func TestResolveVAAPIDevice_MissingNode_Errors(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_VAAPI_STUB", "")
	t.Setenv("HELIXQA_VAAPI_DEVICE", "")

	_, err := vaapipkg.NewProductionEncoder(
		vaapipkg.RecordConfig{
			Width:      16,
			Height:     16,
			FrameRate:  1,
			DeviceNode: "/dev/dri/__no_vaapi_device__",
		},
		nil,
	)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_RECORD_VAAPI_STUB=1 must force
// ErrNotWired from NewProductionEncoder regardless of whether ffmpeg or the
// device node is installed.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_VAAPI_STUB", "1")

	_, err := vaapipkg.NewProductionEncoder(
		vaapipkg.RecordConfig{Width: 16, Height: 16, FrameRate: 1},
		nil,
	)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestBuildFFmpegArgs_VAAPIFlags — BuildFFmpegArgs must produce an argv that
// contains all mandatory VAAPI flags: -c:v h264_vaapi, -vf
// format=nv12,hwupload, the correct -init_hw_device vaapi=intel:<device>,
// the correct -s WxH, and the correct -r FR.
func TestBuildFFmpegArgs_VAAPIFlags(t *testing.T) {
	cfg := vaapipkg.RecordConfig{Width: 1920, Height: 1080, FrameRate: 25}
	device := "/dev/dri/renderD128"
	args := vaapipkg.BuildFFmpegArgs(cfg, "/usr/bin/ffmpeg", device)

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

	assertContainsPair("-c:v", "h264_vaapi")
	assertContainsPair("-vf", "format=nv12,hwupload")
	assertContainsPair("-init_hw_device", "vaapi=intel:/dev/dri/renderD128")
	assertContainsPair("-s", "1920x1080")
	assertContainsPair("-r", "25")
	assertContainsPair("-pix_fmt", "bgra")
}
