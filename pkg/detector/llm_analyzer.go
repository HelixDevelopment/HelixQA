// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
)

// CrashSeverity indicates how severe a crash is.
type CrashSeverity string

const (
	// CrashSeverityCritical means data loss or security impact.
	CrashSeverityCritical CrashSeverity = "critical"
	// CrashSeverityHigh means a crash with user-visible impact.
	CrashSeverityHigh CrashSeverity = "high"
	// CrashSeverityMedium means a recoverable crash.
	CrashSeverityMedium CrashSeverity = "medium"
	// CrashSeverityLow means a minor issue or edge case.
	CrashSeverityLow CrashSeverity = "low"
)

// CrashAnalysis holds the result of an LLM-powered crash
// analysis, providing root cause and recommendations.
type CrashAnalysis struct {
	// RootCause describes the likely root cause of the crash.
	RootCause string `json:"root_cause"`

	// AffectedComponent identifies the module or component.
	AffectedComponent string `json:"affected_component"`

	// ReproductionLikelihood is the probability of
	// reproducing the crash (0.0 - 1.0).
	ReproductionLikelihood float64 `json:"reproduction_likelihood"`

	// Severity indicates how severe the crash is.
	Severity CrashSeverity `json:"severity"`

	// Recommendations lists suggested fixes or mitigations.
	Recommendations []string `json:"recommendations"`
}

// LLMCrashAnalyzer analyzes crash detection results using an
// LLM to determine root cause, affected component, and
// recommendations. This is an optional enhancement -- the
// existing Check() and CheckApp() methods work without it.
type LLMCrashAnalyzer interface {
	// AnalyzeCrash sends a DetectionResult to an LLM for
	// root cause analysis and returns a CrashAnalysis.
	AnalyzeCrash(
		ctx context.Context,
		result *DetectionResult,
	) (*CrashAnalysis, error)
}

// AnalyzeCrashWith uses the provided LLMCrashAnalyzer to
// analyze a detection result. Returns nil analysis if the
// analyzer is nil (graceful degradation).
func AnalyzeCrashWith(
	ctx context.Context,
	analyzer LLMCrashAnalyzer,
	result *DetectionResult,
) (*CrashAnalysis, error) {
	if analyzer == nil {
		return nil, nil
	}
	if result == nil {
		return nil, fmt.Errorf("detection result is nil")
	}
	if !result.HasCrash && !result.HasANR {
		return nil, fmt.Errorf(
			"no crash or ANR detected, nothing to analyze",
		)
	}
	return analyzer.AnalyzeCrash(ctx, result)
}

// Validate checks that the CrashAnalysis has required fields.
func (ca *CrashAnalysis) Validate() error {
	if ca.RootCause == "" {
		return fmt.Errorf("root cause is required")
	}
	if ca.AffectedComponent == "" {
		return fmt.Errorf("affected component is required")
	}
	if ca.ReproductionLikelihood < 0 ||
		ca.ReproductionLikelihood > 1 {
		return fmt.Errorf(
			"reproduction likelihood must be 0.0-1.0, got %f",
			ca.ReproductionLikelihood,
		)
	}
	if !isValidCrashSeverity(ca.Severity) {
		return fmt.Errorf(
			"invalid severity: %q", ca.Severity,
		)
	}
	return nil
}

func isValidCrashSeverity(s CrashSeverity) bool {
	switch s {
	case CrashSeverityCritical, CrashSeverityHigh,
		CrashSeverityMedium, CrashSeverityLow:
		return true
	}
	return false
}
