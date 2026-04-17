// Package desktop is the Nexus desktop engine. It offers three
// platform-specific drivers behind a shared DesktopEngine interface:
// Windows (WinAppDriver over HTTP), macOS (XCUITest + AppleScript), and
// Linux (AT-SPI over DBus with X11 fallback).
//
// Like the mobile engine, the package is pure Go and tests never require
// a real Windows / macOS / Linux desktop or GUI stack. Each driver's
// integration with its native surface is exercised via httptest.Server
// or a small osascript / dbus-send harness so the core logic stays
// reviewable and portable.
package desktop
