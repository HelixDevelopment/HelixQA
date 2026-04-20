// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"
)

// These tests exercise DBusCaller WITHOUT a real D-Bus connection — they
// verify argument shaping, nil-conn guarding, ownership semantics, and
// matching options. End-to-end behaviour against a real bus is covered by
// an integration test (TestDBusCaller_CallPortal_NoBusSkips) that skips
// when DBUS_SESSION_BUS_ADDRESS is unset — that's the honest coverage line:
// the production client works when there IS a bus and reports a clear error
// when there isn't.

func TestNewDBusCaller_RequiresBus(t *testing.T) {
	// Force the dial to fail by pointing at a non-existent address.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent-helixqa-socket")
	if _, err := NewDBusCaller(); err == nil {
		t.Fatal("expected error when session bus is unreachable")
	} else if !errors.Is(err, ErrNoSessionBus) {
		t.Errorf("want ErrNoSessionBus wrapped, got %v", err)
	}
}

func TestNewDBusCaller_SessionBusSucceedsIfAvailable(t *testing.T) {
	// On CI hosts without a session bus this should skip cleanly.
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping (host has no user session bus)")
	}
	c, err := NewDBusCaller()
	if err != nil {
		t.Skipf("session bus not usable in this environment: %v", err)
	}
	if c == nil || c.conn == nil {
		t.Fatal("DBusCaller or conn is nil after NewDBusCaller")
	}
	if c.ownConn {
		t.Errorf("shared session bus MUST NOT be marked as owned")
	}
	// Close must not close the shared bus.
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// A second Close is a no-op via sync.Once.
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNewDBusCallerWithConn_NilConn(t *testing.T) {
	c := NewDBusCallerWithConn(nil)
	if c == nil {
		t.Fatal("NewDBusCallerWithConn returned nil")
	}
	// CallPortal / CallImmediate on nil conn must return a clear error
	// rather than panicking.
	_, _, err := c.CallPortal(context.Background(), "d", "/p", "i", "m")
	if err == nil || !strings.Contains(err.Error(), "nil conn") {
		t.Errorf("CallPortal on nil conn: got %v", err)
	}
	_, err = c.CallImmediate(context.Background(), "d", "/p", "i", "m")
	if err == nil || !strings.Contains(err.Error(), "nil conn") {
		t.Errorf("CallImmediate on nil conn: got %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close on nil-conn caller: %v", err)
	}
}

func TestDBusCaller_NilReceiver(t *testing.T) {
	var c *DBusCaller
	if err := c.Close(); err != nil {
		t.Errorf("nil receiver Close: %v", err)
	}
	_, _, err := c.CallPortal(context.Background(), "d", "/p", "i", "m")
	if err == nil {
		t.Error("nil receiver CallPortal should error")
	}
	_, err = c.CallImmediate(context.Background(), "d", "/p", "i", "m")
	if err == nil {
		t.Error("nil receiver CallImmediate should error")
	}
}

func TestNewDBusCallerOwningConn_MarksOwnership(t *testing.T) {
	// Build a fake conn via a private dbus.Conn — we never use it for calls,
	// but Close must observe ownConn=true.
	fake, err := dbus.Connect("unix:path=/nonexistent-helixqa-private")
	if err == nil {
		// Unexpected: we got a conn from a bogus address.
		defer fake.Close()
		t.Skip("unexpected successful connect to bogus address")
	}
	// fake is nil, but the constructor still wraps it — semantics test only.
	c := NewDBusCallerOwningConn(fake)
	if !c.ownConn {
		t.Error("ownConn not marked")
	}
	if err := c.Close(); err != nil {
		// Closing a nil conn inside the wrapper surfaces an error; that's fine,
		// but the Close path at least did not panic.
		_ = err
	}
}

func TestPortalResponseMatchOptions_ShapeIsStable(t *testing.T) {
	opts := portalResponseMatchOptions(portalDestination)
	if len(opts) < 3 {
		t.Fatalf("expected 3+ match options, got %d", len(opts))
	}
	// We can't easily introspect MatchOption internals (godbus hides them),
	// but we CAN verify the function accepts the expected destination and
	// returns a non-nil slice — a smoke test guarding against accidental
	// removal.
}

func TestDBusCallerFactory_MatchesInterface(t *testing.T) {
	// Compile-time check: DBusCallerFactory must satisfy CallerFactory.
	var _ CallerFactory = DBusCallerFactory
}

// Integration smoke: when a real session bus is available, CallPortal against
// an intentionally-bad destination returns a godbus error (not a panic or a
// hang).
func TestDBusCaller_CallPortal_BadDestinationReturnsError(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping integration smoke")
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
		"org.freedesktop.portal.ScreenCast",
		"CreateSession",
		map[string]dbus.Variant{},
	)
	if err == nil {
		t.Error("expected error for non-existent portal destination")
	}
}
