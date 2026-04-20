// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package dbusportal holds the shared D-Bus plumbing for talking to any
// xdg-desktop-portal interface — ScreenCast, RemoteDesktop, FileChooser,
// Notifications, and so on.
//
// Portal interfaces (ScreenCast, RemoteDesktop, …) follow a common
// Request/Response handshake:
//
//   - Most methods return an `org.freedesktop.portal.Request` object path
//     synchronously; the real result arrives asynchronously as a
//     `Response(u status, a{sv} results)` signal on that path.
//   - A handful of methods (OpenPipeWireRemote, ConnectToEIS) return
//     their result directly in the method call's Body — no signal handshake.
//
// This package defines:
//
//   - The Caller interface, a minimal transport abstraction with two
//     methods: CallPortal (Request/Response handshake) and CallImmediate
//     (direct-return methods). Portal-specific packages
//     (pkg/capture/linux, pkg/navigator/linux/libei, …) build typed
//     clients on top.
//   - DBusCaller, the production Caller wrapping `*dbus.Conn`, including
//     ErrNoSessionBus surfacing when DBUS_SESSION_BUS_ADDRESS is missing.
//     Three constructors let the caller pick shared / injected / owned
//     connection lifetimes.
//   - ErrPortalStatus + IsUserCancelled: the common "the portal returned
//     non-zero status" error, with status=1 as the user-cancelled sentinel.
//   - DecodeVariantMap: a small helper that converts
//     `map[string]dbus.Variant` (what godbus hands out) into a plain
//     `map[string]any` so downstream packages don't import godbus.
//
// All types in this package are CGO-free and do not depend on anything
// outside the Go standard library plus `github.com/godbus/dbus/v5`.
//
// See:
//   - docs/openclawing/OpenClawing4.md §5.1.1 (ScreenCast portal)
//   - docs/openclawing/OpenClawing4.md §5.2.1 (RemoteDesktop / libei)
package dbusportal
