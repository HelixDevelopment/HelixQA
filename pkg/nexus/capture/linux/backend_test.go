// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestDetectBackend_PrefersXwd verifies that when both xwd and convert are on
// PATH the backend name "xwd" is returned.  Skips when xwd is absent so CI
// machines without X11 tools stay green.
func TestDetectBackend_PrefersXwd(t *testing.T) {
	if _, err := exec.LookPath("xwd"); err != nil {
		t.Skip("xwd not on PATH — skipping xwd-preference test")
	}
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("convert not on PATH — skipping xwd-preference test")
	}
	backend, err := detectBackend()
	require.NoError(t, err)
	require.Equal(t, backendXwd, backend)
}

// TestDetectBackend_FallsBackToGnome verifies that when xwd is absent but
// gnome-screenshot is present the backend resolves to "gnome-screenshot".
// Skipped unless gnome-screenshot is on PATH and xwd is not.
func TestDetectBackend_FallsBackToGnome(t *testing.T) {
	if _, err := exec.LookPath("xwd"); err == nil {
		t.Skip("xwd is present — fallback-to-gnome test not applicable")
	}
	if _, err := exec.LookPath("gnome-screenshot"); err != nil {
		t.Skip("gnome-screenshot not on PATH — skipping fallback test")
	}
	backend, err := detectBackend()
	require.NoError(t, err)
	require.Equal(t, backendGnome, backend)
}

// TestDetectBackend_NoBackend_ReturnsErrNotWired verifies that detectBackend
// returns ErrNotWired when no screenshot tool is on PATH.  It achieves this
// by temporarily redirecting PATH to an empty directory.
func TestDetectBackend_NoBackend_ReturnsErrNotWired(t *testing.T) {
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", t.TempDir()) // empty dir — nothing on PATH

	_, err := detectBackend()
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotWired), "expected ErrNotWired, got: %v", err)
}

// TestStubEnv_ForcesErrNotWired verifies that HELIXQA_CAPTURE_LINUX_STUB=1
// makes Open() return ErrNotWired regardless of which tools are installed.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_CAPTURE_LINUX_STUB", "1")
	// Ensure the display-check doesn't fire before the stub-check.
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Setenv("DISPLAY", ":0")
	}

	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = productionFrameProducer // production path

	_, err := Open(context.Background(), contracts.CaptureConfig{})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotWired), "expected ErrNotWired, got: %v", err)
}

// TestBMPToBGRA8_SynthesizedHeader verifies convertBMPBytesToBGRA8 on a
// minimal hand-crafted 2×2 24-bit BMP.  Expected output: 2*2*4 = 16 bytes,
// with rows flipped (BMP is bottom-up) and alpha=0xFF appended.
func TestBMPToBGRA8_SynthesizedHeader(t *testing.T) {
	const w, h = 2, 2
	rowStride := ((w * 3) + 3) &^ 3 // = 8 bytes (4-byte-aligned)
	pixelDataOffset := 14 + 40
	fileSize := pixelDataOffset + rowStride*h

	bmp := make([]byte, fileSize)

	// --- File header (14 bytes) ---
	bmp[0], bmp[1] = 'B', 'M'
	putU32LE(bmp[2:], uint32(fileSize))
	// bytes 6-9: reserved = 0
	putU32LE(bmp[10:], uint32(pixelDataOffset))

	// --- BITMAPINFOHEADER (40 bytes starting at offset 14) ---
	putU32LE(bmp[14:], 40)
	putI32LE(bmp[18:], int32(w))
	putI32LE(bmp[22:], int32(h)) // positive = bottom-up
	putU16LE(bmp[26:], 1)        // color planes
	putU16LE(bmp[28:], 24)       // bits per pixel

	// --- Pixel data ---
	// BMP stores rows bottom-up:
	//   file-row 0 → output row h-1 (= row 1)
	//   file-row 1 → output row h-2 (= row 0)
	p := pixelDataOffset
	// file-row 0 (output row 1): col0=(B=0x10,G=0x20,R=0x30), col1=(B=0x40,G=0x50,R=0x60)
	bmp[p+0], bmp[p+1], bmp[p+2] = 0x10, 0x20, 0x30
	bmp[p+3], bmp[p+4], bmp[p+5] = 0x40, 0x50, 0x60
	// file-row 1 (output row 0): col0=(B=0x70,G=0x80,R=0x90), col1=(B=0xA0,G=0xB0,R=0xC0)
	bmp[p+rowStride+0], bmp[p+rowStride+1], bmp[p+rowStride+2] = 0x70, 0x80, 0x90
	bmp[p+rowStride+3], bmp[p+rowStride+4], bmp[p+rowStride+5] = 0xA0, 0xB0, 0xC0

	out := convertBMPBytesToBGRA8(bmp, w, h)
	require.Len(t, out, w*h*4)

	// output row 0 comes from file-row 1
	require.Equal(t, []byte{0x70, 0x80, 0x90, 0xFF}, out[0:4], "out[row0,col0] BGRA")
	require.Equal(t, []byte{0xA0, 0xB0, 0xC0, 0xFF}, out[4:8], "out[row0,col1] BGRA")
	// output row 1 comes from file-row 0
	require.Equal(t, []byte{0x10, 0x20, 0x30, 0xFF}, out[8:12], "out[row1,col0] BGRA")
	require.Equal(t, []byte{0x40, 0x50, 0x60, 0xFF}, out[12:16], "out[row1,col1] BGRA")
}

// TestParseBMPDimensions_TopDown verifies that a negative BMP height (top-down
// storage) is returned as a positive integer.
func TestParseBMPDimensions_TopDown(t *testing.T) {
	bmp := make([]byte, 26)
	bmp[0], bmp[1] = 'B', 'M'
	putI32LE(bmp[18:], int32(640))
	putI32LE(bmp[22:], int32(-480))
	w, h, err := parseBMPDimensions(bmp)
	require.NoError(t, err)
	require.Equal(t, 640, w)
	require.Equal(t, 480, h)
}

// TestParseBMPDimensions_TooShort verifies that a truncated buffer returns an
// error and does not panic.
func TestParseBMPDimensions_TooShort(t *testing.T) {
	_, _, err := parseBMPDimensions([]byte{'B', 'M'})
	require.Error(t, err)
}

// TestParseBMPDimensions_BadSignature verifies that a non-BMP buffer returns
// an error.
func TestParseBMPDimensions_BadSignature(t *testing.T) {
	buf := make([]byte, 26)
	buf[0], buf[1] = 0x89, 'P' // PNG-like signature
	_, _, err := parseBMPDimensions(buf)
	require.Error(t, err)
}

// ---- little-endian write helpers used to build synthetic BMP headers --------

func putU32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func putI32LE(b []byte, v int32) { putU32LE(b, uint32(v)) }

func putU16LE(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}
