// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package dbusportal

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"
)

// --- ErrPortalStatus + IsUserCancelled ---

func TestErrPortalStatus_Error(t *testing.T) {
	e := &ErrPortalStatus{Method: "CreateSession", Status: 1, Result: map[string]any{"k": "v"}}
	if !strings.Contains(e.Error(), "CreateSession") {
		t.Errorf("Error() = %q", e.Error())
	}
	if !strings.Contains(e.Error(), "status=1") {
		t.Errorf("status missing from Error(): %q", e.Error())
	}
}

func TestIsUserCancelled(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"random", errors.New("boom"), false},
		{"status-1", &ErrPortalStatus{Status: 1}, true},
		{"status-2", &ErrPortalStatus{Status: 2}, false},
		{"wrapped-status-1", errors.Join(errors.New("wrap"), &ErrPortalStatus{Status: 1}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUserCancelled(tc.err); got != tc.want {
				t.Errorf("got %v, want %v (err=%v)", got, tc.want, tc.err)
			}
		})
	}
}

// --- DecodeVariantMap ---

func TestDecodeVariantMap(t *testing.T) {
	in := map[string]dbus.Variant{
		"a": dbus.MakeVariant(uint32(5)),
		"b": dbus.MakeVariant("hello"),
	}
	got := DecodeVariantMap(in)
	if got["a"].(uint32) != 5 || got["b"].(string) != "hello" {
		t.Errorf("got %+v", got)
	}
}

func TestDecodeVariantMap_PlainMap(t *testing.T) {
	in := map[string]any{"k": "v"}
	got := DecodeVariantMap(in)
	if got["k"].(string) != "v" {
		t.Errorf("got %+v", got)
	}
}

func TestDecodeVariantMap_Wrong(t *testing.T) {
	got := DecodeVariantMap("nope")
	if got == nil || len(got) != 0 {
		t.Errorf("got %+v, want empty non-nil map", got)
	}
}

// --- DBusCaller constructors + nil safety ---

func TestNewDBusCaller_RequiresBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent-helixqa-dbusportal-socket")
	if _, err := NewDBusCaller(); err == nil {
		t.Fatal("expected error when session bus is unreachable")
	} else if !errors.Is(err, ErrNoSessionBus) {
		t.Errorf("want ErrNoSessionBus wrapped, got %v", err)
	}
}

func TestNewDBusCaller_SessionBusSucceedsIfAvailable(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping")  // SKIP-OK: #legacy-untriaged
	}
	c, err := NewDBusCaller()
	if err != nil {
		t.Skipf("session bus not usable: %v", err)
	}
	if c == nil || c.conn == nil {
		t.Fatal("nil after NewDBusCaller")
	}
	if c.ownConn {
		t.Error("shared session bus MUST NOT be marked as owned")
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// second Close is no-op via sync.Once
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNewDBusCallerWithConn_NilConn(t *testing.T) {
	c := NewDBusCallerWithConn(nil)
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	_, _, err := c.CallPortal(context.Background(), "d", "/p", "i", "m")
	if err == nil || !strings.Contains(err.Error(), "nil conn") {
		t.Errorf("CallPortal: %v", err)
	}
	_, err = c.CallImmediate(context.Background(), "d", "/p", "i", "m")
	if err == nil || !strings.Contains(err.Error(), "nil conn") {
		t.Errorf("CallImmediate: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestDBusCaller_NilReceiver(t *testing.T) {
	var c *DBusCaller
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	_, _, err := c.CallPortal(context.Background(), "d", "/p", "i", "m")
	if err == nil {
		t.Error("CallPortal on nil receiver should error")
	}
	_, err = c.CallImmediate(context.Background(), "d", "/p", "i", "m")
	if err == nil {
		t.Error("CallImmediate on nil receiver should error")
	}
}

func TestNewDBusCallerOwningConn(t *testing.T) {
	// Wrap a nil conn with ownConn=true (we don't have a real private bus
	// here). Semantics-level test: ownConn flag is set.
	c := NewDBusCallerOwningConn(nil)
	if !c.ownConn {
		t.Error("ownConn not set")
	}
	// Close should not panic even with nil conn (closeOnce guards, and the
	// ownConn && c.conn != nil check prevents the Close call).
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestResponseMatchOptions_Stable(t *testing.T) {
	// Smoke guard: the function still returns 3 options (sender/interface/member).
	opts := responseMatchOptions(PortalDestination)
	if len(opts) < 3 {
		t.Errorf("got %d match options, want 3+", len(opts))
	}
}

func TestDBusCallerFactory_SatisfiesInterface(t *testing.T) {
	var _ CallerFactory = DBusCallerFactory
}

// --- Integration smoke ---

func TestDBusCaller_CallPortal_BadDestinationReturnsError(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping")  // SKIP-OK: #legacy-untriaged
	}
	c, err := NewDBusCaller()
	if err != nil {
		t.Skipf("session bus not usable: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err = c.CallPortal(
		ctx,
		"org.freedesktop.portal.DoesNotExist.HelixQA",
		"/",
		"org.freedesktop.portal.RemoteDesktop",
		"CreateSession",
		map[string]dbus.Variant{},
	)
	if err == nil {
		t.Error("expected error for non-existent portal destination")
	}
}
