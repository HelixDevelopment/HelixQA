// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package hash implements perceptual hashing primitives for the HelixQA
// Phase-2 perception tier. See docs/openclawing/OpenClawing4.md §5.8
// (stagnation / change-point detection tiers).
//
// Planned contents:
//
//   - dhash.go       — dHash-64 and dHash-256 via corona10/goimagehash. Sub-
//                      millisecond per 1080p frame on CPU; the tier-1
//                      "did the screen change at all?" primitive.
//   - phash.go       — pHash / wHash wrappers (DCT / wavelet bases). Used
//                      as fallback when dHash is too aggressive.
//   - block_mean.go  — BlockMean via ajdnik/imghash for partial-screen
//                      change detection (which 4×4 tile of the UI moved).
//
// Interface target (hash.Hasher):
//
//	type Hasher interface {
//	    Hash(img image.Image) (uint64, error)
//	    Distance(a, b uint64) int
//	}
//
// Nothing is implemented in this commit — the package exists so that
// pkg/autonomous/stagnation.go can start importing
// `digital.vasic.helixqa/pkg/vision/hash.Hasher` in Phase 2 without a
// dependency inversion. See Phase 2 kickoff notes in
// OpenClawing4-Handover.md §4.
package hash
