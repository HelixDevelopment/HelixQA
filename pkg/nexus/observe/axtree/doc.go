// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package axtree unifies accessibility-tree access across every platform
// HelixQA targets, for deterministic target resolution where pixel-level
// vision is fuzzy. See OpenClawing4.md §5.3.
//
// Planned contents:
//
//   - node.go     — common `Node{Role, Name, Value, Bounds, Enabled,
//                   Focused, Selected, Children, Platform, RawID}` type
//                   and `Snapshotter` interface.
//   - linux.go    — AT-SPI2 over D-Bus (godbus on the a11y bus).
//   - web.go      — CDP Accessibility.getFullAXTree via go-rod/chromedp.
//   - android.go  — UiAutomator2 HTTP client (existing in pkg/navigator/
//                   android/uia2_http.go; axtree wraps).
//   - darwin.go   — Swift sidecar emitting JSON AXUIElement tree
//                   (cmd/helixqa-axtree-darwin/, future).
//   - windows.go  — go-ole client wrapping IUIAutomation.
//   - ios.go      — idb describe-ui JSON parser.
//
// The unified Node type lets higher layers (navigator / action resolution)
// treat every platform uniformly — `AXNodeRawID` is the platform-native
// identifier; `Bounds + Role + Name` lets the grounding VLM cross-check
// against pixel-level hypotheses.
//
// Nothing is implemented in this commit — placeholder for Phase 2.
package axtree
