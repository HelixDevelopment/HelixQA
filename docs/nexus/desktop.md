---
title: Helix Nexus — Desktop Engine
phase: 3
status: ready
---

# Helix Nexus — Desktop Engine

Three platform-specific drivers behind one `desktop.Engine` interface:

| Platform | Driver | Transport |
|---|---|---|
| Windows | WinAppDriver (UIA) | HTTP on 127.0.0.1:4723 |
| macOS   | XCUITest + AppleScript | `osascript` + `screencapture` |
| Linux   | AT-SPI (primary) + X11/xdotool (fallback) + Wayland (wtype) | DBus + shell helpers |

## Connecting

- **Windows**: launch WinAppDriver.exe as current user; never bind to
  `0.0.0.0`. The engine refuses to attach to empty session ids.
- **macOS**: no hub required; `osascript` + `screencapture` ship with
  the OS.
- **Linux**: install `at-spi2-core` and our tiny `atspi-find` /
  `atspi-action` / `atspi-type` helpers (vendored under
  `tools/atspi-helpers/`). Under Wayland, install `wtype` for shortcuts.

## Testing without real desktops

Every driver takes a command-runner / httptest injection so the full
Windows + macOS + Linux test suite runs on a CI-less Linux workstation
with zero GUI dependencies. See `desktop_test.go` for the pattern.

## Installers, trays, dialogs

- Windows: `FindByName("Install")` + `Click`; tray icon via
  `FindByRole("System.Tray")`.
- macOS: `PickMenu(["App", "Quit"])`; tray uses `System Events` menu bar.
- Linux: Desktop notifications accessed via the `atspi-find` bus query
  targeting the shell process.

## SQL

`docs/nexus/sql/helixqa_desktop_hosts.sql` captures every desktop host
(Windows VM, macOS runner, Linux workstation) HelixQA can reach.

## Scenarios covered by banks/nexus-desktop-*

- Launch + shortcut + menu pick across all three platforms
- MSI installer happy path (Windows)
- DMG + notarisation + Gatekeeper assessment (macOS)
- AppImage + tray icon (Linux)
- Multi-window, file-open dialog, print preview, crash-on-launch detection
