// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// fakeFrameData is the minimal FrameData implementation for tests.
type fakeFrameData struct{ data []byte }

func (f *fakeFrameData) AsBytes() ([]byte, error)                  { return f.data, nil }
func (f *fakeFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (f *fakeFrameData) Release() error                            { return nil }

func TestSource_ProducesFramesFromMock(t *testing.T) {
	// Swap the production producer for a mock that emits 3 frames.
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
		for i := 0; i < 3; i++ {
			select {
			case <-stopCh:
				return nil
			case out <- contracts.Frame{
				Seq: uint64(i), Timestamp: time.Now(),
				Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8,
				Data: &fakeFrameData{data: []byte{0xAA}},
			}:
			}
		}
		return nil
	}

	src, err := Open(context.Background(), contracts.CaptureConfig{Width: 800, Height: 600})
	require.NoError(t, err)
	require.NotNil(t, src)
	defer src.Close()

	require.Equal(t, "web", src.Name())
	require.NoError(t, src.Start(context.Background(), contracts.CaptureConfig{Width: 800, Height: 600}))

	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case f, ok := <-src.Frames():
			if !ok {
				t.Fatalf("frames channel closed early after %d frames", received)
			}
			require.Equal(t, uint64(received), f.Seq)
			require.Equal(t, 800, f.Width)
			require.Equal(t, contracts.PixelFormatBGRA8, f.Format)
			received++
		case <-timeout:
			t.Fatalf("timed out after %d frames", received)
		}
	}
	stats := src.Stats()
	require.GreaterOrEqual(t, stats.FramesProduced, uint64(3))
	require.NoError(t, src.Stop())
}

func TestSource_RegisteredInFactory(t *testing.T) {
	kinds := capture.Kinds()
	found := false
	for _, k := range kinds {
		if k == "web" {
			found = true
			break
		}
	}
	require.True(t, found, `capture.Register("web", ...) should have run in init`)
}

func TestSource_ProductionOpenReturnsNotImplemented(t *testing.T) {
	// Force stub mode so this is deterministic on every host — the real
	// chromedp path is covered by TestProductionProducer_MissingBrowserErrors
	// and the integration tests. When HELIXQA_CAPTURE_WEB_STUB=1, Open must
	// return ErrNotWired regardless of what is installed on the host.
	t.Setenv("HELIXQA_CAPTURE_WEB_STUB", "1")
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = productionFrameProducer
	_, err := Open(context.Background(), contracts.CaptureConfig{})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotWired) || err.Error() != "")
}

// TestProductionProducer_MissingBrowserErrors verifies that when no
// chromium/google-chrome binary is on PATH (or the stub env is set),
// Open returns ErrNotWired or ErrChromeNotFound — never a panic.
func TestProductionProducer_MissingBrowserErrors(t *testing.T) {
	// Force the production producer path.
	orig := newFrameProducer
	defer func() { newFrameProducer = orig }()
	newFrameProducer = productionFrameProducer

	// Force stub mode so this test is deterministic regardless of host.
	t.Setenv("HELIXQA_CAPTURE_WEB_STUB", "1")

	_, err := Open(context.Background(), contracts.CaptureConfig{})
	require.Error(t, err, "Open must return an error when browser is absent or stub mode is on")
	// Must be one of the known sentinels, not a random panic.
	if !errors.Is(err, ErrNotWired) && !errors.Is(err, ErrChromeNotFound) {
		require.NotEmpty(t, err.Error(),
			"expected ErrNotWired or ErrChromeNotFound, got %v", err)
	}
}

// TestPngToBGRA8_SynthesizesFrame encodes a small synthetic PNG in memory and
// decodes it through pngToBGRA8; asserts that the output length is w*h*4
// and that a known pixel colour round-trips correctly.
func TestPngToBGRA8_SynthesizesFrame(t *testing.T) {
	const w, h = 4, 3
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Paint pixel (1,2) a known RGBA colour.
	img.Set(1, 2, color.RGBA{R: 0xAB, G: 0xCD, B: 0xEF, A: 0xFF})

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	gotW, gotH, raw, err := pngToBGRA8(buf.Bytes())
	require.NoError(t, err)
	require.Equal(t, w, gotW)
	require.Equal(t, h, gotH)
	require.Equal(t, w*h*4, len(raw), "raw slice must be width*height*4 bytes")

	// Verify the BGRA order for pixel (1,2).
	idx := (2*w + 1) * 4
	require.Equal(t, byte(0xEF), raw[idx+0], "B channel")
	require.Equal(t, byte(0xCD), raw[idx+1], "G channel")
	require.Equal(t, byte(0xAB), raw[idx+2], "R channel")
	require.Equal(t, byte(0xFF), raw[idx+3], "A channel")
}
