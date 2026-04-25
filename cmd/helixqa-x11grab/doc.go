// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-x11grab is the X11 capture sidecar the HelixQA Go host
// spawns when pkg/capture/linux.NewX11GrabFactory is active. It shells out to
// ffmpeg with `-f x11grab`, reads ffmpeg's raw H.264 Annex-B bytestream,
// splits it into NAL units, and emits one envelope per NAL on its own stdout
// in the format documented by pkg/capture/linux.EncodeEnvelope
// (see docs/openclawing/OpenClawing4.md §5.1.1).
//
// The binary is intentionally small — it adds ~200 LoC of Go around a
// subprocess. No CGO, no linkage to libav/ffmpeg; the child process does all
// the heavy lifting.
//
// # Usage
//
//	helixqa-x11grab --display :0 --width 1920 --height 1080 --fps 30
//
// Argv:
//
//	--display <str>   X11 DISPLAY to capture (defaults to $DISPLAY, then ":0")
//	--width  <int>    Captured width in pixels (required)
//	--height <int>    Captured height in pixels (required)
//	--fps    <int>    Capture framerate in Hz (default 30)
//	--ffmpeg <path>   ffmpeg binary path (default "ffmpeg")
//	--extra  <str>    additional ffmpeg argv, space-separated (placed before -i)
//	--health          prints "ok\n" and exits 0 (sidecar health-probe contract)
//
// # Output
//
// Stdout carries the envelope stream consumed by pkg/capture/linux.SidecarRunner.
// One envelope per NAL: [4-byte BE body_len][8-byte BE pts_us][NAL bytes].
// PTS is monotonic microseconds since Start — ffmpeg's internal PTS is not
// surfaced here because the sidecar's consumers care about wall-clock
// alignment with concurrent screenshots, not encoder timing.
//
// Stderr carries ffmpeg's own log output (muted to "error" level by default)
// plus any wrapper diagnostics. SidecarRunner tees stderr into the QA session
// archive.
//
// # Signals
//
// SIGINT / SIGTERM trigger graceful ffmpeg shutdown; the wrapper waits up to
// 5s for ffmpeg to flush and exit before force-killing. `--health` bypasses
// the whole capture path.
package main
