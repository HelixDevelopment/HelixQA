// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package dbusportal

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
//     freshly-created `org.freedesktop.portal.Request` object.
//   - The real result arrives asynchronously as a `Response(u, a{sv})`
//     signal on that path.
//
// To avoid racing the signal against the call, DBusCaller registers an
// AddMatchSignal for `Response` on the portal's request interface BEFORE
// issuing the method call. If the portal emits the signal before
// AddMatchSignal is processed, the bus queues it and delivers once the match
// is in place — so the `AddMatch + conn.Signal(ch)` pair is race-free.
type DBusCaller struct {
	conn      *dbus.Conn
	closeOnce sync.Once
	closeErr  error
	// ownConn=true means this DBusCaller dialed the conn and must close it;
	// false means the conn was passed in (shared session bus) and we leave
	// it alone.
	ownConn bool
}

// ErrNoSessionBus is returned by NewDBusCaller when dbus.SessionBus() fails —
// typically because DBUS_SESSION_BUS_ADDRESS is unset or the host is a
// headless container without a user session bus.
var ErrNoSessionBus = errors.New("dbusportal: no D-Bus session bus available (DBUS_SESSION_BUS_ADDRESS unset?)")

// NewDBusCaller dials the user-session bus via dbus.SessionBus() and returns
// a ready-to-use Caller. Because SessionBus() returns a process-wide
// singleton, Close() does NOT close the shared connection.
func NewDBusCaller() (*DBusCaller, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoSessionBus, err)
	}
	return &DBusCaller{conn: conn, ownConn: false}, nil
}

// NewDBusCallerWithConn wraps an existing connection. Caller retains
// ownership; Close on the returned DBusCaller does NOT close the conn.
// Useful for integration tests against dbus-launch'd private buses.
func NewDBusCallerWithConn(conn *dbus.Conn) *DBusCaller {
	return &DBusCaller{conn: conn, ownConn: false}
}

// NewDBusCallerOwningConn wraps a connection that this DBusCaller owns and
// must close on Close.
func NewDBusCallerOwningConn(conn *dbus.Conn) *DBusCaller {
	return &DBusCaller{conn: conn, ownConn: true}
}

// responseMatchOptions builds the AddMatchSignal options every CallPortal
// invocation registers. One match rule covers every method call the process
// will make against this destination.
func responseMatchOptions(dest string) []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchSender(dest),
		dbus.WithMatchInterface(RequestInterface),
		dbus.WithMatchMember("Response"),
	}
}

// CallPortal implements Caller.CallPortal: register Response match → issue
// method call → extract request object path → wait for the matching Response
// signal → return (status, results). The match rule is removed after the
// signal lands so long-running programs don't accumulate stale rules.
func (c *DBusCaller) CallPortal(
	ctx context.Context,
	dest, path, iface, method string,
	args ...any,
) (uint32, map[string]any, error) {
	if c == nil || c.conn == nil {
		return 0, nil, errors.New("dbusportal: DBusCaller: nil conn")
	}

	signalCh := make(chan *dbus.Signal, 16)
	c.conn.Signal(signalCh)
	defer c.conn.RemoveSignal(signalCh)

	match := responseMatchOptions(dest)
	if err := c.conn.AddMatchSignal(match...); err != nil {
		return 0, nil, fmt.Errorf("dbusportal: AddMatchSignal: %w", err)
	}
	defer func() { _ = c.conn.RemoveMatchSignal(match...) }()

	obj := c.conn.Object(dest, dbus.ObjectPath(path))
	var requestPath dbus.ObjectPath
	call := obj.CallWithContext(ctx, iface+"."+method, 0, args...)
	if call.Err != nil {
		return 0, nil, fmt.Errorf("dbusportal: %s.%s: %w", iface, method, call.Err)
	}
	if err := call.Store(&requestPath); err != nil {
		return 0, nil, fmt.Errorf("dbusportal: %s.%s: store request path: %w", iface, method, err)
	}

	for {
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case sig, ok := <-signalCh:
			if !ok {
				return 0, nil, errors.New("dbusportal: signal channel closed waiting for Response")
			}
			if sig == nil {
				continue
			}
			if sig.Path != requestPath || sig.Name != RequestInterface+".Response" {
				continue
			}
			if len(sig.Body) < 2 {
				return 0, nil, fmt.Errorf("dbusportal: malformed Response body: got %d values", len(sig.Body))
			}
			status, ok := sig.Body[0].(uint32)
			if !ok {
				return 0, nil, fmt.Errorf("dbusportal: Response body[0] = %T, want uint32", sig.Body[0])
			}
			return status, DecodeVariantMap(sig.Body[1]), nil
		}
	}
}

// CallImmediate implements Caller.CallImmediate: direct method call, no
// Request/Response handshake. Returns the raw godbus Body so dbus.UnixFD
// values survive the call boundary intact.
func (c *DBusCaller) CallImmediate(
	ctx context.Context,
	dest, path, iface, method string,
	args ...any,
) ([]any, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("dbusportal: DBusCaller: nil conn")
	}
	obj := c.conn.Object(dest, dbus.ObjectPath(path))
	call := obj.CallWithContext(ctx, iface+"."+method, 0, args...)
	if call.Err != nil {
		return nil, fmt.Errorf("dbusportal: %s.%s: %w", iface, method, call.Err)
	}
	return call.Body, nil
}

// Close releases the underlying bus connection iff this DBusCaller owns it.
// The shared session bus is never closed — doing so would break every other
// package in the process that uses godbus. Idempotent via sync.Once.
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

// DBusCallerFactory is a convenience adapter satisfying CallerFactory — it
// calls NewDBusCaller every time, so each portal session gets its own
// signal-match lifetime against the shared session bus.
func DBusCallerFactory() (Caller, error) { return NewDBusCaller() }
