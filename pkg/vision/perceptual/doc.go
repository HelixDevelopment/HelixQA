// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package perceptual implements human-aligned image similarity metrics for
// HelixQA Phase-2 verification. See OpenClawing4.md §5.8 (tier-2 / tier-3
// stagnation detection).
//
// Contents:
//
//   - ssim.go       — ✅ Pure-Go SSIM (Wang 2004) on non-overlapping 8×8
//                     blocks. < 5 ms / 480p on commodity CPU. Tier-2
//                     verifier that runs only when tier-1 dHash flags a
//                     suspicious frame. Shipped M33.
//   - dreamsim.go   — ⏳ DreamSim REST client against a Triton-hosted
//                     model (tier 3; 96% human agreement).
//   - lpips.go      — ⏳ Optional LPIPS fallback when DreamSim isn't
//                     deployed.
//
// Interface (perceptual.Comparator) — satisfied by SSIM:
//
//	type Comparator interface {
//	    Compare(ctx context.Context, a, b image.Image) (similarity float64, err error)
//	}
//
// Rationale for pure-Go SSIM over the Kickoff-brief gocv plan:
//
//   - gocv requires CGO + OpenCV dev headers on every build host,
//     contradicting HelixQA's CGO-free discipline.
//   - SSIM is ~80 LoC of arithmetic; OpenCV's advantage is NEON/AVX
//     intrinsics that the pure-Go block-based loop already beats
//     within the Phase-2 budget.
package perceptual
