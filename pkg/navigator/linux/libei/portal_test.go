// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	dbus "github.com/godbus/dbus/v5"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
)

// --- fakeCaller (identical pattern to capture/linux portal_test.go) ---

type fakeCaller struct {
	mu          sync.Mutex
	portalCalls []portalCall
	immCalls    []portalCall
	portalResps []portalResp
	immRespBody []any
	immRespErr  error
	closed      bool
}

type portalCall struct {
	Dest, Path, Iface, Method string
	Args                      []any
}

type portalResp struct {
	status  uint32
	results map[string]any
	err     error
}

func (f *fakeCaller) CallPortal(ctx context.Context, dest, path, iface, method string, args ...any) (uint32, map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.portalCalls = append(f.portalCalls, portalCall{Dest: dest, Path: path, Iface: iface, Method: method, Args: append([]any(nil), args...)})
	if len(f.portalResps) == 0 {
		return 0, map[string]any{}, nil
	}
	r := f.portalResps[0]
	f.portalResps = f.portalResps[1:]
	return r.status, r.results, r.err
}

func (f *fakeCaller) CallImmediate(ctx context.Context, dest, path, iface, method string, args ...any) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.immCalls = append(f.immCalls, portalCall{Dest: dest, Path: path, Iface: iface, Method: method, Args: append([]any(nil), args...)})
	return f.immRespBody, f.immRespErr
}

func (f *fakeCaller) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// Compile-time check — fakeCaller satisfies dbusportal.Caller.
var _ dbusportal.Caller = (*fakeCaller)(nil)

// --- CreateSession ---

func TestPortal_CreateSession_Success(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status:  0,
			results: map[string]any{"session_handle": "/org/freedesktop/portal/desktop/session/libei/1"},
		}},
	}
	p := NewPortal(fc)
	sess, err := p.CreateSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sess, "/org/freedesktop/portal/desktop/session/libei/") {
		t.Errorf("session = %q", sess)
	}
	c := fc.portalCalls[0]
	if c.Dest != dbusportal.PortalDestination || c.Path != dbusportal.PortalObjectPath ||
		c.Iface != portalRemoteDesktopIface || c.Method != "CreateSession" {
		t.Errorf("wrong call target: %+v", c)
	}
	opts := c.Args[0].(map[string]dbus.Variant)
	for _, key := range []string{"handle_token", "session_handle_token"} {
		v, present := opts[key]
		if !present {
			t.Errorf("missing %q", key)
			continue
		}
		if s, ok := v.Value().(string); !ok || !strings.HasPrefix(s, "helixqa_libei") {
			t.Errorf("%q = %v, want helixqa_libei-prefixed", key, v.Value())
		}
	}
}

func TestPortal_CreateSession_StatusCancelled(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 1, results: map[string]any{}}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if !dbusportal.IsUserCancelled(err) {
		t.Errorf("want user-cancelled, got %v", err)
	}
}

func TestPortal_CreateSession_MissingSessionHandle(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "session_handle") {
		t.Errorf("got %v", err)
	}
}

func TestPortal_CreateSession_CallerError(t *testing.T) {
	boom := errors.New("bus down")
	fc := &fakeCaller{portalResps: []portalResp{{err: boom}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("want boom wrapped, got %v", err)
	}
}

func TestPortal_CreateSession_TokensAreUnique(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{
			{status: 0, results: map[string]any{"session_handle": "s1"}},
			{status: 0, results: map[string]any{"session_handle": "s2"}},
		},
	}
	p := NewPortal(fc)
	if _, err := p.CreateSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CreateSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range fc.portalCalls {
		opts := c.Args[0].(map[string]dbus.Variant)
		tok := opts["handle_token"].Value().(string)
		if seen[tok] {
			t.Errorf("duplicate handle_token: %q", tok)
		}
		seen[tok] = true
	}
}

// --- SelectDevices ---

func TestPortal_SelectDevices_ArgumentShape(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	err := p.SelectDevices(context.Background(), "/s", SelectDevicesOptions{
		Types:        DeviceKeyboard | DevicePointer | DeviceTouchscreen,
		Persist:      PersistForever,
		RestoreToken: "tok-xyz",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := fc.portalCalls[0]
	if c.Method != "SelectDevices" || c.Path != "/s" || c.Iface != portalRemoteDesktopIface {
		t.Errorf("wrong call: %+v", c)
	}
	opts := c.Args[0].(map[string]dbus.Variant)
	if got := opts["types"].Value().(uint32); got != uint32(DeviceKeyboard|DevicePointer|DeviceTouchscreen) {
		t.Errorf("types = %d", got)
	}
	if got := opts["persist_mode"].Value().(uint32); got != uint32(PersistForever) {
		t.Errorf("persist_mode = %d", got)
	}
	if got := opts["restore_token"].Value().(string); got != "tok-xyz" {
		t.Errorf("restore_token = %q", got)
	}
}

func TestPortal_SelectDevices_Defaults(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	if err := p.SelectDevices(context.Background(), "/s", SelectDevicesOptions{}); err != nil {
		t.Fatal(err)
	}
	opts := fc.portalCalls[0].Args[0].(map[string]dbus.Variant)
	if got := opts["types"].Value().(uint32); got != uint32(DeviceKeyboard|DevicePointer) {
		t.Errorf("default types = %d, want keyboard|pointer", got)
	}
	if _, present := opts["persist_mode"]; present {
		t.Errorf("persist_mode should be omitted when PersistNever")
	}
	if _, present := opts["restore_token"]; present {
		t.Errorf("restore_token should be omitted when empty")
	}
}

func TestPortal_SelectDevices_EmptySessionPath(t *testing.T) {
	p := NewPortal(&fakeCaller{})
	if err := p.SelectDevices(context.Background(), "", SelectDevicesOptions{}); err == nil {
		t.Error("empty sessionPath should error")
	}
}

// --- Start ---

func TestPortal_Start_Success(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status: 0,
			results: map[string]any{
				"devices":       uint32(DeviceKeyboard | DevicePointer),
				"restore_token": "restore-xyz",
			},
		}},
	}
	p := NewPortal(fc)
	res, err := p.Start(context.Background(), "/s", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.ChosenDevices != DeviceKeyboard|DevicePointer {
		t.Errorf("ChosenDevices = %d", res.ChosenDevices)
	}
	if res.RestoreToken != "restore-xyz" {
		t.Errorf("RestoreToken = %q", res.RestoreToken)
	}
}

func TestPortal_Start_NoDevices(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status: 0,
			results: map[string]any{"devices": uint32(0)},
		}},
	}
	p := NewPortal(fc)
	_, err := p.Start(context.Background(), "/s", "")
	if err == nil || !strings.Contains(err.Error(), "no devices") {
		t.Errorf("want no-devices error, got %v", err)
	}
}

func TestPortal_Start_StatusNonZero(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 2, results: map[string]any{}}}}
	p := NewPortal(fc)
	_, err := p.Start(context.Background(), "/s", "")
	if err == nil {
		t.Fatal("expected error")
	}
	var s *dbusportal.ErrPortalStatus
	if !errors.As(err, &s) || s.Method != "Start" || s.Status != 2 {
		t.Errorf("want ErrPortalStatus{Method=Start, Status=2}, got %v", err)
	}
}

func TestPortal_Start_ParentWindowPassedThrough(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status: 0,
			results: map[string]any{"devices": uint32(DeviceKeyboard)},
		}},
	}
	p := NewPortal(fc)
	if _, err := p.Start(context.Background(), "/s", "wayland:xdg-toplevel:42"); err != nil {
		t.Fatal(err)
	}
	if got := fc.portalCalls[0].Args[0]; got != "wayland:xdg-toplevel:42" {
		t.Errorf("parentWindow = %v", got)
	}
}

// --- ConnectToEIS ---

func TestPortal_ConnectToEIS_Success(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	fc := &fakeCaller{immRespBody: []any{dbus.UnixFD(int32(w.Fd()))}}
	p := NewPortal(fc)
	f, err := p.ConnectToEIS(context.Background(), "/session")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("nil file")
	}
	if uintptr(f.Fd()) != w.Fd() {
		t.Errorf("fd mismatch: %d vs %d", f.Fd(), w.Fd())
	}
}

func TestPortal_ConnectToEIS_WrongBodyType(t *testing.T) {
	fc := &fakeCaller{immRespBody: []any{"not-an-fd"}}
	p := NewPortal(fc)
	_, err := p.ConnectToEIS(context.Background(), "/s")
	if err == nil || !strings.Contains(err.Error(), "UnixFD") {
		t.Errorf("want UnixFD type error, got %v", err)
	}
}

func TestPortal_ConnectToEIS_EmptyBody(t *testing.T) {
	fc := &fakeCaller{immRespBody: nil}
	p := NewPortal(fc)
	_, err := p.ConnectToEIS(context.Background(), "/s")
	if err == nil || !strings.Contains(err.Error(), "no body") {
		t.Errorf("want empty-body error, got %v", err)
	}
}

func TestPortal_ConnectToEIS_EmptySession(t *testing.T) {
	p := NewPortal(&fakeCaller{})
	if _, err := p.ConnectToEIS(context.Background(), ""); err == nil {
		t.Error("empty sessionPath should error")
	}
}

// --- Close ---

func TestPortal_Close_CallsCallerClose(t *testing.T) {
	fc := &fakeCaller{}
	p := NewPortal(fc)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if !fc.closed {
		t.Error("caller.Close not invoked")
	}
}

func TestPortal_Close_Nil(t *testing.T) {
	var p *Portal
	if err := p.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
}
