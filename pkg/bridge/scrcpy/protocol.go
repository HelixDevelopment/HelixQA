// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// --- Control-message types (client → server) ---

// ControlType identifies the kind of control message. Values match the scrcpy
// v3 enum; do not reorder.
type ControlType uint8

const (
	// CtrlInjectKeycode — AKeyEvent (down/up) with Android keycode.
	CtrlInjectKeycode ControlType = 0
	// CtrlInjectText — UTF-8 text injection.
	CtrlInjectText ControlType = 1
	// CtrlInjectTouchEvent — MotionEvent with pointer, pressure, action_button, buttons.
	CtrlInjectTouchEvent ControlType = 2
	// CtrlInjectScrollEvent — wheel event, hscroll+vscroll halves.
	CtrlInjectScrollEvent ControlType = 3
	// CtrlBackOrScreenOn — virtual back press that also wakes the screen.
	CtrlBackOrScreenOn ControlType = 4
	// CtrlExpandNotificationPanel — v3.
	CtrlExpandNotificationPanel ControlType = 5
	// CtrlExpandSettingsPanel — v3.
	CtrlExpandSettingsPanel ControlType = 6
	// CtrlCollapsePanels — v3.
	CtrlCollapsePanels ControlType = 7
	// CtrlGetClipboard — request server clipboard contents.
	CtrlGetClipboard ControlType = 8
	// CtrlSetClipboard — set device clipboard.
	CtrlSetClipboard ControlType = 9
	// CtrlSetScreenPowerMode — ON/OFF/NORMAL.
	CtrlSetScreenPowerMode ControlType = 10
	// CtrlRotateDevice — toggle device rotation.
	CtrlRotateDevice ControlType = 11
	// CtrlUhidCreate — create a virtual HID device.
	CtrlUhidCreate ControlType = 12
	// CtrlUhidInput — send HID report.
	CtrlUhidInput ControlType = 13
	// CtrlUhidDestroy — destroy virtual HID device.
	CtrlUhidDestroy ControlType = 14
	// CtrlOpenHardKeyboardSettings — Android Settings intent.
	CtrlOpenHardKeyboardSettings ControlType = 15
	// CtrlStartApp — launch an app by package.
	CtrlStartApp ControlType = 16
	// CtrlResetVideo — request an IDR frame; forces reset of decoder state.
	CtrlResetVideo ControlType = 17
)

// String gives a stable token for each ControlType; used in logs and test
// diagnostics. Unknown values render as "ctrl(N)".
func (c ControlType) String() string {
	switch c {
	case CtrlInjectKeycode:
		return "inject-keycode"
	case CtrlInjectText:
		return "inject-text"
	case CtrlInjectTouchEvent:
		return "inject-touch"
	case CtrlInjectScrollEvent:
		return "inject-scroll"
	case CtrlBackOrScreenOn:
		return "back-or-screen-on"
	case CtrlExpandNotificationPanel:
		return "expand-notif"
	case CtrlExpandSettingsPanel:
		return "expand-settings"
	case CtrlCollapsePanels:
		return "collapse-panels"
	case CtrlGetClipboard:
		return "get-clipboard"
	case CtrlSetClipboard:
		return "set-clipboard"
	case CtrlSetScreenPowerMode:
		return "set-screen-power"
	case CtrlRotateDevice:
		return "rotate-device"
	case CtrlUhidCreate:
		return "uhid-create"
	case CtrlUhidInput:
		return "uhid-input"
	case CtrlUhidDestroy:
		return "uhid-destroy"
	case CtrlOpenHardKeyboardSettings:
		return "open-hardkb-settings"
	case CtrlStartApp:
		return "start-app"
	case CtrlResetVideo:
		return "reset-video"
	default:
		return fmt.Sprintf("ctrl(%d)", uint8(c))
	}
}

// KeyAction maps Android AKeyEvent actions.
type KeyAction uint8

const (
	KeyActionDown KeyAction = 0
	KeyActionUp   KeyAction = 1
)

// TouchAction maps Android MotionEvent actions.
type TouchAction uint8

const (
	TouchActionDown       TouchAction = 0
	TouchActionUp         TouchAction = 1
	TouchActionMove       TouchAction = 2
	TouchActionCancel     TouchAction = 3
	TouchActionOutside    TouchAction = 4
	TouchActionPointerDown TouchAction = 5
	TouchActionPointerUp   TouchAction = 6
	TouchActionHoverMove   TouchAction = 7
	TouchActionScroll      TouchAction = 8
	TouchActionHoverEnter  TouchAction = 9
	TouchActionHoverExit   TouchAction = 10
	TouchActionButtonPress TouchAction = 11
	TouchActionButtonReleased TouchAction = 12
)

// ScreenPowerMode values for CtrlSetScreenPowerMode.
type ScreenPowerMode uint8

const (
	ScreenPowerOff    ScreenPowerMode = 0
	ScreenPowerNormal ScreenPowerMode = 2
)

// --- ControlMessage: serialised into the control socket stream ---

// ControlMessage is any message the client sends to the server. Implementations
// cover the subset of v3 messages HelixQA needs today; extend here when we add
// HID / start-app support.
type ControlMessage interface {
	Type() ControlType
	// MarshalBinary writes the message body (post-type byte) to w.
	MarshalBinary(w io.Writer) error
}

// WriteControlMessage prepends the type byte and writes the body to w.
// Safe for concurrent use only when guarded externally.
func WriteControlMessage(w io.Writer, m ControlMessage) error {
	if _, err := w.Write([]byte{byte(m.Type())}); err != nil {
		return fmt.Errorf("scrcpy: write type: %w", err)
	}
	if err := m.MarshalBinary(w); err != nil {
		return fmt.Errorf("scrcpy: marshal %s: %w", m.Type(), err)
	}
	return nil
}

// --- Keycode (type 0) ---

// InjectKeycode sends an Android KeyEvent.
type InjectKeycode struct {
	Action     KeyAction
	Keycode    int32 // AndroidX KeyEvent keycode
	Repeat     uint32
	MetaState  uint32 // AndroidX KeyEvent meta-state mask
}

func (InjectKeycode) Type() ControlType { return CtrlInjectKeycode }

func (m InjectKeycode) MarshalBinary(w io.Writer) error {
	var buf [13]byte
	buf[0] = byte(m.Action)
	binary.BigEndian.PutUint32(buf[1:5], uint32(m.Keycode))
	binary.BigEndian.PutUint32(buf[5:9], m.Repeat)
	binary.BigEndian.PutUint32(buf[9:13], m.MetaState)
	_, err := w.Write(buf[:])
	return err
}

// --- Text (type 1) ---

// ErrTextTooLong is returned when InjectText.Text exceeds the v3 wire limit.
var ErrTextTooLong = errors.New("scrcpy: inject-text exceeds 300 bytes")

// MaxTextLen is the scrcpy v3 cap for injected-text size in bytes.
const MaxTextLen = 300

// InjectText sends a UTF-8 text injection; maximum 300 bytes.
type InjectText struct {
	Text string
}

func (InjectText) Type() ControlType { return CtrlInjectText }

func (m InjectText) MarshalBinary(w io.Writer) error {
	body := []byte(m.Text)
	if len(body) > MaxTextLen {
		return fmt.Errorf("%w: len=%d", ErrTextTooLong, len(body))
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// --- Touch (type 2) ---

// InjectTouchEvent is the v3 MotionEvent message. Includes the v3 additions
// ActionButton (uint32) and Buttons (uint32) that OpenClawing3 incorrectly
// omitted; see OpenClawing4-Audit.md §D.3 / FIX-OC3-011.
type InjectTouchEvent struct {
	Action       TouchAction
	PointerID    uint64
	X, Y         int32
	ScreenW      uint16
	ScreenH      uint16
	Pressure     uint16 // 0..65535 — multiply by 1/65535 for [0,1]
	ActionButton uint32
	Buttons      uint32
}

func (InjectTouchEvent) Type() ControlType { return CtrlInjectTouchEvent }

func (m InjectTouchEvent) MarshalBinary(w io.Writer) error {
	var buf [31]byte // 1 + 8 + 4 + 4 + 2 + 2 + 2 + 4 + 4 = 31
	buf[0] = byte(m.Action)
	binary.BigEndian.PutUint64(buf[1:9], m.PointerID)
	binary.BigEndian.PutUint32(buf[9:13], uint32(m.X))
	binary.BigEndian.PutUint32(buf[13:17], uint32(m.Y))
	binary.BigEndian.PutUint16(buf[17:19], m.ScreenW)
	binary.BigEndian.PutUint16(buf[19:21], m.ScreenH)
	binary.BigEndian.PutUint16(buf[21:23], m.Pressure)
	binary.BigEndian.PutUint32(buf[23:27], m.ActionButton)
	binary.BigEndian.PutUint32(buf[27:31], m.Buttons)
	_, err := w.Write(buf[:])
	return err
}

// --- Scroll (type 3) ---

// InjectScrollEvent mirrors InjectTouchEvent layout for the initial fields and
// adds HScroll / VScroll halves plus a button mask.
type InjectScrollEvent struct {
	X, Y      int32
	ScreenW   uint16
	ScreenH   uint16
	HScroll   int16
	VScroll   int16
	Buttons   uint32
}

func (InjectScrollEvent) Type() ControlType { return CtrlInjectScrollEvent }

func (m InjectScrollEvent) MarshalBinary(w io.Writer) error {
	var buf [20]byte // 4 + 4 + 2 + 2 + 2 + 2 + 4 = 20
	binary.BigEndian.PutUint32(buf[0:4], uint32(m.X))
	binary.BigEndian.PutUint32(buf[4:8], uint32(m.Y))
	binary.BigEndian.PutUint16(buf[8:10], m.ScreenW)
	binary.BigEndian.PutUint16(buf[10:12], m.ScreenH)
	binary.BigEndian.PutUint16(buf[12:14], uint16(m.HScroll))
	binary.BigEndian.PutUint16(buf[14:16], uint16(m.VScroll))
	binary.BigEndian.PutUint32(buf[16:20], m.Buttons)
	_, err := w.Write(buf[:])
	return err
}

// --- Zero-body control messages ---

// zeroBody is a helper for messages whose body is empty (just the type byte).
type zeroBody struct{ t ControlType }

func (z zeroBody) Type() ControlType                  { return z.t }
func (zeroBody) MarshalBinary(w io.Writer) error      { return nil }

// BackOrScreenOn — CtrlBackOrScreenOn.
func BackOrScreenOn() ControlMessage { return zeroBody{CtrlBackOrScreenOn} }

// ExpandNotificationPanel — CtrlExpandNotificationPanel.
func ExpandNotificationPanel() ControlMessage { return zeroBody{CtrlExpandNotificationPanel} }

// ExpandSettingsPanel — CtrlExpandSettingsPanel.
func ExpandSettingsPanel() ControlMessage { return zeroBody{CtrlExpandSettingsPanel} }

// CollapsePanels — CtrlCollapsePanels.
func CollapsePanels() ControlMessage { return zeroBody{CtrlCollapsePanels} }

// RotateDevice — CtrlRotateDevice.
func RotateDevice() ControlMessage { return zeroBody{CtrlRotateDevice} }

// OpenHardKeyboardSettings — CtrlOpenHardKeyboardSettings.
func OpenHardKeyboardSettings() ControlMessage { return zeroBody{CtrlOpenHardKeyboardSettings} }

// ResetVideo — CtrlResetVideo; forces the server to emit a new IDR frame.
func ResetVideo() ControlMessage { return zeroBody{CtrlResetVideo} }

// --- SetScreenPowerMode (type 10) ---

type SetScreenPowerMode struct{ Mode ScreenPowerMode }

func (SetScreenPowerMode) Type() ControlType { return CtrlSetScreenPowerMode }

func (m SetScreenPowerMode) MarshalBinary(w io.Writer) error {
	_, err := w.Write([]byte{byte(m.Mode)})
	return err
}

// --- GetClipboard (type 8) ---

// ClipboardCopyKey is the v3 enum for how server-side clipboard read is done.
type ClipboardCopyKey uint8

const (
	ClipboardCopyNone ClipboardCopyKey = 0
	ClipboardCopyCopy ClipboardCopyKey = 1
	ClipboardCopyCut  ClipboardCopyKey = 2
)

type GetClipboard struct{ CopyKey ClipboardCopyKey }

func (GetClipboard) Type() ControlType { return CtrlGetClipboard }

func (m GetClipboard) MarshalBinary(w io.Writer) error {
	_, err := w.Write([]byte{byte(m.CopyKey)})
	return err
}

// --- SetClipboard (type 9) ---

// SetClipboard sets the device clipboard contents.
type SetClipboard struct {
	Sequence uint64 // monotonic counter used to correlate SetClipboard ↔ AckClipboard
	Paste    bool
	Text     string
}

func (SetClipboard) Type() ControlType { return CtrlSetClipboard }

func (m SetClipboard) MarshalBinary(w io.Writer) error {
	body := []byte(m.Text)
	if len(body) > MaxTextLen {
		return fmt.Errorf("%w: len=%d", ErrTextTooLong, len(body))
	}
	var buf [13]byte // 8 + 1 + 4
	binary.BigEndian.PutUint64(buf[0:8], m.Sequence)
	if m.Paste {
		buf[8] = 1
	}
	binary.BigEndian.PutUint32(buf[9:13], uint32(len(body)))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// --- Device messages (server → client) ---

// DeviceMessageType identifies the message kind.
type DeviceMessageType uint8

const (
	DevClipboard     DeviceMessageType = 0
	DevAckClipboard  DeviceMessageType = 1
	DevUhidOutput    DeviceMessageType = 2
)

// DeviceMessage is the decoded form of any server → client message.
type DeviceMessage struct {
	Type     DeviceMessageType
	Text     string // valid when Type == DevClipboard
	Sequence uint64 // valid when Type == DevAckClipboard
	UhidID   uint16 // valid when Type == DevUhidOutput
	UhidData []byte // valid when Type == DevUhidOutput
}

// ErrProtocol is returned for any unexpected structure in the server stream.
var ErrProtocol = errors.New("scrcpy: protocol error")

// MaxDeviceTextBytes caps what we will read for clipboard / UHID payloads from
// the device. Values above this suggest a framing bug and are rejected.
const MaxDeviceTextBytes = 1 << 20 // 1 MiB

// ReadDeviceMessage decodes one server → client message from r.
func ReadDeviceMessage(r io.Reader) (DeviceMessage, error) {
	var typ [1]byte
	if _, err := io.ReadFull(r, typ[:]); err != nil {
		return DeviceMessage{}, err
	}
	switch DeviceMessageType(typ[0]) {
	case DevClipboard:
		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return DeviceMessage{}, fmt.Errorf("%w: clipboard length: %w", ErrProtocol, err)
		}
		n := binary.BigEndian.Uint32(hdr[:])
		if n > MaxDeviceTextBytes {
			return DeviceMessage{}, fmt.Errorf("%w: clipboard payload %d > %d", ErrProtocol, n, MaxDeviceTextBytes)
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return DeviceMessage{}, fmt.Errorf("%w: clipboard body: %w", ErrProtocol, err)
		}
		return DeviceMessage{Type: DevClipboard, Text: string(body)}, nil
	case DevAckClipboard:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return DeviceMessage{}, fmt.Errorf("%w: ack-clipboard seq: %w", ErrProtocol, err)
		}
		return DeviceMessage{Type: DevAckClipboard, Sequence: binary.BigEndian.Uint64(buf[:])}, nil
	case DevUhidOutput:
		var hdr [2 + 2]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return DeviceMessage{}, fmt.Errorf("%w: uhid header: %w", ErrProtocol, err)
		}
		id := binary.BigEndian.Uint16(hdr[0:2])
		size := binary.BigEndian.Uint16(hdr[2:4])
		if uint32(size) > MaxDeviceTextBytes {
			return DeviceMessage{}, fmt.Errorf("%w: uhid size %d > %d", ErrProtocol, size, MaxDeviceTextBytes)
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return DeviceMessage{}, fmt.Errorf("%w: uhid body: %w", ErrProtocol, err)
		}
		return DeviceMessage{Type: DevUhidOutput, UhidID: id, UhidData: body}, nil
	default:
		return DeviceMessage{}, fmt.Errorf("%w: unknown device-message type=%d", ErrProtocol, typ[0])
	}
}

// --- Video packet ---

// VideoPacket is one H.264 packet emitted on the video socket. The server
// prefixes every packet with a 12-byte header: {pts_us uint64, len uint32}.
// Flag bits are carried in the high bits of the length field per v3.
type VideoPacket struct {
	PTSMicros   int64  // -1 for config packets (SPS/PPS)
	IsConfig    bool   // true iff the high bit of the length field is set
	IsKeyframe  bool   // true iff the second-highest bit is set (v3)
	Payload     []byte // raw H.264 bytes (NALs; Annex-B start codes included)
}

// videoPacketFlagConfig is the high bit of the length field marking a
// config (SPS/PPS/VPS) packet.
const videoPacketFlagConfig uint32 = 1 << 31

// videoPacketFlagKey is the second-highest bit marking an IDR / keyframe.
const videoPacketFlagKey uint32 = 1 << 30

// MaxVideoPacketBytes caps video packet size as a defence against framing bugs.
// Real 1080p H.264 IDR frames rarely exceed 1 MiB; 16 MiB is a very generous
// ceiling that still bounds memory growth on malformed streams.
const MaxVideoPacketBytes = 16 * 1024 * 1024

// ErrShortVideoPacket is returned when the header advertises a zero-byte body.
var ErrShortVideoPacket = errors.New("scrcpy: video packet body length is zero")

// ReadVideoPacket decodes one packet from r.
func ReadVideoPacket(r io.Reader) (VideoPacket, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return VideoPacket{}, err
	}
	raw := binary.BigEndian.Uint64(hdr[0:8])
	lenWithFlags := binary.BigEndian.Uint32(hdr[8:12])
	pkt := VideoPacket{}
	// PTS field: server sends pts_us; config packets use -1 (all-ones uint64).
	if raw == ^uint64(0) {
		pkt.PTSMicros = -1
	} else {
		pkt.PTSMicros = int64(raw)
	}
	pkt.IsConfig = lenWithFlags&videoPacketFlagConfig != 0
	pkt.IsKeyframe = lenWithFlags&videoPacketFlagKey != 0
	size := lenWithFlags &^ (videoPacketFlagConfig | videoPacketFlagKey)
	if size == 0 {
		return VideoPacket{}, ErrShortVideoPacket
	}
	if size > MaxVideoPacketBytes {
		return VideoPacket{}, fmt.Errorf("%w: video packet %d > %d", ErrProtocol, size, MaxVideoPacketBytes)
	}
	pkt.Payload = make([]byte, size)
	if _, err := io.ReadFull(r, pkt.Payload); err != nil {
		return VideoPacket{}, fmt.Errorf("%w: video body: %w", ErrProtocol, err)
	}
	return pkt, nil
}

// --- Audio packet ---

// AudioPacket mirrors VideoPacket with a narrower flag set (v3: only config flag).
type AudioPacket struct {
	PTSMicros int64
	IsConfig  bool
	Payload   []byte
}

// audioPacketFlagConfig — high bit of the length field on an audio packet.
const audioPacketFlagConfig uint32 = 1 << 31

// ReadAudioPacket decodes one packet from r.
func ReadAudioPacket(r io.Reader) (AudioPacket, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return AudioPacket{}, err
	}
	raw := binary.BigEndian.Uint64(hdr[0:8])
	lenWithFlags := binary.BigEndian.Uint32(hdr[8:12])
	pkt := AudioPacket{}
	if raw == ^uint64(0) {
		pkt.PTSMicros = -1
	} else {
		pkt.PTSMicros = int64(raw)
	}
	pkt.IsConfig = lenWithFlags&audioPacketFlagConfig != 0
	size := lenWithFlags &^ audioPacketFlagConfig
	if size == 0 {
		return AudioPacket{}, ErrShortVideoPacket
	}
	if size > MaxVideoPacketBytes {
		return AudioPacket{}, fmt.Errorf("%w: audio packet %d > %d", ErrProtocol, size, MaxVideoPacketBytes)
	}
	pkt.Payload = make([]byte, size)
	if _, err := io.ReadFull(r, pkt.Payload); err != nil {
		return AudioPacket{}, fmt.Errorf("%w: audio body: %w", ErrProtocol, err)
	}
	return pkt, nil
}
