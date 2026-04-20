// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package perceptual implements human-aligned image similarity metrics for
// HelixQA Phase-2 verification. See OpenClawing4.md §5.8 (tier-2 / tier-3
// stagnation detection).
//
// Planned contents:
//
//   - ssim.go       — SSIM / MS-SSIM via gocv (tier 2; ~3 ms on 480p luma).
//                     Runs only when tier-1 dHash flags a suspicious frame.
//   - dreamsim.go   — DreamSim REST client against a Triton-hosted model
//                     (tier 3; 96% human agreement; used as the tiebreaker
//                     on long stagnation segments).
//   - lpips.go      — Optional LPIPS fallback when DreamSim isn't deployed.
//
// Interface target (perceptual.Comparator):
//
//	type Comparator interface {
//	    Compare(ctx context.Context, a, b image.Image) (similarity float64, err error)
//	}
//
// Nothing is implemented in this commit — placeholder for the Phase 2
// perception tier. See OpenClawing4-Handover.md §4.
package perceptual
