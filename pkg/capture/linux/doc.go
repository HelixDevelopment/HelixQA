// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package linux holds the Wayland-first Linux capture stack used by HelixQA.
// See docs/openclawing/OpenClawing4.md §5.1.1.
//
// Layers:
//
//   - sidecar.go   — SidecarRunner, the generic "spawn a capture binary, read
//                    length-framed envelopes from its stdout, publish
//                    frames.Frame" plumbing. Every other backend in this
//                    package wraps SidecarRunner.
//   - router.go    — NewSource dispatch that picks between portal, kmsgrab,
//                    and the legacy x11grab path based on HELIX_LINUX_CAPTURE
//                    + XDG_SESSION_TYPE.
//   - portal.go    — xdg-desktop-portal ScreenCast client (godbus). Provides
//                    the PipeWire FD + stream list the capture sidecar needs.
//   - kmsgrab.go   — capability-granted sidecar path (requires operator
//                    installation with cap_sys_admin+ep); optional.
//   - xcbshm.go    — pure X11 fallback (future; today callers drop back to
//                    pkg/capture.AndroidCapture's X11 path via x11grab).
//
// # Envelope wire format
//
// Every capture sidecar emits frames as a stream of envelopes on its stdout:
//
//	[4-byte BE body_length uint32]
//	[8-byte BE pts_micros uint64, sentinel ^uint64(0) means "no timestamp"]
//	[body_length bytes of payload (H.264 Annex-B or raw NV12)]
//
// The envelope is deliberately simple so C / Rust / Go sidecars can all
// encode it in a few lines. SidecarRunner is the reference decoder; the
// round-trip invariant (EncodeEnvelope -> ReadEnvelope) is verified in
// sidecar_test.go byte-exact.
//
// # No CGO on the Go host
//
// Sidecar binaries (helixqa-capture-linux C+GStreamer, helixqa-kmsgrab C+DRM)
// live in cmd/* and have their own build chains. The HelixQA Go host only
// speaks the envelope format, so CGO_ENABLED=0 stays intact.
package linux
