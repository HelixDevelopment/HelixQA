// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package planning provides types and utilities for test
// planning, including plan generation, bank reconciliation,
// and priority-based ranking of planned tests.
package planning

// PlannedTest represents a single test case within a test
// plan. It carries both the test definition fields and
// reconciliation metadata (IsExisting, IsNew, BankSource).
type PlannedTest struct {
	// ID uniquely identifies the test case.
	ID string `json:"id"`

	// Name is the human-readable test name.
	Name string `json:"name"`

	// Description explains what this test validates.
	Description string `json:"description"`

	// Category groups related tests (e.g., "functional",
	// "edge_case", "integration", "security").
	Category string `json:"category"`

	// Priority indicates test importance (1=critical, higher
	// numbers are lower priority).
	Priority int `json:"priority"`

	// Platforms specifies which platforms this test targets.
	Platforms []string `json:"platforms"`

	// Screen identifies the UI screen or area under test.
	Screen string `json:"screen"`

	// Steps lists the ordered test steps to execute.
	Steps []string `json:"steps"`

	// Expected describes the expected outcome.
	Expected string `json:"expected"`

	// IsExisting is true if this test matches an entry in the
	// test bank.
	IsExisting bool `json:"is_existing"`

	// IsNew is true if this test has no corresponding bank
	// entry.
	IsNew bool `json:"is_new"`

	// BankSource identifies the bank file or source where the
	// existing test was found.
	BankSource string `json:"bank_source"`
}

// TestPlan holds the full set of planned tests for a QA
// session, along with summary statistics.
type TestPlan struct {
	// SessionID is the unique identifier for this QA session.
	SessionID string `json:"session_id"`

	// Generated is the timestamp when the plan was created
	// (ISO 8601 format).
	Generated string `json:"generated"`

	// TotalTests is the total number of tests in this plan.
	TotalTests int `json:"total_tests"`

	// ExistingTests is the count of tests matched in the bank.
	ExistingTests int `json:"existing_tests"`

	// NewTests is the count of tests with no bank match.
	NewTests int `json:"new_tests"`

	// Platforms lists the platforms covered by this plan.
	Platforms []string `json:"platforms"`

	// Tests holds all planned tests in this session.
	Tests []PlannedTest `json:"tests"`
}

// PlanStats contains aggregate counts for a test plan,
// broken down by category and platform.
type PlanStats struct {
	// Total is the total number of planned tests.
	Total int `json:"total"`

	// Existing is the number of tests matched in the bank.
	Existing int `json:"existing"`

	// New is the number of tests with no bank match.
	New int `json:"new"`

	// ByCategory maps category names to test counts.
	ByCategory map[string]int `json:"by_category"`

	// ByPlatform maps platform names to test counts.
	ByPlatform map[string]int `json:"by_platform"`
}

// PlanStats computes aggregate statistics for the test plan.
// Each test is counted once per category. For platforms, a
// test with multiple platforms is counted once per platform.
func (tp *TestPlan) PlanStats() PlanStats {
	stats := PlanStats{
		Total:      len(tp.Tests),
		ByCategory: make(map[string]int),
		ByPlatform: make(map[string]int),
	}

	for i := range tp.Tests {
		t := &tp.Tests[i]
		if t.IsExisting {
			stats.Existing++
		}
		if t.IsNew {
			stats.New++
		}
		if t.Category != "" {
			stats.ByCategory[t.Category]++
		}
		for _, p := range t.Platforms {
			if p != "" {
				stats.ByPlatform[p]++
			}
		}
	}

	return stats
}

// ForPlatform returns all planned tests that target the given
// platform. A test with an empty Platforms list is included
// for every platform.
func (tp *TestPlan) ForPlatform(platform string) []PlannedTest {
	var result []PlannedTest
	for _, t := range tp.Tests {
		if len(t.Platforms) == 0 {
			result = append(result, t)
			continue
		}
		for _, p := range t.Platforms {
			if p == platform {
				result = append(result, t)
				break
			}
		}
	}
	return result
}
