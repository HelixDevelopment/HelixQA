// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// bytesFrameData wraps a []byte and satisfies contracts.FrameData.
type bytesFrameData struct{ data []byte }

func (d *bytesFrameData) AsBytes() ([]byte, error)                  { return d.data, nil }
func (d *bytesFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (d *bytesFrameData) Release() error                            { return nil }

// splitH264NALUnits splits a byte slice on H.264 start codes (00 00 00 01
// or 00 00 01) and returns each NAL unit as a separate slice including its
// leading start code. An empty input returns nil.
func splitH264NALUnits(data []byte) [][]byte {
	var nals [][]byte
	start := -1
	n := len(data)
	for i := 0; i < n; i++ {
		// 4-byte start code: 00 00 00 01
		if i+3 < n && data[i] == 0x00 && data[i+1] == 0x00 &&
			data[i+2] == 0x00 && data[i+3] == 0x01 {
			if start >= 0 {
				nals = append(nals, data[start:i])
			}
			start = i
			i += 3 // skip past the start code bytes
			continue
		}
		// 3-byte start code: 00 00 01
		if i+2 < n && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			if start >= 0 {
				nals = append(nals, data[start:i])
			}
			start = i
			i += 2
			continue
		}
	}
	if start >= 0 && start < n {
		nals = append(nals, data[start:])
	}
	return nals
}
