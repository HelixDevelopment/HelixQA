---
title: Helix Nexus — iOS real-device lane
phase: 2
---

# iOS real-device lane (P2-07)

This runbook describes how to stand up an iOS real-device lane for
Nexus mobile tests without exposing an Apple ID or DEV-ID.

## Prerequisites

- macOS host (Intel or Apple Silicon) with Xcode 15+ installed.
- An Apple Developer team with a dedicated "CI / QA" role seat.
- A dedicated provisioning profile named `HelixQA-WebDriverAgent` scoped
  to the target devices. Profile rotation is monthly; automate the
  renewal via the App Store Connect API.
- The physical device must be paired (`idevice_id -l` must list its UDID).

## Build WebDriverAgent

```bash
git clone https://github.com/appium/WebDriverAgent
cd WebDriverAgent

xcodebuild -project WebDriverAgent.xcodeproj \
  -scheme WebDriverAgentRunner \
  -destination "id=<udid>" \
  -derivedDataPath ./build \
  CODE_SIGN_STYLE=Manual \
  DEVELOPMENT_TEAM=<team-id> \
  PROVISIONING_PROFILE_SPECIFIER=HelixQA-WebDriverAgent \
  test
```

The `test` target keeps WDA running on the device until interrupted.

## Point Nexus at the running agent

```go
caps := mobile.NewIOSCaps("iPhone 15 Pro", "com.example.app", udid,
    "<team-id>", "iPhone Developer")
caps.WebDriverAgentURL = "http://192.168.1.50:8100" // the mac mini
```

`WebDriverAgentURL` tells Appium to skip the default "build & install WDA"
step. The hub still mediates all commands so Nexus never talks to the
agent directly.

## Appium hub

Run Appium 2.0 on the same mac host:

```bash
appium --base-path=/ --port=4723
```

## Secret hygiene

- The team ID is not secret but the provisioning profile is; store it in
  `~/.helixqa/profiles/` mode 0600.
- Never commit certificates or profiles; CI-less local runs reload them
  from `~/.helixqa/` on every invocation.
- Rotate the QA seat's App Store Connect API key monthly.

## Recovery

If a device gets into a stuck state:

1. `idevicedate -u <udid> -c` — confirm it's alive.
2. `ideviceinstaller -u <udid> -l` — list apps; ensure WDA is installed.
3. Re-run the `xcodebuild ... test` step above. That redeploys WDA.
