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

// perTestTimeout is the maximum time a single test
// iteration (screenshot + crash check per platform) is
// allowed to take before being abandoned. This prevents a
// hung ADB screencap or crash-check from blocking the
// entire pipeline.
const perTestTimeout = 2 * time.Minute

// perMaestroFlowTimeout limits individual Maestro flow
// runs so a single stuck flow cannot consume the session.
const perMaestroFlowTimeout = 3 * time.Minute

// maxVisionScreenshots caps how many screenshots are sent
// to the LLM vision API during the analysis phase.
const maxVisionScreenshots = 15

// maxVisionFrames caps how many video frames per video
// are sent to vision analysis.
const maxVisionFrames = 3

// maxCuriositySteps limits exploration steps per platform.
// 50 steps allow the agent to navigate through login,
// browse ALL content rails, open details, test favorites,
// play media, explore settings, and test edge cases —
// like a thorough human QA session.
const maxCuriositySteps = 50

// logcatTimeout limits the logcat dump so a large log
// buffer cannot stall the pipeline.
const logcatTimeout = 15 * time.Second

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
	phaseStart := time.Now()
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
	fmt.Printf(
		"[pipeline]   Learn completed in %v\n",
		time.Since(phaseStart).Round(time.Millisecond),
	)

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
	phaseStart = time.Now()
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
		"[pipeline]   %d tests planned in %v\n",
		result.TestsPlanned,
		time.Since(phaseStart).Round(time.Millisecond),
	)

	// ── Phase 3: Execute ────────────────────────────────
	phaseStart = time.Now()
	fmt.Println("[pipeline] Phase 3/4: Execute")

	// Create executor factory from config.
	execFactory := NewRealExecutorFactory(RealExecutorConfig{
		AndroidDevice:  sp.config.AndroidDevice,
		AndroidPackage: sp.config.AndroidPackage,
		WebURL:         sp.config.WebURL,
		DesktopDisplay: sp.config.DesktopDisplay,
	})

	// Clear logcat for clean baseline (with timeout).
	if sp.config.AndroidDevice != "" {
		for _, platform := range sp.config.Platforms {
			if platform == "android" ||
				platform == "androidtv" {
				logcatCtx, logcatCancel :=
					context.WithTimeout(
						ctx, logcatTimeout,
					)
				_ = osexec.CommandContext(
					logcatCtx, "adb", "-s",
					sp.config.AndroidDevice,
					"logcat", "-c",
				).Run()
				logcatCancel()
				fmt.Println(
					"  [exec] logcat cleared",
				)
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
			if mkErr := os.MkdirAll(
				filepath.Dir(videoPath), 0o755,
			); mkErr != nil {
				fmt.Printf(
					"  [exec] mkdir for video failed: %v\n",
					mkErr,
				)
			}
			rec := video.NewScrcpyRecorder(
				sp.config.AndroidDevice, videoPath,
				video.WithMethod(
					video.MethodADBScreenrecord,
				),
			)
			if err := rec.Start(ctx); err == nil {
				recorders[platform] = rec
				fmt.Printf(
					"  [exec] video recording "+
						"started for %s\n",
					platform,
				)
			} else {
				fmt.Printf(
					"  [exec] video recording "+
						"failed for %s: %v\n",
					platform, err,
				)
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

	// Run Maestro flows if available (with per-flow
	// timeout).
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
				"  [exec] Maestro flow: %s\n", name,
			)
			flowCtx, flowCancel :=
				context.WithTimeout(
					ctx, perMaestroFlowTimeout,
				)
			flowResult, flowErr := runner.RunFlow(
				flowCtx, flowPath,
				sp.config.AndroidDevice,
			)
			flowCancel()

			if flowErr != nil {
				fmt.Printf(
					"  [exec] Maestro flow %s "+
						"error: %v\n",
					name, flowErr,
				)
			}
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
	// Each test gets its own timeout so a hung ADB
	// command cannot block the entire pipeline.
	screenshotDir := filepath.Join(
		sp.config.OutputDir, "screenshots",
	)
	if mkErr := os.MkdirAll(screenshotDir, 0o755); mkErr != nil {
		fmt.Printf("  [exec] mkdir screenshots failed: %v\n", mkErr)
	}
	var allScreenshots []string

	testsRun := 0
	testsSkipped := 0
	for _, t := range plan.Tests {
		select {
		case <-ctx.Done():
			fmt.Printf(
				"  [exec] pipeline context expired "+
					"after %d/%d tests "+
					"(elapsed %v)\n",
				testsRun, len(plan.Tests),
				time.Since(start).Round(
					time.Millisecond,
				),
			)
			result.Status = StatusFailed
			result.Error = fmt.Sprintf(
				"context canceled during execution "+
					"after %d/%d tests",
				testsRun, len(plan.Tests),
			)
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
		testStart := time.Now()
		fmt.Printf(
			"  [%d/%d] %s (%s) ...\n",
			testsRun, len(plan.Tests),
			t.Name, t.Category,
		)

		// Per-test timeout context.
		testCtx, testCancel := context.WithTimeout(
			ctx, perTestTimeout,
		)

		// Take screenshot for each platform this test
		// targets.
		for _, platform := range t.Platforms {
			executor, err := execFactory.Create(
				platform,
			)
			if err != nil {
				fmt.Printf(
					"    [%s] executor error: %v\n",
					platform, err,
				)
				continue
			}

			ssStart := time.Now()
			screenshot, err :=
				executor.Screenshot(testCtx)
			ssDur := time.Since(ssStart)
			if err != nil {
				fmt.Printf(
					"    [%s] screenshot failed "+
						"(%v): %v\n",
					platform, ssDur.Round(
						time.Millisecond,
					), err,
				)
				testsSkipped++
				continue
			}
			if len(screenshot) == 0 {
				fmt.Printf(
					"    [%s] screenshot empty "+
						"(%v)\n",
					platform, ssDur.Round(
						time.Millisecond,
					),
				)
				continue
			}
			fmt.Printf(
				"    [%s] screenshot OK "+
					"(%dKB, %v)\n",
				platform,
				len(screenshot)/1024,
				ssDur.Round(time.Millisecond),
			)

			fname := filepath.Join(
				screenshotDir,
				fmt.Sprintf(
					"%s-%03d-%s.png",
					platform,
					testsRun,
					sanitizeFilename(t.Screen),
				),
			)
			if wErr := os.WriteFile(
				fname, screenshot, 0o644,
			); wErr != nil {
				fmt.Printf(
					"    [%s] write screenshot failed: %v\n",
					platform, wErr,
				)
			}
			allScreenshots = append(
				allScreenshots, fname,
			)

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
				dr, derr := det.Check(testCtx)
				if derr == nil && dr != nil &&
					(dr.HasCrash || dr.HasANR) {
					crashType := "crash"
					if dr.HasANR {
						crashType = "ANR"
					}
					fmt.Printf(
						"    [%s] %s detected!\n",
						platform, crashType,
					)
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
				} else if derr != nil {
					fmt.Printf(
						"    [%s] crash check "+
							"error: %v\n",
						platform, derr,
					)
				}
			}
		}

		testCancel()

		// Record coverage.
		screen := t.Screen
		if screen == "" {
			screen = t.Name
		}
		for _, p := range t.Platforms {
			if covErr := sp.store.RecordCoverage(
				screen, p, "executed",
			); covErr != nil {
				fmt.Printf(
					"    [coverage] record failed: %v\n",
					covErr,
				)
			}
		}

		fmt.Printf(
			"  [%d/%d] done in %v\n",
			testsRun, len(plan.Tests),
			time.Since(testStart).Round(
				time.Millisecond,
			),
		)
	}
	result.TestsRun = testsRun

	// Stop video recorders.
	for p, rec := range recorders {
		if err := rec.Stop(); err != nil {
			fmt.Printf(
				"  [exec] video stop %s: %v\n",
				p, err,
			)
		} else {
			fmt.Printf(
				"  [exec] video stopped for %s\n", p,
			)
		}
	}

	// Collect logcat (with dedicated timeout).
	if sp.config.AndroidDevice != "" {
		for _, platform := range sp.config.Platforms {
			if platform == "android" ||
				platform == "androidtv" {
				logcatPath := filepath.Join(
					sp.config.OutputDir, "evidence",
					platform+"-logcat.txt",
				)
				if mkErr := os.MkdirAll(
						filepath.Dir(logcatPath), 0o755,
					); mkErr != nil {
						fmt.Printf(
							"  [exec] mkdir logcat failed: %v\n",
							mkErr,
						)
					}
				lcCtx, lcCancel :=
					context.WithTimeout(
						ctx, logcatTimeout,
					)
				out, err := osexec.CommandContext(
					lcCtx, "adb", "-s",
					sp.config.AndroidDevice,
					"logcat", "-d",
				).Output()
				lcCancel()
				if err == nil {
					if wErr := os.WriteFile(
						logcatPath, out, 0o644,
					); wErr != nil {
						fmt.Printf(
							"  [exec] write logcat failed: %v\n",
							wErr,
						)
					}
					fmt.Printf(
						"  [exec] logcat saved "+
							"(%dKB)\n",
						len(out)/1024,
					)
				} else {
					fmt.Printf(
						"  [exec] logcat failed: "+
							"%v\n", err,
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
		"[pipeline]   %d tests executed, "+
			"%d skipped, Execute took %v\n",
		testsRun, testsSkipped,
		time.Since(phaseStart).Round(time.Millisecond),
	)

	// ── Phase 3.5: Curiosity-Driven Exploration ────────
	if sp.config.CuriosityEnabled {
		phaseStart = time.Now()
		curiosityBudget := sp.config.CuriosityTimeout
		fmt.Printf(
			"[pipeline] Phase 3.5: "+
				"Curiosity-driven exploration "+
				"(budget %v)\n",
			curiosityBudget,
		)
		curiosityCtx, curiosityCancel :=
			context.WithTimeout(ctx, curiosityBudget)
		defer curiosityCancel()

		preCuriosityCount := len(allScreenshots)

		// Launch the app on Android platforms before
		// curiosity exploration to ensure it is in the
		// foreground. This is essential for fire-and-forget
		// reliability — the LLM must see the app, not the
		// home screen or another app.
		if sp.config.AndroidPackage != "" &&
			sp.config.AndroidDevice != "" {
			for _, platform := range sp.config.Platforms {
				if platform == "android" ||
					platform == "androidtv" {
					launchCtx, launchCancel :=
						context.WithTimeout(ctx, 10*time.Second)
					_, _ = osexec.CommandContext(
						launchCtx, "adb", "-s",
						sp.config.AndroidDevice,
						"shell", "monkey", "-p",
						sp.config.AndroidPackage,
						"-c", "android.intent.category.LEANBACK_LAUNCHER",
						"1",
					).CombinedOutput()
					launchCancel()
					time.Sleep(3 * time.Second)
					fmt.Printf(
						"  [curiosity] launched %s on %s\n",
						sp.config.AndroidPackage, platform,
					)
				}
			}
		}

		for _, platform := range sp.config.Platforms {
			executor, err := execFactory.Create(
				platform,
			)
			if err != nil {
				continue
			}

			// stepHistory tracks actions from previous
			// steps so the LLM avoids repeating itself.
			var stepHistory []string

			for i := 0; i < maxCuriositySteps; i++ {
				if curiosityCtx.Err() != nil {
					break
				}

				// Step 1: Take screenshot.
				stepStart := time.Now()
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
					time.Sleep(1 * time.Second)
					continue
				}

				fname := filepath.Join(
					screenshotDir,
					fmt.Sprintf(
						"%s-curiosity-%03d.png",
						platform, i+1,
					),
				)
				if wErr := os.WriteFile(
					fname, screenshot, 0o644,
				); wErr != nil {
					fmt.Printf(
						"  [curiosity %s #%d] write screenshot failed: %v\n",
						platform, i+1, wErr,
					)
				}
				allScreenshots = append(
					allScreenshots, fname,
				)

				// Step 2: Send resized screenshot to
				// LLM for navigation guidance.
				if !sp.provider.SupportsVision() {
					// Use structured fallback when no
					// vision is available.
					fbActions := fallbackActions(i)
					for _, a := range fbActions {
						_ = executeAction(
							curiosityCtx, executor, a,
						)
						time.Sleep(1 * time.Second)
					}
					continue
				}

				// Resize before sending to LLM to
				// reduce latency and token cost.
				resized := resizeScreenshot(screenshot)
				actions := sp.llmNavigate(
					curiosityCtx,
					resized,
					platform,
					i+1,
					stepHistory,
				)

				// Step 3: Execute LLM-suggested actions.
				// If the LLM returned no actions (rate
				// limit, parse error), use a structured
				// fallback pattern that progresses through
				// login and navigation like a real user.
				if len(actions) == 0 {
					actions = fallbackActions(i)
					fmt.Printf(
						"  [curiosity %s #%d] using "+
							"fallback navigation\n",
						platform, i+1,
					)
				}

				var stepActions []string
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
								"action %q "+
								"failed: %v\n",
							platform, i+1,
							action.Type, execErr,
						)
					} else {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"executed: %s "+
								"(%s)\n",
							platform, i+1,
							action.Type,
							action.Reason,
						)
					}
					desc := action.Type
					if action.Value != "" {
						desc += "(" + action.Value + ")"
					}
					stepActions = append(
						stepActions, desc,
					)
					// Pause between actions. Typing
					// and keyboard dismiss need extra
					// time on Android TV.
					switch action.Type {
					case "type":
						time.Sleep(2 * time.Second)
					case "back":
						time.Sleep(2 * time.Second)
					default:
						time.Sleep(1 * time.Second)
					}
				}
				// Record what was done for context.
				stepHistory = append(
					stepHistory,
					fmt.Sprintf(
						"Step %d: %s",
						i+1,
						strings.Join(stepActions, ", "),
					),
				)

				fmt.Printf(
					"  [curiosity %s #%d] "+
						"step done in %v\n",
					platform, i+1,
					time.Since(stepStart).Round(
						time.Millisecond,
					),
				)
			}
		}
		fmt.Printf(
			"  Curiosity: captured %d additional "+
				"screenshots in %v\n",
			len(allScreenshots)-preCuriosityCount,
			time.Since(phaseStart).Round(
				time.Millisecond,
			),
		)
	}

	// ── Phase 4: Analyze ────────────────────────────────
	phaseStart = time.Now()
	fmt.Println("[pipeline] Phase 4/4: Analyze")

	// Analyze screenshots with LLM vision — bounded to
	// maxVisionScreenshots to prevent timeout. We select
	// evenly spaced screenshots for best coverage.
	if sp.provider.SupportsVision() &&
		len(allScreenshots) > 0 {
		visionAnalyzer := analysis.NewVisionAnalyzer(
			sp.provider,
		)
		toAnalyze := selectEvenly(
			allScreenshots, maxVisionScreenshots,
		)
		fmt.Printf(
			"  [analyze] analysing %d/%d "+
				"screenshots via LLM vision\n",
			len(toAnalyze), len(allScreenshots),
		)
		for i, ssPath := range toAnalyze {
			if ctx.Err() != nil {
				fmt.Printf(
					"  [analyze] context expired "+
						"after %d screenshots\n",
					i,
				)
				break
			}
			imgData, err := os.ReadFile(ssPath)
			if err != nil {
				continue
			}
			// Resize to reduce LLM latency.
			imgData = resizeScreenshot(imgData)
			base := filepath.Base(ssPath)

			vStart := time.Now()
			findings, err :=
				visionAnalyzer.AnalyzeScreenshot(
					ctx, imgData, base, "",
				)
			vDur := time.Since(vStart)
			if err != nil {
				fmt.Printf(
					"  [analyze] vision %s "+
						"failed (%v): %v\n",
					base, vDur.Round(
						time.Millisecond,
					), err,
				)
				continue
			}
			fmt.Printf(
				"  [analyze] vision %s: "+
					"%d findings (%v)\n",
				base, len(findings),
				vDur.Round(time.Millisecond),
			)
			allFindings = append(
				allFindings, findings...,
			)
		}
	}

	// Extract and analyze video frames — bounded.
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
			if ctx.Err() != nil {
				break
			}
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
				fmt.Printf(
					"  [analyze] frame extract "+
						"failed for %s: %v\n",
					entry.Name(), err,
				)
				continue
			}

			limit := maxVisionFrames
			if len(frames) < limit {
				limit = len(frames)
			}
			if sp.provider.SupportsVision() && limit > 0 {
				va := analysis.NewVisionAnalyzer(
					sp.provider,
				)
				for _, framePath := range frames[:limit] {
					if ctx.Err() != nil {
						break
					}
					imgData, err := os.ReadFile(
						framePath,
					)
					if err != nil {
						continue
					}
					imgData = resizeScreenshot(imgData)
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
				"  [analyze] %d frames from %s\n",
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
		"[pipeline]   %d issues found, "+
			"Analyze took %v\n",
		result.IssuesFound,
		time.Since(phaseStart).Round(time.Millisecond),
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
		"[pipeline] Complete: %d/%d tests, "+
			"%.1f%% coverage, %v total\n",
		result.TestsRun,
		result.TestsPlanned,
		result.CoveragePct,
		result.Duration.Round(time.Millisecond),
	)

	return result, nil
}

// selectEvenly returns up to max elements from the slice,
// picking elements at evenly-spaced indices for
// representative coverage. If the slice has fewer than max
// elements, all are returned.
func selectEvenly(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	step := float64(len(items)) / float64(max)
	selected := make([]string, 0, max)
	for i := 0; i < max; i++ {
		idx := int(float64(i) * step)
		if idx >= len(items) {
			idx = len(items) - 1
		}
		selected = append(selected, items[idx])
	}
	return selected
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
	if err := sp.store.UpdateSession(id, u); err != nil {
		fmt.Printf("[pipeline] update session failed: %v\n", err)
	}
}

// navigationPromptTemplate is the system prompt sent to
// the LLM when requesting navigation guidance from a
// screenshot. It is formatted at runtime with app-specific
// context from the Learn phase knowledge base.
const navigationPromptTemplate = `You are an expert QA tester performing a FULL autonomous QA session on the Catalogizer Android TV application — a media collection manager. You must test EVERY feature like a real human QA tester would.

APPLICATION CONTEXT:
- App name: Catalogizer — Advanced Multi-Protocol Media Collection Management System
- Login credentials: username "admin", password "admin123"
- Server URL: The app connects to a catalog-api backend (already configured)
- Key screens: Login, Home/Dashboard (content rails), Media Browser, Entity Details, Collections, Favorites, Settings, Search
- The app uses Jetpack Compose with a dark theme on Android TV

YOUR MISSION (in order):
1. LOGIN SCREEN: Use this EXACT sequence (tested and verified):
   a) dpad_up, dpad_up, dpad_up — navigate to Username field
   b) dpad_center — ACTIVATE the text field (opens keyboard, REQUIRED before typing)
   c) type "admin" — enters username
   d) tab — moves focus AND text cursor to Password field, dismisses keyboard
   e) dpad_center — ACTIVATE the password field
   f) type "admin123" — enters password
   g) tab — moves to Sign In button
   h) dpad_center — submit login
   CRITICAL RULES: You MUST press dpad_center to activate a text field BEFORE using "type".
   Use "tab" (not back+dpad_down) to move between fields — tab moves the text input cursor.
2. SERVER URL "localhost:8080" is correct. Do NOT change it.
3. After login: browse ALL content rails on the home screen (scroll down, right)
4. Open media items to view details
5. Test favorites: add/remove items from favorites
6. Test collections: browse existing collections
7. Test search functionality
8. Navigate to settings and explore all options
9. Test edge cases: press back from various screens, try invalid navigation
10. Look for UI bugs: misaligned elements, empty screens, broken layouts, missing data

CRITICAL ANDROID TV RULES:
- Navigation is D-pad ONLY. NO touch input.
- You MUST press dpad_center to ACTIVATE a text field before "type" will work. Without activation, text goes nowhere.
- "type" action sends text to the activated text field via ADB.
- Use "tab" to move between form fields — it moves BOTH D-pad focus and text cursor, and dismisses the keyboard.
- Do NOT use "back" + "dpad_down" to move between fields — that leaves the text cursor in the wrong field.
- DPAD_CENTER on a non-text element = click/select it.
- The login screen field order top-to-bottom: Username, Password, Sign In, Server URL, Discover, Connect.

RESPONSE FORMAT:
Return ONLY a JSON array of 1-5 actions. Each action:
- "type": one of "dpad_up", "dpad_down", "dpad_left", "dpad_right", "dpad_center", "back", "home", "type", "tab", "key", "wait"
- "value": for "type" = the text to enter; otherwise omit
- "reason": brief explanation of what you expect this action to achieve

The "wait" action pauses for 3 seconds — use it after login submission or before checking screen transitions.

IMPORTANT TESTING GOALS (cover ALL of these during the session):
- Login successfully (follow the exact sequence above)
- After login, wait for home screen to load, then browse ALL content rails (scroll down and right)
- Open at least 2 media items to view their detail screens
- Test favorites: select an item and toggle favorite on/off
- Test media playback: open a media item and press play
- Navigate to Settings and explore all options
- Test Search: navigate to search, type a query, verify results
- Press Back from various screens to test navigation stack
- Look for bugs: empty screens, broken layouts, missing data, unresponsive elements

Think step by step about what screen you see and what the NEXT logical QA action is.
Respond with ONLY the JSON array, no other text.`

// llmAction is a single navigation action suggested by the LLM.
type llmAction struct {
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// llmNavigateTimeout caps a single LLM vision call during
// curiosity navigation so one slow API response cannot
// stall the exploration phase.
// llmNavigateTimeout caps a single LLM vision call. Set to
// 90s to allow Gemini's internal 5-retry backoff (up to ~75s)
// to succeed before the call is abandoned.
const llmNavigateTimeout = 90 * time.Second

// llmNavigate sends a (pre-resized) screenshot to the LLM
// vision endpoint and parses the response into a list of
// actions to execute. The screenshot should already be
// resized by the caller. Returns nil on any error (graceful
// degradation). A per-call timeout prevents slow API
// responses from blocking the curiosity loop.
func (sp *SessionPipeline) llmNavigate(
	ctx context.Context,
	screenshot []byte,
	platform string,
	step int,
	history []string,
) []llmAction {
	// Build the prompt with step history so the LLM
	// knows what it already did and can progress.
	prompt := navigationPromptTemplate
	if len(history) > 0 {
		prompt += "\n\nPREVIOUS ACTIONS IN THIS SESSION " +
			"(do NOT repeat these — move to the NEXT " +
			"logical step):\n"
		for _, h := range history {
			prompt += "- " + h + "\n"
		}
		prompt += "\nBased on the screenshot and your " +
			"previous actions, decide the NEXT step. " +
			"Do NOT repeat what you already did."
	}

	// Apply a per-call timeout on top of the parent
	// context.
	callCtx, callCancel := context.WithTimeout(
		ctx, llmNavigateTimeout,
	)
	defer callCancel()

	visionStart := time.Now()
	resp, err := sp.provider.Vision(
		callCtx, screenshot, prompt,
	)
	visionDur := time.Since(visionStart)
	if err != nil {
		fmt.Printf(
			"  [curiosity %s #%d] LLM vision "+
				"error (%v): %v\n",
			platform, step,
			visionDur.Round(time.Millisecond), err,
		)
		return nil
	}
	fmt.Printf(
		"  [curiosity %s #%d] LLM responded in %v\n",
		platform, step,
		visionDur.Round(time.Millisecond),
	)

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
	case "tab":
		return exec.KeyPress(ctx, "KEYCODE_TAB")
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
	case "type":
		if action.Value == "" {
			return nil
		}
		return exec.Type(ctx, action.Value)
	case "key":
		keyCode := action.Value
		if keyCode == "" {
			keyCode = "KEYCODE_MENU"
		}
		return exec.KeyPress(ctx, keyCode)
	case "wait":
		// Allow the LLM to insert deliberate pauses for
		// screen transitions, login processing, etc.
		time.Sleep(3 * time.Second)
		return nil
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

// fallbackActions returns a deterministic sequence of actions
// based on the step number, simulating a real QA session when
// the LLM vision provider is unavailable. The sequence covers:
// login (steps 0-9), home browsing (10-24), detail views (25-34),
// favorites (35-39), and settings (40-49).
func fallbackActions(step int) []llmAction {
	switch {
	case step == 0:
		// Navigate to username field and activate it.
		return []llmAction{
			{Type: "dpad_up", Reason: "navigate to username field"},
			{Type: "dpad_up", Reason: "ensure at top"},
			{Type: "dpad_up", Reason: "ensure at top"},
			{Type: "dpad_center", Reason: "activate username field"},
		}
	case step == 1:
		return []llmAction{
			{Type: "type", Value: "admin", Reason: "enter username"},
		}
	case step == 2:
		return []llmAction{
			{Type: "tab", Reason: "move to password field"},
			{Type: "dpad_center", Reason: "activate password field"},
		}
	case step == 3:
		return []llmAction{
			{Type: "type", Value: "admin123", Reason: "enter password"},
		}
	case step == 4:
		return []llmAction{
			{Type: "tab", Reason: "move to Sign In button"},
			{Type: "dpad_center", Reason: "submit login"},
		}
	case step == 5:
		return []llmAction{
			{Type: "wait", Reason: "wait for login to complete"},
		}
	case step >= 6 && step <= 10:
		// Browse home screen rails — scroll down.
		return []llmAction{
			{Type: "dpad_down", Reason: "browse content rails"},
		}
	case step >= 11 && step <= 15:
		// Browse horizontally through rails.
		return []llmAction{
			{Type: "dpad_right", Reason: "browse rail items"},
		}
	case step == 16:
		// Open a media item.
		return []llmAction{
			{Type: "dpad_center", Reason: "open media item detail"},
		}
	case step == 17:
		return []llmAction{
			{Type: "wait", Reason: "wait for detail screen"},
		}
	case step == 18:
		// Scroll detail screen.
		return []llmAction{
			{Type: "dpad_down", Reason: "scroll detail screen"},
			{Type: "dpad_down", Reason: "see more details"},
		}
	case step == 19:
		// Try to play media.
		return []llmAction{
			{Type: "dpad_center", Reason: "attempt to play media"},
		}
	case step == 20:
		return []llmAction{
			{Type: "wait", Reason: "wait for playback"},
		}
	case step == 21:
		return []llmAction{
			{Type: "back", Reason: "go back from player/detail"},
		}
	case step >= 22 && step <= 25:
		return []llmAction{
			{Type: "dpad_down", Reason: "continue browsing"},
			{Type: "dpad_right", Reason: "explore more items"},
		}
	case step == 26:
		return []llmAction{
			{Type: "dpad_center", Reason: "open another item"},
		}
	case step == 27:
		return []llmAction{
			{Type: "dpad_up", Reason: "navigate to favorite button"},
			{Type: "dpad_center", Reason: "toggle favorite"},
		}
	case step == 28:
		return []llmAction{
			{Type: "back", Reason: "go back to browse"},
		}
	case step >= 29 && step <= 32:
		return []llmAction{
			{Type: "dpad_left", Reason: "navigate left"},
		}
	case step >= 33 && step <= 36:
		return []llmAction{
			{Type: "dpad_up", Reason: "scroll up to top"},
		}
	case step >= 37 && step <= 40:
		// Navigate to settings or search.
		return []llmAction{
			{Type: "dpad_down", Reason: "explore more UI"},
			{Type: "dpad_center", Reason: "select element"},
		}
	default:
		// Later steps: back navigation and re-exploration.
		if step%3 == 0 {
			return []llmAction{
				{Type: "back", Reason: "test back navigation"},
			}
		}
		if step%3 == 1 {
			return []llmAction{
				{Type: "dpad_down", Reason: "continue exploration"},
				{Type: "dpad_right", Reason: "explore rail"},
			}
		}
		return []llmAction{
			{Type: "dpad_center", Reason: "interact with element"},
		}
	}
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
