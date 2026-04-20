// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package uinput

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Event types (linux/input-event-codes.h EV_*).
const (
	EventTypeSyn uint16 = 0x00
	EventTypeKey uint16 = 0x01
	EventTypeRel uint16 = 0x02
	EventTypeAbs uint16 = 0x03
)

// SynReport is the commit-event code for EV_SYN. Every batch of events MUST
// end with EV_SYN/SYN_REPORT or the kernel will not dispatch them.
const SynReport uint16 = 0

// Button / key codes — a minimal curated set covering the pointer + keyboard
// surface HelixQA cares about. Extend as needed; the full list is in
// linux/input-event-codes.h.
const (
	BtnLeft   uint16 = 0x110
	BtnRight  uint16 = 0x111
	BtnMiddle uint16 = 0x112
	BtnTouch  uint16 = 0x14a

	KeyEsc       uint16 = 1
	KeyEnter     uint16 = 28
	KeyLeftCtrl  uint16 = 29
	KeyLeftShift uint16 = 42
	KeyBackspace uint16 = 14
	KeyTab       uint16 = 15
	KeySpace     uint16 = 57
	KeyBack      uint16 = 158 // AC_BACK — Android TV back
	KeyHome      uint16 = 172 // HOMEPAGE

	KeyA uint16 = 30
	KeyB uint16 = 48
	KeyC uint16 = 46
	KeyD uint16 = 32
	KeyE uint16 = 18
	KeyF uint16 = 33
	KeyG uint16 = 34
	KeyH uint16 = 35
	KeyI uint16 = 23
	KeyJ uint16 = 36
	KeyK uint16 = 37
	KeyL uint16 = 38
	KeyM uint16 = 50
	KeyN uint16 = 49
	KeyO uint16 = 24
	KeyP uint16 = 25
	KeyQ uint16 = 16
	KeyR uint16 = 19
	KeyS uint16 = 31
	KeyT uint16 = 20
	KeyU uint16 = 22
	KeyV uint16 = 47
	KeyW uint16 = 17
	KeyX uint16 = 45
	KeyY uint16 = 21
	KeyZ uint16 = 44
)

// Relative axes.
const (
	RelX      uint16 = 0x00
	RelY      uint16 = 0x01
	RelWheel  uint16 = 0x08
	RelHWheel uint16 = 0x06
)

// Absolute axes.
const (
	AbsX uint16 = 0x00
	AbsY uint16 = 0x01
)

// KeyValue encodes the three values EV_KEY events use: 0=release, 1=press,
// 2=autorepeat. HelixQA only emits press/release explicitly.
type KeyValue int32

const (
	KeyRelease  KeyValue = 0
	KeyPress    KeyValue = 1
	KeyAutorepeat KeyValue = 2
)

// EventSize is the on-the-wire size of one input_event on 64-bit Linux.
const EventSize = 24

// ErrShortWrite is returned when the underlying Writer accepts fewer than
// EventSize bytes; the kernel requires all-or-nothing writes.
var ErrShortWrite = errors.New("uinput: writer accepted fewer than EventSize bytes")

// Event is the decoded form of a uinput input_event.
type Event struct {
	Type  uint16
	Code  uint16
	Value int32
}

// EncodeEvent produces an EventSize-byte buffer in the kernel's native
// little-endian on-the-wire layout. The time fields are zero; the kernel
// fills them on receipt.
func EncodeEvent(typ, code uint16, value int32) [EventSize]byte {
	var buf [EventSize]byte
	// buf[0:16] is time { tv_sec, tv_usec }; left zero so the kernel stamps it.
	binary.LittleEndian.PutUint16(buf[16:18], typ)
	binary.LittleEndian.PutUint16(buf[18:20], code)
	binary.LittleEndian.PutUint32(buf[20:24], uint32(value))
	return buf
}

// DecodeEvent is the inverse of EncodeEvent; time fields are ignored.
func DecodeEvent(buf [EventSize]byte) Event {
	return Event{
		Type:  binary.LittleEndian.Uint16(buf[16:18]),
		Code:  binary.LittleEndian.Uint16(buf[18:20]),
		Value: int32(binary.LittleEndian.Uint32(buf[20:24])),
	}
}

// WriteEvent writes a single event to w. Always writes exactly EventSize
// bytes; returns ErrShortWrite if the Writer accepts fewer.
func WriteEvent(w io.Writer, typ, code uint16, value int32) error {
	buf := EncodeEvent(typ, code, value)
	n, err := w.Write(buf[:])
	if err != nil {
		return fmt.Errorf("uinput: write: %w", err)
	}
	if n != EventSize {
		return ErrShortWrite
	}
	return nil
}

// WriteSync commits buffered events by emitting EV_SYN/SYN_REPORT.
func WriteSync(w io.Writer) error { return WriteEvent(w, EventTypeSyn, SynReport, 0) }

// WriteKey emits one EV_KEY event (press when pressed=true, release otherwise).
func WriteKey(w io.Writer, code uint16, pressed bool) error {
	v := KeyRelease
	if pressed {
		v = KeyPress
	}
	return WriteEvent(w, EventTypeKey, code, int32(v))
}

// WriteKeyTap emits press, sync, release, sync — the minimal "tap" sequence
// dispatched by the kernel as a single keystroke.
func WriteKeyTap(w io.Writer, code uint16) error {
	if err := WriteKey(w, code, true); err != nil {
		return err
	}
	if err := WriteSync(w); err != nil {
		return err
	}
	if err := WriteKey(w, code, false); err != nil {
		return err
	}
	return WriteSync(w)
}

// WriteClickAbs emits absolute-position pointer events followed by a left-button
// press/release. Requires that Config.EnableAbs was set and ABS_X / ABS_Y were
// declared at device creation.
func WriteClickAbs(w io.Writer, x, y int32) error {
	if err := WriteEvent(w, EventTypeAbs, AbsX, x); err != nil {
		return err
	}
	if err := WriteEvent(w, EventTypeAbs, AbsY, y); err != nil {
		return err
	}
	if err := WriteKey(w, BtnLeft, true); err != nil {
		return err
	}
	if err := WriteSync(w); err != nil {
		return err
	}
	if err := WriteKey(w, BtnLeft, false); err != nil {
		return err
	}
	return WriteSync(w)
}

// WriteMoveRel emits EV_REL deltas + SYN_REPORT.
func WriteMoveRel(w io.Writer, dx, dy int32) error {
	if dx != 0 {
		if err := WriteEvent(w, EventTypeRel, RelX, dx); err != nil {
			return err
		}
	}
	if dy != 0 {
		if err := WriteEvent(w, EventTypeRel, RelY, dy); err != nil {
			return err
		}
	}
	return WriteSync(w)
}

// WriteScroll emits a wheel event — positive ticks scroll up, negative down.
func WriteScroll(w io.Writer, ticks int32) error {
	if ticks == 0 {
		return nil
	}
	if err := WriteEvent(w, EventTypeRel, RelWheel, ticks); err != nil {
		return err
	}
	return WriteSync(w)
}
