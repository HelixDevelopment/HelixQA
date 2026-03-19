// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAnnotator is a test double for Annotator.
type mockAnnotator struct {
	result *AnnotatedItem
	err    error
	called int
	// Captured args.
	lastImagePath string
	lastIssues    []string
}

func (m *mockAnnotator) AnnotateScreenshot(
	_ context.Context,
	imagePath string,
	issues []string,
) (*AnnotatedItem, error) {
	m.called++
	m.lastImagePath = imagePath
	m.lastIssues = issues
	return m.result, m.err
}

func TestRect_Validate_Valid(t *testing.T) {
	tests := []struct {
		name string
		rect Rect
	}{
		{"positive", Rect{10, 20, 100, 50}},
		{"zero_size", Rect{0, 0, 0, 0}},
		{"zero_width", Rect{10, 10, 0, 50}},
		{"zero_height", Rect{10, 10, 50, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.rect.Validate())
		})
	}
}

func TestRect_Validate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		rect    Rect
		message string
	}{
		{
			"negative_width",
			Rect{0, 0, -1, 10},
			"width",
		},
		{
			"negative_height",
			Rect{0, 0, 10, -1},
			"height",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rect.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestRect_Contains(t *testing.T) {
	r := Rect{10, 20, 100, 50}
	tests := []struct {
		name     string
		x, y     int
		expected bool
	}{
		{"inside", 50, 40, true},
		{"top_left", 10, 20, true},
		{"bottom_right_exclusive", 110, 70, false},
		{"just_inside_br", 109, 69, true},
		{"outside_left", 5, 40, false},
		{"outside_right", 115, 40, false},
		{"outside_top", 50, 15, false},
		{"outside_bottom", 50, 75, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected,
				r.Contains(tt.x, tt.y))
		})
	}
}

func TestRect_Area(t *testing.T) {
	tests := []struct {
		name     string
		rect     Rect
		expected int
	}{
		{"normal", Rect{0, 0, 10, 20}, 200},
		{"zero_width", Rect{0, 0, 0, 20}, 0},
		{"zero_height", Rect{0, 0, 10, 0}, 0},
		{"both_zero", Rect{0, 0, 0, 0}, 0},
		{"negative_width", Rect{0, 0, -5, 10}, 0},
		{"large", Rect{0, 0, 1920, 1080}, 2073600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.rect.Area())
		})
	}
}

func TestAnnotation_Fields(t *testing.T) {
	ann := Annotation{
		Description: "Button text truncated",
		Region:      Rect{100, 200, 80, 40},
		Severity:    "medium",
	}
	assert.Equal(t, "Button text truncated", ann.Description)
	assert.Equal(t, 100, ann.Region.X)
	assert.Equal(t, "medium", ann.Severity)
}

func TestAnnotatedItem_Validate_Valid(t *testing.T) {
	ai := &AnnotatedItem{
		OriginalPath:  "/tmp/screenshot.png",
		AnnotatedPath: "/tmp/screenshot-annotated.png",
		Annotations: []Annotation{
			{
				Description: "Low contrast text",
				Region:      Rect{10, 20, 100, 30},
				Severity:    "high",
			},
		},
	}
	assert.NoError(t, ai.Validate())
}

func TestAnnotatedItem_Validate_MissingOriginalPath(t *testing.T) {
	ai := &AnnotatedItem{
		AnnotatedPath: "/tmp/annotated.png",
	}
	err := ai.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "original path")
}

func TestAnnotatedItem_Validate_MissingAnnotatedPath(t *testing.T) {
	ai := &AnnotatedItem{
		OriginalPath: "/tmp/orig.png",
	}
	err := ai.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "annotated path")
}

func TestAnnotatedItem_Validate_BadAnnotation(t *testing.T) {
	tests := []struct {
		name    string
		ann     Annotation
		message string
	}{
		{
			"missing_description",
			Annotation{
				Description: "",
				Region:      Rect{0, 0, 10, 10},
				Severity:    "low",
			},
			"description",
		},
		{
			"missing_severity",
			Annotation{
				Description: "issue",
				Region:      Rect{0, 0, 10, 10},
				Severity:    "",
			},
			"severity",
		},
		{
			"negative_region_width",
			Annotation{
				Description: "issue",
				Region:      Rect{0, 0, -1, 10},
				Severity:    "medium",
			},
			"width",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai := &AnnotatedItem{
				OriginalPath:  "/tmp/orig.png",
				AnnotatedPath: "/tmp/ann.png",
				Annotations:   []Annotation{tt.ann},
			}
			err := ai.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestAnnotatedItem_Validate_EmptyAnnotations(t *testing.T) {
	ai := &AnnotatedItem{
		OriginalPath:  "/tmp/orig.png",
		AnnotatedPath: "/tmp/ann.png",
		Annotations:   nil,
	}
	assert.NoError(t, ai.Validate())
}

func TestAnnotateWith_Success(t *testing.T) {
	mock := &mockAnnotator{
		result: &AnnotatedItem{
			OriginalPath:  "/tmp/screen.png",
			AnnotatedPath: "/tmp/screen-annotated.png",
			Annotations: []Annotation{
				{
					Description: "Button overlap",
					Region:      Rect{50, 100, 120, 40},
					Severity:    "high",
				},
				{
					Description: "Missing label",
					Region:      Rect{200, 300, 80, 30},
					Severity:    "medium",
				},
			},
		},
	}
	ai, err := AnnotateWith(
		context.Background(),
		mock,
		"/tmp/screen.png",
		[]string{"Button overlap", "Missing label"},
	)
	require.NoError(t, err)
	require.NotNil(t, ai)
	assert.Equal(t, "/tmp/screen.png", ai.OriginalPath)
	assert.Equal(t, "/tmp/screen-annotated.png", ai.AnnotatedPath)
	assert.Len(t, ai.Annotations, 2)
	assert.Equal(t, 1, mock.called)
	assert.Equal(t, "/tmp/screen.png", mock.lastImagePath)
	assert.Equal(t, []string{"Button overlap", "Missing label"},
		mock.lastIssues)
}

func TestAnnotateWith_NilAnnotator(t *testing.T) {
	ai, err := AnnotateWith(
		context.Background(),
		nil,
		"/tmp/image.png",
		[]string{"issue"},
	)
	assert.NoError(t, err)
	assert.Nil(t, ai)
}

func TestAnnotateWith_EmptyImagePath(t *testing.T) {
	mock := &mockAnnotator{}
	ai, err := AnnotateWith(
		context.Background(),
		mock,
		"",
		[]string{"issue"},
	)
	require.Error(t, err)
	assert.Nil(t, ai)
	assert.Contains(t, err.Error(), "image path")
	assert.Equal(t, 0, mock.called)
}

func TestAnnotateWith_EmptyIssues(t *testing.T) {
	mock := &mockAnnotator{}
	ai, err := AnnotateWith(
		context.Background(),
		mock,
		"/tmp/image.png",
		nil,
	)
	require.Error(t, err)
	assert.Nil(t, ai)
	assert.Contains(t, err.Error(), "at least one issue")
	assert.Equal(t, 0, mock.called)
}

func TestAnnotateWith_EmptyIssueSlice(t *testing.T) {
	mock := &mockAnnotator{}
	ai, err := AnnotateWith(
		context.Background(),
		mock,
		"/tmp/image.png",
		[]string{},
	)
	require.Error(t, err)
	assert.Nil(t, ai)
	assert.Contains(t, err.Error(), "at least one issue")
}

func TestAnnotateWith_AnnotatorError(t *testing.T) {
	mock := &mockAnnotator{
		err: fmt.Errorf("vision API timeout"),
	}
	ai, err := AnnotateWith(
		context.Background(),
		mock,
		"/tmp/image.png",
		[]string{"issue"},
	)
	require.Error(t, err)
	assert.Nil(t, ai)
	assert.Contains(t, err.Error(), "vision API timeout")
}

func TestAnnotateWith_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock := &mockAnnotator{
		err: ctx.Err(),
	}
	ai, err := AnnotateWith(
		ctx, mock, "/tmp/img.png", []string{"issue"},
	)
	require.Error(t, err)
	assert.Nil(t, ai)
}

func TestAnnotateWith_MultipleIssues(t *testing.T) {
	issues := []string{
		"Low contrast",
		"Truncated text",
		"Overlapping elements",
		"Missing icon",
		"Wrong color",
	}
	mock := &mockAnnotator{
		result: &AnnotatedItem{
			OriginalPath:  "/img.png",
			AnnotatedPath: "/img-ann.png",
			Annotations: []Annotation{
				{Description: "Low contrast",
					Region: Rect{0, 0, 10, 10}, Severity: "high"},
				{Description: "Truncated text",
					Region: Rect{20, 20, 50, 20}, Severity: "medium"},
				{Description: "Overlapping elements",
					Region: Rect{100, 100, 80, 80}, Severity: "high"},
				{Description: "Missing icon",
					Region: Rect{200, 50, 30, 30}, Severity: "low"},
				{Description: "Wrong color",
					Region: Rect{300, 300, 40, 40}, Severity: "low"},
			},
		},
	}
	ai, err := AnnotateWith(
		context.Background(), mock, "/img.png", issues,
	)
	require.NoError(t, err)
	require.NotNil(t, ai)
	assert.Len(t, ai.Annotations, 5)
	assert.Equal(t, issues, mock.lastIssues)
}

func TestAnnotatedItem_Validate_MultipleAnnotations(t *testing.T) {
	ai := &AnnotatedItem{
		OriginalPath:  "/orig.png",
		AnnotatedPath: "/ann.png",
		Annotations: []Annotation{
			{Description: "a", Region: Rect{0, 0, 10, 10},
				Severity: "low"},
			{Description: "b", Region: Rect{20, 20, 30, 30},
				Severity: "high"},
			{Description: "c", Region: Rect{50, 50, 5, 5},
				Severity: "medium"},
		},
	}
	assert.NoError(t, ai.Validate())
}

func TestAnnotatedItem_Validate_SecondAnnotationBad(t *testing.T) {
	ai := &AnnotatedItem{
		OriginalPath:  "/orig.png",
		AnnotatedPath: "/ann.png",
		Annotations: []Annotation{
			{Description: "ok", Region: Rect{0, 0, 10, 10},
				Severity: "low"},
			{Description: "", Region: Rect{20, 20, 30, 30},
				Severity: "high"},
		},
	}
	err := ai.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "annotation 1")
	assert.Contains(t, err.Error(), "description")
}
