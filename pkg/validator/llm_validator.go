// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
)

// SemanticValidation holds the result of an LLM-powered
// semantic comparison between expected and actual outcomes.
type SemanticValidation struct {
	// Matches indicates whether the actual outcome
	// semantically matches the expected outcome.
	Matches bool `json:"matches"`

	// Confidence is the LLM's confidence in its assessment
	// (0.0 - 1.0).
	Confidence float64 `json:"confidence"`

	// Reasoning explains why the LLM determined the outcome
	// matches or does not match.
	Reasoning string `json:"reasoning"`
}

// LLMValidator performs semantic validation of test step
// outcomes using an LLM. This is an optional enhancement --
// the existing ValidateStep() method works without it.
type LLMValidator interface {
	// ValidateSemantic uses an LLM to compare expected and
	// actual outcomes for a test step, returning a semantic
	// assessment.
	ValidateSemantic(
		ctx context.Context,
		stepName string,
		expected string,
		actual string,
	) (*SemanticValidation, error)
}

// ValidateSemanticWith uses the provided LLMValidator to
// perform semantic validation. Returns nil if the validator
// is nil (graceful degradation).
func ValidateSemanticWith(
	ctx context.Context,
	llmVal LLMValidator,
	stepName string,
	expected string,
	actual string,
) (*SemanticValidation, error) {
	if llmVal == nil {
		return nil, nil
	}
	if stepName == "" {
		return nil, fmt.Errorf("step name is required")
	}
	if expected == "" {
		return nil, fmt.Errorf("expected outcome is required")
	}
	if actual == "" {
		return nil, fmt.Errorf("actual outcome is required")
	}
	return llmVal.ValidateSemantic(
		ctx, stepName, expected, actual,
	)
}

// Validate checks that the SemanticValidation has valid fields.
func (sv *SemanticValidation) Validate() error {
	if sv.Confidence < 0 || sv.Confidence > 1 {
		return fmt.Errorf(
			"confidence must be 0.0-1.0, got %f",
			sv.Confidence,
		)
	}
	if sv.Reasoning == "" {
		return fmt.Errorf("reasoning is required")
	}
	return nil
}

// IsHighConfidence returns true if confidence exceeds the
// given threshold.
func (sv *SemanticValidation) IsHighConfidence(
	threshold float64,
) bool {
	return sv.Confidence >= threshold
}
