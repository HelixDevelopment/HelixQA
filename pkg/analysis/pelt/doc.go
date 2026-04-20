// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package pelt wraps the ruptures PELT (Pruned Exact Linear Time)
// change-point-detection algorithm for HelixQA Phase-2 post-session
// analysis. See OpenClawing4.md §5.8 (stagnation / change-point tiers).
//
// Planned contents:
//
//   - client.go — gRPC / subprocess client against a Python-hosted
//                 ruptures implementation. Offline; runs after a session
//                 closes to segment the frame-similarity time series
//                 into "phases" (each phase's boundary marks a
//                 screen-level transition).
//
// Interface target (pelt.Segmenter):
//
//	type Segmenter interface {
//	    Segment(ctx context.Context, series []float64, penalty float64) ([]int, error)
//	}
//
// Nothing is implemented in this commit — placeholder for Phase 2.
// BOCPD (online change-point detection for live stagnation alerts) lives
// in a sibling package `pkg/analysis/bocpd` that will land in Phase 2 too.
package pelt
