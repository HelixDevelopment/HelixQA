// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type closeableBuffer struct {
	bytes.Buffer
	closed bool
	err    error
}

func (b *closeableBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.Buffer.Write(p)
}

func (b *closeableBuffer) Close() error { b.closed = true; return nil }

// decodeBuf extracts the next (event_type, payload) from the
// closeableBuffer — wraps DecodeFrame for ergonomic test reads.
func decodeBuf(t *testing.T, b *closeableBuffer) (EventType, []byte) {
	t.Helper()
	et, payload, err := DecodeFrame(&b.Buffer)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	return et, payload
}

// ---------------------------------------------------------------------------
// SendKey
// ---------------------------------------------------------------------------

func TestSendKey_EmitsCorrectFrame(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	if err := c.SendKey(context.Background(), 28 /*KEY_ENTER*/, true); err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	et, payload := decodeBuf(t, buf)
	if et != EventKey {
		t.Fatalf("event type = 0x%04x, want EventKey", et)
	}
	if len(payload) != 5 {
		t.Fatalf("payload len = %d, want 5", len(payload))
	}
	if kc := binary.BigEndian.Uint32(payload[0:4]); kc != 28 {
		t.Errorf("keycode = %d, want 28", kc)
	}
	if payload[4] != 1 {
		t.Errorf("down byte = %d, want 1", payload[4])
	}
}

func TestSendKey_UpEvent(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendKey(context.Background(), 30 /*KEY_A*/, false)
	_, payload := decodeBuf(t, buf)
	if payload[4] != 0 {
		t.Fatalf("up byte = %d, want 0", payload[4])
	}
}

func TestSendKey_ZeroKeycodeError(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	if err := c.SendKey(context.Background(), 0, true); !errors.Is(err, ErrInvalidKeycode) {
		t.Fatalf("zero keycode: %v, want ErrInvalidKeycode", err)
	}
}

// ---------------------------------------------------------------------------
// SendPointerMotion
// ---------------------------------------------------------------------------

func TestSendPointerMotion_EmitsCorrectFrame(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendPointerMotion(context.Background(), -5, 10)
	et, payload := decodeBuf(t, buf)
	if et != EventPointerMotion {
		t.Fatalf("event type = 0x%04x, want EventPointerMotion", et)
	}
	if len(payload) != 8 {
		t.Fatalf("payload len = %d, want 8", len(payload))
	}
	dx := int32(binary.BigEndian.Uint32(payload[0:4]))
	dy := int32(binary.BigEndian.Uint32(payload[4:8]))
	if dx != -5 || dy != 10 {
		t.Fatalf("dx=%d dy=%d, want -5, 10", dx, dy)
	}
}

// ---------------------------------------------------------------------------
// SendButton
// ---------------------------------------------------------------------------

func TestSendButton_EmitsCorrectFrame(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendButton(context.Background(), 0x110 /*BTN_LEFT*/, true)
	et, payload := decodeBuf(t, buf)
	if et != EventPointerButton {
		t.Fatalf("event type = 0x%04x", et)
	}
	if btn := binary.BigEndian.Uint32(payload[0:4]); btn != 0x110 {
		t.Errorf("button = 0x%x, want 0x110", btn)
	}
	if payload[4] != 1 {
		t.Errorf("down = %d, want 1", payload[4])
	}
}

// ---------------------------------------------------------------------------
// SendScroll
// ---------------------------------------------------------------------------

func TestSendScroll_EmitsCorrectFrame(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendScroll(context.Background(), 0, -3)
	et, payload := decodeBuf(t, buf)
	if et != EventPointerScroll {
		t.Fatalf("event type = 0x%04x", et)
	}
	dx := int32(binary.BigEndian.Uint32(payload[0:4]))
	dy := int32(binary.BigEndian.Uint32(payload[4:8]))
	if dx != 0 || dy != -3 {
		t.Errorf("dx=%d dy=%d, want 0, -3", dx, dy)
	}
}

// ---------------------------------------------------------------------------
// SendTouch
// ---------------------------------------------------------------------------

func TestSendTouch_Down(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	if err := c.SendTouch(context.Background(), 2, EventTouchDown, 100, 200); err != nil {
		t.Fatalf("SendTouch: %v", err)
	}
	et, payload := decodeBuf(t, buf)
	if et != EventTouchDown {
		t.Fatalf("event type = 0x%04x", et)
	}
	if len(payload) != 12 {
		t.Fatalf("payload len = %d, want 12", len(payload))
	}
	slot := binary.BigEndian.Uint32(payload[0:4])
	x := int32(binary.BigEndian.Uint32(payload[4:8]))
	y := int32(binary.BigEndian.Uint32(payload[8:12]))
	if slot != 2 || x != 100 || y != 200 {
		t.Fatalf("slot=%d x=%d y=%d", slot, x, y)
	}
}

func TestSendTouch_MotionAndUp(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendTouch(context.Background(), 0, EventTouchMotion, 50, 50)
	_ = c.SendTouch(context.Background(), 0, EventTouchUp, 50, 50)
	et1, _ := decodeBuf(t, buf)
	et2, _ := decodeBuf(t, buf)
	if et1 != EventTouchMotion {
		t.Errorf("first event = 0x%04x, want EventTouchMotion", et1)
	}
	if et2 != EventTouchUp {
		t.Errorf("second event = 0x%04x, want EventTouchUp", et2)
	}
}

func TestSendTouch_NegativeCoordsError(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	if err := c.SendTouch(context.Background(), 0, EventTouchDown, -1, 0); !errors.Is(err, ErrInvalidCoords) {
		t.Fatalf("negative x: %v, want ErrInvalidCoords", err)
	}
	if err := c.SendTouch(context.Background(), 0, EventTouchDown, 0, -1); !errors.Is(err, ErrInvalidCoords) {
		t.Fatalf("negative y: %v, want ErrInvalidCoords", err)
	}
}

func TestSendTouch_InvalidKindError(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	err := c.SendTouch(context.Background(), 0, EventKey, 0, 0) // wrong kind
	if err == nil {
		t.Fatal("non-touch kind should fail")
	}
}

// ---------------------------------------------------------------------------
// Frame structure invariants
// ---------------------------------------------------------------------------

func TestFrame_HeaderBigEndianLength(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendKey(context.Background(), 28, true)
	raw := buf.Bytes()
	if len(raw) < 4 {
		t.Fatal("frame too short")
	}
	bodyLen := binary.BigEndian.Uint32(raw[0:4])
	if bodyLen != 7 /* 2B event_type + 5B key payload */ {
		t.Fatalf("bodyLen = %d, want 7", bodyLen)
	}
	if int(bodyLen) != len(raw)-4 {
		t.Fatalf("bodyLen %d doesn't match actual %d", bodyLen, len(raw)-4)
	}
}

func TestFrame_MultipleSendsConcatenate(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	_ = c.SendKey(context.Background(), 28, true)
	_ = c.SendKey(context.Background(), 28, false)
	_ = c.SendPointerMotion(context.Background(), 5, 5)

	// Decode all three in order.
	for i, wantET := range []EventType{EventKey, EventKey, EventPointerMotion} {
		et, _, err := DecodeFrame(&buf.Buffer)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if et != wantET {
			t.Errorf("frame %d event = 0x%04x, want 0x%04x", i, et, wantET)
		}
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if !buf.closed {
		t.Fatal("underlying buffer not closed")
	}
	// Second call is a no-op.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestSendAfterClose_Error(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	c.Close()
	if err := c.SendKey(context.Background(), 28, true); !errors.Is(err, ErrClosed) {
		t.Fatalf("post-Close send: %v, want ErrClosed", err)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation + write errors
// ---------------------------------------------------------------------------

func TestSend_ContextCanceledBeforeWrite(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.SendKey(ctx, 28, true); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestSend_WriteErrorPropagates(t *testing.T) {
	buf := &closeableBuffer{err: errors.New("disk full")}
	c := NewEIClient(buf)
	err := c.SendKey(context.Background(), 28, true)
	if err == nil || err.Error() == "" {
		t.Fatalf("write error not propagated")
	}
}

// ---------------------------------------------------------------------------
// DecodeFrame unit tests
// ---------------------------------------------------------------------------

func TestDecodeFrame_ReturnsErrorOnShortHeader(t *testing.T) {
	r := bytes.NewReader([]byte{0x00, 0x00}) // only 2 bytes; need 4
	_, _, err := DecodeFrame(r)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short header: %v, want ErrUnexpectedEOF", err)
	}
}

func TestDecodeFrame_ReturnsErrorOnBodyTooShort(t *testing.T) {
	// Claims 10-byte body but only 3 bytes follow.
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(10))
	buf.Write([]byte{0x00, 0x01, 0x02})
	_, _, err := DecodeFrame(&buf)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short body: %v, want ErrUnexpectedEOF", err)
	}
}

func TestDecodeFrame_RejectsBodyLengthBelowHeaderSize(t *testing.T) {
	// body_length=1 but we need at least 2B for event_type.
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(1))
	buf.WriteByte(0x00)
	_, _, err := DecodeFrame(&buf)
	if err == nil {
		t.Fatal("bodyLen=1 should error")
	}
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func TestStats_CountsWrittenFrames(t *testing.T) {
	c := NewEIClient(&closeableBuffer{})
	if c.Stats() != 0 {
		t.Fatal("initial stats != 0")
	}
	_ = c.SendKey(context.Background(), 28, true)
	_ = c.SendKey(context.Background(), 28, false)
	if c.Stats() != 2 {
		t.Fatalf("stats = %d, want 2", c.Stats())
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestSend_ConcurrentWritesSerializeCleanly(t *testing.T) {
	buf := &closeableBuffer{}
	c := NewEIClient(buf)
	const N = 50
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = c.SendKey(context.Background(), 28, true)
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	// Every frame should be exactly 11 bytes (4B header + 2B event
	// type + 5B payload) — mutex prevents interleaved writes.
	if len(buf.Bytes()) != N*11 {
		t.Fatalf("expected %d bytes, got %d", N*11, len(buf.Bytes()))
	}
	// Decode all N frames cleanly.
	for i := 0; i < N; i++ {
		et, _, err := DecodeFrame(&buf.Buffer)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if et != EventKey {
			t.Fatalf("frame %d event = 0x%04x", i, et)
		}
	}
}
