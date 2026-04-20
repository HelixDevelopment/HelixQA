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
//   - deltae.go     — ✅ CIEDE2000 perceptual colour delta (Sharma 2005
//                     reference-validated), sRGB→CIELAB via D65,
//                     CheckBrandCompliance helper. Pure Go, ~200 LoC.
//                     Shipped M43.
//   - report.go     — ✅ Self-contained HTML reporter. Embeds every
//                     PNG as a base64 data URL (template.URL to
//                     bypass html/template's data-URL sanitization),
//                     inline CSS, no external assets. Sessions +
//                     Summary + optional BrandComplianceReport panels.
//                     Shipped M44.
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
