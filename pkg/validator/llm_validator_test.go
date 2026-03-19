// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMValidator is a test double for LLMValidator.
type mockLLMValidator struct {
	result *SemanticValidation
	err    error
	called int
	// Captured args for verification.
	lastStepName string
	lastExpected string
	lastActual   string
}

func (m *mockLLMValidator) ValidateSemantic(
	_ context.Context,
	stepName string,
	expected string,
	actual string,
) (*SemanticValidation, error) {
	m.called++
	m.lastStepName = stepName
	m.lastExpected = expected
	m.lastActual = actual
	return m.result, m.err
}

func TestSemanticValidation_Validate_Valid(t *testing.T) {
	sv := &SemanticValidation{
		Matches:    true,
		Confidence: 0.95,
		Reasoning:  "Both describe the same settings screen",
	}
	assert.NoError(t, sv.Validate())
}

func TestSemanticValidation_Validate_InvalidConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
	}{
		{"negative", -0.1},
		{"too_high", 1.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &SemanticValidation{
				Matches:    true,
				Confidence: tt.confidence,
				Reasoning:  "some reasoning",
			}
			err := sv.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "confidence")
		})
	}
}

func TestSemanticValidation_Validate_MissingReasoning(t *testing.T) {
	sv := &SemanticValidation{
		Matches:    true,
		Confidence: 0.9,
		Reasoning:  "",
	}
	err := sv.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reasoning")
}

func TestSemanticValidation_Validate_BoundaryConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"half", 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &SemanticValidation{
				Matches:    false,
				Confidence: tt.confidence,
				Reasoning:  "analysis complete",
			}
			assert.NoError(t, sv.Validate())
		})
	}
}

func TestSemanticValidation_IsHighConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		threshold  float64
		expected   bool
	}{
		{"above", 0.95, 0.9, true},
		{"equal", 0.9, 0.9, true},
		{"below", 0.85, 0.9, false},
		{"zero_threshold", 0.1, 0.0, true},
		{"full_threshold", 0.99, 1.0, false},
		{"exact_match", 1.0, 1.0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := &SemanticValidation{
				Confidence: tt.confidence,
			}
			assert.Equal(t, tt.expected,
				sv.IsHighConfidence(tt.threshold),
			)
		})
	}
}

func TestValidateSemanticWith_Success(t *testing.T) {
	mock := &mockLLMValidator{
		result: &SemanticValidation{
			Matches:    true,
			Confidence: 0.92,
			Reasoning:  "Settings page shows expected options",
		},
	}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"open_settings",
		"Settings page with theme option",
		"Settings page displaying Theme toggle",
	)
	require.NoError(t, err)
	require.NotNil(t, sv)
	assert.True(t, sv.Matches)
	assert.InDelta(t, 0.92, sv.Confidence, 0.001)
	assert.Equal(t, 1, mock.called)
	assert.Equal(t, "open_settings", mock.lastStepName)
	assert.Equal(t, "Settings page with theme option",
		mock.lastExpected)
	assert.Equal(t, "Settings page displaying Theme toggle",
		mock.lastActual)
}

func TestValidateSemanticWith_NoMatch(t *testing.T) {
	mock := &mockLLMValidator{
		result: &SemanticValidation{
			Matches:    false,
			Confidence: 0.88,
			Reasoning:  "Expected editor but got file browser",
		},
	}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"open_editor",
		"Text editor with toolbar",
		"File browser showing directory listing",
	)
	require.NoError(t, err)
	require.NotNil(t, sv)
	assert.False(t, sv.Matches)
	assert.Equal(t, "Expected editor but got file browser",
		sv.Reasoning)
}

func TestValidateSemanticWith_NilValidator(t *testing.T) {
	sv, err := ValidateSemanticWith(
		context.Background(),
		nil,
		"step",
		"expected",
		"actual",
	)
	assert.NoError(t, err)
	assert.Nil(t, sv)
}

func TestValidateSemanticWith_EmptyStepName(t *testing.T) {
	mock := &mockLLMValidator{}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"",
		"expected",
		"actual",
	)
	require.Error(t, err)
	assert.Nil(t, sv)
	assert.Contains(t, err.Error(), "step name")
	assert.Equal(t, 0, mock.called)
}

func TestValidateSemanticWith_EmptyExpected(t *testing.T) {
	mock := &mockLLMValidator{}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"step",
		"",
		"actual",
	)
	require.Error(t, err)
	assert.Nil(t, sv)
	assert.Contains(t, err.Error(), "expected outcome")
}

func TestValidateSemanticWith_EmptyActual(t *testing.T) {
	mock := &mockLLMValidator{}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"step",
		"expected",
		"",
	)
	require.Error(t, err)
	assert.Nil(t, sv)
	assert.Contains(t, err.Error(), "actual outcome")
}

func TestValidateSemanticWith_ValidatorError(t *testing.T) {
	mock := &mockLLMValidator{
		err: fmt.Errorf("LLM rate limited"),
	}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"step",
		"expected",
		"actual",
	)
	require.Error(t, err)
	assert.Nil(t, sv)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestValidateSemanticWith_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock := &mockLLMValidator{
		err: ctx.Err(),
	}
	sv, err := ValidateSemanticWith(
		ctx, mock, "step", "expected", "actual",
	)
	require.Error(t, err)
	assert.Nil(t, sv)
}

func TestValidateSemanticWith_LowConfidence(t *testing.T) {
	mock := &mockLLMValidator{
		result: &SemanticValidation{
			Matches:    true,
			Confidence: 0.3,
			Reasoning:  "Unclear match, might be different screen",
		},
	}
	sv, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"verify_screen",
		"Home screen",
		"Some screen with icons",
	)
	require.NoError(t, err)
	require.NotNil(t, sv)
	assert.True(t, sv.Matches)
	assert.False(t, sv.IsHighConfidence(0.8))
}

func TestSemanticValidation_Validate_MatchesFalse_Valid(t *testing.T) {
	sv := &SemanticValidation{
		Matches:    false,
		Confidence: 0.99,
		Reasoning:  "Screens are completely different",
	}
	assert.NoError(t, sv.Validate())
}

func TestValidateSemanticWith_PassesArgsCorrectly(t *testing.T) {
	mock := &mockLLMValidator{
		result: &SemanticValidation{
			Matches:    true,
			Confidence: 1.0,
			Reasoning:  "perfect match",
		},
	}
	_, err := ValidateSemanticWith(
		context.Background(),
		mock,
		"my-step-name",
		"my expected outcome",
		"my actual outcome",
	)
	require.NoError(t, err)
	assert.Equal(t, "my-step-name", mock.lastStepName)
	assert.Equal(t, "my expected outcome", mock.lastExpected)
	assert.Equal(t, "my actual outcome", mock.lastActual)
}
