// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package text implements text-region detection for HelixQA Phase-2, used
// to hint a grounding VLM with candidate clickable-text bounding boxes
// before LLM inference. See OpenClawing4.md §5.4.4.
//
// Planned contents:
//
//   - east.go — cv::dnn::TextDetectionModel_EAST via gocv DNN. 13 FPS on
//               720p CPU; cuts LLM token cost by feeding it structured
//               ROI hints instead of the whole screen.
//   - mser.go — MSER + Stroke Width Transform for well-structured UI
//               chrome (buttons on solid backgrounds).
//
// Interface target (text.Detector):
//
//	type Detector interface {
//	    Detect(img image.Image) ([]Region, error)
//	}
//
// Nothing is implemented in this commit — placeholder for Phase 2.
package text
