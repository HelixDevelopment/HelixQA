// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package flow wraps OpenCV optical-flow algorithms for HelixQA Phase-2
// motion analysis. See OpenClawing4.md §5.4.3 (DIS optical flow on CPU,
// NVOF 2.0 on GPU).
//
// Planned contents:
//
//   - dis.go  — cv::DISOpticalFlow via gocv. 4–8 ms on 720p CPU; the
//               "is the list scrolling or frozen?" primitive.
//   - nvof.go — cv::cuda::NvidiaOpticalFlow_2_0 via C++ sidecar with
//               gRPC + SHM; HelixQA Go calls into it for GPU-accelerated
//               optical flow when a CUDA-capable host is available.
//
// Interface target (flow.Computer):
//
//	type Computer interface {
//	    Compute(ctx context.Context, prev, next image.Image) (FlowField, error)
//	}
//
// Nothing is implemented in this commit — placeholder for Phase 2.
package flow
