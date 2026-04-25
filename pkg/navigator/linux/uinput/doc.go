// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package uinput is a pure-Go /dev/uinput driver used by HelixQA as the
// Wayland-correct fallback when libei is unavailable (or the session type is
// not a portal-capable Wayland). See docs/openclawing/OpenClawing4.md §5.2.1.
//
// The package is strictly Linux-only (`//go:build linux` on the implementation
// files). It uses only stdlib + golang.org/x/sys/unix; zero CGO.
//
// # Permissions (no sudo)
//
// Access to /dev/uinput requires membership in a group that owns the device.
// The expected operator setup is a single udev rule installed once per host:
//
//	KERNEL=="uinput", GROUP="helixqa", MODE="0660"
//
// The HelixQA user is added to the `helixqa` group by the operator at install
// time (non-sudo, one-time). No runtime privilege escalation is required or
// permitted (CLAUDE.md § NO SUDO OR ROOT EXECUTION).
//
// # Wire format (input_event)
//
// On 64-bit Linux, `struct input_event` is 24 bytes:
//
//	struct input_event {
//	    __kernel_ulong_t tv_sec;   // 8 bytes
//	    __kernel_ulong_t tv_usec;  // 8 bytes
//	    __u16            type;     // 2 bytes
//	    __u16            code;     // 2 bytes
//	    __s32            value;    // 4 bytes
//	};
//
// The kernel fills in tv_sec / tv_usec when it processes the event, so callers
// may write zeros. This package writes zeros.
//
// # ioctl numbers
//
// Computed explicitly rather than imported from a generated header so the
// package stays cgo-free. All values below are for amd64 / arm64 Linux and
// verified byte-for-byte against `linux/uinput.h`:
//
//	UI_DEV_CREATE  = _IO ('U', 1)              = 0x5501
//	UI_DEV_DESTROY = _IO ('U', 2)              = 0x5502
//	UI_DEV_SETUP   = _IOW('U', 3, 92)          = 0x405c5503
//	UI_SET_EVBIT   = _IOW('U', 100, sizeof int) = 0x40045564
//	UI_SET_KEYBIT  = _IOW('U', 101, sizeof int) = 0x40045565
//	UI_SET_RELBIT  = _IOW('U', 102, sizeof int) = 0x40045566
//	UI_SET_ABSBIT  = _IOW('U', 103, sizeof int) = 0x40045567
//
// # Testing
//
// The package factors the event encoder (`EncodeEvent`, `Encoded`) out of the
// Linux-specific ioctl sequence (`Open`, `configure`, `Close`) so the wire
// format can be verified byte-for-byte on any host. The ioctl path is
// exercised only on Linux with a real /dev/uinput; tests that need a Writer
// inject a `bytes.Buffer` via `NewWriter(w)`.
package uinput
