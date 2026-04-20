// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package frames defines the normalised Frame type used across every HelixQA
// capture backend (scrcpy, PipeWire, SCKit, WGC, Xvfb, ffmpeg). Every backend
// MUST emit Frame values; every consumer (vision, analysis, regression) MUST
// accept Frame values. Keeping the type dependency-free preserves the CGO-free
// invariant on the HelixQA Go host.
//
// See docs/openclawing/OpenClawing4.md §6.2 for the design contract.
package frames

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// Format identifies the on-the-wire pixel layout (or codec, for encoded formats).
type Format int

const (
	// FormatUnknown is the zero value and MUST NOT appear in valid Frames.
	FormatUnknown Format = iota
	// FormatNV12 is Y'CbCr 4:2:0 semi-planar — NVDEC / VideoToolbox / MediaCodec default.
	FormatNV12
	// FormatRGBA is 8-bit per channel RGBA, typically produced by OpenGL readbacks.
	FormatRGBA
	// FormatBGRA is 8-bit per channel BGRA, typically produced by macOS SCKit and Windows WGC.
	FormatBGRA
	// FormatH264AnnexB is H.264 NAL units with Annex-B start codes — scrcpy, NVENC.
	FormatH264AnnexB
)

// String gives a stable human-readable token for each Format.
func (f Format) String() string {
	switch f {
	case FormatNV12:
		return "nv12"
	case FormatRGBA:
		return "rgba"
	case FormatBGRA:
		return "bgra"
	case FormatH264AnnexB:
		return "h264-annexb"
	default:
		return "unknown"
	}
}

// Valid reports whether f is a recognised, non-zero format.
func (f Format) Valid() bool {
	return f >= FormatNV12 && f <= FormatH264AnnexB
}

// Frame is the canonical in-process representation of one captured frame.
//
// Large pixel payloads travel through a file descriptor (DataFD ≥ 0) backed by
// memfd_create, giving zero-copy hand-off between sidecars and the Go host.
// Small frames or tests may pass payloads inline via Data; DataFD must be -1
// in that case.
//
// AXTree is opaque here to keep this package dependency-free; consumers that
// need the accessibility tree cast it through pkg/nexus/observe/axtree. A nil
// AXTree is valid — many fast paths never snapshot the tree.
//
// Frame values are safe to copy by value but Close must be called exactly once
// on each Frame that owns a DataFD ≥ 0; the Go host owns that FD and leaking
// it shows up as a session-long resource leak.
type Frame struct {
	// PTS is the presentation timestamp since session start.
	PTS time.Duration
	// Width and Height are in pixels (encoded resolution for H264AnnexB).
	Width  int
	Height int
	// Format identifies the pixel / codec layout. Valid() MUST hold.
	Format Format
	// Source names the capture backend ("pipewire", "scrcpy", "sckit", "wgc", …).
	Source string
	// DataFD is a memfd (or other SCM_RIGHTS-passed fd) whose contents are the
	// pixel payload. Set to -1 when Data is used instead.
	DataFD int
	// DataLen is the payload length in bytes when DataFD ≥ 0; ignored otherwise.
	DataLen int
	// Data is an inline payload; empty when DataFD ≥ 0.
	Data []byte
	// AXTree is the optional accessibility-tree snapshot taken at capture time.
	// Opaque here; cast to *axtree.Node downstream.
	AXTree any
}

// ErrInvalid is returned by Validate for frames that would break downstream
// invariants (missing format, negative dimensions, both or neither payload kind).
var ErrInvalid = errors.New("frames: invalid frame")

// New constructs a validated Frame with an inline byte payload.
// Callers supplying a memfd should use NewFromFD instead.
func New(pts time.Duration, width, height int, format Format, source string, data []byte) (Frame, error) {
	f := Frame{
		PTS:    pts,
		Width:  width,
		Height: height,
		Format: format,
		Source: source,
		DataFD: -1,
		Data:   data,
	}
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// NewFromFD constructs a validated Frame whose payload lives in a memfd or
// other SCM_RIGHTS-passed file descriptor. Ownership transfers to the Frame —
// the caller MUST NOT close fd independently.
func NewFromFD(pts time.Duration, width, height int, format Format, source string, fd, length int) (Frame, error) {
	f := Frame{
		PTS:     pts,
		Width:   width,
		Height:  height,
		Format:  format,
		Source:  source,
		DataFD:  fd,
		DataLen: length,
	}
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// Validate reports whether the frame satisfies the invariants documented on the
// type. Zero-value frames, negative dimensions, unknown formats, and
// inline+FD-at-the-same-time are all rejected.
func (f Frame) Validate() error {
	if !f.Format.Valid() {
		return fmt.Errorf("%w: format=%v", ErrInvalid, f.Format)
	}
	if f.Width <= 0 || f.Height <= 0 {
		return fmt.Errorf("%w: width=%d height=%d", ErrInvalid, f.Width, f.Height)
	}
	if f.Source == "" {
		return fmt.Errorf("%w: source empty", ErrInvalid)
	}
	switch {
	case f.DataFD >= 0 && len(f.Data) > 0:
		return fmt.Errorf("%w: both DataFD and Data set", ErrInvalid)
	case f.DataFD >= 0 && f.DataLen <= 0:
		return fmt.Errorf("%w: DataFD=%d but DataLen=%d", ErrInvalid, f.DataFD, f.DataLen)
	case f.DataFD < 0 && len(f.Data) == 0:
		return fmt.Errorf("%w: no payload (DataFD=%d, Data empty)", ErrInvalid, f.DataFD)
	}
	return nil
}

// HasFD reports whether the payload travels through a file descriptor.
func (f Frame) HasFD() bool { return f.DataFD >= 0 }

// Close releases the file descriptor if one is owned. Safe to call on frames
// with inline payloads — returns nil. Safe to call multiple times; only the
// first call with a non-negative DataFD actually closes the FD, and the second
// call returns nil. This method mutates f's receiver via a pointer copy to
// achieve idempotency; callers using value-Frame should re-assign.
func (f *Frame) Close() error {
	if f == nil || f.DataFD < 0 {
		return nil
	}
	fd := f.DataFD
	f.DataFD = -1
	f.DataLen = 0
	if err := syscall.Close(fd); err != nil {
		return fmt.Errorf("frames: close fd=%d: %w", fd, err)
	}
	return nil
}
