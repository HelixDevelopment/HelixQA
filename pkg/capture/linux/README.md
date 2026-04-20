# pkg/capture/linux

Wayland-first Linux screen capture subsystem for HelixQA, layered so every
backend shares the same `Source` contract.

## Quick start

```go
import capturelinux "digital.vasic.helixqa/pkg/capture/linux"

src, err := capturelinux.NewDefaultSource(capturelinux.ServiceConfig{
    Width:  1920,
    Height: 1080,
})
if err != nil { return err }
if err := src.Start(ctx); err != nil { return err }
defer src.Stop()

for f := range src.Frames() { /* … */ }
```

`NewDefaultSource` resolves a backend from `HELIX_LINUX_CAPTURE`, then
`XDG_SESSION_TYPE`, defaulting to `BackendPortal`. It wires the production
`dbusportal.DBusCallerFactory` and HelixQA-native sidecar binaries
(`helixqa-capture-linux`, `helixqa-kmsgrab`, `helixqa-x11grab`).

## Backends

| Backend | Factory | Requires | Notes |
|---|---|---|---|
| Portal | `NewPortalFactory` | `xdg-desktop-portal`, `pipewire`, `helixqa-capture-linux` | Wayland-correct; default on GNOME / KDE Plasma 6 / Hyprland. User consent dialog on first Start. |
| KMSGrab | `NewKMSGrabFactory` | `helixqa-kmsgrab` with `cap_sys_admin+ep` | Zero-copy DRM capture; operator installs with `setcap` once, no runtime sudo. |
| X11Grab | `NewX11GrabFactory` | `ffmpeg`, `helixqa-x11grab` | Legacy X11 fallback. `helixqa-x11grab` wraps ffmpeg x11grab in the envelope format — see `cmd/helixqa-x11grab/`. |
| XCBShm | `XCBShmFactory` | — | **Not implemented.** Returns `ErrXCBShmNotImplemented`. X11Grab covers the surface today. |

## Envelope wire format

Every HelixQA-native capture sidecar emits this format on its stdout:

```
[4-byte BE body_length uint32]
[8-byte BE pts_micros uint64, sentinel ^uint64(0) means "no timestamp"]
[body_length bytes of payload]
```

See `sidecar.go` for the reference decoder.

## Coexistence with the legacy `pkg/capture` package

The parent `pkg/capture` directory contains a pre-existing `DesktopCapture`
type (linux_capture.go) that uses its own `pkg/capture.Frame` type and
shells out to gstreamer directly. That legacy path remains untouched; this
subpackage is an additive Wayland-correct alternative.

Consumers migrate by:

1. Replace `pkg/capture.DesktopCapture` with `capturelinux.NewDefaultSource`.
2. Replace `pkg/capture.Frame` with `pkg/capture/frames.Frame`.
3. Drop the gstreamer command-line dependency; use HelixQA sidecars (shipped
   separately).

Migration is per-call-site — there is no global flag that swaps the whole
subsystem. When every call-site has migrated, `pkg/capture/linux_capture.go`
can be removed.

## Testing the install

Use `cmd/helixqa-capture-demo`:

```
go install digital.vasic.helixqa/cmd/helixqa-capture-demo
helixqa-capture-demo --platform linux --width 1920 --height 1080 --duration 3s
```

See `banks/phase1-gocore.yaml` and `banks/capture-linux.yaml` for the full
test bank coverage.
