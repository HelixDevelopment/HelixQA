// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-capture-demo is an operator-runnable smoke test for the
// Phase-1 Linux + Android capture stack. It's the shortest route from a
// fresh install to "captures work on this host".
//
// The demo:
//
//   - Picks a capture backend (Linux: auto/portal/kmsgrab/x11grab; Android:
//     scrcpy-direct) from argv + env.
//   - Instantiates the service layer one-liner
//     (capturelinux.NewDefaultSource or android.NewDirectFromServerConfig).
//   - Consumes Source.Frames() for a bounded duration, printing per-frame
//     metadata (PTS, width, height, format, source, payload length) to
//     stdout in a human-readable line-per-frame format.
//   - Exits 0 on clean completion, non-zero on any error.
//
// Not a production tool — the helixqa CLI itself will integrate the service
// layer in a future commit. This binary exists so operators can verify their
// D-Bus portal + sidecar installation works before wiring up the full
// autonomous pipeline.
//
// # Usage
//
//   # Auto-detected Linux capture (requires portal + helixqa-capture-linux, or kmsgrab, or x11grab):
//   helixqa-capture-demo --platform linux --width 1920 --height 1080 --duration 5s
//
//   # Force X11Grab:
//   helixqa-capture-demo --platform linux --backend x11grab --display :0 --fps 30 --width 1920 --height 1080
//
//   # Health probe (uniform across all HelixQA sidecars):
//   helixqa-capture-demo --health
//
// Android scrcpy-direct flow needs a real device and scrcpy-server.jar;
// this commit ships the Linux demo only. The Android path lands in a
// later commit once the scrcpy-server JAR pin is in place.
package main
