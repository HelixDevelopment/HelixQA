// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package pelt implements PELT (Pruned Exact Linear Time) change-point
// detection for HelixQA Phase-2 post-session analysis. See
// OpenClawing4.md §5.8 (stagnation / change-point tiers).
//
// Contents:
//
//   - pelt.go — ✅ Pure-Go PELT (Killick, Fearnhead & Eckley 2012) with
//               pluggable cost functions (Gaussian mean-change default;
//               VarianceCost also exported). O(n²) worst case, O(n)
//               expected with pruning. Shipped M35.
//
// Interface (pelt.Segmenter) — satisfied by PELT{}:
//
//	type Segmenter interface {
//	    Segment(ctx context.Context, series []float64, penalty float64) ([]int, error)
//	}
//
// Rationale for pure-Go PELT over the Kickoff-brief ruptures-sidecar plan:
//
//   - Ruptures is Python — a sidecar dependency means gRPC between the
//     Go host and a Python process for what is ~120 LoC of dynamic
//     programming.
//   - Post-session segmentation workloads are ≤ a few thousand samples
//     — well within Go's pure-CPU comfort zone. No GPU/SIMD advantage
//     to exploit.
//   - Same decision pattern as pkg/vision/perceptual/ssim.go: keeping
//     the Go host CGO-free + sidecar-free wins across every deployment.
//
// Sibling: pkg/autonomous.BOCPD is the ONLINE change-point detector
// (fires live while a session is recording); PELT runs post-session on
// the completed time-series for optimal (offline) segmentation.
package pelt
