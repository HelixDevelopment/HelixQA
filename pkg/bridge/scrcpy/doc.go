// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package scrcpy implements a pure-Go client for the scrcpy-server v3 wire
// protocol. It replaces the HelixQA dependency on the `scrcpy` desktop binary
// with a direct speaker of the server's three sockets — video, audio, control.
//
// Why not reuse the desktop scrcpy binary?
//
//   - Packaging fragility: the desktop binary links SDL2 + FFmpeg; HelixQA
//     containers would pull in a full desktop stack for what is ultimately an
//     H.264 decoder plus input writer.
//   - Stream multiplexing: HelixQA often drives several devices in parallel
//     from one orchestrator; managing N subprocesses each owning a TCP tunnel
//     is less robust than N goroutines sharing one Go event loop.
//   - Control-channel flexibility: we need to emit synthetic UHID events,
//     clipboard reads, and rotation commands at arbitrary moments — most
//     exposed by scrcpy v3 but not all plumbed through the desktop binary's
//     CLI surface.
//
// Wire format reference: https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md
// Protocol version pinned by the JAR bundled under testdata/; the HelixQA
// orchestrator refuses to start if the JAR's reported version differs.
//
// Scope of this package:
//
//   - protocol.go — Types, constants, encoders/decoders for control and device
//     messages, and the VideoPacket / AudioPacket readers.
//   - devguard.go — `.devignore` enforcement against `adb shell getprop
//     ro.product.model`. Called before any code in this package opens a
//     control socket.
//   - server.go (future) — ADB forward setup + `app_process` launch + 3-socket
//     accept. Requires an adb binary at runtime; unit tests cover only the
//     wire format.
//   - session.go (future) — High-level Session(Send, Frames, Close).
//
// Phase 1 M3 delivers protocol.go + devguard.go with full unit coverage.
// server.go and session.go arrive later in Phase 1 once an adb-capable CI
// environment is available.
//
// See docs/openclawing/OpenClawing4.md §5.1.3 / §7.5.
package scrcpy
