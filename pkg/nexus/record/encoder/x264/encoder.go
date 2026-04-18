// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package x264 is the OCU P5 software H.264 encoder stub. It registers the
// "x264" kind in the parent encoder factory. Production Encode() returns
// ErrNotWired; real libx264 CGO binding arrives in P5.5. Tests inject a mock
// via the package-level newEncoder variable.
package x264

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

func init() {
	encoder.Register("x264", func() encoder.Encoder {
		return newEncoder()
	})
}

// newEncoder is package-level injectable; tests replace it with a mock.
var newEncoder = func() encoder.Encoder {
	return &productionEncoder{}
}

// productionEncoder is the not-yet-wired stub.
type productionEncoder struct{}

// Encode implements encoder.Encoder. Returns ErrNotWired in P5.
func (e *productionEncoder) Encode(_ contracts.Frame) error {
	return encoder.ErrNotWired
}

// Close implements encoder.Encoder.
func (e *productionEncoder) Close() error {
	return nil
}
