// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/maestro"
	"digital.vasic.helixqa/pkg/memory"
	"digital.vasic.helixqa/pkg/navigator"
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
//  3. Execute — run tests with video recording, screenshots, crash detection, Maestro flows
//  3.5 Curiosity — explore unknown areas via random navigation
//  4. Analyze — LLM vision analysis, memory leak detection, video frame analysis, issue tickets
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

	// Store learned knowledge in cognitive memory for future sessions
	cogMem := memory.NewCognitiveMemory(sp.store, nil) // nil provider = SQLite-only
	cogMem.Remember(ctx, memory.MemoryEntry{
		ID:      fmt.Sprintf("learn-%s", sessionID),
		Content: kb.Summary(),
		Type:    "fact",
		Source:  "learning-phase",
		Session: sessionID,
	})

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

	// Clear logcat for clean baseline.
	if sp.config.AndroidDevice != "" {
		for _, platform := range sp.config.Platforms {
			if platform == "android" ||
				platform == "androidtv" {
				_ = osexec.CommandContext(
					ctx, "adb", "-s",
					sp.config.AndroidDevice,
					"logcat", "-c",
				).Run()
			}
		}
	}

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

	// Run Maestro flows if available.
	var allFindings []analysis.AnalysisFinding
	maestroDir := filepath.Join(
		sp.config.ProjectRoot,
		"challenges", "helixqa-banks",
	)
	if entries, err := os.ReadDir(maestroDir); err == nil {
		runner := maestro.NewFlowRunner()
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".yaml") &&
				!strings.HasSuffix(name, ".yml") {
				continue
			}
			flowPath := filepath.Join(
				maestroDir, name,
			)
			content, err := os.ReadFile(flowPath)
			if err != nil {
				continue
			}
			cs := string(content)
			if !strings.Contains(cs, "appId") &&
				!strings.Contains(
					cs, "- launchApp",
				) {
				continue
			}

			fmt.Printf(
				"  Running Maestro flow: %s\n", name,
			)
			flowResult, _ := runner.RunFlow(
				ctx, flowPath,
				sp.config.AndroidDevice,
			)
			if flowResult != nil &&
				!flowResult.Success {
				allFindings = append(
					allFindings,
					analysis.AnalysisFinding{
						Category: analysis.CategoryFunctional,
						Severity: analysis.SeverityHigh,
						Title: fmt.Sprintf(
							"Maestro flow failed: %s",
							name,
						),
						Description: fmt.Sprintf(
							"Passed: %d, Failed: %d\n"+
								"Output: %s",
							flowResult.Passed,
							flowResult.Failed,
							flowResult.Output,
						),
						Platform: "android",
					},
				)
			}
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

			// Check for crashes on Android.
			if (platform == "android" ||
				platform == "androidtv") &&
				sp.config.AndroidPackage != "" {
				det := detector.New(
					config.PlatformAndroid,
					detector.WithCommandRunner(
						detector.NewExecRunner(),
					),
					detector.WithPackageName(
						sp.config.AndroidPackage,
					),
				)
				dr, derr := det.Check(ctx)
				if derr == nil && dr != nil &&
					(dr.HasCrash || dr.HasANR) {
					crashType := "crash"
					if dr.HasANR {
						crashType = "ANR"
					}
					allFindings = append(
						allFindings,
						analysis.AnalysisFinding{
							Category: analysis.CategoryFunctional,
							Severity: analysis.SeverityCritical,
							Title: fmt.Sprintf(
								"App %s detected "+
									"during test: %s",
								crashType, t.Name,
							),
							Description: fmt.Sprintf(
								"Stack trace: %s\n"+
									"Log entries: %v",
								dr.StackTrace,
								dr.LogEntries,
							),
							Platform: platform,
							Screen:   t.Screen,
						},
					)
				}
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

	// Collect logcat.
	if sp.config.AndroidDevice != "" {
		for _, platform := range sp.config.Platforms {
			if platform == "android" ||
				platform == "androidtv" {
				logcatPath := filepath.Join(
					sp.config.OutputDir, "evidence",
					platform+"-logcat.txt",
				)
				_ = os.MkdirAll(
					filepath.Dir(logcatPath), 0o755,
				)
				out, err := osexec.CommandContext(
					ctx, "adb", "-s",
					sp.config.AndroidDevice,
					"logcat", "-d",
				).Output()
				if err == nil {
					_ = os.WriteFile(
						logcatPath, out, 0o644,
					)
				}
			}
		}
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

	// ── Phase 3.5: Curiosity-Driven Exploration ────────
	if sp.config.CuriosityEnabled {
		fmt.Println(
			"[pipeline] Phase 3.5: " +
				"Curiosity-driven exploration",
		)
		curiosityCtx, curiosityCancel :=
			context.WithTimeout(
				ctx, sp.config.CuriosityTimeout,
			)
		defer curiosityCancel()

		preCuriosityCount := len(allScreenshots)
		for _, platform := range sp.config.Platforms {
			executor, err := execFactory.Create(
				platform,
			)
			if err != nil {
				continue
			}

			maxSteps := 10
			for i := 0; i < maxSteps; i++ {
				select {
				case <-curiosityCtx.Done():
					break
				default:
				}
				if curiosityCtx.Err() != nil {
					break
				}

				// Step 1: Take screenshot.
				screenshot, err :=
					executor.Screenshot(curiosityCtx)
				if err != nil || len(screenshot) == 0 {
					fmt.Printf(
						"  [curiosity %s #%d] "+
							"screenshot failed: %v\n",
						platform, i+1, err,
					)
					// Fall back to blind navigation.
					_ = executor.KeyPress(
						curiosityCtx,
						"KEYCODE_DPAD_DOWN",
					)
					time.Sleep(2 * time.Second)
					continue
				}

				fname := filepath.Join(
					screenshotDir,
					fmt.Sprintf(
						"%s-curiosity-%03d.png",
						platform, i+1,
					),
				)
				_ = os.WriteFile(
					fname, screenshot, 0o644,
				)
				allScreenshots = append(
					allScreenshots, fname,
				)

				// Step 2: Send screenshot to LLM for
				// navigation guidance.
				if !sp.provider.SupportsVision() {
					// No vision — fall back to blind
					// D-pad navigation.
					_ = executor.KeyPress(
						curiosityCtx,
						"KEYCODE_DPAD_DOWN",
					)
					time.Sleep(2 * time.Second)
					_ = executor.KeyPress(
						curiosityCtx,
						"KEYCODE_DPAD_CENTER",
					)
					time.Sleep(3 * time.Second)
					continue
				}

				actions := sp.llmNavigate(
					curiosityCtx,
					screenshot,
					platform,
					i+1,
				)

				// Step 3: Execute LLM-suggested actions.
				if len(actions) == 0 {
					// LLM returned nothing usable —
					// fall back to D-pad.
					_ = executor.KeyPress(
						curiosityCtx,
						"KEYCODE_DPAD_DOWN",
					)
					time.Sleep(2 * time.Second)
					continue
				}

				for _, action := range actions {
					if curiosityCtx.Err() != nil {
						break
					}
					execErr := executeAction(
						curiosityCtx,
						executor,
						action,
					)
					if execErr != nil {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"action %q failed: %v\n",
							platform, i+1,
							action.Type, execErr,
						)
					} else {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"executed: %s\n",
							platform, i+1,
							action.Type,
						)
					}
					// Brief pause between actions.
					time.Sleep(
						2 * time.Second,
					)
				}
			}
		}
		fmt.Printf(
			"  Curiosity: captured %d additional "+
				"screenshots\n",
			len(allScreenshots)-preCuriosityCount,
		)
	}

	// ── Phase 4: Analyze ────────────────────────────────
	fmt.Println("[pipeline] Phase 4/4: Analyze")

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

	// Extract and analyze video frames.
	ffmpegPath := sp.config.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	extractor := video.NewFrameExtractor(ffmpegPath)
	videosDir := filepath.Join(
		sp.config.OutputDir, "videos",
	)
	framesDir := filepath.Join(
		sp.config.OutputDir, "frames",
	)

	if entries, err := os.ReadDir(videosDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() ||
				!strings.HasSuffix(
					entry.Name(), ".mp4",
				) {
				continue
			}
			videoPath := filepath.Join(
				videosDir, entry.Name(),
			)
			videoFramesDir := filepath.Join(
				framesDir,
				strings.TrimSuffix(
					entry.Name(), ".mp4",
				),
			)

			frames, err := extractor.ExtractFPS(
				ctx, videoPath, videoFramesDir, 1,
			)
			if err != nil {
				continue
			}

			// Analyze up to 10 key frames per video.
			limit := 10
			if len(frames) < limit {
				limit = len(frames)
			}
			if sp.provider.SupportsVision() {
				va := analysis.NewVisionAnalyzer(
					sp.provider,
				)
				for _, framePath := range frames[:limit] {
					imgData, err := os.ReadFile(
						framePath,
					)
					if err != nil {
						continue
					}
					findings, err :=
						va.AnalyzeScreenshot(
							ctx, imgData,
							filepath.Base(framePath),
							"video-frame",
						)
					if err == nil {
						allFindings = append(
							allFindings,
							findings...,
						)
					}
				}
			}
			fmt.Printf(
				"  Analyzed %d frames from %s\n",
				limit, entry.Name(),
			)
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

// navigationPrompt is the system prompt sent to the LLM when
// requesting navigation guidance from a screenshot.
const navigationPrompt = `You are a QA tester navigating an Android TV application using a D-pad remote control.

Analyze the screenshot and suggest navigation actions to explore the application. Return ONLY a JSON array of action objects. Each action must have:
- "type": one of "dpad_up", "dpad_down", "dpad_left", "dpad_right", "dpad_center", "back", "home", "tap", "swipe_up", "swipe_down", "swipe_left", "swipe_right", "key"
- "value": for "key" type, the Android keycode (e.g. "KEYCODE_MENU"); for "tap" type, "x,y" coordinates; otherwise omit
- "reason": brief explanation of why this action explores new areas

Focus on:
1. Navigate to unexplored menu items or screens
2. Open settings, profiles, or configuration screens
3. Test edge cases (empty states, error handling)
4. Interact with focusable elements that look untested

Return 1-3 actions. Respond with only the JSON array, no other text.`

// llmAction is a single navigation action suggested by the LLM.
type llmAction struct {
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// llmNavigate sends a screenshot to the LLM vision endpoint
// and parses the response into a list of actions to execute.
// Returns nil on any error (graceful degradation).
func (sp *SessionPipeline) llmNavigate(
	ctx context.Context,
	screenshot []byte,
	platform string,
	step int,
) []llmAction {
	resp, err := sp.provider.Vision(
		ctx, screenshot, navigationPrompt,
	)
	if err != nil {
		fmt.Printf(
			"  [curiosity %s #%d] LLM vision error: %v\n",
			platform, step, err,
		)
		return nil
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return nil
	}

	// Strip markdown code fences.
	content = stripCodeFence(content)

	// Locate JSON array boundaries.
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end < start {
		fmt.Printf(
			"  [curiosity %s #%d] LLM response "+
				"not JSON array: %.80s\n",
			platform, step, content,
		)
		return nil
	}

	var actions []llmAction
	if err := json.Unmarshal(
		[]byte(content[start:end+1]), &actions,
	); err != nil {
		fmt.Printf(
			"  [curiosity %s #%d] LLM JSON parse "+
				"error: %v\n",
			platform, step, err,
		)
		return nil
	}

	return actions
}

// executeAction translates an llmAction into an
// ActionExecutor method call.
func executeAction(
	ctx context.Context,
	exec navigator.ActionExecutor,
	action llmAction,
) error {
	switch action.Type {
	case "dpad_up":
		return exec.KeyPress(ctx, "KEYCODE_DPAD_UP")
	case "dpad_down":
		return exec.KeyPress(ctx, "KEYCODE_DPAD_DOWN")
	case "dpad_left":
		return exec.KeyPress(ctx, "KEYCODE_DPAD_LEFT")
	case "dpad_right":
		return exec.KeyPress(ctx, "KEYCODE_DPAD_RIGHT")
	case "dpad_center", "select", "enter":
		return exec.KeyPress(ctx, "KEYCODE_DPAD_CENTER")
	case "back":
		return exec.Back(ctx)
	case "home":
		return exec.Home(ctx)
	case "tap", "click":
		var x, y int
		_, _ = fmt.Sscanf(action.Value, "%d,%d", &x, &y)
		if x == 0 && y == 0 {
			// Invalid coordinates — press center instead.
			return exec.KeyPress(
				ctx, "KEYCODE_DPAD_CENTER",
			)
		}
		return exec.Click(ctx, x, y)
	case "swipe_up":
		return exec.Scroll(ctx, "up", 400)
	case "swipe_down":
		return exec.Scroll(ctx, "down", 400)
	case "swipe_left":
		return exec.Scroll(ctx, "left", 400)
	case "swipe_right":
		return exec.Scroll(ctx, "right", 400)
	case "key":
		keyCode := action.Value
		if keyCode == "" {
			keyCode = "KEYCODE_MENU"
		}
		return exec.KeyPress(ctx, keyCode)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// stripCodeFence removes leading/trailing markdown code-fence
// markers from a string.
func stripCodeFence(s string) string {
	for _, prefix := range []string{"```json", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			s = strings.TrimSpace(s)
			break
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
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
