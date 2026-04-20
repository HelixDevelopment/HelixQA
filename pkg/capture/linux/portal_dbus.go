// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"fmt"
	"sync"

	dbus "github.com/godbus/dbus/v5"
)

// DBusCaller is the production Caller implementation. It wraps a user-session
// *dbus.Conn and implements the Request/Response handshake documented in the
// xdg-desktop-portal spec:
//
//   - A portal method call synchronously returns the object path of a
//     freshly-created org.freedesktop.portal.Request object.
//   - The *real* result arrives asynchronously as a Response signal on that
//     path: `org.freedesktop.portal.Request.Response(u status, a{sv} results)`.
//
// To avoid racing the signal against the call, DBusCaller registers a match
// rule for `Response` on the portal's interface BEFORE issuing the call. If
// the portal happens to emit the signal before `AddMatchSignal` is processed,
// the signal is queued by the bus and delivered once the match is in place —
// so the `AddMatch` + `conn.Signal(ch)` pair is race-free.
//
// Construction: `NewDBusCaller` dials the session bus. Operators running
// HelixQA in a headless container may not have a session bus — in that case
// the constructor returns an error naming the missing envvar
// `DBUS_SESSION_BUS_ADDRESS`, which the caller surfaces to the operator.
type DBusCaller struct {
	conn      *dbus.Conn
	closeOnce sync.Once
	closeErr  error
	// ownConn=true means we dialed the conn and must close it; false means
	// the conn was passed in (NewDBusCallerWithConn) and we leave it alone.
	ownConn bool
}

// ErrNoSessionBus is returned by NewDBusCaller when the D-Bus session bus
// is not reachable — typically because DBUS_SESSION_BUS_ADDRESS is unset
// or the host is a headless container without a session.
var ErrNoSessionBus = errors.New("linux/capture: no D-Bus session bus available (DBUS_SESSION_BUS_ADDRESS unset?)")

// NewDBusCaller dials the user-session D-Bus and returns a ready-to-use
// Caller. Callers that need to inject a pre-built conn (e.g. for integration
// tests against a dbus-launch'd private bus) should use
// NewDBusCallerWithConn.
func NewDBusCaller() (*DBusCaller, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoSessionBus, err)
	}
	// SessionBus() is a process-wide singleton — we must NOT close it on
	// Close. Mark ownConn=false.
	return &DBusCaller{conn: conn, ownConn: false}, nil
}

// NewDBusCallerWithConn wraps an existing connection. The caller retains
// ownership of the connection; Close on the returned DBusCaller does NOT
// close the wrapped conn.
func NewDBusCallerWithConn(conn *dbus.Conn) *DBusCaller {
	return &DBusCaller{conn: conn, ownConn: false}
}

// NewDBusCallerOwningConn wraps a connection that this DBusCaller owns and
// must close on Close. Useful when the caller spun up a private bus
// specifically for this capture session.
func NewDBusCallerOwningConn(conn *dbus.Conn) *DBusCaller {
	return &DBusCaller{conn: conn, ownConn: true}
}

// portalResponseMatchOptions builds the `AddMatchSignal` options shared by
// every CallPortal invocation — one rule matches Response signals for every
// portal method the process will ever make.
func portalResponseMatchOptions(dest string) []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchSender(dest),
		dbus.WithMatchInterface(portalRequestInterface),
		dbus.WithMatchMember("Response"),
	}
}

// CallPortal implements Caller.CallPortal: add Response match → call method →
// wait for Response on the returned Request object path → return (status,
// results). The match rule is removed after the signal lands so long-running
// programs don't accumulate stale rules.
func (c *DBusCaller) CallPortal(
	ctx context.Context,
	dest, path, iface, method string,
	args ...any,
) (uint32, map[string]any, error) {
	if c == nil || c.conn == nil {
		return 0, nil, errors.New("linux/capture: DBusCaller: nil conn")
	}

	// Subscribe to signals on this conn. The channel is buffered so signals
	// that arrive while we're computing the request path are not dropped.
	signalCh := make(chan *dbus.Signal, 16)
	c.conn.Signal(signalCh)
	defer c.conn.RemoveSignal(signalCh)

	match := portalResponseMatchOptions(dest)
	if err := c.conn.AddMatchSignal(match...); err != nil {
		return 0, nil, fmt.Errorf("linux/capture: AddMatchSignal: %w", err)
	}
	defer func() { _ = c.conn.RemoveMatchSignal(match...) }()

	obj := c.conn.Object(dest, dbus.ObjectPath(path))
	var requestPath dbus.ObjectPath
	call := obj.CallWithContext(ctx, iface+"."+method, 0, args...)
	if call.Err != nil {
		return 0, nil, fmt.Errorf("linux/capture: %s.%s: %w", iface, method, call.Err)
	}
	if err := call.Store(&requestPath); err != nil {
		return 0, nil, fmt.Errorf("linux/capture: %s.%s: store request path: %w", iface, method, err)
	}

	// Drain signals until ours arrives, the context cancels, or the bus
	// closes.
	for {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case sig, ok := <-signalCh:
			if !ok {
				return 0, nil, errors.New("linux/capture: signal channel closed waiting for Response")
			}
			if sig == nil {
				continue
			}
			if sig.Path != requestPath {
				continue
			}
			if sig.Name != portalRequestInterface+".Response" {
				continue
			}
			if len(sig.Body) < 2 {
				return 0, nil, fmt.Errorf("linux/capture: malformed Response body: got %d values", len(sig.Body))
			}
			status, ok := sig.Body[0].(uint32)
			if !ok {
				return 0, nil, fmt.Errorf("linux/capture: Response body[0] = %T, want uint32", sig.Body[0])
			}
			results := decodeVariantMap(sig.Body[1])
			return status, results, nil
		}
	}
}

// CallImmediate implements Caller.CallImmediate: direct method call, no
// Request/Response handshake. Returns the raw godbus Body so UnixFD values
// survive the call boundary intact.
func (c *DBusCaller) CallImmediate(
	ctx context.Context,
	dest, path, iface, method string,
	args ...any,
) ([]any, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("linux/capture: DBusCaller: nil conn")
	}
	obj := c.conn.Object(dest, dbus.ObjectPath(path))
	call := obj.CallWithContext(ctx, iface+"."+method, 0, args...)
	if call.Err != nil {
		return nil, fmt.Errorf("linux/capture: %s.%s: %w", iface, method, call.Err)
	}
	return call.Body, nil
}

// Close releases the underlying bus connection iff this DBusCaller owns it.
// The shared session bus is never closed here — closing it would break every
// other package in the process that uses godbus. Idempotent.
func (c *DBusCaller) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if c.ownConn && c.conn != nil {
			c.closeErr = c.conn.Close()
		}
	})
	return c.closeErr
}

// DBusCallerFactory is a convenience adapter matching CallerFactory — it
// calls NewDBusCaller every time, so each capture session gets its own
// (shared-session-bus backed) Caller with its own match-rule lifetime.
func DBusCallerFactory() (Caller, error) { return NewDBusCaller() }
