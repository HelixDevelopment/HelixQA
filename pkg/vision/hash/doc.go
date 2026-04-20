// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package hash implements perceptual hashing primitives for the HelixQA
// Phase-2 perception tier. See docs/openclawing/OpenClawing4.md §5.8
// (stagnation / change-point detection tiers).
//
// Contents:
//
//   - dhash.go       — ✅ dHash-64 and dHash-256, pure-Go (no CGO, no
//                      external deps). ~1 µs per 1080p frame via
//                      nearest-neighbor + fast-paths for RGBA / NRGBA /
//                      Gray / YCbCr. The tier-1 "did the screen change
//                      at all?" primitive. Shipped M28.
//   - phash.go       — ✅ pHash via 32×32 DCT-II + 8×8 low-freq
//                      median split. More shift/rotation-robust than
//                      dHash. ~175 µs / 1080p CPU. Shipped M45.
//   - block_mean.go  — ⏳ BlockMean for partial-screen change detection
//                      (which 4×4 tile of the UI moved).
//
// Interface (hash.Hasher) — satisfied by DHasher{Kind: DHash64}:
//
//	type Hasher interface {
//	    Hash(img image.Image) (uint64, error)
//	    Distance(a, b uint64) int
//	}
//
// The 256-bit variant uses DHasher.Hash256 → *BigHash + BigHash.Distance
// directly; see dhash.go for the full type contract.
package hash
