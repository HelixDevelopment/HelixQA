// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package testbank provides QA-specific test bank management
// with YAML-based test case definitions. It bridges to the
// Challenges framework's bank package for execution while
// adding QA metadata like platform targeting, priority, and
// documentation references.
package testbank

import (
	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.helixqa/pkg/config"
)

// Priority levels for test cases.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

// TestCase is a QA-specific test definition that extends the
// Challenges Definition with platform, priority, and
// documentation references.
type TestCase struct {
	// ID uniquely identifies the test case.
	ID string `yaml:"id" json:"id"`

	// Name is the human-readable test name.
	Name string `yaml:"name" json:"name"`

	// Description explains what this test validates.
	Description string `yaml:"description" json:"description"`

	// Category groups related tests (e.g., "functional",
	// "edge_case", "integration", "security").
	Category string `yaml:"category" json:"category"`

	// Priority indicates test importance for scheduling.
	Priority Priority `yaml:"priority" json:"priority"`

	// Platforms specifies which platforms this test targets.
	// Empty means all platforms.
	Platforms []config.Platform `yaml:"platforms" json:"platforms"`

	// Steps lists the ordered test steps to execute.
	Steps []TestStep `yaml:"steps" json:"steps"`

	// Dependencies lists test case IDs that must pass first.
	Dependencies []string `yaml:"dependencies" json:"dependencies"`

	// DocumentationRefs links to relevant docs for
	// inconsistency detection.
	DocumentationRefs []DocRef `yaml:"documentation_refs" json:"documentation_refs"`

	// Tags provides free-form labels for filtering.
	Tags []string `yaml:"tags" json:"tags"`

	// EstimatedDuration is the expected execution time.
	EstimatedDuration string `yaml:"estimated_duration" json:"estimated_duration"`

	// ExpectedResult describes the expected outcome.
	ExpectedResult string `yaml:"expected_result" json:"expected_result"`
}

// TestStep is a single step within a test case.
type TestStep struct {
	// Name identifies this step.
	Name string `yaml:"name" json:"name"`

	// Action describes what to do.
	Action string `yaml:"action" json:"action"`

	// Expected describes the expected outcome.
	Expected string `yaml:"expected" json:"expected"`

	// Platform limits this step to a specific platform.
	// Empty means it applies to all.
	Platform config.Platform `yaml:"platform,omitempty" json:"platform,omitempty"`
}

// DocRef references a documentation source for consistency
// verification.
type DocRef struct {
	// Type is the doc type (e.g., "user_guide", "api_spec",
	// "video_course", "architecture").
	Type string `yaml:"type" json:"type"`

	// Section is the specific section or page reference.
	Section string `yaml:"section" json:"section"`

	// Path is the file path or URL.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// BankFile represents the YAML structure of a test bank file.
type BankFile struct {
	// Version is the bank file format version.
	Version string `yaml:"version" json:"version"`

	// Name identifies this bank.
	Name string `yaml:"name" json:"name"`

	// Description explains the bank's purpose.
	Description string `yaml:"description" json:"description"`

	// TestCases holds all test cases in this bank.
	TestCases []TestCase `yaml:"test_cases" json:"test_cases"`

	// Metadata holds arbitrary key-value pairs.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ToDefinition converts a TestCase to a Challenges Definition
// for execution by the runner.
func (tc *TestCase) ToDefinition() *challenge.Definition {
	deps := make([]challenge.ID, len(tc.Dependencies))
	for i, d := range tc.Dependencies {
		deps[i] = challenge.ID(d)
	}

	return &challenge.Definition{
		ID:                challenge.ID(tc.ID),
		Name:              tc.Name,
		Description:       tc.Description,
		Category:          tc.Category,
		Dependencies:      deps,
		EstimatedDuration: tc.EstimatedDuration,
	}
}

// AppliesToPlatform returns true if this test case targets the
// given platform. An empty Platforms list means all platforms.
func (tc *TestCase) AppliesToPlatform(p config.Platform) bool {
	if len(tc.Platforms) == 0 {
		return true
	}
	for _, tp := range tc.Platforms {
		if tp == p || tp == config.PlatformAll {
			return true
		}
	}
	return false
}

// IsValid returns an error message if the test case has
// missing required fields, or empty string if valid.
func (tc *TestCase) IsValid() string {
	if tc.ID == "" {
		return "test case missing ID"
	}
	if tc.Name == "" {
		return "test case " + tc.ID + " missing name"
	}
	return ""
}
