// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package dbusportal

import (
	"context"
	"errors"
	"fmt"

	dbus "github.com/godbus/dbus/v5"
)

// PortalDestination is the D-Bus destination for every xdg-desktop-portal
// interface — the portal frontend runs at a stable well-known name.
const PortalDestination = "org.freedesktop.portal.Desktop"

// PortalObjectPath is the root object for portal interfaces. Individual
// interfaces (ScreenCast, RemoteDesktop, …) live under subinterfaces of
// this same object.
const PortalObjectPath = "/org/freedesktop/portal/desktop"

// RequestInterface is the interface every Request object implements. Response
// signals are `<RequestInterface>.Response`.
const RequestInterface = "org.freedesktop.portal.Request"

// Caller abstracts the godbus handshake portal clients use. Production code
// uses DBusCaller (wrapping *dbus.Conn); tests inject a fake that records
// invocations and returns scripted responses. The interface has two methods:
//
//   - CallPortal: for methods that return a Request object and deliver
//     their result via a `Response` signal. Implementations MUST add a
//     signal match before issuing the call, then wait for the Response
//     on the Request object path returned by the call. Returns
//     (status, results). status == 0 means success per
//     `org.freedesktop.portal.Request`.
//   - CallImmediate: for methods whose method-call Body IS the result
//     (OpenPipeWireRemote, ConnectToEIS). Returns the raw godbus Body
//     slice so `dbus.UnixFD` values survive.
type Caller interface {
	CallPortal(
		ctx context.Context,
		dest, path, iface, method string,
		args ...any,
	) (status uint32, results map[string]any, err error)

	CallImmediate(
		ctx context.Context,
		dest, path, iface, method string,
		args ...any,
	) (raw []any, err error)

	Close() error
}

// CallerFactory constructs a Caller on demand. Portal clients accept this
// factory (rather than a Caller directly) so each session gets its own
// signal-match lifetime and the factory can be evaluated lazily, for
// example only when a capture actually starts.
type CallerFactory func() (Caller, error)

// ErrPortalStatus is returned by typed portal clients when the portal emits
// a non-zero Response status. status == 1 per the portal spec means "user
// cancelled the consent dialog"; status >= 2 means technical failure.
// The Method field names which portal method failed (CreateSession,
// SelectSources, Start, …).
type ErrPortalStatus struct {
	Method string
	Status uint32
	Result map[string]any
}

func (e *ErrPortalStatus) Error() string {
	return fmt.Sprintf("dbusportal: %s returned status=%d results=%v", e.Method, e.Status, e.Result)
}

// IsUserCancelled reports whether err is an ErrPortalStatus with status=1 —
// the portal spec's sentinel for "user dismissed the consent dialog". Use
// this to distinguish "operator said no" from technical failures in typed
// portal clients.
func IsUserCancelled(err error) bool {
	var s *ErrPortalStatus
	if errors.As(err, &s) {
		return s.Status == 1
	}
	return false
}

// DecodeVariantMap converts a `map[string]dbus.Variant` (the shape godbus
// hands out for `a{sv}` parameters) into a plain `map[string]any` so callers
// don't need to import godbus themselves. Accepts plain maps as a no-op.
// Returns a non-nil empty map for any other input.
func DecodeVariantMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]dbus.Variant:
		out := make(map[string]any, len(m))
		for k, vv := range m {
			out[k] = vv.Value()
		}
		return out
	case map[string]any:
		return m
	}
	return map[string]any{}
}
