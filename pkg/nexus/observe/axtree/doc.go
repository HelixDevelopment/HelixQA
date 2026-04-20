// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package axtree unifies accessibility-tree access across every platform
// HelixQA targets, for deterministic target resolution where pixel-level
// vision is fuzzy. See OpenClawing4.md §5.3.
//
// Contents:
//
//   - node.go     — ✅ common Node + Snapshotter + Walk/Find/
//                   CountDescendants + PlatformLinux/Web/Android/Darwin/
//                   Windows/IOS constants. Shipped M31.
//   - linux.go    — ✅ AT-SPI2 over D-Bus (godbus on the a11y bus) with
//                   LinuxBus abstraction for test mocking. Covers 31
//                   AT-SPI role codes → ARIA mapping. Shipped M31.
//   - web.go      — ✅ CDP Accessibility.getFullAXTree via
//                   chromedp/cdproto. WebFetcher narrow abstraction
//                   (ChromedpFetcher is the production impl driving
//                   a real Chromium). Flat CDP AXNode list
//                   reassembled into tree form with cycle-guarded
//                   recursion + ignored-node hoisting. Shipped M47.
//   - android.go  — ✅ UIAutomator dump parser via AndroidDumper
//                   abstraction (ADBDumper shells out to
//                   `adb -s <serial> exec-out uiautomator dump /dev/tty`).
//                   24-class ARIA role mapping, bounds regex, full
//                   attribute propagation. Shipped M32.
//   - darwin.go   — ✅ Swift sidecar over HTTP emitting JSON
//                   AXUIElement tree. DarwinFetcher narrow
//                   abstraction (DarwinHTTPFetcher is the production
//                   impl). 44-role AXRole→ARIA mapping. Shipped M42.
//                   Swift sidecar in cmd/helixqa-axtree-darwin/
//                   remains operator-action (§10.3).
//   - windows.go  — ⏳ go-ole client wrapping IUIAutomation.
//   - ios.go      — ✅ idb describe-all JSON parser via IDBDumper
//                   abstraction (IDBShellDumper shells out to
//                   `idb ui describe-all --udid <UDID> --json`).
//                   30-type AXUIElement→ARIA mapping, AXLabel →
//                   Name fallback, array-or-object top-level tolerance.
//                   Shipped M41.
//
// The unified Node type lets higher layers (navigator / action
// resolution) treat every platform uniformly — RawID is the platform-
// native identifier; Bounds + Role + Name lets the grounding VLM
// cross-check against pixel-level hypotheses.
package axtree
