// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package uinput

import (
	"os"
	"strings"
	"testing"
)

func TestValidateConfig_OK(t *testing.T) {
	cases := []Config{
		{Name: "a", EnableKey: true, KeyBits: []uint16{KeyA}},
		{Name: "b", EnableRel: true, RelBits: []uint16{RelX}},
		{Name: "c", EnableAbs: true, AbsBits: []uint16{AbsX}, AbsMin: 0, AbsMax: 1000},
		{Name: "minimal"},
	}
	for i, tc := range cases {
		if err := validateConfig(tc); err != nil {
			t.Errorf("[%d] want nil, got %v", i, err)
		}
	}
}

func TestValidateConfig_Rejects(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		sub  string
	}{
		{"key-bits-without-enable", Config{Name: "x", KeyBits: []uint16{KeyA}}, "KeyBits"},
		{"rel-bits-without-enable", Config{Name: "x", RelBits: []uint16{RelX}}, "RelBits"},
		{"abs-bits-without-enable", Config{Name: "x", AbsBits: []uint16{AbsX}}, "AbsBits"},
		{"abs-inverted-range", Config{Name: "x", EnableAbs: true, AbsMin: 100, AbsMax: 10}, "bad abs range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.sub) {
				t.Errorf("got %v, want substring %q", err, tc.sub)
			}
		})
	}
}

func TestOpen_EmptyName(t *testing.T) {
	if _, err := Open(Config{}); err == nil || !strings.Contains(err.Error(), "empty name") {
		t.Errorf("empty name should error, got %v", err)
	}
}

func TestOpen_BadConfig(t *testing.T) {
	// EnableAbs=false but AbsBits populated -> validateConfig rejects before
	// /dev/uinput is ever touched.
	if _, err := Open(Config{Name: "x", AbsBits: []uint16{AbsX}}); err == nil {
		t.Error("bad config should error before /dev/uinput open")
	}
}

func TestOpen_MissingDevice(t *testing.T) {
	// Sanity: if /dev/uinput is not accessible (common in CI containers), Open
	// should fail with an unambiguous error pointing at the device path. We
	// skip when /dev/uinput IS available (rare in CI) so the test doesn't
	// attempt to create a real device as a side effect.
	if _, err := os.Stat("/dev/uinput"); err == nil {
		// Check permission — opening for write may still fail here without
		// the udev rule, which is the error we document.
		if f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0); err == nil {
			_ = f.Close()
			t.Skip("/dev/uinput is writable — can't exercise the error branch on this host")
		}
	}
	_, err := Open(Config{Name: "probe", EnableKey: true, KeyBits: []uint16{KeyA}})
	if err == nil {
		t.Fatal("expected error when /dev/uinput is unavailable or unwritable")
	}
	if !strings.Contains(err.Error(), "/dev/uinput") {
		t.Errorf("error should name the device path, got %q", err)
	}
}

func TestDevice_CloseNil(t *testing.T) {
	var d *Device
	if err := d.Close(); err != nil {
		t.Errorf("nil receiver close: got %v", err)
	}
}

func TestDevice_Name_NilReceiver(t *testing.T) {
	var d *Device
	if d.Name() != "" {
		t.Errorf("nil receiver Name() = %q", d.Name())
	}
}

func TestDevice_Close_ClosedTwice(t *testing.T) {
	// Construct a fake Device whose *os.File is a pipe's read end — Close
	// should release it once and be a no-op the second time.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	d := &Device{f: r, name: "fake", created: false}
	if err := d.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}
