---
title: Helix Nexus — Mobile Engine
phase: 2
status: ready
---

# Helix Nexus — Mobile Engine

Appium 2.0 WebDriver client in pure Go with iOS (XCUITest) + Android
(UiAutomator2) + Android TV capability profiles, gesture helpers, and an
accessibility-tree parser.

## Packages

- `pkg/nexus/mobile/appium.go` — HTTP Appium client with session lifecycle, findElement, click/sendKeys, source, screenshot, executeScript.
- `pkg/nexus/mobile/capabilities.go` — typed profile builders + validator.
- `pkg/nexus/mobile/gestures.go` — Tap, LongPress, Swipe, Scroll, Pinch, Rotate, Key (hardware buttons).
- `pkg/nexus/mobile/accessibility.go` — `ParseAccessibilityTree(xml)` → platform-neutral `AccessibilityNode`.
- `pkg/nexus/mobile/recording.go` — `StartRecording` / `StopRecording` for on-device screen recordings.
- `pkg/nexus/mobile/adapter.go` — `Engine` satisfies `nexus.Adapter`; used by the userflow bridge and cross-platform orchestrator.

## Connecting

Appium hub is expected on `http://127.0.0.1:4723` by default. Override via
`NewAppiumClient(url)`. Tests stand up an `httptest.Server` so no real hub
is required.

## iOS real devices

Requires a WebDriverAgent build signed with a developer profile. Set
`XcodeOrgID` + `XcodeSigningID` in the `Capabilities` and point
`WebDriverAgentURL` at the running WDA if you want Nexus to reuse an
existing agent. Full provisioning guide in `docs/nexus/runbooks/ios-real-devices.md` (Phase 2 P2-07).

## SQL

`docs/nexus/sql/helixqa_mobile_devices.sql` captures every device / emulator
that HelixQA touches so the dashboard can present device availability.

## Scenarios covered by banks/nexus-mobile-*

- Login + share sheet + push notification + deep link
- Permission dialog, battery-saver, airplane-mode
- Subscription upgrade (StoreKit / Google Play sandbox)
- iOS biometric unlock simulator toggles
- Android TV channel browse + playback
