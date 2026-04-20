// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package regression implements visual-regression primitives for HelixQA
// Phase-2 pixel-diff reports. See OpenClawing4.md §5.9.
//
// Planned contents:
//
//   - pixelmatch.go — Port of mapbox/pixelmatch (150 LoC, MIT, AA-aware
//                     with YIQ colour diff). The fast "did any pixel
//                     change?" primitive.
//   - deltae.go     — CIEDE2000 perceptual colour delta on changed tiles
//                     (brand-compliance / dark-mode verification).
//   - report.go     — HTML reporter emitting per-session analysis under
//                     docs/reports/qa-sessions/.../analysis/ (reg-cli
//                     compatible).
//
// Interface target (regression.Differ):
//
//	type Differ interface {
//	    Diff(a, b image.Image, opts DiffOptions) (DiffReport, error)
//	}
//
// Nothing is implemented in this commit — placeholder for Phase 2.
package regression
