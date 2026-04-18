// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"io"
	"time"
)

// RecordConfig parameterises a recording session.
type RecordConfig struct {
	FrameRate     int
	BitrateKbps   int
	SegmentLength time.Duration
	Codec         string
	Encoder       string
	OutputDir     string
}

// ClipOptions controls post-processing applied when cutting a clip.
type ClipOptions struct {
	BurntInTimestamp   bool
	BurntInActionArrow bool
	AnchorPoint        Point
	Annotation         string
}

// Recorder captures frames from a CaptureSource and writes video output.
type Recorder interface {
	// AttachSource binds the recorder to a live capture source.
	AttachSource(src CaptureSource) error

	// Start begins recording with the given configuration.
	Start(ctx context.Context, cfg RecordConfig) error

	// Clip extracts and encodes a time-windowed segment from the recording
	// buffer, writing the result to out.
	Clip(around time.Time, window time.Duration, out io.Writer, opts ClipOptions) error

	// LiveStream starts a WebRTC / WHIP ingest session and returns the WHIP
	// endpoint URL that consumers can connect to.
	LiveStream(ctx context.Context) (whipURL string, err error)

	// Stop finalises any open segment files and releases resources.
	Stop() error
}
