// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package libei holds the Wayland-correct input-emulation stack HelixQA uses
// on modern Linux desktops (OpenClawing4 §5.2.1). It has two layers:
//
//   - The xdg-desktop-portal RemoteDesktop handshake, which returns a Unix
//     socket FD the EI (Emulated Input) client uses to speak libei's binary
//     protocol. This layer is implemented here — `Portal` in portal.go
//     drives CreateSession → SelectDevices → Start → ConnectToEIS.
//
//   - The EI wire protocol itself — sending KeyDown / KeyUp / PointerMotion /
//     Button / ScrollDiscrete events over the socket returned by
//     ConnectToEIS. This layer is NOT in this commit; it will land in a
//     follow-up commit that implements the flatbuffers-based wire format
//     documented at https://gitlab.freedesktop.org/libinput/libei.
//
// The separation is deliberate: the RemoteDesktop handshake is well-defined
// and testable today (the portal client mirrors the pkg/capture/linux
// ScreenCast portal with matching test patterns), and callers that already
// have an EI client (e.g. libei itself, bound through cgo by a separate
// project) can use Portal.ConnectToEIS to obtain the FD. HelixQA's own
// pure-Go EI client lands separately.
//
// All code in this package is CGO-free; the shared D-Bus plumbing lives in
// pkg/bridge/dbusportal.
package libei
