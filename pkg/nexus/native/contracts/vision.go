// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import "context"

// Rect is an axis-aligned bounding box in screen coordinates.
// All values are in pixels, with origin at the top-left corner.
type Rect struct {
	X, Y int
	W, H int
}

// UIElement represents a single interactive widget detected by the vision
// pipeline (button, text field, list item, etc.).
type UIElement struct {
	Kind       string
	Rect       Rect
	Label      string
	Confidence float64
	// Source identifies which detection backend found this
	// element: "cv" (computer vision), "dom", "ax" (accessibility
	// tree), or "merged".
	Source     string
	Attributes map[string]string
}

// OCRBlock is a contiguous run of recognised text and its screen location.
type OCRBlock struct {
	Text       string
	Rect       Rect
	Confidence float64
	Lang       string
}

// OCRResult collects all OCR blocks from a single frame analysis.
type OCRResult struct {
	Blocks   []OCRBlock
	FullText string
}

// ChangeRegion describes a rectangular area that changed between two frames.
type ChangeRegion struct {
	Rect       Rect
	Magnitude  float64
	PixelCount int
}

// DiffResult summarises the pixel-level diff between two frames.
type DiffResult struct {
	Regions    []ChangeRegion
	TotalDelta float64
	SameShape  bool
}

// Match is one occurrence of a template found inside a frame.
type Match struct {
	Rect       Rect
	Confidence float64
}

// Template is a reference image used for template matching.
type Template struct {
	Name  string
	Bytes []byte
	// Mask is an optional alpha mask; nil means no masking.
	Mask []byte
}

// Analysis is the rich output produced by VisionPipeline.Analyze for a single
// frame.
type Analysis struct {
	Elements        []UIElement
	TextRegions     []OCRBlock
	DetectedChanges []ChangeRegion
	Confidence      float64
	// DispatchedTo is the worker or model that produced this analysis.
	DispatchedTo string
	LatencyMs    int
}

// VisionPipeline is the interface that vision backends must implement.
// All methods are safe for concurrent use.
type VisionPipeline interface {
	// Analyze performs a full UI+OCR+change analysis on a single frame.
	Analyze(ctx context.Context, frame Frame) (*Analysis, error)

	// Match searches frame for all occurrences of tmpl above the backend's
	// default confidence threshold.
	Match(ctx context.Context, frame Frame, tmpl Template) ([]Match, error)

	// Diff computes the pixel-level difference between two frames captured at
	// different times.
	Diff(ctx context.Context, before, after Frame) (*DiffResult, error)

	// OCR extracts text from the sub-region of frame defined by region.
	// Pass a zero-value Rect to OCR the entire frame.
	OCR(ctx context.Context, frame Frame, region Rect) (OCRResult, error)
}
