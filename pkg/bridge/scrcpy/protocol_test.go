// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

// --- ControlType.String ---

func TestControlType_String(t *testing.T) {
	cases := map[ControlType]string{
		CtrlInjectKeycode:            "inject-keycode",
		CtrlInjectText:               "inject-text",
		CtrlInjectTouchEvent:         "inject-touch",
		CtrlInjectScrollEvent:        "inject-scroll",
		CtrlBackOrScreenOn:           "back-or-screen-on",
		CtrlExpandNotificationPanel:  "expand-notif",
		CtrlExpandSettingsPanel:      "expand-settings",
		CtrlCollapsePanels:           "collapse-panels",
		CtrlGetClipboard:             "get-clipboard",
		CtrlSetClipboard:             "set-clipboard",
		CtrlSetScreenPowerMode:       "set-screen-power",
		CtrlRotateDevice:             "rotate-device",
		CtrlUhidCreate:               "uhid-create",
		CtrlUhidInput:                "uhid-input",
		CtrlUhidDestroy:              "uhid-destroy",
		CtrlOpenHardKeyboardSettings: "open-hardkb-settings",
		CtrlStartApp:                 "start-app",
		CtrlResetVideo:               "reset-video",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("ControlType(%d).String() = %q, want %q", typ, got, want)
		}
	}
	if got := ControlType(250).String(); got != "ctrl(250)" {
		t.Errorf("unknown ControlType: got %q", got)
	}
}

// --- InjectKeycode ---

func TestInjectKeycode_Marshal(t *testing.T) {
	var buf bytes.Buffer
	msg := InjectKeycode{Action: KeyActionDown, Keycode: 0x42, Repeat: 7, MetaState: 0x1000}
	if err := WriteControlMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	// expected: type(0) action(0) keycode(4) repeat(4) metaState(4) = 14 bytes
	if len(got) != 14 {
		t.Fatalf("len = %d, want 14", len(got))
	}
	if got[0] != byte(CtrlInjectKeycode) {
		t.Errorf("type byte = %d, want %d", got[0], CtrlInjectKeycode)
	}
	if got[1] != byte(KeyActionDown) {
		t.Errorf("action = %d, want %d", got[1], KeyActionDown)
	}
	if kc := binary.BigEndian.Uint32(got[2:6]); kc != 0x42 {
		t.Errorf("keycode = %d", kc)
	}
	if rp := binary.BigEndian.Uint32(got[6:10]); rp != 7 {
		t.Errorf("repeat = %d", rp)
	}
	if ms := binary.BigEndian.Uint32(got[10:14]); ms != 0x1000 {
		t.Errorf("metaState = %d", ms)
	}
}

// --- InjectText ---

func TestInjectText_Marshal(t *testing.T) {
	var buf bytes.Buffer
	msg := InjectText{Text: "Hello"}
	if err := WriteControlMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	// type(1) + length(4) + body(5) = 10
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
	if got[0] != byte(CtrlInjectText) {
		t.Errorf("type byte = %d", got[0])
	}
	if n := binary.BigEndian.Uint32(got[1:5]); n != 5 {
		t.Errorf("length = %d, want 5", n)
	}
	if string(got[5:]) != "Hello" {
		t.Errorf("body = %q", string(got[5:]))
	}
}

func TestInjectText_TooLong(t *testing.T) {
	var buf bytes.Buffer
	msg := InjectText{Text: strings.Repeat("x", MaxTextLen+1)}
	err := WriteControlMessage(&buf, msg)
	if !errors.Is(err, ErrTextTooLong) {
		t.Errorf("want ErrTextTooLong, got %v", err)
	}
}

// --- InjectTouchEvent (this is the message OpenClawing3 got wrong) ---

func TestInjectTouchEvent_Marshal_v3Fields(t *testing.T) {
	var buf bytes.Buffer
	msg := InjectTouchEvent{
		Action: TouchActionDown, PointerID: 0xFFFFFFFFFFFFFFFF,
		X: 123, Y: 456, ScreenW: 1080, ScreenH: 1920, Pressure: 65535,
		ActionButton: 0xAAAAAAAA, Buttons: 0xBBBBBBBB,
	}
	if err := WriteControlMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	// type(1) + 31 = 32 bytes; v1.x used 28. Our wire format MUST include the
	// action_button (uint32) and buttons (uint32) additions from v3 per
	// OpenClawing4.md §11.3 FIX-OC3-011 — 4 + 4 = 8 extra bytes.
	if len(got) != 32 {
		t.Fatalf("len = %d, want 32 (scrcpy v3 includes action_button + buttons)", len(got))
	}
	if got[0] != byte(CtrlInjectTouchEvent) {
		t.Errorf("type = %d", got[0])
	}
	if got[1] != byte(TouchActionDown) {
		t.Errorf("action = %d", got[1])
	}
	if pid := binary.BigEndian.Uint64(got[2:10]); pid != 0xFFFFFFFFFFFFFFFF {
		t.Errorf("pointerID = %x", pid)
	}
	if x := int32(binary.BigEndian.Uint32(got[10:14])); x != 123 {
		t.Errorf("x = %d", x)
	}
	if y := int32(binary.BigEndian.Uint32(got[14:18])); y != 456 {
		t.Errorf("y = %d", y)
	}
	if w := binary.BigEndian.Uint16(got[18:20]); w != 1080 {
		t.Errorf("screenW = %d", w)
	}
	if h := binary.BigEndian.Uint16(got[20:22]); h != 1920 {
		t.Errorf("screenH = %d", h)
	}
	if p := binary.BigEndian.Uint16(got[22:24]); p != 65535 {
		t.Errorf("pressure = %d", p)
	}
	if ab := binary.BigEndian.Uint32(got[24:28]); ab != 0xAAAAAAAA {
		t.Errorf("actionButton = %x", ab)
	}
	if b := binary.BigEndian.Uint32(got[28:32]); b != 0xBBBBBBBB {
		t.Errorf("buttons = %x", b)
	}
}

// --- InjectScrollEvent ---

func TestInjectScrollEvent_Marshal(t *testing.T) {
	var buf bytes.Buffer
	msg := InjectScrollEvent{X: 100, Y: 200, ScreenW: 720, ScreenH: 1280, HScroll: -1, VScroll: 2, Buttons: 0xCAFEBABE}
	if err := WriteControlMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	// type(1) + 20 = 21
	if len(got) != 21 {
		t.Fatalf("len = %d, want 21", len(got))
	}
	if int32(binary.BigEndian.Uint32(got[1:5])) != 100 {
		t.Errorf("x wrong")
	}
	if int16(binary.BigEndian.Uint16(got[13:15])) != -1 {
		t.Errorf("hscroll wrong")
	}
	if int16(binary.BigEndian.Uint16(got[15:17])) != 2 {
		t.Errorf("vscroll wrong")
	}
	if binary.BigEndian.Uint32(got[17:21]) != 0xCAFEBABE {
		t.Errorf("buttons wrong")
	}
}

// --- Zero-body messages ---

func TestZeroBodyMessages(t *testing.T) {
	cases := map[ControlType]ControlMessage{
		CtrlBackOrScreenOn:           BackOrScreenOn(),
		CtrlExpandNotificationPanel:  ExpandNotificationPanel(),
		CtrlExpandSettingsPanel:      ExpandSettingsPanel(),
		CtrlCollapsePanels:           CollapsePanels(),
		CtrlRotateDevice:             RotateDevice(),
		CtrlOpenHardKeyboardSettings: OpenHardKeyboardSettings(),
		CtrlResetVideo:               ResetVideo(),
	}
	for want, msg := range cases {
		var buf bytes.Buffer
		if err := WriteControlMessage(&buf, msg); err != nil {
			t.Fatalf("%s: %v", want, err)
		}
		if got := buf.Bytes(); len(got) != 1 || got[0] != byte(want) {
			t.Errorf("%s: got %v", want, got)
		}
	}
}

// --- SetScreenPowerMode ---

func TestSetScreenPowerMode(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteControlMessage(&buf, SetScreenPowerMode{Mode: ScreenPowerOff}); err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); len(got) != 2 || got[0] != byte(CtrlSetScreenPowerMode) || got[1] != byte(ScreenPowerOff) {
		t.Errorf("got %v", got)
	}
}

// --- Clipboard ---

func TestGetClipboard(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteControlMessage(&buf, GetClipboard{CopyKey: ClipboardCopyCut}); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if len(got) != 2 || got[0] != byte(CtrlGetClipboard) || got[1] != byte(ClipboardCopyCut) {
		t.Errorf("got %v", got)
	}
}

func TestSetClipboard(t *testing.T) {
	var buf bytes.Buffer
	msg := SetClipboard{Sequence: 0xDEADBEEF, Paste: true, Text: "paste me"}
	if err := WriteControlMessage(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	// type(1) + seq(8) + paste(1) + len(4) + body(8) = 22
	if len(got) != 22 {
		t.Fatalf("len = %d, want 22", len(got))
	}
	if binary.BigEndian.Uint64(got[1:9]) != 0xDEADBEEF {
		t.Errorf("sequence wrong")
	}
	if got[9] != 1 {
		t.Errorf("paste bit not set")
	}
	if binary.BigEndian.Uint32(got[10:14]) != 8 {
		t.Errorf("text length wrong")
	}
	if string(got[14:]) != "paste me" {
		t.Errorf("body = %q", string(got[14:]))
	}
}

func TestSetClipboard_TooLong(t *testing.T) {
	var buf bytes.Buffer
	msg := SetClipboard{Text: strings.Repeat("a", MaxTextLen+1)}
	err := WriteControlMessage(&buf, msg)
	if !errors.Is(err, ErrTextTooLong) {
		t.Errorf("want ErrTextTooLong, got %v", err)
	}
}

// --- Device messages ---

func TestReadDeviceMessage_Clipboard(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(DevClipboard))
	text := "copied text"
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(text)))
	buf.Write(hdr[:])
	buf.WriteString(text)
	msg, err := ReadDeviceMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DevClipboard || msg.Text != text {
		t.Errorf("got %+v", msg)
	}
}

func TestReadDeviceMessage_AckClipboard(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(DevAckClipboard))
	var seq [8]byte
	binary.BigEndian.PutUint64(seq[:], 0xDEADBEEF)
	buf.Write(seq[:])
	msg, err := ReadDeviceMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DevAckClipboard || msg.Sequence != 0xDEADBEEF {
		t.Errorf("got %+v", msg)
	}
}

func TestReadDeviceMessage_UhidOutput(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(DevUhidOutput))
	var hdr [4]byte
	binary.BigEndian.PutUint16(hdr[0:2], 42)
	binary.BigEndian.PutUint16(hdr[2:4], 3)
	buf.Write(hdr[:])
	buf.Write([]byte{1, 2, 3})
	msg, err := ReadDeviceMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DevUhidOutput || msg.UhidID != 42 || !bytes.Equal(msg.UhidData, []byte{1, 2, 3}) {
		t.Errorf("got %+v", msg)
	}
}

func TestReadDeviceMessage_UnknownType(t *testing.T) {
	_, err := ReadDeviceMessage(bytes.NewReader([]byte{0xFF}))
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("want ErrProtocol, got %v", err)
	}
}

func TestReadDeviceMessage_EOF(t *testing.T) {
	_, err := ReadDeviceMessage(bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) {
		t.Errorf("want EOF, got %v", err)
	}
}

func TestReadDeviceMessage_ClipboardTooLarge(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(DevClipboard))
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], MaxDeviceTextBytes+1)
	buf.Write(hdr[:])
	_, err := ReadDeviceMessage(&buf)
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("want ErrProtocol, got %v", err)
	}
}

// --- VideoPacket ---

func TestReadVideoPacket_Basic(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], 123456) // pts_us
	binary.BigEndian.PutUint32(hdr[8:12], 4|videoPacketFlagKey)
	buf.Write(hdr[:])
	buf.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF})
	pkt, err := ReadVideoPacket(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PTSMicros != 123456 {
		t.Errorf("pts = %d", pkt.PTSMicros)
	}
	if pkt.IsConfig {
		t.Errorf("IsConfig should be false")
	}
	if !pkt.IsKeyframe {
		t.Errorf("IsKeyframe should be true")
	}
	if !bytes.Equal(pkt.Payload, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("payload = %v", pkt.Payload)
	}
}

func TestReadVideoPacket_Config(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], ^uint64(0)) // -1 → config PTS
	binary.BigEndian.PutUint32(hdr[8:12], 2|videoPacketFlagConfig)
	buf.Write(hdr[:])
	buf.Write([]byte{0x67, 0x42})
	pkt, err := ReadVideoPacket(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PTSMicros != -1 {
		t.Errorf("pts should be -1 for config, got %d", pkt.PTSMicros)
	}
	if !pkt.IsConfig {
		t.Errorf("IsConfig should be true")
	}
	if pkt.IsKeyframe {
		t.Errorf("IsKeyframe should be false")
	}
}

func TestReadVideoPacket_ShortBody(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], 0)
	binary.BigEndian.PutUint32(hdr[8:12], 0)
	buf.Write(hdr[:])
	_, err := ReadVideoPacket(&buf)
	if !errors.Is(err, ErrShortVideoPacket) {
		t.Errorf("want ErrShortVideoPacket, got %v", err)
	}
}

func TestReadVideoPacket_TooLarge(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], 0)
	binary.BigEndian.PutUint32(hdr[8:12], uint32(MaxVideoPacketBytes+1))
	buf.Write(hdr[:])
	_, err := ReadVideoPacket(&buf)
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("want ErrProtocol, got %v", err)
	}
}

// --- AudioPacket ---

func TestReadAudioPacket(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], 987654)
	binary.BigEndian.PutUint32(hdr[8:12], 3)
	buf.Write(hdr[:])
	buf.Write([]byte{1, 2, 3})
	pkt, err := ReadAudioPacket(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PTSMicros != 987654 {
		t.Errorf("pts = %d", pkt.PTSMicros)
	}
	if pkt.IsConfig {
		t.Errorf("IsConfig should be false")
	}
	if !bytes.Equal(pkt.Payload, []byte{1, 2, 3}) {
		t.Errorf("payload = %v", pkt.Payload)
	}
}

func TestReadAudioPacket_Config(t *testing.T) {
	var buf bytes.Buffer
	var hdr [12]byte
	binary.BigEndian.PutUint64(hdr[0:8], ^uint64(0))
	binary.BigEndian.PutUint32(hdr[8:12], 1|audioPacketFlagConfig)
	buf.Write(hdr[:])
	buf.Write([]byte{0x99})
	pkt, err := ReadAudioPacket(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !pkt.IsConfig || pkt.PTSMicros != -1 {
		t.Errorf("got %+v", pkt)
	}
}
