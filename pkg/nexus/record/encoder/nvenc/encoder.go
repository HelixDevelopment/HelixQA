// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package nvenc is the OCU P5 NVIDIA NVENC encoder stub. It registers the
// "nvenc" kind in the parent encoder factory. Production Encode() returns
// ErrNotWired; in P5.5 this backend will dispatch H.264/HEVC encode jobs to
// thinker.local via ocuremote.Dispatcher (reusing the SSH trust established
// by P2), so no new credential or firewall rule is needed. Tests inject a
// mock via the package-level newEncoder variable.
package nvenc

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

func init() {
	encoder.Register("nvenc", func() encoder.Encoder {
		return newEncoder()
	})
}

// newEncoder is package-level injectable; tests replace it with a mock.
var newEncoder = func() encoder.Encoder {
	return &productionEncoder{}
}

// productionEncoder is the not-yet-wired stub.
// P5.5 plan: replace with a gRPC call to the CUDA sidecar on thinker.local
// via ocuremote.Dispatcher (same SSH tunnel used by P2 vision dispatch).
type productionEncoder struct{}

// Encode implements encoder.Encoder. Returns ErrNotWired in P5.
func (e *productionEncoder) Encode(_ contracts.Frame) error {
	return encoder.ErrNotWired
}

// Close implements encoder.Encoder.
func (e *productionEncoder) Close() error {
	return nil
}
