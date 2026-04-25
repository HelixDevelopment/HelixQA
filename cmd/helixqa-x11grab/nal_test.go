// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestSplitNALs_ThreeByteStartCodes(t *testing.T) {
	// Two NALs separated by a 3-byte start code, preceded by another start code.
	stream := []byte{
		0x00, 0x00, 0x01, // start (leading — skipped)
		0x67, 0x42, // NAL 1: SPS-ish
		0x00, 0x00, 0x01, // start
		0x68, 0x99, // NAL 2: PPS-ish
	}
	got := collectNALs(t, stream)
	want := [][]byte{{0x67, 0x42}, {0x68, 0x99}}
	assertNALs(t, got, want)
}

func TestSplitNALs_FourByteStartCodes(t *testing.T) {
	stream := []byte{
		0x00, 0x00, 0x00, 0x01, // 4-byte start (leading)
		0x41, 0xA0, 0xFF,
		0x00, 0x00, 0x00, 0x01, // 4-byte start
		0x65, 0x88, 0x7A, 0x01,
	}
	got := collectNALs(t, stream)
	want := [][]byte{{0x41, 0xA0, 0xFF}, {0x65, 0x88, 0x7A, 0x01}}
	assertNALs(t, got, want)
}

func TestSplitNALs_MixedStartCodes(t *testing.T) {
	stream := []byte{
		0x00, 0x00, 0x01, 0xAA, // NAL {0xAA}
		0x00, 0x00, 0x00, 0x01, 0xBB, // NAL {0xBB}
		0x00, 0x00, 0x01, 0xCC, 0xDD, // NAL {0xCC,0xDD}
	}
	got := collectNALs(t, stream)
	want := [][]byte{{0xAA}, {0xBB}, {0xCC, 0xDD}}
	assertNALs(t, got, want)
}

func TestSplitNALs_LeadingGarbageDropped(t *testing.T) {
	// Bytes before the first start code must NOT be emitted.
	stream := []byte{
		0xFF, 0xFE, 0xFD, // junk
		0x00, 0x00, 0x01,
		0x67, 0x42,
	}
	got := collectNALs(t, stream)
	want := [][]byte{{0x67, 0x42}}
	assertNALs(t, got, want)
}

func TestSplitNALs_NoStartCode_EmitsNothing(t *testing.T) {
	stream := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	got := collectNALs(t, stream)
	if len(got) != 0 {
		t.Errorf("expected 0 NALs, got %d: %v", len(got), got)
	}
}

func TestSplitNALs_RunsOfZerosDontFalseTrigger(t *testing.T) {
	// Three zeros in the payload (emulation prevention byte territory) must
	// not be misread as a start code.
	stream := []byte{
		0x00, 0x00, 0x01, 0x67, 0x00, 0x00, 0x03, 0x42, // NAL with 0x03 escape
		0x00, 0x00, 0x01, 0x68,
	}
	got := collectNALs(t, stream)
	// Emulation bytes (0x03) are NOT stripped by SplitNALs — that's the
	// decoder's job. We just confirm we emit the NAL bodies intact.
	want := [][]byte{{0x67, 0x00, 0x00, 0x03, 0x42}, {0x68}}
	assertNALs(t, got, want)
}

func TestSplitNALs_CleanEOF(t *testing.T) {
	var got [][]byte
	err := SplitNALs(bytes.NewReader(nil), func(n []byte) error {
		got = append(got, append([]byte(nil), n...))
		return nil
	})
	if !errors.Is(err, io.EOF) {
		t.Errorf("empty reader: want io.EOF, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want zero NALs, got %v", got)
	}
}

func TestSplitNALs_HandlerError(t *testing.T) {
	stream := []byte{0x00, 0x00, 0x01, 0x67, 0x00, 0x00, 0x01, 0x68}
	boom := errors.New("boom")
	err := SplitNALs(bytes.NewReader(stream), func([]byte) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("want boom, got %v", err)
	}
}

func TestSplitNALs_FinalNALFlushedOnEOF(t *testing.T) {
	// A stream that ends in the middle of a NAL (no trailing start code)
	// should still emit the last NAL before returning EOF.
	stream := []byte{0x00, 0x00, 0x01, 0x67, 0x42, 0x88}
	got := collectNALs(t, stream)
	want := [][]byte{{0x67, 0x42, 0x88}}
	assertNALs(t, got, want)
}

func TestEncodeStartCode3(t *testing.T) {
	nal := []byte{0x67, 0x42}
	out := EncodeStartCode3(nal)
	want := []byte{0x00, 0x00, 0x01, 0x67, 0x42}
	if !bytes.Equal(out, want) {
		t.Errorf("got %v, want %v", out, want)
	}
}

func TestEncodeStartCode4(t *testing.T) {
	nal := []byte{0x67}
	out := EncodeStartCode4(nal)
	want := []byte{0x00, 0x00, 0x00, 0x01, 0x67}
	if !bytes.Equal(out, want) {
		t.Errorf("got %v, want %v", out, want)
	}
}

func TestSplitNALs_Roundtrip(t *testing.T) {
	// Encode several NALs with mixed start codes and split back — the
	// payloads MUST round-trip byte-exact.
	nals := [][]byte{
		{0x67, 0x42, 0xAA},
		{0x68, 0x00, 0x00, 0x03, 0x01},
		{0x65, 0x88, 0x55, 0xCC},
		{0x06, 0x01, 0x02},
	}
	var stream bytes.Buffer
	for i, nal := range nals {
		if i%2 == 0 {
			stream.Write(EncodeStartCode3(nal))
		} else {
			stream.Write(EncodeStartCode4(nal))
		}
	}
	got := collectNALs(t, stream.Bytes())
	assertNALs(t, got, nals)
}

// --- helpers ---

func collectNALs(t *testing.T, stream []byte) [][]byte {
	t.Helper()
	var out [][]byte
	err := SplitNALs(bytes.NewReader(stream), func(n []byte) error {
		out = append(out, append([]byte(nil), n...))
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("SplitNALs: %v", err)
	}
	return out
}

func assertNALs(t *testing.T, got, want [][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("NAL count: got %d, want %d\ngot=%v\nwant=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("NAL %d: got %v, want %v", i, got[i], want[i])
		}
	}
}
