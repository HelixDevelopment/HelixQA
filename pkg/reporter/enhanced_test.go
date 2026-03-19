// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
)

// mockSummarizer is a test double for LLMSummarizer.
type mockSummarizer struct {
	result string
	err    error
	called int
}

func (m *mockSummarizer) Summarize(
	_ context.Context,
	_ string,
) (string, error) {
	m.called++
	return m.result, m.err
}

func makeQAReport(
	crashes, anrs, passed, failed int,
	platforms ...*PlatformResult,
) *QAReport {
	return &QAReport{
		Title:            "Test Report",
		GeneratedAt:      time.Now(),
		PlatformResults:  platforms,
		TotalChallenges:  passed + failed,
		PassedChallenges: passed,
		FailedChallenges: failed,
		TotalCrashes:     crashes,
		TotalANRs:        anrs,
		TotalDuration:    5 * time.Minute,
	}
}

func makePlatformResult(
	platform config.Platform,
	crashes, anrs int,
) *PlatformResult {
	return &PlatformResult{
		Platform:   platform,
		CrashCount: crashes,
		ANRCount:   anrs,
		StartTime:  time.Now().Add(-time.Minute),
		EndTime:    time.Now(),
		Duration:   time.Minute,
	}
}

func TestGenerateExecutiveSummary_Stable(t *testing.T) {
	qa := makeQAReport(0, 0, 10, 0,
		makePlatformResult(config.PlatformAndroid, 0, 0),
		makePlatformResult(config.PlatformDesktop, 0, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, "Stable", summary.OverallStatus)
	assert.Contains(t, summary.RiskAssessment, "Low risk")
	assert.Contains(t, summary.CoverageHighlights, "100%")
	assert.Contains(t, summary.CoverageHighlights, "10/10")
	assert.Contains(t, summary.Recommendations[0], "ready for release")
	assert.Empty(t, summary.TopIssues)
}

func TestGenerateExecutiveSummary_AtRisk(t *testing.T) {
	qa := makeQAReport(0, 0, 8, 2,
		makePlatformResult(config.PlatformWeb, 0, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	assert.Equal(t, "At Risk", summary.OverallStatus)
	assert.Contains(t, summary.RiskAssessment, "Medium risk")
	assert.Contains(t, summary.RiskAssessment, "2 of 10")
}

func TestGenerateExecutiveSummary_Critical(t *testing.T) {
	qa := makeQAReport(3, 1, 5, 5,
		makePlatformResult(config.PlatformAndroid, 2, 1),
		makePlatformResult(config.PlatformDesktop, 1, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	assert.Equal(t, "Critical Issues Found", summary.OverallStatus)
	assert.Contains(t, summary.RiskAssessment, "High risk")
	assert.Contains(t, summary.RiskAssessment, "3 crashes")
	assert.Contains(t, summary.RiskAssessment, "1 ANRs")
	assert.Len(t, summary.TopIssues, 3)
}

func TestGenerateExecutiveSummary_NilReport(t *testing.T) {
	summary, err := GenerateExecutiveSummary(nil, nil)
	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "nil")
}

func TestGenerateExecutiveSummary_NoChallenges(t *testing.T) {
	qa := makeQAReport(0, 0, 0, 0)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	assert.Equal(t, "No challenges executed",
		summary.CoverageHighlights)
}

func TestGenerateExecutiveSummary_WithLLM(t *testing.T) {
	mock := &mockSummarizer{
		result: "LLM-enhanced risk assessment: low overall risk",
	}
	qa := makeQAReport(0, 0, 5, 0,
		makePlatformResult(config.PlatformWeb, 0, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, mock)
	require.NoError(t, err)
	assert.Equal(t, 1, mock.called)
	assert.Equal(t, "LLM-enhanced risk assessment: low overall risk",
		summary.RiskAssessment)
}

func TestGenerateExecutiveSummary_LLMError_Fallback(t *testing.T) {
	mock := &mockSummarizer{
		err: fmt.Errorf("API unavailable"),
	}
	qa := makeQAReport(0, 0, 5, 0,
		makePlatformResult(config.PlatformAndroid, 0, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, mock)
	require.NoError(t, err)
	// Should still have deterministic summary.
	assert.Contains(t, summary.RiskAssessment, "Low risk")
	assert.Equal(t, 1, mock.called)
}

func TestGenerateExecutiveSummary_LLMEmptyResult(t *testing.T) {
	mock := &mockSummarizer{
		result: "",
	}
	qa := makeQAReport(1, 0, 3, 2,
		makePlatformResult(config.PlatformDesktop, 1, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, mock)
	require.NoError(t, err)
	// Empty LLM result should keep deterministic value.
	assert.Contains(t, summary.RiskAssessment, "High risk")
}

func TestGenerateExecutiveSummary_Recommendations(t *testing.T) {
	qa := makeQAReport(2, 1, 5, 3,
		makePlatformResult(config.PlatformAndroid, 2, 1),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	// Should have 3 recommendations: crashes, ANRs, failures.
	assert.Len(t, summary.Recommendations, 3)
	found := map[string]bool{}
	for _, r := range summary.Recommendations {
		if contains(r, "crash") {
			found["crash"] = true
		}
		if contains(r, "ANR") {
			found["anr"] = true
		}
		if contains(r, "failing") {
			found["failing"] = true
		}
	}
	assert.True(t, found["crash"])
	assert.True(t, found["anr"])
	assert.True(t, found["failing"])
}

func TestGenerateExecutiveSummary_TopIssues(t *testing.T) {
	qa := makeQAReport(5, 2, 0, 0,
		makePlatformResult(config.PlatformAndroid, 3, 2),
		makePlatformResult(config.PlatformDesktop, 2, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	// Android: 1 crash issue + 1 ANR issue, Desktop: 1 crash
	assert.Len(t, summary.TopIssues, 3)
}

func TestExecutiveSummary_Validate_Valid(t *testing.T) {
	es := &ExecutiveSummary{
		OverallStatus:  "Stable",
		RiskAssessment: "Low risk",
	}
	assert.NoError(t, es.Validate())
}

func TestExecutiveSummary_Validate_MissingStatus(t *testing.T) {
	es := &ExecutiveSummary{
		RiskAssessment: "Low risk",
	}
	err := es.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overall status")
}

func TestExecutiveSummary_Validate_MissingRisk(t *testing.T) {
	es := &ExecutiveSummary{
		OverallStatus: "Stable",
	}
	err := es.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "risk assessment")
}

func TestNavigationMapEmbed_Validate_Mermaid(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format: "mermaid",
		Content: `graph TD
    A[Home] --> B[Settings]
    A --> C[Editor]`,
	}
	assert.NoError(t, nm.Validate())
}

func TestNavigationMapEmbed_Validate_Dot(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format: "dot",
		Content: `digraph {
    Home -> Settings
    Home -> Editor
}`,
	}
	assert.NoError(t, nm.Validate())
}

func TestNavigationMapEmbed_Validate_InvalidFormat(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format:  "svg",
		Content: "content",
	}
	err := nm.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mermaid")
}

func TestNavigationMapEmbed_Validate_EmptyFormat(t *testing.T) {
	nm := &NavigationMapEmbed{
		Content: "content",
	}
	err := nm.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestNavigationMapEmbed_Validate_EmptyContent(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format: "mermaid",
	}
	err := nm.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

func TestExecutiveSummary_RenderMarkdown(t *testing.T) {
	es := &ExecutiveSummary{
		OverallStatus:      "At Risk",
		RiskAssessment:     "Medium risk: 2 failures",
		CoverageHighlights: "80% pass rate",
		TopIssues: []string{
			"Crash on Android settings",
			"ANR during file save",
		},
		Recommendations: []string{
			"Fix crash in settings module",
			"Optimize file I/O",
		},
	}

	md := es.RenderMarkdown()

	assert.Contains(t, md, "## Executive Summary")
	assert.Contains(t, md, "**Status:** At Risk")
	assert.Contains(t, md, "**Risk:** Medium risk")
	assert.Contains(t, md, "**Coverage:** 80%")
	assert.Contains(t, md, "### Top Issues")
	assert.Contains(t, md, "Crash on Android")
	assert.Contains(t, md, "ANR during file")
	assert.Contains(t, md, "### Recommendations")
	assert.Contains(t, md, "Fix crash")
	assert.Contains(t, md, "Optimize file")
}

func TestExecutiveSummary_RenderMarkdown_Minimal(t *testing.T) {
	es := &ExecutiveSummary{
		OverallStatus:  "Stable",
		RiskAssessment: "Low risk",
	}

	md := es.RenderMarkdown()

	assert.Contains(t, md, "**Status:** Stable")
	assert.NotContains(t, md, "### Top Issues")
	assert.NotContains(t, md, "### Recommendations")
}

func TestNavigationMapEmbed_RenderMarkdown(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format: "mermaid",
		Content: `graph TD
    A[Home] --> B[Settings]`,
	}

	md := nm.RenderMarkdown()

	assert.Contains(t, md, "## Navigation Map")
	assert.Contains(t, md, "```mermaid")
	assert.Contains(t, md, "graph TD")
	assert.Contains(t, md, "A[Home]")
}

func TestNavigationMapEmbed_RenderMarkdown_Dot(t *testing.T) {
	nm := &NavigationMapEmbed{
		Format:  "dot",
		Content: `digraph { A -> B }`,
	}

	md := nm.RenderMarkdown()

	assert.Contains(t, md, "```dot")
	assert.Contains(t, md, "digraph")
}

func TestGenerateExecutiveSummary_PassRate(t *testing.T) {
	tests := []struct {
		name     string
		passed   int
		failed   int
		expected string
	}{
		{"all_pass", 10, 0, "100%"},
		{"half", 5, 5, "50%"},
		{"one_third", 1, 2, "33%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qa := makeQAReport(0, 0, tt.passed, tt.failed)
			summary, err := GenerateExecutiveSummary(qa, nil)
			require.NoError(t, err)
			assert.Contains(t, summary.CoverageHighlights,
				tt.expected)
		})
	}
}

func TestGenerateExecutiveSummary_PlatformCount(t *testing.T) {
	qa := makeQAReport(0, 0, 10, 0,
		makePlatformResult(config.PlatformAndroid, 0, 0),
		makePlatformResult(config.PlatformDesktop, 0, 0),
		makePlatformResult(config.PlatformWeb, 0, 0),
	)
	summary, err := GenerateExecutiveSummary(qa, nil)
	require.NoError(t, err)
	assert.Contains(t, summary.CoverageHighlights, "3 platform")
}

func TestBuildReportText(t *testing.T) {
	qa := &QAReport{
		Title:            "Test",
		TotalChallenges:  10,
		PassedChallenges: 8,
		FailedChallenges: 2,
		TotalCrashes:     1,
		TotalANRs:        0,
		TotalDuration:    5 * time.Minute,
		PlatformResults: []*PlatformResult{
			{
				Platform:         config.PlatformAndroid,
				CrashCount:       1,
				ANRCount:         0,
				ChallengeResults: []*challenge.Result{{}, {}},
			},
		},
	}
	text := buildReportText(qa)
	assert.Contains(t, text, "Total challenges: 10")
	assert.Contains(t, text, "Passed: 8, Failed: 2")
	assert.Contains(t, text, "Crashes: 1")
}

// contains checks if s contains substr (case-insensitive not
// needed here -- just plain check).
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
