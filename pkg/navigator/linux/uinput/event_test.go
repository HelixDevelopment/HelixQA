// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package uinput

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestEncodeEvent_ByteExact(t *testing.T) {
	buf := EncodeEvent(EventTypeKey, KeyA, int32(KeyPress))
	// First 16 bytes (time) MUST be zero — the kernel stamps them.
	for i := 0; i < 16; i++ {
		if buf[i] != 0 {
			t.Errorf("time byte %d = %d, want 0", i, buf[i])
		}
	}
	// type at offset 16
	if got := binary.LittleEndian.Uint16(buf[16:18]); got != EventTypeKey {
		t.Errorf("type = %d, want %d", got, EventTypeKey)
	}
	// code at offset 18
	if got := binary.LittleEndian.Uint16(buf[18:20]); got != KeyA {
		t.Errorf("code = %d, want %d", got, KeyA)
	}
	// value at offset 20
	if got := int32(binary.LittleEndian.Uint32(buf[20:24])); got != int32(KeyPress) {
		t.Errorf("value = %d, want %d", got, KeyPress)
	}
}

func TestEncodeEvent_Size(t *testing.T) {
	buf := EncodeEvent(0, 0, 0)
	if len(buf) != EventSize {
		t.Errorf("event size = %d, want %d", len(buf), EventSize)
	}
	if EventSize != 24 {
		t.Errorf("EventSize = %d, want 24 for amd64/arm64 Linux", EventSize)
	}
}

func TestEncodeEvent_NegativeValue(t *testing.T) {
	// Ensure two's-complement encoding round-trips through the wire.
	buf := EncodeEvent(EventTypeRel, RelX, -1)
	if got := int32(binary.LittleEndian.Uint32(buf[20:24])); got != -1 {
		t.Errorf("-1 value round-trip: got %d", got)
	}
}

func TestDecodeEvent_Roundtrip(t *testing.T) {
	in := Event{Type: EventTypeKey, Code: KeyEnter, Value: int32(KeyAutorepeat)}
	buf := EncodeEvent(in.Type, in.Code, in.Value)
	out := DecodeEvent(buf)
	if out != in {
		t.Errorf("roundtrip: got %+v want %+v", out, in)
	}
}

// --- high-level writers ---

func TestWriteKeyTap_Sequence(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteKeyTap(&buf, KeyEnter); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 4*EventSize {
		t.Fatalf("KeyTap should emit 4 events, got %d bytes", buf.Len())
	}
	// Expected sequence: press, syn, release, syn.
	evts := decodeAll(t, buf.Bytes())
	expected := []Event{
		{Type: EventTypeKey, Code: KeyEnter, Value: int32(KeyPress)},
		{Type: EventTypeSyn, Code: SynReport, Value: 0},
		{Type: EventTypeKey, Code: KeyEnter, Value: int32(KeyRelease)},
		{Type: EventTypeSyn, Code: SynReport, Value: 0},
	}
	compareEvents(t, evts, expected)
}

func TestWriteClickAbs_Sequence(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteClickAbs(&buf, 500, 400); err != nil {
		t.Fatal(err)
	}
	// Expected: ABS_X=500, ABS_Y=400, BtnLeft down, SYN, BtnLeft up, SYN = 6 events
	if buf.Len() != 6*EventSize {
		t.Fatalf("ClickAbs should emit 6 events, got %d bytes", buf.Len())
	}
	evts := decodeAll(t, buf.Bytes())
	expected := []Event{
		{Type: EventTypeAbs, Code: AbsX, Value: 500},
		{Type: EventTypeAbs, Code: AbsY, Value: 400},
		{Type: EventTypeKey, Code: BtnLeft, Value: int32(KeyPress)},
		{Type: EventTypeSyn, Code: SynReport, Value: 0},
		{Type: EventTypeKey, Code: BtnLeft, Value: int32(KeyRelease)},
		{Type: EventTypeSyn, Code: SynReport, Value: 0},
	}
	compareEvents(t, evts, expected)
}

func TestWriteMoveRel_EmitsOnlyNonZero(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMoveRel(&buf, 10, 0); err != nil {
		t.Fatal(err)
	}
	// Expect: REL_X=10, SYN — only 2 events (dy=0 is suppressed)
	if buf.Len() != 2*EventSize {
		t.Fatalf("expected 2 events (dx only + sync), got %d bytes", buf.Len())
	}
	evts := decodeAll(t, buf.Bytes())
	if evts[0].Code != RelX || evts[0].Value != 10 {
		t.Errorf("first event: %+v", evts[0])
	}
	if evts[1].Type != EventTypeSyn {
		t.Errorf("second event must be SYN, got %+v", evts[1])
	}
}

func TestWriteMoveRel_AllZero_JustSync(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMoveRel(&buf, 0, 0); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != EventSize {
		t.Fatalf("all-zero motion should emit 1 SYN event, got %d bytes", buf.Len())
	}
	if got := DecodeEvent(to24(buf.Bytes())); got.Type != EventTypeSyn {
		t.Errorf("got %+v, want SYN_REPORT", got)
	}
}

func TestWriteScroll_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteScroll(&buf, 0); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("zero-ticks scroll should emit nothing, got %d bytes", buf.Len())
	}
}

func TestWriteScroll_PositiveAndNegative(t *testing.T) {
	for _, ticks := range []int32{3, -5} {
		var buf bytes.Buffer
		if err := WriteScroll(&buf, ticks); err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 2*EventSize {
			t.Fatalf("ticks=%d: got %d bytes, want %d", ticks, buf.Len(), 2*EventSize)
		}
		evts := decodeAll(t, buf.Bytes())
		if evts[0].Type != EventTypeRel || evts[0].Code != RelWheel || evts[0].Value != ticks {
			t.Errorf("first event wrong: %+v", evts[0])
		}
	}
}

// --- ShortWrite detection ---

type shortWriter struct{ accept int }

func (s shortWriter) Write(p []byte) (int, error) {
	if len(p) < s.accept {
		return len(p), nil
	}
	return s.accept, nil
}

func TestWriteEvent_ShortWrite(t *testing.T) {
	sw := shortWriter{accept: EventSize - 1}
	if err := WriteEvent(sw, EventTypeKey, KeyA, 1); !errors.Is(err, ErrShortWrite) {
		t.Errorf("want ErrShortWrite, got %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

func TestWriteEvent_WriterError(t *testing.T) {
	if err := WriteEvent(failingWriter{}, EventTypeKey, KeyA, 1); err == nil {
		t.Error("want writer error, got nil")
	}
}

// --- test helpers ---

func decodeAll(t *testing.T, raw []byte) []Event {
	t.Helper()
	if len(raw)%EventSize != 0 {
		t.Fatalf("buffer length %d not a multiple of EventSize=%d", len(raw), EventSize)
	}
	out := make([]Event, 0, len(raw)/EventSize)
	for i := 0; i < len(raw); i += EventSize {
		out = append(out, DecodeEvent(to24(raw[i:i+EventSize])))
	}
	return out
}

func to24(b []byte) [EventSize]byte {
	if len(b) < EventSize {
		panic("short buffer")
	}
	var out [EventSize]byte
	copy(out[:], b[:EventSize])
	return out
}

func compareEvents(t *testing.T, got, want []Event) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %+v, want %+v", i, got[i], want[i])
		}
	}
}
