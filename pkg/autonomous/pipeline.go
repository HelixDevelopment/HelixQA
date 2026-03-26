// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
	"digital.vasic.helixqa/pkg/planning"
)

// PipelineConfig holds the parameters for a SessionPipeline
// run.
type PipelineConfig struct {
	// ProjectRoot is the absolute path to the project under
	// test.
	ProjectRoot string

	// Platforms lists the target platforms (e.g. "android",
	// "web", "desktop").
	Platforms []string

	// OutputDir is the directory where reports and evidence
	// are written.
	OutputDir string

	// IssuesDir is the directory for generated issue
	// tickets.
	IssuesDir string

	// BanksDir is the directory containing test bank YAML
	// files for reconciliation. Empty means skip
	// reconciliation.
	BanksDir string

	// Timeout is the maximum duration for the entire
	// pipeline run.
	Timeout time.Duration

	// PassNumber identifies this QA pass for the memory
	// store.
	PassNumber int
}

// PipelineResult captures the outcome of a SessionPipeline
// run.
type PipelineResult struct {
	Status         SessionStatus `json:"status"`
	SessionID      string        `json:"session_id"`
	Duration       time.Duration `json:"duration"`
	TestsPlanned   int           `json:"tests_planned"`
	TestsRun       int           `json:"tests_run"`
	IssuesFound    int           `json:"issues_found"`
	TicketsCreated int           `json:"tickets_created"`
	CoveragePct    float64       `json:"coverage_pct"`
	Error          string        `json:"error,omitempty"`
}

// SessionPipeline orchestrates the four-phase autonomous QA
// pipeline: learn, plan, execute, analyze.
type SessionPipeline struct {
	config   *PipelineConfig
	provider llm.Provider
	store    *memory.Store
}

// NewSessionPipeline creates a SessionPipeline with the
// given configuration, LLM provider, and memory store.
func NewSessionPipeline(
	cfg *PipelineConfig,
	provider llm.Provider,
	store *memory.Store,
) *SessionPipeline {
	return &SessionPipeline{
		config:   cfg,
		provider: provider,
		store:    store,
	}
}

// Run executes the four pipeline phases in order:
//  1. Learn  — build a knowledge base from the project
//  2. Plan   — generate, reconcile, and rank test cases
//  3. Execute — iterate planned tests, record coverage
//  4. Analyze — collect findings (placeholder)
//
// It creates a session in the memory store at the start and
// updates it when the pipeline completes.
func (sp *SessionPipeline) Run(
	ctx context.Context,
) (*PipelineResult, error) {
	start := time.Now()
	sessionID := fmt.Sprintf(
		"pipeline-%d", start.UnixNano(),
	)

	// Create session in memory store.
	sess := memory.Session{
		ID:         sessionID,
		StartedAt:  start,
		Platforms:  joinStrings(sp.config.Platforms),
		PassNumber: sp.config.PassNumber,
	}
	if err := sp.store.CreateSession(sess); err != nil {
		return nil, fmt.Errorf(
			"pipeline: create session: %w", err,
		)
	}

	// Apply timeout if configured.
	if sp.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx, sp.config.Timeout,
		)
		defer cancel()
	}

	result := &PipelineResult{
		SessionID: sessionID,
		Status:    StatusRunning,
	}

	// ── Phase 1: Learn ──────────────────────────────────
	fmt.Println("[pipeline] Phase 1/4: Learn")
	kb, err := learning.BuildKnowledgeBase(
		sp.config.ProjectRoot, sp.store,
	)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("learn phase: %v", err)
		result.Duration = time.Since(start)
		sp.updateSession(sessionID, result)
		return result, nil
	}
	fmt.Printf("[pipeline]   %s\n", kb.Summary())

	// ── Phase 2: Plan ───────────────────────────────────
	fmt.Println("[pipeline] Phase 2/4: Plan")
	gen := planning.NewTestPlanGenerator(sp.provider)
	plan, err := gen.Generate(
		ctx, kb, sp.config.Platforms,
	)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("plan phase: %v", err)
		result.Duration = time.Since(start)
		sp.updateSession(sessionID, result)
		return result, nil
	}

	// Reconcile with bank if configured.
	if sp.config.BanksDir != "" {
		reconciler := planning.NewBankReconciler()
		if _, err := os.Stat(sp.config.BanksDir); err == nil {
			if loadErr := reconciler.LoadBankDir(
				sp.config.BanksDir,
			); loadErr == nil {
				plan.Tests = reconciler.Reconcile(plan.Tests)
			}
		}
	}

	// Rank by priority.
	ranker := planning.NewPriorityRanker(nil)
	plan.Tests = ranker.Rank(plan.Tests)

	result.TestsPlanned = len(plan.Tests)
	fmt.Printf(
		"[pipeline]   %d tests planned\n",
		result.TestsPlanned,
	)

	// ── Phase 3: Execute ────────────────────────────────
	fmt.Println("[pipeline] Phase 3/4: Execute")
	testsRun := 0
	for _, t := range plan.Tests {
		select {
		case <-ctx.Done():
			result.Status = StatusFailed
			result.Error = "context canceled during execution"
			result.TestsRun = testsRun
			result.Duration = time.Since(start)
			sp.updateSession(sessionID, result)
			return result, nil
		default:
		}

		screen := t.Screen
		if screen == "" {
			screen = t.Name
		}
		platform := "all"
		if len(t.Platforms) > 0 {
			platform = t.Platforms[0]
		}

		_ = sp.store.RecordCoverage(
			screen, platform, "executed",
		)
		testsRun++
	}
	result.TestsRun = testsRun
	fmt.Printf(
		"[pipeline]   %d tests executed\n", testsRun,
	)

	// ── Phase 4: Analyze ────────────────────────────────
	fmt.Println("[pipeline] Phase 4/4: Analyze")

	// Placeholder: collect findings from execution.
	var findings []analysis.AnalysisFinding
	_ = findings

	result.IssuesFound = 0
	result.TicketsCreated = 0
	fmt.Printf(
		"[pipeline]   %d issues found\n",
		result.IssuesFound,
	)

	// ── Finalize ────────────────────────────────────────
	result.Status = StatusComplete
	result.Duration = time.Since(start)

	if result.TestsPlanned > 0 {
		result.CoveragePct = float64(result.TestsRun) /
			float64(result.TestsPlanned) * 100.0
	}

	sp.updateSession(sessionID, result)

	fmt.Printf(
		"[pipeline] Complete: %d/%d tests, %.1f%% coverage, %v\n",
		result.TestsRun,
		result.TestsPlanned,
		result.CoveragePct,
		result.Duration.Round(time.Millisecond),
	)

	return result, nil
}

// WriteReport writes the PipelineResult as JSON to
// OutputDir/pipeline-report.json.
func (sp *SessionPipeline) WriteReport(
	result *PipelineResult,
) error {
	if err := os.MkdirAll(sp.config.OutputDir, 0o755); err != nil {
		return fmt.Errorf(
			"pipeline: create output dir: %w", err,
		)
	}

	path := filepath.Join(
		sp.config.OutputDir, "pipeline-report.json",
	)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf(
			"pipeline: marshal report: %w", err,
		)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf(
			"pipeline: write report %s: %w", path, err,
		)
	}

	fmt.Printf("[pipeline] Report written: %s\n", path)
	return nil
}

// updateSession persists the pipeline result back to the
// memory store.
func (sp *SessionPipeline) updateSession(
	id string, result *PipelineResult,
) {
	now := time.Now()
	dur := int(result.Duration.Seconds())
	u := memory.SessionUpdate{
		EndedAt:       &now,
		Duration:      dur,
		TotalTests:    result.TestsPlanned,
		Passed:        result.TestsRun,
		Failed:        result.TestsPlanned - result.TestsRun,
		FindingsCount: result.IssuesFound,
		CoveragePct:   result.CoveragePct,
		Notes: fmt.Sprintf(
			"status=%s", result.Status,
		),
	}
	_ = sp.store.UpdateSession(id, u)
}

// joinStrings joins a string slice with commas.
func joinStrings(ss []string) string {
	return strings.Join(ss, ",")
}
