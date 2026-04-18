// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// xwdProducer captures the X11 root window by piping:
//
//	xwd -root -silent | convert xwd:- bmp:-
//
// It decodes each BMP frame into a contracts.Frame{BGRA8} and sends it on out.
// The loop runs at cfg.FrameRate fps (default 10).
func xwdProducer(
	ctx context.Context,
	cfg contracts.CaptureConfig,
	out chan<- contracts.Frame,
	stopCh <-chan struct{},
) error {
	xwdPath, err := exec.LookPath("xwd")
	if err != nil {
		return fmt.Errorf("capture/linux/xwd: xwd not on PATH: %w", err)
	}
	convertPath, err := exec.LookPath("convert")
	if err != nil {
		return fmt.Errorf("capture/linux/xwd: convert (ImageMagick) not on PATH: %w", err)
	}

	fps := cfg.FrameRate
	if fps <= 0 {
		fps = 10
	}
	interval := time.Second / time.Duration(fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var seq uint64
	for {
		select {
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		bmpData, w, h, ferr := captureXwdFrame(ctx, xwdPath, convertPath)
		if ferr != nil {
			// Non-fatal: skip frame, keep running.
			continue
		}

		raw := convertBMPBytesToBGRA8(bmpData, w, h)
		f := contracts.Frame{
			Seq:       seq,
			Timestamp: time.Now(),
			Width:     w,
			Height:    h,
			Format:    contracts.PixelFormatBGRA8,
			Data:      &bytesFrameData{data: raw},
		}
		seq++
		select {
		case out <- f:
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// captureXwdFrame runs the xwd|convert pipeline and returns the raw BMP bytes
// along with the width and height parsed from the BMP header.
func captureXwdFrame(ctx context.Context, xwdPath, convertPath string) ([]byte, int, int, error) {
	// Wire up xwd stdout → pipe → convert stdin.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("os.Pipe: %w", err)
	}

	xwd := exec.CommandContext(ctx, xwdPath, "-root", "-silent")
	xwd.Stdout = pw

	convert := exec.CommandContext(ctx, convertPath, "xwd:-", "bmp:-")
	convert.Stdin = pr

	var buf bytes.Buffer
	convert.Stdout = &buf

	if err := xwd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, 0, 0, fmt.Errorf("xwd start: %w", err)
	}
	if err := convert.Start(); err != nil {
		pw.Close()
		pr.Close()
		_ = xwd.Wait()
		return nil, 0, 0, fmt.Errorf("convert start: %w", err)
	}

	// Wait for xwd to finish writing, then close the write-end of the pipe
	// so convert receives EOF.
	xwdErr := xwd.Wait()
	pw.Close()
	pr.Close()
	if xwdErr != nil {
		_ = convert.Wait()
		return nil, 0, 0, fmt.Errorf("xwd wait: %w", xwdErr)
	}

	if err := convert.Wait(); err != nil {
		return nil, 0, 0, fmt.Errorf("convert wait: %w", err)
	}

	data := buf.Bytes()
	w, h, err := parseBMPDimensions(data)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("parseBMPDimensions: %w", err)
	}
	return data, w, h, nil
}

// parseBMPDimensions reads width and height from a Windows BMP file header.
//
// BMP header layout (all little-endian):
//
//	bytes  0– 1: signature "BM"
//	bytes  2– 5: file size (uint32)
//	bytes  6– 9: reserved (uint32)
//	bytes 10–13: pixel data offset (uint32)
//	bytes 14–17: info-header size (uint32, == 40 for BITMAPINFOHEADER)
//	bytes 18–21: width  (int32)
//	bytes 22–25: height (int32, negative = top-down)
func parseBMPDimensions(data []byte) (int, int, error) {
	if len(data) < 26 {
		return 0, 0, fmt.Errorf("BMP too short (%d bytes)", len(data))
	}
	if data[0] != 'B' || data[1] != 'M' {
		return 0, 0, fmt.Errorf("not a BMP (signature %02x %02x)", data[0], data[1])
	}
	w := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	h := int(int32(binary.LittleEndian.Uint32(data[22:26])))
	if h < 0 {
		h = -h // top-down BMP — negate to get positive height
	}
	return w, h, nil
}

// convertBMPBytesToBGRA8 extracts raw pixel data from a 24-bit BMP byte slice
// and returns a w*h*4 BGRA8 slice (rows reordered top→bottom, alpha=0xFF).
//
// BMP stores pixels in BGR order, bottom-up, with each row padded to a
// 4-byte boundary.
func convertBMPBytesToBGRA8(bmp []byte, w, h int) []byte {
	out := make([]byte, w*h*4)
	if len(bmp) < 14+4 {
		return out
	}
	offset := int(binary.LittleEndian.Uint32(bmp[10:14]))
	// 24-bit BGR, padded to 4-byte row stride.
	rowStride := ((w * 3) + 3) &^ 3
	needed := offset + rowStride*h
	if len(bmp) < needed {
		return out
	}
	for row := 0; row < h; row++ {
		srcRow := h - 1 - row // BMP is bottom-up
		src := bmp[offset+srcRow*rowStride:]
		dst := out[row*w*4:]
		for col := 0; col < w; col++ {
			dst[col*4+0] = src[col*3+0] // B
			dst[col*4+1] = src[col*3+1] // G
			dst[col*4+2] = src[col*3+2] // R
			dst[col*4+3] = 0xFF         // A (opaque)
		}
	}
	return out
}

// pngToBGRA8 decodes a PNG-encoded byte slice and returns (width, height,
// BGRA8 raw pixels, error).  The raw slice is width*height*4 bytes long.
func pngToBGRA8(buf []byte) (int, int, []byte, error) {
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("pngToBGRA8: decode: %w", err)
	}
	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y
	raw := make([]byte, w*h*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			idx := ((y-bounds.Min.Y)*w + (x - bounds.Min.X)) * 4
			raw[idx+0] = byte(b >> 8) // B
			raw[idx+1] = byte(g >> 8) // G
			raw[idx+2] = byte(r >> 8) // R
			raw[idx+3] = byte(a >> 8) // A
		}
	}
	return w, h, raw, nil
}

// bytesFrameData wraps a []byte and satisfies contracts.FrameData.
type bytesFrameData struct{ data []byte }

func (d *bytesFrameData) AsBytes() ([]byte, error)                  { return d.data, nil }
func (d *bytesFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (d *bytesFrameData) Release() error                            { return nil }
