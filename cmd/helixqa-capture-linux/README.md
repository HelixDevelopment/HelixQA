# cmd/helixqa-capture-linux

Native sidecar that captures a Wayland / X11 screen via PipeWire and emits
envelope-framed H.264 NAL units on stdout for `pkg/capture/linux`'s
`PortalFactory` to consume.

**This directory ships build recipes and documentation, not a committed
binary.** C + GStreamer linkage differs across distros; operators build
locally against their system's headers, or pull from a distribution
container image.

## Contract

Stdout envelope format (matches `pkg/capture/linux.EncodeEnvelope`):

```
[4-byte BE body_length uint32]
[8-byte BE pts_micros uint64, sentinel ^uint64(0) for "no PTS"]
[body_length bytes of H.264 Annex-B payload]
```

Stderr: GStreamer diagnostics + wrapper log lines.

Argv:

```
helixqa-capture-linux --pipewire-fd <N> --node <M> [--bitrate 8000000]
helixqa-capture-linux --health              # prints "ok\n", exits 0
```

The `--pipewire-fd` value is the PipeWire FD the HelixQA Go host obtained
from `OpenPipeWireRemote` and passed via `exec.Cmd.ExtraFiles[0]` (so fd=3
by convention). `--node` is the PipeWire stream node id from the portal's
`Start` response.

## Build

```sh
# Operator installs build deps ONCE. HelixQA runtime never uses "sudo";
# these are one-time manual steps documented for the operator's reference.

# Fedora / RHEL:
# dnf install gstreamer1-devel gstreamer1-plugins-base-devel \
#             gstreamer1-plugins-good gstreamer1-plugins-bad \
#             pipewire-devel pkgconf

# Debian / Ubuntu:
# apt install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev \
#             gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
#             libpipewire-0.3-dev pkg-config

# Arch:
# pacman -S --needed gstreamer gst-plugins-base gst-plugins-good \
#                    gst-plugins-bad pipewire pkgconf

make                           # produces ./helixqa-capture-linux
install -m 0755 helixqa-capture-linux ~/.local/bin/
```

## Source layout (to be written)

```
cmd/helixqa-capture-linux/
├── README.md              (this file)
├── Makefile               GStreamer pkg-config + cflags/ldflags
├── build.sh               wrapper that picks system vs. container build
├── src/
│   ├── main.c             argv parsing + gst init + signal handling
│   ├── pipeline.c         pipewiresrc → videoconvert → x264enc → appsink
│   ├── envelope.c         EncodeEnvelope reference implementation (C)
│   └── envelope.h
└── tests/
    └── envelope_test.c    byte-exact round-trip vs. Go reference
```

## Why not Go + cgo?

GStreamer's C API is big; cgo bindings add ~2 MB to every HelixQA binary
that accidentally imports them. The sidecar boundary keeps the Go host
CGO-free and makes ABI upgrades surgical — when GStreamer 1.28 ships with
new `pipewiresrc` semantics, the fix is rebuilding one binary, not the
entire stack.

## Operator quickstart

1. Install build deps (see above).
2. `cd cmd/helixqa-capture-linux && make`
3. `install -m 0755 helixqa-capture-linux ~/.local/bin/`
4. Verify: `helixqa-capture-linux --health` → `ok`
5. Smoke test: `helixqa-capture-demo --platform linux --duration 3s`

## Reference implementation

A starter `src/` is not committed in this release. See the scrcpy-server
documentation's PipeWire sample at:
<https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md> for the
equivalent Android pattern, and the GStreamer pipewire wiki at:
<https://gstreamer.freedesktop.org/documentation/pipewire/pipewiresrc.html>
for the src element's properties.

## Status

Planned. Release blocker for `BackendPortal` end-to-end; stubbed out by
`pkg/capture/linux/xcbshm.go`-style placeholder if absent. Tracked in
`docs/openclawing/OpenClawing4-Handover.md` §3.1.
