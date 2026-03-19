// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// mockTestGenerator is a test double for TestGenerator.
type mockTestGenerator struct {
	generateResult    []TestCase
	edgeCaseResult    []TestCase
	generateErr       error
	edgeCaseErr       error
	generateCalled    int
	edgeCaseCalled    int
	lastFeature       Feature
	lastTestCase      TestCase
}

func (m *mockTestGenerator) GenerateTests(
	_ context.Context,
	feature Feature,
) ([]TestCase, error) {
	m.generateCalled++
	m.lastFeature = feature
	return m.generateResult, m.generateErr
}

func (m *mockTestGenerator) GenerateEdgeCases(
	_ context.Context,
	tc TestCase,
) ([]TestCase, error) {
	m.edgeCaseCalled++
	m.lastTestCase = tc
	return m.edgeCaseResult, m.edgeCaseErr
}

// mockMultiTestGenerator returns different results per feature.
type mockMultiTestGenerator struct {
	results map[string][]TestCase
	called  int
}

func (m *mockMultiTestGenerator) GenerateTests(
	_ context.Context,
	feature Feature,
) ([]TestCase, error) {
	m.called++
	if cases, ok := m.results[feature.ID]; ok {
		return cases, nil
	}
	return nil, nil
}

func (m *mockMultiTestGenerator) GenerateEdgeCases(
	_ context.Context,
	tc TestCase,
) ([]TestCase, error) {
	return nil, nil
}

func TestFeature_Validate_Valid(t *testing.T) {
	f := Feature{
		ID:          "feat-markdown",
		Name:        "Markdown Editing",
		Description: "Basic markdown editing support",
		Category:    "format",
		Platforms:   []config.Platform{config.PlatformAll},
	}
	assert.NoError(t, f.Validate())
}

func TestFeature_Validate_MissingID(t *testing.T) {
	f := Feature{Name: "Feature"}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature ID")
}

func TestFeature_Validate_MissingName(t *testing.T) {
	f := Feature{ID: "feat-1"}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature name")
}

func TestFeature_Validate_MinimalValid(t *testing.T) {
	f := Feature{ID: "f", Name: "n"}
	assert.NoError(t, f.Validate())
}

func TestGenerateFromFeatureMap_Success(t *testing.T) {
	mock := &mockTestGenerator{
		generateResult: []TestCase{
			{ID: "tc-1", Name: "Test 1"},
			{ID: "tc-2", Name: "Test 2"},
		},
	}
	features := []Feature{
		{ID: "feat-1", Name: "Feature 1"},
	}

	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.NoError(t, err)
	assert.Len(t, cases, 2)
	assert.Equal(t, 1, mock.generateCalled)
	assert.Equal(t, "feat-1", mock.lastFeature.ID)
}

func TestGenerateFromFeatureMap_MultipleFeatures(t *testing.T) {
	mock := &mockMultiTestGenerator{
		results: map[string][]TestCase{
			"feat-md": {
				{ID: "tc-md-1", Name: "MD Test 1"},
				{ID: "tc-md-2", Name: "MD Test 2"},
			},
			"feat-csv": {
				{ID: "tc-csv-1", Name: "CSV Test 1"},
			},
		},
	}
	features := []Feature{
		{ID: "feat-md", Name: "Markdown"},
		{ID: "feat-csv", Name: "CSV"},
	}

	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.NoError(t, err)
	assert.Len(t, cases, 3)
	assert.Equal(t, 2, mock.called)
}

func TestGenerateFromFeatureMap_DeduplicatesIDs(t *testing.T) {
	mock := &mockMultiTestGenerator{
		results: map[string][]TestCase{
			"feat-a": {
				{ID: "tc-shared", Name: "Shared Test"},
				{ID: "tc-a-only", Name: "A Only"},
			},
			"feat-b": {
				{ID: "tc-shared", Name: "Shared Test"},
				{ID: "tc-b-only", Name: "B Only"},
			},
		},
	}
	features := []Feature{
		{ID: "feat-a", Name: "Feature A"},
		{ID: "feat-b", Name: "Feature B"},
	}

	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.NoError(t, err)
	assert.Len(t, cases, 3) // tc-shared counted once
}

func TestGenerateFromFeatureMap_NilAgent(t *testing.T) {
	features := []Feature{
		{ID: "feat-1", Name: "Feature 1"},
	}
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, nil,
	)
	assert.NoError(t, err)
	assert.Nil(t, cases)
}

func TestGenerateFromFeatureMap_EmptyFeatures(t *testing.T) {
	mock := &mockTestGenerator{}
	cases, err := GenerateFromFeatureMap(
		context.Background(), nil, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "at least one feature")
	assert.Equal(t, 0, mock.generateCalled)
}

func TestGenerateFromFeatureMap_EmptyFeatureSlice(t *testing.T) {
	mock := &mockTestGenerator{}
	cases, err := GenerateFromFeatureMap(
		context.Background(), []Feature{}, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
}

func TestGenerateFromFeatureMap_InvalidFeature(t *testing.T) {
	mock := &mockTestGenerator{}
	features := []Feature{
		{ID: "", Name: ""}, // invalid
	}
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "invalid feature")
}

func TestGenerateFromFeatureMap_GeneratorError(t *testing.T) {
	mock := &mockTestGenerator{
		generateErr: fmt.Errorf("LLM timeout"),
	}
	features := []Feature{
		{ID: "feat-1", Name: "Feature"},
	}
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "LLM timeout")
}

func TestGenerateFromFeatureMap_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mock := &mockTestGenerator{
		generateErr: ctx.Err(),
	}
	features := []Feature{
		{ID: "feat-1", Name: "Feature"},
	}
	cases, err := GenerateFromFeatureMap(
		ctx, features, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
}

func TestGenerateFromFeatureMap_EmptyResult(t *testing.T) {
	mock := &mockTestGenerator{
		generateResult: nil,
	}
	features := []Feature{
		{ID: "feat-1", Name: "Feature"},
	}
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.NoError(t, err)
	assert.Nil(t, cases)
}

func TestExpandEdgeCases_Success(t *testing.T) {
	mock := &mockTestGenerator{
		edgeCaseResult: []TestCase{
			{ID: "tc-edge-1", Name: "Empty input"},
			{ID: "tc-edge-2", Name: "Very long input"},
			{ID: "tc-edge-3", Name: "Special chars"},
		},
	}
	tc := TestCase{
		ID:       "tc-base",
		Name:     "Basic test",
		Category: "functional",
	}

	cases, err := ExpandEdgeCases(
		context.Background(), tc, mock,
	)
	require.NoError(t, err)
	assert.Len(t, cases, 3)
	assert.Equal(t, 1, mock.edgeCaseCalled)
	assert.Equal(t, "tc-base", mock.lastTestCase.ID)
}

func TestExpandEdgeCases_NilAgent(t *testing.T) {
	tc := TestCase{ID: "tc-1", Name: "Test"}
	cases, err := ExpandEdgeCases(
		context.Background(), tc, nil,
	)
	assert.NoError(t, err)
	assert.Nil(t, cases)
}

func TestExpandEdgeCases_InvalidTestCase(t *testing.T) {
	mock := &mockTestGenerator{}
	tc := TestCase{} // missing ID and Name
	cases, err := ExpandEdgeCases(
		context.Background(), tc, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "invalid test case")
	assert.Equal(t, 0, mock.edgeCaseCalled)
}

func TestExpandEdgeCases_GeneratorError(t *testing.T) {
	mock := &mockTestGenerator{
		edgeCaseErr: fmt.Errorf("API rate limited"),
	}
	tc := TestCase{ID: "tc-1", Name: "Test"}
	cases, err := ExpandEdgeCases(
		context.Background(), tc, mock,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestExpandEdgeCases_EmptyResult(t *testing.T) {
	mock := &mockTestGenerator{
		edgeCaseResult: nil,
	}
	tc := TestCase{ID: "tc-1", Name: "Test"}
	cases, err := ExpandEdgeCases(
		context.Background(), tc, mock,
	)
	require.NoError(t, err)
	assert.Nil(t, cases)
}

func TestExpandEdgeCases_PassesFullTestCase(t *testing.T) {
	mock := &mockTestGenerator{
		edgeCaseResult: []TestCase{},
	}
	tc := TestCase{
		ID:          "tc-full",
		Name:        "Full test",
		Description: "A comprehensive test",
		Category:    "integration",
		Priority:    PriorityHigh,
		Platforms:   []config.Platform{config.PlatformAndroid},
		Steps: []TestStep{
			{Name: "step1", Action: "open app"},
		},
		Tags: []string{"regression"},
	}
	_, err := ExpandEdgeCases(
		context.Background(), tc, mock,
	)
	require.NoError(t, err)
	assert.Equal(t, "tc-full", mock.lastTestCase.ID)
	assert.Equal(t, "Full test", mock.lastTestCase.Name)
	assert.Equal(t, "A comprehensive test",
		mock.lastTestCase.Description)
	assert.Equal(t, PriorityHigh, mock.lastTestCase.Priority)
}

func TestGenerateFromFeatureMap_FeatureWithPlatforms(t *testing.T) {
	mock := &mockTestGenerator{
		generateResult: []TestCase{
			{ID: "tc-1", Name: "Platform test"},
		},
	}
	features := []Feature{
		{
			ID:       "feat-platform",
			Name:     "Platform Feature",
			Category: "ui",
			Platforms: []config.Platform{
				config.PlatformAndroid,
				config.PlatformWeb,
			},
		},
	}

	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	require.NoError(t, err)
	assert.Len(t, cases, 1)
	// Verify the feature was passed with platforms.
	assert.Equal(t, "feat-platform", mock.lastFeature.ID)
	assert.Len(t, mock.lastFeature.Platforms, 2)
}

func TestFeature_Fields(t *testing.T) {
	f := Feature{
		ID:          "feat-test",
		Name:        "Test Feature",
		Description: "A test feature",
		Category:    "testing",
		Platforms: []config.Platform{
			config.PlatformAndroid,
			config.PlatformDesktop,
		},
	}
	assert.Equal(t, "feat-test", f.ID)
	assert.Equal(t, "Test Feature", f.Name)
	assert.Equal(t, "A test feature", f.Description)
	assert.Equal(t, "testing", f.Category)
	assert.Len(t, f.Platforms, 2)
}

func TestGenerateFromFeatureMap_FirstFeatureInvalid(t *testing.T) {
	mock := &mockTestGenerator{
		generateResult: []TestCase{
			{ID: "tc-1", Name: "Test"},
		},
	}
	features := []Feature{
		{ID: "feat-valid", Name: "Valid"},
		{ID: "", Name: "Invalid"},
	}

	// First one is valid, second is invalid -- should fail
	// when reaching the invalid one.
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, mock,
	)
	// The first call succeeds, then the second feature is
	// invalid. But validation happens before generation.
	// Actually, let me reorder: first valid, second invalid.
	// The loop processes feat-valid first (generates tc-1),
	// then hits the invalid feature.
	require.Error(t, err)
	assert.Nil(t, cases)
}

func TestGenerateFromFeatureMap_SecondFeatureErrors(t *testing.T) {
	callCount := 0
	mock := &mockTestGenerator{
		generateResult: nil,
	}
	// Override behavior: succeed first time, fail second.
	// Use a wrapper instead.
	wrapper := &conditionalGenerator{
		successResult: []TestCase{{ID: "tc-1", Name: "T1"}},
		failOnCall:    2,
		failErr:       fmt.Errorf("quota exceeded"),
	}
	_ = mock
	_ = callCount

	features := []Feature{
		{ID: "feat-1", Name: "Feature 1"},
		{ID: "feat-2", Name: "Feature 2"},
	}
	cases, err := GenerateFromFeatureMap(
		context.Background(), features, wrapper,
	)
	require.Error(t, err)
	assert.Nil(t, cases)
	assert.Contains(t, err.Error(), "quota exceeded")
}

// conditionalGenerator fails on a specific call number.
type conditionalGenerator struct {
	successResult []TestCase
	failOnCall    int
	failErr       error
	callCount     int
}

func (g *conditionalGenerator) GenerateTests(
	_ context.Context,
	_ Feature,
) ([]TestCase, error) {
	g.callCount++
	if g.callCount == g.failOnCall {
		return nil, g.failErr
	}
	return g.successResult, nil
}

func (g *conditionalGenerator) GenerateEdgeCases(
	_ context.Context,
	_ TestCase,
) ([]TestCase, error) {
	return nil, nil
}
