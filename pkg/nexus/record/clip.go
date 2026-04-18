// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// frameMetadata is the P5 wire format written per frame by Clip. Real MKV/MP4
// muxing lands in P5.5; for now we emit newline-delimited JSON so that
// consumers can at least introspect the clip timeline without a video decoder.
type frameMetadata struct {
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
}

// clipWrite is the shared implementation used by Recorder.Clip. Extracted so
// that tests can call it directly without a fully-wired Recorder.
func clipWrite(
	ring *FrameRing,
	around time.Time,
	window time.Duration,
	out io.Writer,
	opts contracts.ClipOptions,
) error {
	frames := ring.SnapshotAround(around, window)
	enc := json.NewEncoder(out)
	for _, f := range frames {
		meta := frameMetadata{
			Seq:       f.Seq,
			Timestamp: f.Timestamp,
			Width:     f.Width,
			Height:    f.Height,
		}
		if err := enc.Encode(meta); err != nil {
			return fmt.Errorf("clip write: %w", err)
		}
	}
	if opts.Annotation != "" {
		if _, err := fmt.Fprintln(out, "annotation:", opts.Annotation); err != nil {
			return fmt.Errorf("clip annotation write: %w", err)
		}
	}
	return nil
}
