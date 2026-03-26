// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package analysis_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/llm"
)

// mockVisionLLM is a test double for llm.Provider that
// returns a fixed response from Vision() and records what
// it was called with.
type mockVisionLLM struct {
	visionResponse string
	visionErr      error
	lastPrompt     string
	callCount      int
}

func (m *mockVisionLLM) Chat(
	_ context.Context,
	_ []llm.Message,
) (*llm.Response, error) {
	return &llm.Response{Content: ""}, nil
}

func (m *mockVisionLLM) Vision(
	_ context.Context,
	_ []byte,
	prompt string,
) (*llm.Response, error) {
	m.lastPrompt = prompt
	m.callCount++
	if m.visionErr != nil {
		return nil, m.visionErr
	}
	return &llm.Response{Content: m.visionResponse}, nil
}

func (m *mockVisionLLM) Name() string        { return "mock" }
func (m *mockVisionLLM) SupportsVision() bool { return true }

// singleFindingJSON is a minimal valid JSON array with one
// finding that omits the platform/screen fields (the
// analyser sets those from call arguments).
const singleFindingJSON = `[
  {
    "category": "visual",
    "severity": "high",
    "title": "Button label truncated",
    "description": "The primary CTA label is cut off mid-word.",
    "evidence": "Button shows 'Subm...' instead of 'Submit'"
  }
]`

// TestVisionAnalyzer_AnalyzeScreenshot verifies that a
// valid JSON response is parsed into findings with
// screen and platform injected.
func TestVisionAnalyzer_AnalyzeScreenshot(t *testing.T) {
	mock := &mockVisionLLM{visionResponse: singleFindingJSON}
	analyzer := analysis.NewVisionAnalyzer(mock)

	findings, err := analyzer.AnalyzeScreenshot(
		context.Background(),
		[]byte("fake-png"),
		"home",
		"android",
	)

	require.NoError(t, err)
	require.Len(t, findings, 1)

	f := findings[0]
	assert.Equal(t, analysis.CategoryVisual, f.Category)
	assert.Equal(t, analysis.SeverityHigh, f.Severity)
	assert.Equal(t, "home", f.Screen)
	assert.Equal(t, "android", f.Platform)
	assert.Equal(t, 1, mock.callCount)
}

// TestVisionAnalyzer_AnalyzeScreenshot_NoIssues verifies
// that an empty JSON array response yields an empty slice
// without error.
func TestVisionAnalyzer_AnalyzeScreenshot_NoIssues(t *testing.T) {
	mock := &mockVisionLLM{visionResponse: "[]"}
	analyzer := analysis.NewVisionAnalyzer(mock)

	findings, err := analyzer.AnalyzeScreenshot(
		context.Background(),
		[]byte("fake-png"),
		"settings",
		"web",
	)

	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestVisionAnalyzer_AnalyzeScreenshot_MalformedResponse
// verifies that a prose response (no JSON array) returns
// an empty slice without error.
func TestVisionAnalyzer_AnalyzeScreenshot_MalformedResponse(
	t *testing.T,
) {
	mock := &mockVisionLLM{
		visionResponse: "looks fine to me, no issues found",
	}
	analyzer := analysis.NewVisionAnalyzer(mock)

	findings, err := analyzer.AnalyzeScreenshot(
		context.Background(),
		[]byte("fake-png"),
		"login",
		"desktop",
	)

	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestVisionAnalyzer_CompareScreenshots verifies that the
// compare path also parses findings and injects context.
func TestVisionAnalyzer_CompareScreenshots(t *testing.T) {
	mock := &mockVisionLLM{visionResponse: singleFindingJSON}
	analyzer := analysis.NewVisionAnalyzer(mock)

	findings, err := analyzer.CompareScreenshots(
		context.Background(),
		[]byte("before-png"),
		[]byte("after-png"),
		"player",
		"androidtv",
	)

	require.NoError(t, err)
	require.Len(t, findings, 1)

	f := findings[0]
	assert.Equal(t, analysis.CategoryVisual, f.Category)
	assert.Equal(t, "player", f.Screen)
	assert.Equal(t, "androidtv", f.Platform)
	assert.Equal(t, 1, mock.callCount)
}

// TestVisionAnalyzer_AnalyzeScreenshot_MarkdownWrapped
// verifies that markdown code-fence wrapping is stripped
// before JSON parsing.
func TestVisionAnalyzer_AnalyzeScreenshot_MarkdownWrapped(
	t *testing.T,
) {
	wrapped := "```json\n" + singleFindingJSON + "\n```"
	mock := &mockVisionLLM{visionResponse: wrapped}
	analyzer := analysis.NewVisionAnalyzer(mock)

	findings, err := analyzer.AnalyzeScreenshot(
		context.Background(),
		[]byte("fake-png"),
		"dashboard",
		"web",
	)

	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "dashboard", findings[0].Screen)
	assert.Equal(t, "web", findings[0].Platform)
}
