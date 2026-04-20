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
//   - web.go      — ⏳ CDP Accessibility.getFullAXTree via go-rod /
//                   chromedp (Phase 2 Step 2.6).
//   - android.go  — ✅ UIAutomator dump parser via AndroidDumper
//                   abstraction (ADBDumper shells out to
//                   `adb -s <serial> exec-out uiautomator dump /dev/tty`).
//                   24-class ARIA role mapping, bounds regex, full
//                   attribute propagation. Shipped M32.
//   - darwin.go   — ⏳ Swift sidecar emitting JSON AXUIElement tree
//                   (cmd/helixqa-axtree-darwin/, future).
//   - windows.go  — ⏳ go-ole client wrapping IUIAutomation.
//   - ios.go      — ⏳ idb describe-ui JSON parser.
//
// The unified Node type lets higher layers (navigator / action
// resolution) treat every platform uniformly — RawID is the platform-
// native identifier; Bounds + Role + Name lets the grounding VLM
// cross-check against pixel-level hypotheses.
package axtree
