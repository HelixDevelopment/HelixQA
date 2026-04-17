---
module: 03
title: "Desktop UI under HelixQA"
length: 13 minutes
---

# 03 — Desktop UI under HelixQA

## Shot list

1. **00:00 — Cold open.** Three-panel screenshare: Windows VM, macOS,
   Linux. One HelixQA flow drives all three.
2. **01:00 — WinAppDriver setup.** Launch WinAppDriver on 127.0.0.1.
   Live-code `NewWindowsEngine` + `Launch("calc.exe", nil)`.
3. **03:00 — AppleScript + osascript.** Menu walk via
   `PickMenu(["File","Open..."])`. Shortcut mapping demo.
4. **05:00 — AT-SPI on Linux.** Use the vendored `atspi-find` helper
   to locate `"Save"` and click through it.
5. **06:30 — Wayland fallback.** Flip the engine to `AsWayland()` and
   try to click with xdotool — show the explicit refusal.
6. **08:00 — Installer flow.** Walk an MSI wizard with `PickMenu`.
7. **10:00 — Tray + notifications.** Find the system tray element,
   click it, read the notification.
8. **12:00 — Outro.**

## VO script excerpt

> "Desktop automation usually pulls in native dependencies. Our
> engines avoid that by talking to platform-provided WebDriver and
> accessibility services over HTTP, AppleScript, and DBus."

## Exercise

Take a five-screen installer of the Catalogizer desktop client and
write a Nexus flow that walks it from end to end. The flow must
succeed on Windows 11 + macOS Sonoma + Ubuntu 24.04 without changes.
Starter at `tools/nexus-course/03-desktop/exercise.go`.
