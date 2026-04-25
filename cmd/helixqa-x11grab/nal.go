// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"io"
)

// MaxNALBytes caps a single NAL unit's payload length. Real 1080p H.264 IDR
// NALs rarely exceed 1 MiB; 16 MiB is a very generous ceiling that bounds
// the buffer growth on a malformed / hostile stream.
const MaxNALBytes = 16 * 1024 * 1024

// ErrNALTooLarge is returned by SplitNALs if the accumulated pre-start-code
// buffer exceeds MaxNALBytes.
var ErrNALTooLarge = errors.New("helixqa-x11grab: NAL exceeds MaxNALBytes")

// SplitNALs reads an H.264 Annex-B bytestream from r and invokes emit for
// every complete NAL unit found (payload bytes only, start code stripped).
//
// Annex-B start codes are either three bytes (0x00 0x00 0x01) or four
// (0x00 0x00 0x00 0x01). SplitNALs accepts both. Leading bytes before the
// first start code are dropped silently.
//
// Returns io.EOF when r closes cleanly between NALs (last NAL is emitted
// before EOF is returned). Any handler error terminates the scan and is
// surfaced to the caller.
func SplitNALs(r io.Reader, emit func(nal []byte) error) error {
	br := bufio.NewReader(r)
	buf := make([]byte, 0, 64*1024)
	zeroCount := 0
	sawFirstStartCode := false

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		if !sawFirstStartCode {
			buf = buf[:0]
			return nil
		}
		err := emit(buf)
		buf = buf[:0]
		return err
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if ferr := flush(); ferr != nil {
					return ferr
				}
				return io.EOF
			}
			return err
		}

		// Watch for start-code sequences: ...00 00 01  OR  ...00 00 00 01.
		if b == 0x00 {
			zeroCount++
			buf = append(buf, b)
			if len(buf) > MaxNALBytes {
				return ErrNALTooLarge
			}
			continue
		}
		if b == 0x01 && zeroCount >= 2 {
			// Start code hit. Strip the trailing zeros AND the preceding
			// run of 0x00 we had in `buf` to isolate the NAL that ended
			// at this position.
			nal := buf[:len(buf)-zeroCount]
			if len(nal) > 0 {
				if !sawFirstStartCode {
					// First start code — discard anything prior (usually empty).
					buf = buf[:0]
				} else {
					if err := emit(nal); err != nil {
						return err
					}
				}
			}
			sawFirstStartCode = true
			buf = buf[:0]
			zeroCount = 0
			continue
		}
		// Ordinary byte — reset zero run.
		buf = append(buf, b)
		if len(buf) > MaxNALBytes {
			return ErrNALTooLarge
		}
		zeroCount = 0
	}
}

// EncodeStartCode3 prepends a 3-byte Annex-B start code. Useful when a test
// or downstream tool needs to reassemble the NAL stream.
func EncodeStartCode3(nal []byte) []byte {
	out := make([]byte, 0, len(nal)+3)
	out = append(out, 0x00, 0x00, 0x01)
	out = append(out, nal...)
	return out
}

// EncodeStartCode4 prepends a 4-byte Annex-B start code. Identical wire
// meaning as the 3-byte form; some encoders prefer 4-byte for alignment.
func EncodeStartCode4(nal []byte) []byte {
	out := make([]byte, 0, len(nal)+4)
	out = append(out, 0x00, 0x00, 0x00, 0x01)
	out = append(out, nal...)
	return out
}
