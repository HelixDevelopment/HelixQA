// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// mockCrashAnalyzer is a test double for LLMCrashAnalyzer.
type mockCrashAnalyzer struct {
	analysis *CrashAnalysis
	err      error
	called   int
}

func (m *mockCrashAnalyzer) AnalyzeCrash(
	_ context.Context,
	_ *DetectionResult,
) (*CrashAnalysis, error) {
	m.called++
	return m.analysis, m.err
}

func TestCrashAnalysis_Validate_Valid(t *testing.T) {
	ca := &CrashAnalysis{
		RootCause:              "NullPointerException in FormatRegistry",
		AffectedComponent:      "format/FormatRegistry",
		ReproductionLikelihood: 0.85,
		Severity:               CrashSeverityHigh,
		Recommendations: []string{
			"Add nil check before accessing formats list",
			"Initialize FormatRegistry lazily",
		},
	}
	assert.NoError(t, ca.Validate())
}

func TestCrashAnalysis_Validate_MissingRootCause(t *testing.T) {
	ca := &CrashAnalysis{
		AffectedComponent:      "component",
		ReproductionLikelihood: 0.5,
		Severity:               CrashSeverityMedium,
	}
	err := ca.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root cause")
}

func TestCrashAnalysis_Validate_MissingComponent(t *testing.T) {
	ca := &CrashAnalysis{
		RootCause:              "some cause",
		ReproductionLikelihood: 0.5,
		Severity:               CrashSeverityMedium,
	}
	err := ca.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "affected component")
}

func TestCrashAnalysis_Validate_InvalidLikelihood(t *testing.T) {
	tests := []struct {
		name       string
		likelihood float64
	}{
		{"negative", -0.1},
		{"too_high", 1.1},
		{"way_too_high", 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := &CrashAnalysis{
				RootCause:              "cause",
				AffectedComponent:      "component",
				ReproductionLikelihood: tt.likelihood,
				Severity:               CrashSeverityLow,
			}
			err := ca.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "reproduction likelihood")
		})
	}
}

func TestCrashAnalysis_Validate_InvalidSeverity(t *testing.T) {
	ca := &CrashAnalysis{
		RootCause:              "cause",
		AffectedComponent:      "component",
		ReproductionLikelihood: 0.5,
		Severity:               "invalid",
	}
	err := ca.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid severity")
}

func TestCrashAnalysis_Validate_BoundaryLikelihood(t *testing.T) {
	tests := []struct {
		name       string
		likelihood float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"mid", 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := &CrashAnalysis{
				RootCause:              "cause",
				AffectedComponent:      "component",
				ReproductionLikelihood: tt.likelihood,
				Severity:               CrashSeverityMedium,
			}
			assert.NoError(t, ca.Validate())
		})
	}
}

func TestCrashAnalysis_Validate_AllSeverities(t *testing.T) {
	severities := []CrashSeverity{
		CrashSeverityCritical,
		CrashSeverityHigh,
		CrashSeverityMedium,
		CrashSeverityLow,
	}
	for _, sev := range severities {
		t.Run(string(sev), func(t *testing.T) {
			ca := &CrashAnalysis{
				RootCause:              "cause",
				AffectedComponent:      "component",
				ReproductionLikelihood: 0.5,
				Severity:               sev,
			}
			assert.NoError(t, ca.Validate())
		})
	}
}

func TestAnalyzeCrashWith_Success(t *testing.T) {
	analyzer := &mockCrashAnalyzer{
		analysis: &CrashAnalysis{
			RootCause:              "OOM in image decoder",
			AffectedComponent:      "media/ImageDecoder",
			ReproductionLikelihood: 0.9,
			Severity:               CrashSeverityCritical,
			Recommendations: []string{
				"Add memory limit checks",
			},
		},
	}
	result := &DetectionResult{
		Platform:   config.PlatformAndroid,
		HasCrash:   true,
		StackTrace: "java.lang.OutOfMemoryError",
		Timestamp:  time.Now(),
	}

	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.NoError(t, err)
	require.NotNil(t, ca)
	assert.Equal(t, "OOM in image decoder", ca.RootCause)
	assert.Equal(t, "media/ImageDecoder", ca.AffectedComponent)
	assert.InDelta(t, 0.9, ca.ReproductionLikelihood, 0.001)
	assert.Equal(t, CrashSeverityCritical, ca.Severity)
	assert.Len(t, ca.Recommendations, 1)
	assert.Equal(t, 1, analyzer.called)
}

func TestAnalyzeCrashWith_NilAnalyzer(t *testing.T) {
	result := &DetectionResult{
		HasCrash: true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), nil, result,
	)
	assert.NoError(t, err)
	assert.Nil(t, ca)
}

func TestAnalyzeCrashWith_NilResult(t *testing.T) {
	analyzer := &mockCrashAnalyzer{}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, nil,
	)
	require.Error(t, err)
	assert.Nil(t, ca)
	assert.Contains(t, err.Error(), "nil")
	assert.Equal(t, 0, analyzer.called)
}

func TestAnalyzeCrashWith_NoCrashOrANR(t *testing.T) {
	analyzer := &mockCrashAnalyzer{}
	result := &DetectionResult{
		Platform:     config.PlatformDesktop,
		HasCrash:     false,
		HasANR:       false,
		ProcessAlive: true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.Error(t, err)
	assert.Nil(t, ca)
	assert.Contains(t, err.Error(), "no crash or ANR")
	assert.Equal(t, 0, analyzer.called)
}

func TestAnalyzeCrashWith_ANROnly(t *testing.T) {
	analyzer := &mockCrashAnalyzer{
		analysis: &CrashAnalysis{
			RootCause:              "Main thread blocked on I/O",
			AffectedComponent:      "network/SyncManager",
			ReproductionLikelihood: 0.7,
			Severity:               CrashSeverityHigh,
		},
	}
	result := &DetectionResult{
		Platform: config.PlatformAndroid,
		HasANR:   true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.NoError(t, err)
	require.NotNil(t, ca)
	assert.Equal(t, "Main thread blocked on I/O", ca.RootCause)
	assert.Equal(t, 1, analyzer.called)
}

func TestAnalyzeCrashWith_AnalyzerError(t *testing.T) {
	analyzer := &mockCrashAnalyzer{
		err: fmt.Errorf("LLM API unavailable"),
	}
	result := &DetectionResult{
		HasCrash: true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.Error(t, err)
	assert.Nil(t, ca)
	assert.Contains(t, err.Error(), "LLM API unavailable")
}

func TestAnalyzeCrashWith_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	analyzer := &mockCrashAnalyzer{
		err: ctx.Err(),
	}
	result := &DetectionResult{
		HasCrash: true,
	}
	ca, err := AnalyzeCrashWith(ctx, analyzer, result)
	require.Error(t, err)
	assert.Nil(t, ca)
}

func TestAnalyzeCrashWith_EmptyRecommendations(t *testing.T) {
	analyzer := &mockCrashAnalyzer{
		analysis: &CrashAnalysis{
			RootCause:              "Unknown crash",
			AffectedComponent:      "unknown",
			ReproductionLikelihood: 0.1,
			Severity:               CrashSeverityLow,
			Recommendations:        nil,
		},
	}
	result := &DetectionResult{
		HasCrash: true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.NoError(t, err)
	require.NotNil(t, ca)
	assert.Empty(t, ca.Recommendations)
}

func TestCrashSeverity_Constants(t *testing.T) {
	assert.Equal(t, CrashSeverity("critical"), CrashSeverityCritical)
	assert.Equal(t, CrashSeverity("high"), CrashSeverityHigh)
	assert.Equal(t, CrashSeverity("medium"), CrashSeverityMedium)
	assert.Equal(t, CrashSeverity("low"), CrashSeverityLow)
}

func TestIsValidCrashSeverity(t *testing.T) {
	assert.True(t, isValidCrashSeverity(CrashSeverityCritical))
	assert.True(t, isValidCrashSeverity(CrashSeverityHigh))
	assert.True(t, isValidCrashSeverity(CrashSeverityMedium))
	assert.True(t, isValidCrashSeverity(CrashSeverityLow))
	assert.False(t, isValidCrashSeverity("invalid"))
	assert.False(t, isValidCrashSeverity(""))
}

func TestCrashAnalysis_Validate_EmptyRecommendations(t *testing.T) {
	ca := &CrashAnalysis{
		RootCause:              "cause",
		AffectedComponent:      "component",
		ReproductionLikelihood: 0.5,
		Severity:               CrashSeverityMedium,
		Recommendations:        nil,
	}
	assert.NoError(t, ca.Validate())
}

func TestAnalyzeCrashWith_MultipleRecommendations(t *testing.T) {
	recommendations := []string{
		"Add try-catch around file I/O",
		"Validate input before processing",
		"Add unit test for edge case",
		"Update documentation",
	}
	analyzer := &mockCrashAnalyzer{
		analysis: &CrashAnalysis{
			RootCause:              "Unhandled IOException",
			AffectedComponent:      "storage/FileManager",
			ReproductionLikelihood: 0.95,
			Severity:               CrashSeverityHigh,
			Recommendations:        recommendations,
		},
	}
	result := &DetectionResult{
		HasCrash:   true,
		StackTrace: "java.io.IOException: No space left",
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.NoError(t, err)
	assert.Len(t, ca.Recommendations, 4)
}

func TestAnalyzeCrashWith_BothCrashAndANR(t *testing.T) {
	analyzer := &mockCrashAnalyzer{
		analysis: &CrashAnalysis{
			RootCause:              "Deadlock causing ANR then crash",
			AffectedComponent:      "concurrency/LockManager",
			ReproductionLikelihood: 0.6,
			Severity:               CrashSeverityCritical,
		},
	}
	result := &DetectionResult{
		HasCrash: true,
		HasANR:   true,
	}
	ca, err := AnalyzeCrashWith(
		context.Background(), analyzer, result,
	)
	require.NoError(t, err)
	require.NotNil(t, ca)
	assert.Equal(t, CrashSeverityCritical, ca.Severity)
}
