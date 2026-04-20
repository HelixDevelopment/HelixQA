// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package flow wraps OpenCV optical-flow algorithms for HelixQA Phase-2
// motion analysis. See OpenClawing4.md §5.4.3 (DIS optical flow on CPU,
// NVOF 2.0 on GPU).
//
// Contents:
//
//   - flow.go — ✅ Pure-Go Lucas-Kanade sparse optical flow
//               (Lucas & Kanade 1981). Per-grid-point velocity
//               vectors + Median summary for dominant direction
//               (scroll / pan / animation detection). Shipped M50.
//               Replaces the doc-planned gocv DIS wrapper —
//               LK on a 16-pixel grid is sufficient for HelixQA's
//               "is the list scrolling or frozen?" primitive, and
//               stays CGO-free.
//   - nvof.go — ⏳ GPU-accelerated optical flow via C++ sidecar
//               (cv::cuda::NvidiaOpticalFlow_2_0). Deferred; LK is
//               already fast enough for current workloads.
package flow
