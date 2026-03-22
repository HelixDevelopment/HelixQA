// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package types provides shared types to avoid import cycles between
// issuedetector and ticket packages.
package types

import (
	"time"
)

// Issue represents a detected problem during QA testing
type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"`
	Platform    string    `json:"platform"`
	ScreenID    string    `json:"screen_id,omitempty"`
	Confidence  float64   `json:"confidence"`
	Timestamp   time.Time `json:"timestamp"`
	Suggestion  string    `json:"suggestion,omitempty"`
}

// IssueCategory represents the type of issue
type IssueCategory string

const (
	CategoryVisual        IssueCategory = "visual"
	CategoryUX            IssueCategory = "ux"
	CategoryAccessibility IssueCategory = "accessibility"
	CategoryFunctional    IssueCategory = "functional"
	CategoryPerformance   IssueCategory = "performance"
	CategoryCrash         IssueCategory = "crash"
)

// IssueSeverity represents the severity level
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityHigh     IssueSeverity = "high"
	SeverityMedium   IssueSeverity = "medium"
	SeverityLow      IssueSeverity = "low"
)
