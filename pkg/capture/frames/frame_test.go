// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package frames

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestFormat_String(t *testing.T) {
	cases := []struct {
		in   Format
		want string
	}{
		{FormatUnknown, "unknown"},
		{FormatNV12, "nv12"},
		{FormatRGBA, "rgba"},
		{FormatBGRA, "bgra"},
		{FormatH264AnnexB, "h264-annexb"},
		{Format(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Format(%d).String() = %q, want %q", int(tc.in), got, tc.want)
		}
	}
}

func TestFormat_Valid(t *testing.T) {
	cases := []struct {
		in   Format
		want bool
	}{
		{FormatUnknown, false},
		{FormatNV12, true},
		{FormatRGBA, true},
		{FormatBGRA, true},
		{FormatH264AnnexB, true},
		{Format(99), false},
	}
	for _, tc := range cases {
		if got := tc.in.Valid(); got != tc.want {
			t.Errorf("Format(%d).Valid() = %v, want %v", int(tc.in), got, tc.want)
		}
	}
}

func TestNew_Valid(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x01, 0x67}
	f, err := New(42*time.Millisecond, 1920, 1080, FormatH264AnnexB, "test", data)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if f.PTS != 42*time.Millisecond {
		t.Errorf("PTS: got %v want 42ms", f.PTS)
	}
	if f.HasFD() {
		t.Errorf("HasFD: got true want false")
	}
	if len(f.Data) != 5 {
		t.Errorf("Data len: got %d want 5", len(f.Data))
	}
}

func TestNew_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		run    func() (Frame, error)
		reason string
	}{
		{"zero-format", func() (Frame, error) { return New(0, 1, 1, FormatUnknown, "x", []byte{1}) }, "format=unknown"},
		{"bad-format", func() (Frame, error) { return New(0, 1, 1, Format(99), "x", []byte{1}) }, "format"},
		{"negative-width", func() (Frame, error) { return New(0, -1, 1, FormatNV12, "x", []byte{1}) }, "width=-1"},
		{"zero-height", func() (Frame, error) { return New(0, 1, 0, FormatNV12, "x", []byte{1}) }, "height=0"},
		{"empty-source", func() (Frame, error) { return New(0, 1, 1, FormatNV12, "", []byte{1}) }, "source empty"},
		{"no-payload", func() (Frame, error) { return New(0, 1, 1, FormatNV12, "x", nil) }, "no payload"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.run()
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestNewFromFD_Valid(t *testing.T) {
	// os.Pipe() is a portable way to obtain a real FD for the test; the
	// write end becomes our "payload FD" and Close() releases it.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// Close the read end at the end regardless; w's fd ownership transfers
	// to the Frame via NewFromFD and is released by f.Close.
	defer func() { _ = r.Close() }()
	fd := int(w.Fd())
	f, err := NewFromFD(0, 640, 480, FormatNV12, "test-memfd", fd, 640*480*3/2)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !f.HasFD() {
		t.Error("HasFD: got false want true")
	}
	if f.DataLen != 640*480*3/2 {
		t.Errorf("DataLen: got %d want %d", f.DataLen, 640*480*3/2)
	}
	// Close releases FD.
	if err := f.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if f.DataFD >= 0 {
		t.Errorf("Close did not reset DataFD, got %d", f.DataFD)
	}
	// Second Close is a no-op.
	if err := f.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNewFromFD_Invalid(t *testing.T) {
	cases := []struct {
		name string
		run  func() (Frame, error)
	}{
		{"missing-length", func() (Frame, error) { return NewFromFD(0, 1, 1, FormatNV12, "x", 3, 0) }},
		{"bad-format", func() (Frame, error) { return NewFromFD(0, 1, 1, FormatUnknown, "x", 3, 10) }},
		{"negative-dim", func() (Frame, error) { return NewFromFD(0, -1, 1, FormatNV12, "x", 3, 10) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.run(); err == nil || !errors.Is(err, ErrInvalid) {
				t.Errorf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestValidate_BothPayloads(t *testing.T) {
	f := Frame{
		PTS: 0, Width: 1, Height: 1, Format: FormatNV12, Source: "x",
		DataFD: 3, DataLen: 1, Data: []byte{1},
	}
	err := f.Validate()
	if err == nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid, got %v", err)
	}
}

func TestClose_NilReceiver(t *testing.T) {
	var f *Frame
	if err := f.Close(); err != nil {
		t.Errorf("Close on nil receiver should be no-op, got %v", err)
	}
}

func TestClose_InlinePayload(t *testing.T) {
	f, err := New(0, 1, 1, FormatNV12, "x", []byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("Close on inline payload should be no-op, got %v", err)
	}
}
