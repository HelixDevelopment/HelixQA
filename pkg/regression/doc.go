// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package regression implements visual-regression primitives for HelixQA
// Phase-2 pixel-diff reports. See OpenClawing4.md §5.9.
//
// Contents:
//
//   - pixelmatch.go — ✅ Pure-Go port of mapbox/pixelmatch (MIT, AA-aware
//                     with YIQ colour diff). The fast "did any pixel
//                     change?" primitive. Shipped M30.
//   - deltae.go     — ⏳ CIEDE2000 perceptual colour delta on changed
//                     tiles (brand-compliance / dark-mode verification).
//   - report.go     — ⏳ HTML reporter emitting per-session analysis
//                     under docs/reports/qa-sessions/.../analysis/
//                     (reg-cli compatible).
//
// Interface (regression.Differ) — satisfied by PixelMatch{}:
//
//	type Differ interface {
//	    Diff(a, b image.Image, opts DiffOptions) (DiffReport, error)
//	}
//
// visual.go in the same package ships the LLM-driven screenshot
// comparison path (unchanged from pre-Phase-2) — the two Differ paths
// complement each other: pixelmatch for deterministic pixel-level diffs,
// visual.go for VLM-driven semantic comparisons.
package regression
