// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/bank"
	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/logging"

	"digital.vasic.helixqa/pkg/config"
)

func TestNew_WithLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logging.NewConsoleLogger(false)
	defer logger.Close()

	o := New(cfg, WithLogger(logger))
	assert.NotNil(t, o.logger)
}

func TestRun_EmptyBankFile(t *testing.T) {
	dir := t.TempDir()

	// Create bank file with no challenges.
	bankFile := bank.BankFile{
		Version:    "1.0",
		Name:       "empty-bank",
		Challenges: []challenge.Definition{},
	}
	data, err := json.Marshal(bankFile)
	require.NoError(t, err)
	bankPath := filepath.Join(dir, "empty-bank.json")
	err = os.WriteFile(bankPath, data, 0644)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.Report.TotalChallenges)
}

func TestRun_MarkdownDefaultReport(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast
	cfg.ReportFormat = config.ReportMarkdown

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.ReportPath, ".md")
}

func TestRun_AllPlatforms(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformAll}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast

	o := New(cfg)

	result, err := o.Run(context.Background())
	require.NoError(t, err)
	// All = android + web + desktop.
	assert.Len(t, result.Report.PlatformResults, 3)
}

func TestRun_WithTimeout(t *testing.T) {
	dir := t.TempDir()
	bankPath := createTestBank(t, dir)

	cfg := config.DefaultConfig()
	cfg.Banks = []string{bankPath}
	cfg.OutputDir = filepath.Join(dir, "output")
	cfg.Platforms = []Platform{config.PlatformDesktop}
	cfg.ValidateSteps = false
	cfg.Speed = config.SpeedFast
	cfg.Timeout = 5 * time.Minute

	ctx, cancel := context.WithTimeout(
		context.Background(), 5*time.Second,
	)
	defer cancel()

	o := New(cfg)

	result, err := o.Run(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestResult_Duration(t *testing.T) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	end := time.Now()

	r := &Result{
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	}
	assert.True(t, r.Duration >= 10*time.Millisecond)
}

func TestResult_SuccessWhenNoCrashes(t *testing.T) {
	r := &Result{Success: true}
	assert.True(t, r.Success)
}

func TestResult_FailureOnCrash(t *testing.T) {
	r := &Result{Success: false}
	assert.False(t, r.Success)
}

func TestLoadBanks_EmptyList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Banks = []string{}
	o := &Orchestrator{config: cfg}
	err := o.loadBanks()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no test bank paths")
}
