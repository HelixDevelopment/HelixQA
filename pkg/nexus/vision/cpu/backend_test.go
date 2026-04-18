// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cpu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ---------------------------------------------------------------------------
// Existing tests (must stay green)
// ---------------------------------------------------------------------------

func TestBackend_Analyze_ReturnsLocalCPU(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}
	res, err := b.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", res.DispatchedTo)
}

func TestBackend_Analyze_RejectsUnsupportedFormat(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatH264}
	_, err := b.Analyze(context.Background(), frame)
	require.Error(t, err)
}

func TestBackend_Match_ReturnsEmpty(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 100, Height: 100, Format: contracts.PixelFormatBGRA8}
	tmpl := contracts.Template{Name: "t", Bytes: []byte{0x00}}
	res, err := b.Match(context.Background(), frame, tmpl)
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestBackend_Diff_FlagsSameShape(t *testing.T) {
	b := New()
	a := contracts.Frame{Width: 10, Height: 10, Format: contracts.PixelFormatBGRA8}
	c := contracts.Frame{Width: 10, Height: 10, Format: contracts.PixelFormatBGRA8}
	res, err := b.Diff(context.Background(), a, c)
	require.NoError(t, err)
	require.True(t, res.SameShape)
}

func TestBackend_OCR_ReturnsEmpty(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 100, Height: 100, Format: contracts.PixelFormatBGRA8}
	rect := contracts.Rect{W: 50, H: 50}
	res, err := b.OCR(context.Background(), frame, rect)
	require.NoError(t, err)
	require.Empty(t, res.FullText)
}

// ---------------------------------------------------------------------------
// P2.5 new tests
// ---------------------------------------------------------------------------

// makeBGRAFrame builds a Frame backed by a plain byte slice via byteData.
func makeBGRAFrame(w, h int, fill []byte) contracts.Frame {
	buf := make([]byte, w*h*4)
	if len(fill) >= w*h*4 {
		copy(buf, fill[:w*h*4])
	} else if len(fill) == 4 {
		// repeat single pixel
		for i := 0; i < w*h; i++ {
			copy(buf[i*4:], fill)
		}
	}
	return contracts.Frame{
		Width:  w,
		Height: h,
		Format: contracts.PixelFormatBGRA8,
		Data:   &byteData{b: buf},
	}
}

// byteData implements contracts.FrameData over a plain []byte.
type byteData struct{ b []byte }

func (d *byteData) AsBytes() ([]byte, error) { return d.b, nil }
func (d *byteData) AsDMABuf() (*contracts.DMABufHandle, bool) {
	return nil, false
}
func (d *byteData) Release() error { return nil }

// TestDiff_SamePixels_ZeroDelta — identical BGRA8 buffers → TotalDelta==0.
func TestDiff_SamePixels_ZeroDelta(t *testing.T) {
	t.Setenv("HELIXQA_VISION_CPU_STUB", "")
	before := makeBGRAFrame(8, 8, []byte{100, 150, 200, 255})
	after := makeBGRAFrame(8, 8, []byte{100, 150, 200, 255})

	b := New()
	res, err := b.Diff(context.Background(), before, after)
	require.NoError(t, err)
	require.True(t, res.SameShape)
	require.Equal(t, 0.0, res.TotalDelta)
}

// TestDiff_DifferentPixels_NonZeroDelta — one pixel different → TotalDelta > 0.
func TestDiff_DifferentPixels_NonZeroDelta(t *testing.T) {
	t.Setenv("HELIXQA_VISION_CPU_STUB", "")
	w, h := 4, 4
	before := makeBGRAFrame(w, h, []byte{0, 0, 0, 255})

	// change one pixel (index 5) to white
	bufAfter := make([]byte, w*h*4)
	after := contracts.Frame{
		Width: w, Height: h,
		Format: contracts.PixelFormatBGRA8,
		Data:   &byteData{b: bufAfter},
	}
	// pixel 5 = BGRA white
	bufAfter[5*4+0] = 255
	bufAfter[5*4+1] = 255
	bufAfter[5*4+2] = 255
	bufAfter[5*4+3] = 255

	b := New()
	res, err := b.Diff(context.Background(), before, after)
	require.NoError(t, err)
	require.True(t, res.SameShape)
	require.Greater(t, res.TotalDelta, 0.0)
}

// TestDiff_EmptyData_GracefulZero — Frame with nil Data does not panic.
func TestDiff_EmptyData_GracefulZero(t *testing.T) {
	t.Setenv("HELIXQA_VISION_CPU_STUB", "")
	nilFrame := contracts.Frame{Width: 10, Height: 10, Format: contracts.PixelFormatBGRA8}

	b := New()
	res, err := b.Diff(context.Background(), nilFrame, nilFrame)
	require.NoError(t, err)
	require.True(t, res.SameShape)
	require.Equal(t, 0.0, res.TotalDelta)
}

// TestAnalyze_EdgeDetection_FindsElements — checkerboard 100×100 BGRA8
// produces ≥1 UIElement.
func TestAnalyze_EdgeDetection_FindsElements(t *testing.T) {
	t.Setenv("HELIXQA_VISION_CPU_STUB", "")
	w, h := 100, 100
	buf := make([]byte, w*h*4)
	// 10×10 checkerboard: alternating black and white 10-pixel squares.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := (x/10 + y/10) % 2
			var v byte
			if cell == 0 {
				v = 255
			}
			i := (y*w + x) * 4
			buf[i+0] = v // B
			buf[i+1] = v // G
			buf[i+2] = v // R
			buf[i+3] = 255
		}
	}
	frame := contracts.Frame{
		Width: w, Height: h,
		Format: contracts.PixelFormatBGRA8,
		Data:   &byteData{b: buf},
	}

	b := New()
	res, err := b.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", res.DispatchedTo)
	require.GreaterOrEqual(t, len(res.Elements), 1, "Sobel edge detection must find at least one contour region")
}

// TestStubEnv_ForcesEmpty — HELIXQA_VISION_CPU_STUB=1 returns empty
// results regardless of input.
func TestStubEnv_ForcesEmpty(t *testing.T) {
	t.Setenv("HELIXQA_VISION_CPU_STUB", "1")

	w, h := 8, 8
	frame := makeBGRAFrame(w, h, []byte{200, 100, 50, 255})

	b := New()

	// Analyze must return no elements.
	analysis, err := b.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", analysis.DispatchedTo)
	require.Empty(t, analysis.Elements)

	// Diff must return zero TotalDelta and no regions.
	diff, err := b.Diff(context.Background(), frame, frame)
	require.NoError(t, err)
	require.Equal(t, 0.0, diff.TotalDelta)
	require.Empty(t, diff.Regions)
}
