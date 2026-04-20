# cmd/helixqa-kmsgrab

Native sidecar that captures via the Linux KMS (DRM) interface and emits
envelope-framed H.264 on stdout for `pkg/capture/linux`'s `KMSGrabFactory`.

**Capability requirement:** the operator runs `setcap cap_sys_admin+ep` on
this binary **once** at install time. HelixQA's runtime NEVER elevates
privileges; the capability is granted out-of-band by the operator.

## Contract

Same envelope format as `cmd/helixqa-capture-linux/README.md`.

Argv:

```
helixqa-kmsgrab --connector <HDMI-A-1|DP-1|...> [--fps 30] [--bitrate 6000000]
helixqa-kmsgrab --list-connectors    # enumerates DRM connectors + exits
helixqa-kmsgrab --health              # prints "ok\n", exits 0
```

## Build

```sh
# Fedora:
# dnf install libdrm-devel libva-devel ffmpeg-devel pkgconf

# Debian / Ubuntu:
# apt install libdrm-dev libva-dev libavformat-dev libavcodec-dev pkg-config

make                           # produces ./helixqa-kmsgrab
install -m 0755 helixqa-kmsgrab ~/.local/bin/

# Operator grants cap once, then never again:
# (operator command; documented here so HelixQA itself never issues it)
# setcap cap_sys_admin+ep ~/.local/bin/helixqa-kmsgrab
```

## Source layout (to be written)

```
cmd/helixqa-kmsgrab/
├── README.md              (this file)
├── Makefile               libdrm + VA-API pkg-config
├── build.sh               build helper with capability-test probe
├── src/
│   ├── main.c             argv + DRM open + VA-API init
│   ├── drm.c              drmModeGetResources + framebuffer grabber
│   ├── vaapi.c            zero-copy H.264 encode via VA-API
│   ├── envelope.c         EncodeEnvelope reference (shared with capture-linux)
│   └── envelope.h
└── tests/
    └── drm_probe.c        lists available connectors; skippable without DRM
```

## Why kmsgrab and not just portal?

KMS-direct skips the compositor; it's the only way to capture a headless
display (one without a running compositor), and it delivers lower-latency
output on hosts already using VA-API. Common for CI servers running
automated QA against offline desktops.

## Status

Planned. Optional — portal covers most deployments; kmsgrab only needed
for headless / high-throughput hosts. Tracked in
`docs/openclawing/OpenClawing4-Handover.md` §3.1.
