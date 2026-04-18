// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package vaapi is the OCU P5 VA-API (Video Acceleration API) encoder stub.
// It registers the "vaapi" kind in the parent encoder factory. Production
// Encode() returns ErrNotWired; real VA-API / FFmpeg CGO binding arrives in
// P5.5. Tests inject a mock via the package-level newEncoder variable.
package vaapi

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

func init() {
	encoder.Register("vaapi", func() encoder.Encoder {
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
