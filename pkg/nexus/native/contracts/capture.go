// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines the versioned, platform-agnostic interfaces and
// data types that all OCU native-bridge implementations must satisfy.
// Implementations live in sibling packages; this package contains only
// contracts — no logic, no CGo, no OS-specific code.
package contracts

import (
	"context"
	"time"
)

// PixelFormat identifies the pixel encoding of a captured frame.
type PixelFormat string

const (
	// PixelFormatBGRA8 is 32-bit BGRA, 8 bits per channel (most common on
	// Linux/X11 and macOS via CoreGraphics).
	PixelFormatBGRA8 PixelFormat = "bgra8"

	// PixelFormatNV12 is a YUV 4:2:0 semi-planar format used by V4L2, NVENC,
	// and hardware decoders.
	PixelFormatNV12 PixelFormat = "nv12"

	// PixelFormatI420 is a YUV 4:2:0 planar format used by libvpx and ffmpeg.
	PixelFormatI420 PixelFormat = "i420"

	// PixelFormatH264 indicates the frame carries an H.264 NAL unit rather
	// than a raw pixel buffer (zero-copy path from hardware encoders).
	PixelFormatH264 PixelFormat = "h264"
)

// CaptureConfig parameterises a capture session.
type CaptureConfig struct {
	// FrameRate is the target frames per second; 0 means uncapped.
	FrameRate int

	// Width and Height are the desired capture resolution in pixels.
	// 0 means "use the source's natural resolution".
	Width  int
	Height int

	// CursorVisible controls whether the mouse cursor is composited into the
	// captured frames.
	CursorVisible bool

	// ZeroCopy requests the DMA-buf / shared-memory zero-copy path when
	// available; falls back to copy-based capture if unsupported.
	ZeroCopy bool
}

// CaptureStats is a snapshot of live capture telemetry.
type CaptureStats struct {
	FramesProduced uint64
	FramesDropped  uint64
	LastFrameAt    time.Time
	AverageLatency time.Duration
}

// DMABufHandle wraps a Linux DMA-buf file descriptor and its associated
// layout metadata.
type DMABufHandle struct {
	// FD is the DMA-buf file descriptor; the caller must close it when done.
	FD int

	Width    int
	Height   int
	Stride   int
	Modifier uint64
}

// FrameData abstracts the backing storage of a captured frame's pixel data.
// Implementations may hold a CPU byte slice, a DMA-buf, or a GPU texture
// handle.
type FrameData interface {
	// AsBytes copies (or maps) the pixel data into a CPU-addressable slice.
	AsBytes() ([]byte, error)

	// AsDMABuf returns the DMA-buf handle if the backing store is a kernel
	// DMA buffer, otherwise returns (nil, false).
	AsDMABuf() (*DMABufHandle, bool)

	// Release returns the underlying resource to its pool or closes the FD.
	// Must be called exactly once when the caller is done with the frame.
	Release() error
}

// Frame is a single captured screen frame.
type Frame struct {
	// Seq is a monotonically increasing counter; 0 is the first frame.
	Seq uint64

	Timestamp time.Time

	Width  int
	Height int

	// Stride is the number of bytes per row (may include padding).
	Stride int

	Format PixelFormat

	// Data holds the pixel payload; may be nil if not yet decoded.
	Data FrameData

	// Metadata carries arbitrary key/value annotations (e.g. window title,
	// display name, GPU fence ID).
	Metadata map[string]string
}

// CaptureSource is the interface that all screen-capture backends must
// implement.
type CaptureSource interface {
	// Name returns a human-readable identifier for the source
	// (e.g. "pipewire", "wlroots-dmabuf", "x11-shm").
	Name() string

	// Start begins capturing with the supplied configuration.
	// The context controls the lifetime of the capture session.
	Start(ctx context.Context, cfg CaptureConfig) error

	// Stop signals the source to cease capture and flush any pending frames.
	Stop() error

	// Frames returns a read-only channel on which captured frames are
	// delivered.  The channel is closed when the source stops.
	Frames() <-chan Frame

	// Stats returns a point-in-time snapshot of capture telemetry.
	Stats() CaptureStats

	// Close releases all resources held by the source.
	Close() error
}
