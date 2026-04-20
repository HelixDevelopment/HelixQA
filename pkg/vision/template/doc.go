// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package template wraps OpenCV's template-matching for HelixQA Phase-2
// "is the logo still on screen?" regression checks. See OpenClawing4.md
// §5.4.1.
//
// Contents:
//
//   - template.go — ✅ Pure-Go NCC (normalized cross-correlation)
//                   matcher on Rec-709 luma. No CGO, no OpenCV
//                   dep — the doc-planned gocv wrapper was replaced
//                   with the straight O(N·M·w·h) loop because
//                   matchTemplate's SIMD advantage is minor at the
//                   typical 50-pixel needle + 1080p haystack sizes
//                   HelixQA uses, and staying CGO-free keeps the
//                   build reproducible across every platform.
//                   Shipped M49.
package template
