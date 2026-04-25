// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// EIClient is HelixQA's input-emulation wire client. It wraps the
// connected *os.File returned by portal.ConnectToEIS and exposes
// high-level SendKey / SendPointerMotion / SendButton / SendScroll
// / SendTouch calls that dispatch binary-framed events to the EIS
// (Emulated Input Server) running under the active compositor.
//
// Wire format — HelixQA-local envelope. The real libei wire is
// flatbuffers-based; a fully wire-compatible implementation needs
// either the flatc-generated Go bindings (external dependency) or
// a hand-written flatbuffers parser (~400 LoC). Until that lands,
// this package ships a SIMPLE LENGTH-PREFIX BINARY FRAMING suitable
// for a paired HelixQA EIS shim:
//
//	[4B BE body_length]
//	[2B BE event_type]
//	[N  B  event payload, big-endian integers]
//
// The event_type enum lives in this file as EventTypeXxx constants
// (stable across the HelixQA + EIS shim boundary). The payload
// layout for each event_type is documented inline below.
//
// The protocol surface is intentionally TINY because HelixQA's
// executor layer (pkg/autonomous/executor.go) only needs:
//
//	key down   / key up     — SendKey(keycode, down)
//	pointer move (dx, dy)   — SendPointerMotion
//	button down / button up — SendButton
//	scroll (dx, dy)         — SendScroll
//	touch down/move/up      — SendTouch
//
// Richer libei features (capability negotiation, seat binding,
// multi-device mapping, relative vs absolute axis metadata) are
// OUT OF SCOPE for this scaffold; they land when the flatbuffers
// reference implementation is wired in.
//
// Safety: concurrent Send calls serialize through an internal
// writer lock. EI is write-only from the client side — the client
// does not read any replies from the EIS socket.
type EIClient struct {
	w   io.WriteCloser
	mu  sync.Mutex
	seq uint32 // reserved for future flatbuffers sequence numbering
}

// NewEIClient wraps an already-connected io.WriteCloser (typically
// the *os.File returned by portal.ConnectToEIS). The client takes
// ownership of conn — Close() closes it.
func NewEIClient(conn io.WriteCloser) *EIClient {
	return &EIClient{w: conn}
}

// EventType enumerates the HelixQA-local EI wire events.
type EventType uint16

const (
	EventKey            EventType = 0x0001
	EventPointerMotion  EventType = 0x0002
	EventPointerButton  EventType = 0x0003
	EventPointerScroll  EventType = 0x0004
	EventTouchDown      EventType = 0x0010
	EventTouchMotion    EventType = 0x0011
	EventTouchUp        EventType = 0x0012
)

// Sentinel errors.
var (
	ErrClosed         = errors.New("helixqa/libei: EIClient is closed")
	ErrNilConn        = errors.New("helixqa/libei: connection must not be nil")
	ErrInvalidCoords  = errors.New("helixqa/libei: coordinates must be non-negative")
	ErrInvalidKeycode = errors.New("helixqa/libei: keycode must be non-zero")
)

// ---------------------------------------------------------------------------
// Public event-send methods
// ---------------------------------------------------------------------------

// SendKey emits a key-down or key-up event. keycode is a Linux
// /usr/include/linux/input-event-codes.h value (KEY_ENTER = 28,
// KEY_A = 30, etc.) — exported so tests + executor code can use
// the canonical Linux keycode set.
//
// Payload: [4B keycode][1B down]
func (c *EIClient) SendKey(ctx context.Context, keycode uint32, down bool) error {
	if keycode == 0 {
		return ErrInvalidKeycode
	}
	payload := make([]byte, 5)
	binary.BigEndian.PutUint32(payload[0:4], keycode)
	if down {
		payload[4] = 1
	}
	return c.send(ctx, EventKey, payload)
}

// SendPointerMotion emits a RELATIVE pointer motion (dx, dy). The
// EIS interprets this as "move the pointer by dx pixels right,
// dy pixels down from its current position".
//
// Payload: [4B BE int32 dx][4B BE int32 dy]
func (c *EIClient) SendPointerMotion(ctx context.Context, dx, dy int32) error {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], uint32(dx))
	binary.BigEndian.PutUint32(payload[4:8], uint32(dy))
	return c.send(ctx, EventPointerMotion, payload)
}

// SendButton emits a pointer-button press or release. button is a
// BTN_ Linux code (BTN_LEFT = 0x110, BTN_RIGHT = 0x111, BTN_MIDDLE
// = 0x112).
//
// Payload: [4B button][1B down]
func (c *EIClient) SendButton(ctx context.Context, button uint32, down bool) error {
	payload := make([]byte, 5)
	binary.BigEndian.PutUint32(payload[0:4], button)
	if down {
		payload[4] = 1
	}
	return c.send(ctx, EventPointerButton, payload)
}

// SendScroll emits scroll axis-offset ticks (dx, dy) in the EI
// "natural units" (typically 1 = one discrete notch).
//
// Payload: [4B BE int32 dx][4B BE int32 dy]
func (c *EIClient) SendScroll(ctx context.Context, dx, dy int32) error {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], uint32(dx))
	binary.BigEndian.PutUint32(payload[4:8], uint32(dy))
	return c.send(ctx, EventPointerScroll, payload)
}

// SendTouch emits a touch event. slot is the EI touch slot (0..N-1
// for multi-touch); kind selects down / motion / up; (x, y) is the
// absolute pixel coordinate on the virtual screen.
//
// Payload: [4B slot][4B x][4B y]
func (c *EIClient) SendTouch(ctx context.Context, slot uint32, kind EventType, x, y int32) error {
	if x < 0 || y < 0 {
		return ErrInvalidCoords
	}
	if kind != EventTouchDown && kind != EventTouchMotion && kind != EventTouchUp {
		return fmt.Errorf("helixqa/libei: invalid touch event type 0x%04x", kind)
	}
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], slot)
	binary.BigEndian.PutUint32(payload[4:8], uint32(x))
	binary.BigEndian.PutUint32(payload[8:12], uint32(y))
	return c.send(ctx, kind, payload)
}

// Close closes the underlying connection. Safe to call multiple
// times (second+ calls are no-ops).
func (c *EIClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.w == nil {
		return nil
	}
	err := c.w.Close()
	c.w = nil
	return err
}

// ---------------------------------------------------------------------------
// Internal wire framing
// ---------------------------------------------------------------------------

// send serializes [4B body_length][2B event_type][payload] and
// writes the whole frame atomically. ctx cancellation is checked
// before the write — writes themselves are synchronous (EI is a
// stream socket; mid-write cancellation would leave a torn frame
// on the wire).
func (c *EIClient) send(ctx context.Context, et EventType, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.w == nil {
		return ErrClosed
	}
	bodyLen := uint32(2 + len(payload)) // 2B event_type + payload
	frame := make([]byte, 4+int(bodyLen))
	binary.BigEndian.PutUint32(frame[0:4], bodyLen)
	binary.BigEndian.PutUint16(frame[4:6], uint16(et))
	copy(frame[6:], payload)
	_, err := c.w.Write(frame)
	if err != nil {
		return fmt.Errorf("helixqa/libei: write: %w", err)
	}
	c.seq++
	return nil
}

// DecodeFrame parses one inbound EI frame (the HelixQA wire shape).
// Exposed for tests and for future bidirectional extensions. Never
// called by the client-side send path today.
func DecodeFrame(r io.Reader) (EventType, []byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	bodyLen := binary.BigEndian.Uint32(hdr[:])
	if bodyLen < 2 {
		return 0, nil, fmt.Errorf("helixqa/libei: body_length %d < 2", bodyLen)
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	et := EventType(binary.BigEndian.Uint16(body[0:2]))
	return et, body[2:], nil
}

// Stats returns the number of frames written since construction.
// Cheap diagnostic for QA dashboards.
func (c *EIClient) Stats() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq
}
