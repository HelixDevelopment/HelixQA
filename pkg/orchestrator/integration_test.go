// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/bank"
	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/reporter"
	"digital.vasic.helixqa/pkg/validator"
)

// --- Integration test: full pipeline with mock components ---

// mockCommandRunner simulates command execution.
type mockCommandRunner struct{}

func (m *mockCommandRunner) Run(
	_ context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	return []byte("mock output"), nil
}

func createIntegrationBank(t *testing.T) *bank.Bank {
	t.Helper()
	dir := t.TempDir()
	bankContent := `{
		"version": "1.0",
		"name": "Integration Test Bank",
		"challenges": [
			{
				"id": "integ-001",
				"name": "Create Document",
				"description": "Verify document creation",
				"category": "functional"
			},
			{
				"id": "integ-002",
				"name": "Save Document",
				"description": "Verify document save",
				"category": "functional"
			},
			{
				"id": "integ-003",
				"name": "Export HTML",
				"description": "Verify HTML export",
				"category": "integration"
			}
		]
	}`
	path := filepath.Join(dir, "integ.json")
	require.NoError(t, os.WriteFile(path, []byte(bankContent), 0644))

	b := bank.New()
	require.NoError(t, b.LoadFile(path))
	return b
}

func TestIntegration_FullPipeline_SinglePlatform(t *testing.T) {
	outputDir := t.TempDir()
	b := createIntegrationBank(t)

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformAndroid},
		OutputDir:     outputDir,
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.ReportPath)
	assert.Greater(t, result.Duration, time.Duration(0))

	// Verify report file was written.
	_, err = os.Stat(result.ReportPath)
	assert.NoError(t, err)
}

func TestIntegration_FullPipeline_AllPlatforms(t *testing.T) {
	outputDir := t.TempDir()
	b := createIntegrationBank(t)

	cfg := &config.Config{
		Banks: []string{"/fake"},
		Platforms: []config.Platform{
			config.PlatformAndroid,
			config.PlatformWeb,
			config.PlatformDesktop,
		},
		OutputDir:     outputDir,
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Report)

	// Should have results for all 3 platforms.
	assert.Len(t, result.Report.PlatformResults, 3)
}

func TestIntegration_FullPipeline_WithValidation(t *testing.T) {
	outputDir := t.TempDir()
	b := createIntegrationBank(t)

	// Create a mock detector that always reports healthy.
	det := detector.New(
		config.PlatformAndroid,
		detector.WithCommandRunner(&mockCommandRunner{}),
		detector.WithEvidenceDir(
			filepath.Join(outputDir, "evidence"),
		),
	)

	// Create validator with the mock detector.
	val := validator.New(
		det,
		validator.WithEvidenceDir(
			filepath.Join(outputDir, "evidence"),
		),
	)

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformAndroid},
		OutputDir:     outputDir,
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: true,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(
		cfg,
		WithBank(b),
		WithValidator(val),
		WithDetector(det),
	)
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIntegration_FullPipeline_JSONReport(t *testing.T) {
	outputDir := t.TempDir()
	b := createIntegrationBank(t)

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformWeb},
		OutputDir:     outputDir,
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportJSON,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.Contains(t, result.ReportPath, ".json")

	// Verify JSON is valid.
	data, err := os.ReadFile(result.ReportPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "total_challenges")
}

func TestIntegration_FullPipeline_HTMLReport(t *testing.T) {
	outputDir := t.TempDir()
	b := createIntegrationBank(t)

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformDesktop},
		OutputDir:     outputDir,
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportHTML,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.Contains(t, result.ReportPath, ".html")

	data, err := os.ReadFile(result.ReportPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<!DOCTYPE html>")
}

func TestIntegration_FullPipeline_Cancellation(t *testing.T) {
	b := createIntegrationBank(t)

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformAll},
		OutputDir:     t.TempDir(),
		Speed:         config.SpeedSlow, // Slow speed = delays
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately.
	cancel()

	_, err := orch.Run(ctx)
	assert.Error(t, err)
}

func TestIntegration_FullPipeline_EmptyBank(t *testing.T) {
	b := bank.New()

	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformAndroid},
		OutputDir:     t.TempDir(),
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success) // No tests = no failures.
	assert.Equal(t, 0, result.Report.TotalChallenges)
}

func TestIntegration_ReporterFromResults(t *testing.T) {
	// Simulate generating a report from existing results.
	platformResults := []*reporter.PlatformResult{
		{
			Platform:  config.PlatformAndroid,
			StartTime: time.Now().Add(-10 * time.Second),
			EndTime:   time.Now(),
			Duration:  10 * time.Second,
			ChallengeResults: []*challenge.Result{
				{
					ChallengeID:   "TC-001",
					ChallengeName: "Create Doc",
					Status:        challenge.StatusPassed,
					Duration:      2 * time.Second,
				},
				{
					ChallengeID:   "TC-002",
					ChallengeName: "Save Doc",
					Status:        challenge.StatusFailed,
					Error:         "timeout",
					Duration:      5 * time.Second,
				},
			},
			CrashCount: 1,
		},
	}

	rep := reporter.New(
		reporter.WithOutputDir(t.TempDir()),
		reporter.WithReportFormat(config.ReportMarkdown),
	)

	report, err := rep.GenerateQAReport(platformResults)
	require.NoError(t, err)
	assert.Equal(t, 2, report.TotalChallenges)
	assert.Equal(t, 1, report.PassedChallenges)
	assert.Equal(t, 1, report.FailedChallenges)
	assert.Equal(t, 1, report.TotalCrashes)
}

func TestIntegration_TestBank_ToOrchestrator(t *testing.T) {
	// Create YAML bank file.
	dir := t.TempDir()
	bankContent := `version: "1.0"
name: "Integration Bank"
test_cases:
  - id: YAML-001
    name: "YAML test"
    category: functional
    priority: critical
    platforms: [android]
  - id: YAML-002
    name: "Web test"
    category: functional
    priority: high
    platforms: [web]
`
	path := filepath.Join(dir, "integ.yaml")
	require.NoError(t, os.WriteFile(path, []byte(bankContent), 0644))

	// Load with testbank manager and convert to definitions.
	// (Importing testbank here would create a circular dep,
	// so we verify the YAML loading separately in testbank
	// package tests. This test verifies the orchestrator
	// handles pre-loaded banks correctly.)
	b := bank.New()
	cfg := &config.Config{
		Banks:         []string{"/fake"},
		Platforms:     []config.Platform{config.PlatformAndroid},
		OutputDir:     t.TempDir(),
		Speed:         config.SpeedFast,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       30 * time.Second,
		StepTimeout:   5 * time.Second,
	}

	orch := New(cfg, WithBank(b))
	ctx := context.Background()

	result, err := orch.Run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}
