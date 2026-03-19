// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"context"
	"fmt"
)

// Rect represents a rectangular region on a screenshot.
type Rect struct {
	// X is the left coordinate.
	X int `json:"x"`
	// Y is the top coordinate.
	Y int `json:"y"`
	// Width is the rectangle width.
	Width int `json:"width"`
	// Height is the rectangle height.
	Height int `json:"height"`
}

// Annotation describes a single annotation on a screenshot.
type Annotation struct {
	// Description explains what this annotation highlights.
	Description string `json:"description"`
	// Region is the bounding box of the annotated area.
	Region Rect `json:"region"`
	// Severity indicates the issue severity (e.g., "critical",
	// "high", "medium", "low", "info").
	Severity string `json:"severity"`
}

// AnnotatedItem holds the result of annotating a screenshot.
type AnnotatedItem struct {
	// OriginalPath is the path to the original screenshot.
	OriginalPath string `json:"original_path"`
	// AnnotatedPath is the path to the annotated image.
	AnnotatedPath string `json:"annotated_path"`
	// Annotations lists all annotations added to the image.
	Annotations []Annotation `json:"annotations"`
}

// Annotator uses an LLM to identify and annotate issues in
// screenshots. This is an optional enhancement -- the
// existing CaptureScreenshot() method works without it.
type Annotator interface {
	// AnnotateScreenshot analyzes a screenshot and produces
	// an annotated version highlighting the given issues.
	AnnotateScreenshot(
		ctx context.Context,
		imagePath string,
		issues []string,
	) (*AnnotatedItem, error)
}

// AnnotateWith uses the provided Annotator to annotate a
// screenshot. Returns nil if the annotator is nil (graceful
// degradation).
func AnnotateWith(
	ctx context.Context,
	annotator Annotator,
	imagePath string,
	issues []string,
) (*AnnotatedItem, error) {
	if annotator == nil {
		return nil, nil
	}
	if imagePath == "" {
		return nil, fmt.Errorf("image path is required")
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf(
			"at least one issue is required for annotation",
		)
	}
	return annotator.AnnotateScreenshot(ctx, imagePath, issues)
}

// Validate checks that the AnnotatedItem has valid fields.
func (ai *AnnotatedItem) Validate() error {
	if ai.OriginalPath == "" {
		return fmt.Errorf("original path is required")
	}
	if ai.AnnotatedPath == "" {
		return fmt.Errorf("annotated path is required")
	}
	for i, ann := range ai.Annotations {
		if ann.Description == "" {
			return fmt.Errorf(
				"annotation %d: description is required", i,
			)
		}
		if ann.Severity == "" {
			return fmt.Errorf(
				"annotation %d: severity is required", i,
			)
		}
		if err := ann.Region.Validate(); err != nil {
			return fmt.Errorf("annotation %d: %w", i, err)
		}
	}
	return nil
}

// Validate checks that the Rect has non-negative dimensions.
func (r Rect) Validate() error {
	if r.Width < 0 {
		return fmt.Errorf("width must be non-negative, got %d",
			r.Width)
	}
	if r.Height < 0 {
		return fmt.Errorf("height must be non-negative, got %d",
			r.Height)
	}
	return nil
}

// Contains returns true if the point (px, py) is within this
// rectangle.
func (r Rect) Contains(px, py int) bool {
	return px >= r.X && px < r.X+r.Width &&
		py >= r.Y && py < r.Y+r.Height
}

// Area returns the area of the rectangle.
func (r Rect) Area() int {
	if r.Width <= 0 || r.Height <= 0 {
		return 0
	}
	return r.Width * r.Height
}
