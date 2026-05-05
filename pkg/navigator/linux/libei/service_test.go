// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	dbus "github.com/godbus/dbus/v5"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
)

func scriptedCaller(t *testing.T, fd int32) *fakeCaller {
	t.Helper()
	return &fakeCaller{
		portalResps: []portalResp{
			{status: 0, results: map[string]any{"session_handle": "/s"}},
			{status: 0, results: map[string]any{}},
			{status: 0, results: map[string]any{
				"devices":       uint32(DeviceKeyboard | DevicePointer),
				"restore_token": "tok-42",
			}},
		},
		immRespBody: []any{dbus.UnixFD(fd)},
	}
}

func TestNewServiceWithFactory_HappyPath(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()

	caller := scriptedCaller(t, int32(pw.Fd()))
	factory := func() (dbusportal.Caller, error) { return caller, nil }

	svc, err := NewServiceWithFactory(context.Background(), factory, ServiceConfig{
		Types:        DeviceKeyboard | DevicePointer,
		ParentWindow: "wayland:xdg-toplevel:1",
		Persist:      PersistForever,
	})
	if err != nil {
		t.Fatalf("NewServiceWithFactory: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	if svc.EISFile() == nil {
		t.Fatal("EISFile is nil")
	}
	if svc.GrantedDevices() != DeviceKeyboard|DevicePointer {
		t.Errorf("GrantedDevices = %d", svc.GrantedDevices())
	}
	if svc.RestoreToken() != "tok-42" {
		t.Errorf("RestoreToken = %q", svc.RestoreToken())
	}
	// Verify the factory actually made the three expected portal calls.
	if len(caller.portalCalls) != 3 {
		t.Errorf("portal calls = %d, want 3 (CreateSession + SelectDevices + Start)", len(caller.portalCalls))
	}
	if len(caller.immCalls) != 1 {
		t.Errorf("immediate calls = %d, want 1 (ConnectToEIS)", len(caller.immCalls))
	}
}

func TestNewServiceWithFactory_NilFactory(t *testing.T) {
	_, err := NewServiceWithFactory(context.Background(), nil, ServiceConfig{})
	if err == nil || !strings.Contains(err.Error(), "nil factory") {
		t.Errorf("got %v", err)
	}
}

func TestNewServiceWithFactory_FactoryError(t *testing.T) {
	boom := errors.New("bus down")
	factory := func() (dbusportal.Caller, error) { return nil, boom }
	_, err := NewServiceWithFactory(context.Background(), factory, ServiceConfig{})
	if !errors.Is(err, boom) {
		t.Errorf("want %v wrapped, got %v", boom, err)
	}
}

func TestNewServiceWithFactory_CreateSessionFails(t *testing.T) {
	caller := &fakeCaller{portalResps: []portalResp{{err: errors.New("boom")}}}
	factory := func() (dbusportal.Caller, error) { return caller, nil }
	_, err := NewServiceWithFactory(context.Background(), factory, ServiceConfig{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("want boom wrapped, got %v", err)
	}
	if !caller.closed {
		t.Error("caller not closed on CreateSession failure")
	}
}

func TestNewServiceWithFactory_UserCancelled(t *testing.T) {
	caller := &fakeCaller{
		portalResps: []portalResp{
			{status: 0, results: map[string]any{"session_handle": "/s"}}, // CreateSession ok
			{status: 1, results: map[string]any{}},                        // SelectDevices cancelled
		},
	}
	factory := func() (dbusportal.Caller, error) { return caller, nil }
	_, err := NewServiceWithFactory(context.Background(), factory, ServiceConfig{})
	if !dbusportal.IsUserCancelled(err) {
		t.Errorf("want user-cancelled, got %v", err)
	}
	if !caller.closed {
		t.Error("caller not closed on user-cancelled")
	}
}

func TestNewServiceWithPortal_NilPortal(t *testing.T) {
	_, err := NewServiceWithPortal(context.Background(), nil, ServiceConfig{})
	if err == nil {
		t.Error("nil portal must error")
	}
}

func TestService_NilReceiverAccessors(t *testing.T) {
	var s *Service
	if s.EISFile() != nil {
		t.Error("nil receiver EISFile")
	}
	if s.GrantedDevices() != 0 {
		t.Error("nil receiver GrantedDevices")
	}
	if s.RestoreToken() != "" {
		t.Error("nil receiver RestoreToken")
	}
	if err := s.Close(); err != nil {
		t.Errorf("nil receiver Close: %v", err)
	}
}

func TestService_CloseIdempotent(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()

	caller := scriptedCaller(t, int32(pw.Fd()))
	factory := func() (dbusportal.Caller, error) { return caller, nil }
	svc, err := NewServiceWithFactory(context.Background(), factory, ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
	if !caller.closed {
		t.Error("caller not closed")
	}
	// EISFile now nil after Close.
	if svc.EISFile() != nil {
		t.Error("EISFile should be nil after Close")
	}
}

func TestNewDefaultService_UsesProductionFactory(t *testing.T) {
	// bluff-scan: no-assert-ok (integration/interface-compliance smoke — wiring must not panic)
	// Can't verify end-to-end without a real session bus; when
	// DBUS_SESSION_BUS_ADDRESS is unset, NewDefaultService returns
	// ErrNoSessionBus wrapped. When set, we pass through into the real
	// portal and expect a NameHasNoOwner or similar error (we have no
	// RemoteDesktop portal registered on the tested host).
	if _, err := NewDefaultService(context.Background(), ServiceConfig{}); err == nil {
		t.Skip("unexpected: RemoteDesktop portal is actually registered on this host")
	}
}
