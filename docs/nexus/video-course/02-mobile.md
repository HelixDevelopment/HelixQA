---
module: 02
title: "Unified Appium from Go — Nexus mobile engine"
length: 14 minutes
---

# 02 — Unified Appium from Go

## Shot list

1. **00:00 — Cold open.** Side-by-side video of iOS + Android apps
   being driven by the same HelixQA test bank.
2. **00:45 — Capability builders.** Live-code `NewAndroidCaps` and
   `NewIOSCaps`. Show `Validate()` rejecting a missing bundleId.
3. **02:30 — Session round-trip.** `NewSession` → `FindElement` →
   `Click` against a mock Appium hub (httptest.Server).
4. **05:00 — Gestures deep-dive.** Tap, swipe, pinch, rotate. Show the
   duration mapping between Android ms and iOS seconds.
5. **07:30 — Accessibility tree.** Parse both Android UIA XML and iOS
   XCUITest XML; show `Find(class=Button)` in the REPL.
6. **09:00 — Recording.** StartRecording / perform flow /
   StopRecording → MP4 bytes. Point to the evidence vault landing.
7. **10:30 — iOS real-device lane.** Reference the WDA build runbook;
   show the `WebDriverAgentURL` reuse pattern.
8. **12:30 — Android TV variant.** Swap platform; show channels flow.
9. **13:30 — Outro.**

## VO script & demo excerpt

> "The new `mobile` package is a pure-Go Appium client. It never
> needs a native SDK, never needs CGo, and the tests in this module
> run without touching a real device."

## Exercise

Write a bank case that logs in to the Catalogizer mobile app, opens
the share sheet, and asserts the share sheet contains the expected
app icons. Stub the Appium hub with `httptest.Server`. Starter file at
`tools/nexus-course/02-mobile/exercise.go`.
