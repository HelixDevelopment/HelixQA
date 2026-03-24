// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package issuedetector provides LLM-powered bug detection
// during autonomous QA sessions. It analyzes screen states,
// user actions, and navigation patterns to identify visual,
// UX, accessibility, functional, performance, and crash issues.
package issuedetector

import "time"

// IssueCategory classifies the type of issue detected.
type IssueCategory string

const (
	// CategoryVisual covers visual bugs (truncation, overlap, misalignment).
	CategoryVisual IssueCategory = "visual"
	// CategoryUX covers user experience issues (confusing navigation, missing feedback).
	CategoryUX IssueCategory = "ux"
	// CategoryAccessibility covers accessibility problems (low contrast, missing labels).
	CategoryAccessibility IssueCategory = "accessibility"
	// CategoryFunctional covers functional bugs (wrong behavior, broken features).
	CategoryFunctional IssueCategory = "functional"
	// CategoryPerformance covers performance issues (slow load, jank).
	CategoryPerformance IssueCategory = "performance"
	// CategoryCrash covers crash and ANR issues.
	CategoryCrash IssueCategory = "crash"
)

// AllCategories returns all valid issue categories.
func AllCategories() []IssueCategory {
	return []IssueCategory{
		CategoryVisual,
		CategoryUX,
		CategoryAccessibility,
		CategoryFunctional,
		CategoryPerformance,
		CategoryCrash,
	}
}

// ValidCategory checks if a string is a valid IssueCategory.
func ValidCategory(s string) bool {
	for _, c := range AllCategories() {
		if string(c) == s {
			return true
		}
	}
	return false
}

// IssueSeverity defines severity levels.
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityHigh     IssueSeverity = "high"
	SeverityMedium   IssueSeverity = "medium"
	SeverityLow      IssueSeverity = "low"
)

// AllSeverities returns all valid severities in priority order.
func AllSeverities() []IssueSeverity {
	return []IssueSeverity{
		SeverityCritical, SeverityHigh,
		SeverityMedium, SeverityLow,
	}
}

// ValidSeverity checks if a string is a valid severity.
func ValidSeverity(s string) bool {
	for _, sev := range AllSeverities() {
		if string(sev) == s {
			return true
		}
	}
	return false
}

// Issue represents a detected problem.
type Issue struct {
	// ID is a unique identifier.
	ID string `json:"id"`

	// Category classifies the issue type.
	Category IssueCategory `json:"category"`

	// Severity indicates priority.
	Severity IssueSeverity `json:"severity"`

	// Title is a short summary.
	Title string `json:"title"`

	// Description is a detailed explanation.
	Description string `json:"description"`

	// Platform where the issue was found.
	Platform string `json:"platform"`

	// ScreenID where the issue was observed.
	ScreenID string `json:"screen_id,omitempty"`

	// Evidence lists paths to screenshots/logs.
	Evidence []string `json:"evidence,omitempty"`

	// Suggestion is a recommended fix.
	Suggestion string `json:"suggestion,omitempty"`

	// Confidence is the detection confidence (0-1).
	Confidence float64 `json:"confidence"`

	// Timestamp is when the issue was detected.
	Timestamp time.Time `json:"timestamp"`
}
