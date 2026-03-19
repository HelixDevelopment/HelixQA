// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/config"
)

// Feature represents a product feature that can be used to
// generate test cases. This is a lightweight struct designed
// for the test generation pipeline.
type Feature struct {
	// ID uniquely identifies the feature.
	ID string `json:"id"`

	// Name is a human-readable feature name.
	Name string `json:"name"`

	// Description explains the feature.
	Description string `json:"description"`

	// Category groups related features (e.g., "format",
	// "ui", "network").
	Category string `json:"category"`

	// Platforms lists which platforms support this feature.
	Platforms []config.Platform `json:"platforms"`
}

// TestGenerator uses an LLM to generate test cases from
// features. This is an optional enhancement -- the existing
// test bank loading and management works without it.
type TestGenerator interface {
	// GenerateTests creates test cases for the given feature.
	GenerateTests(
		ctx context.Context,
		feature Feature,
	) ([]TestCase, error)

	// GenerateEdgeCases creates additional edge case test
	// cases for an existing test case.
	GenerateEdgeCases(
		ctx context.Context,
		tc TestCase,
	) ([]TestCase, error)
}

// GenerateFromFeatureMap generates test cases from a list of
// features using the provided LLM test generator. Returns
// nil if agent is nil (graceful degradation).
func GenerateFromFeatureMap(
	ctx context.Context,
	features []Feature,
	agent TestGenerator,
) ([]TestCase, error) {
	if agent == nil {
		return nil, nil
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("at least one feature is required")
	}

	var allCases []TestCase
	seen := make(map[string]bool)

	for _, feat := range features {
		if err := feat.Validate(); err != nil {
			return nil, fmt.Errorf(
				"invalid feature %q: %w", feat.ID, err,
			)
		}

		cases, err := agent.GenerateTests(ctx, feat)
		if err != nil {
			return nil, fmt.Errorf(
				"generate tests for %q: %w", feat.ID, err,
			)
		}

		for _, tc := range cases {
			if seen[tc.ID] {
				continue // skip duplicates
			}
			seen[tc.ID] = true
			allCases = append(allCases, tc)
		}
	}

	return allCases, nil
}

// ExpandEdgeCases generates edge case variants for an existing
// test case using the provided LLM test generator. Returns
// nil if agent is nil (graceful degradation).
func ExpandEdgeCases(
	ctx context.Context,
	tc TestCase,
	agent TestGenerator,
) ([]TestCase, error) {
	if agent == nil {
		return nil, nil
	}
	if msg := tc.IsValid(); msg != "" {
		return nil, fmt.Errorf("invalid test case: %s", msg)
	}

	cases, err := agent.GenerateEdgeCases(ctx, tc)
	if err != nil {
		return nil, fmt.Errorf(
			"expand edge cases for %q: %w", tc.ID, err,
		)
	}

	return cases, nil
}

// Validate checks that the Feature has required fields.
func (f *Feature) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("feature ID is required")
	}
	if f.Name == "" {
		return fmt.Errorf("feature name is required")
	}
	return nil
}
