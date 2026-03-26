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
	"digital.vasic.helixqa/pkg/performance"
	"digital.vasic.helixqa/pkg/planning"
	"digital.vasic.helixqa/pkg/video"
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

	// AndroidDevice is the ADB device/emulator serial
	// (e.g. "emulator-5554" or "192.168.0.214:5555").
	AndroidDevice string

	// AndroidPackage is the Android application package
	// name (e.g. "com.example.app").
	AndroidPackage string

	// WebURL is the URL for web platform testing.
	WebURL string

	// DesktopDisplay is the X11 display identifier
	// (e.g. ":0").
	DesktopDisplay string

	// FFmpegPath is the path to the ffmpeg binary used
	// for video post-processing.
	FFmpegPath string

	// CuriosityEnabled controls whether the curiosity-
	// driven exploration phase is active.
	CuriosityEnabled bool

	// CuriosityTimeout is the maximum duration for the
	// curiosity-driven exploration phase.
	CuriosityTimeout time.Duration
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

	// Create executor factory from config.
	execFactory := NewRealExecutorFactory(RealExecutorConfig{
		AndroidDevice:  sp.config.AndroidDevice,
		AndroidPackage: sp.config.AndroidPackage,
		WebURL:         sp.config.WebURL,
		DesktopDisplay: sp.config.DesktopDisplay,
	})

	// Start video recording for Android platforms.
	recorders := make(map[string]*video.ScrcpyRecorder)
	for _, platform := range sp.config.Platforms {
		if platform == "android" ||
			platform == "androidtv" {
			videoPath := filepath.Join(
				sp.config.OutputDir, "videos",
				platform+"-session.mp4",
			)
			_ = os.MkdirAll(
				filepath.Dir(videoPath), 0o755,
			)
			rec := video.NewScrcpyRecorder(
				sp.config.AndroidDevice, videoPath,
				video.WithMethod(
					video.MethodADBScreenrecord,
				),
			)
			if err := rec.Start(ctx); err == nil {
				recorders[platform] = rec
			}
		}
	}

	// Collect baseline performance metrics.
	var perfTimelines []*performance.MetricsTimeline
	for _, platform := range sp.config.Platforms {
		if platform == "android" ||
			platform == "androidtv" {
			collector := performance.New(
				sp.config.AndroidPackage, platform,
			)
			tl := &performance.MetricsTimeline{
				Platform: platform,
			}
			if snap, err := collector.CollectMemory(
				ctx,
			); err == nil {
				tl.Add(snap)
			}
			if snap, err := collector.CollectCPU(
				ctx,
			); err == nil {
				tl.Add(snap)
			}
			perfTimelines = append(perfTimelines, tl)
		}
	}

	// Iterate tests: take screenshots, record coverage.
	screenshotDir := filepath.Join(
		sp.config.OutputDir, "screenshots",
	)
	_ = os.MkdirAll(screenshotDir, 0o755)
	var allScreenshots []string

	testsRun := 0
	for _, t := range plan.Tests {
		select {
		case <-ctx.Done():
			result.Status = StatusFailed
			result.Error = "context canceled during execution"
			result.TestsRun = testsRun
			result.Duration = time.Since(start)
			sp.updateSession(sessionID, result)
			// Stop recorders before returning.
			for _, rec := range recorders {
				_ = rec.Stop()
			}
			return result, nil
		default:
		}

		testsRun++
		fmt.Printf(
			"  [%d/%d] %s (%s)\n",
			testsRun, len(plan.Tests),
			t.Name, t.Category,
		)

		// Take screenshot for each platform this test
		// targets.
		for _, platform := range t.Platforms {
			executor, err := execFactory.Create(
				platform,
			)
			if err != nil {
				continue
			}
			screenshot, err := executor.Screenshot(ctx)
			if err == nil && len(screenshot) > 0 {
				fname := filepath.Join(
					screenshotDir,
					fmt.Sprintf(
						"%s-%03d-%s.png",
						platform,
						testsRun,
						sanitizeFilename(t.Screen),
					),
				)
				_ = os.WriteFile(
					fname, screenshot, 0o644,
				)
				allScreenshots = append(
					allScreenshots, fname,
				)
			}
		}

		// Record coverage.
		screen := t.Screen
		if screen == "" {
			screen = t.Name
		}
		for _, p := range t.Platforms {
			_ = sp.store.RecordCoverage(
				screen, p, "executed",
			)
		}
	}
	result.TestsRun = testsRun

	// Stop video recorders.
	for _, rec := range recorders {
		_ = rec.Stop()
	}

	// Collect final performance metrics.
	for _, tl := range perfTimelines {
		collector := performance.New(
			sp.config.AndroidPackage, tl.Platform,
		)
		if snap, err := collector.CollectMemory(
			ctx,
		); err == nil {
			tl.Add(snap)
		}
		if snap, err := collector.CollectCPU(
			ctx,
		); err == nil {
			tl.Add(snap)
		}
	}

	fmt.Printf(
		"[pipeline]   %d tests executed\n", testsRun,
	)

	// ── Phase 4: Analyze ────────────────────────────────
	fmt.Println("[pipeline] Phase 4/4: Analyze")
	var allFindings []analysis.AnalysisFinding

	// Analyze screenshots with LLM vision.
	if sp.provider.SupportsVision() &&
		len(allScreenshots) > 0 {
		visionAnalyzer := analysis.NewVisionAnalyzer(
			sp.provider,
		)
		for _, ssPath := range allScreenshots {
			imgData, err := os.ReadFile(ssPath)
			if err != nil {
				continue
			}
			base := filepath.Base(ssPath)
			findings, err := visionAnalyzer.AnalyzeScreenshot(
				ctx, imgData, base, "",
			)
			if err == nil {
				allFindings = append(
					allFindings, findings...,
				)
			}
		}
	}

	// Check for memory leaks.
	for _, tl := range perfTimelines {
		leak := tl.DetectMemoryLeak(10.0)
		if leak != nil && leak.IsLeak {
			allFindings = append(
				allFindings,
				analysis.AnalysisFinding{
					Category: analysis.CategoryPerformance,
					Severity: analysis.SeverityHigh,
					Title: fmt.Sprintf(
						"Memory leak detected on %s",
						leak.Platform,
					),
					Description: fmt.Sprintf(
						"Memory grew %.1f%% "+
							"(%.0fKB -> %.0fKB) "+
							"over %.0fs",
						leak.GrowthPercent,
						leak.StartKB,
						leak.EndKB,
						leak.DurationSecs,
					),
					Platform: leak.Platform,
				},
			)
		}
	}

	result.IssuesFound = len(allFindings)

	// Create tickets via FindingsBridge.
	if len(allFindings) > 0 {
		bridge := NewFindingsBridge(
			sp.store, sp.config.IssuesDir, sessionID,
		)
		ids, _ := bridge.Process(allFindings)
		result.TicketsCreated = len(ids)
		fmt.Printf(
			"  Created %d issue tickets\n",
			len(ids),
		)
	}

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

// sanitizeFilename converts a screen name or label into a
// safe, lowercase filename component. Slashes and spaces
// are replaced with hyphens, and the result is capped at
// 40 characters.
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ToLower(s)
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "unknown"
	}
	return s
}

// joinStrings joins a string slice with commas.
func joinStrings(ss []string) string {
	return strings.Join(ss, ",")
}
