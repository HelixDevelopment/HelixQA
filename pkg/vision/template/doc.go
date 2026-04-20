// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package template wraps OpenCV's template-matching for HelixQA Phase-2
// "is the logo still on screen?" regression checks. See OpenClawing4.md
// §5.4.1.
//
// Planned contents:
//
//   - match.go — cv::matchTemplate with TM_CCOEFF_NORMED via gocv, ROI-aware.
//                Used for button/icon presence confirmation, NOT for
//                locating clickable targets (too brittle under scaling /
//                anti-aliasing — use a grounding VLM for that).
//
// Interface target (template.Matcher):
//
//	type Matcher interface {
//	    Match(img, tmpl image.Image, mask *image.Image) (Region, float64, error)
//	}
//
// Nothing is implemented in this commit — placeholder for Phase 2.
package template
