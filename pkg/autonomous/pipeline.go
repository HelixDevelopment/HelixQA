// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	visionremote "digital.vasic.visionengine/pkg/remote"
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
	// For single-device mode.
	AndroidDevice string

	// AndroidDevices is a list of all ADB devices to test
	// in parallel. When non-empty, the pipeline creates one
	// executor + vision slot per device and runs curiosity
	// in parallel goroutines.
	AndroidDevices []string

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

	// VisionHost is the hostname of the remote machine
	// running Ollama for vision inference (e.g.
	// "thinker.local"). Empty disables auto-deploy.
	VisionHost string

	// VisionUser is the SSH user for the vision host.
	VisionUser string

	// VisionModel is the Ollama model to use for vision
	// (default "llava:7b").
	VisionModel string

	// UseLlamaCpp switches from Ollama to llama.cpp backend.
	// When true, HelixQA uses llama-server instances (one per
	// platform/device) for true multi-instance vision.
	UseLlamaCpp bool

	// LlamaCppModelPath is the path to the GGUF model on the
	// remote host (e.g. ~/models/llava-7b-q4.gguf).
	LlamaCppModelPath string

	// LlamaCppMMProjPath is the path to the multimodal
	// projector GGUF on the remote host.
	LlamaCppMMProjPath string

	// QACredentials holds login credentials discovered by
	// the Learn phase from .env files. Used to auto-login
	// via intent extras on Android TV.
	QACredentials map[string]string

	// LlamaCppFreeGPU stops Ollama before starting
	// llama-server to free GPU VRAM. Ollama is restored
	// after the QA session completes.
	LlamaCppFreeGPU bool
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

// qaUsername returns the admin username from QA credentials.
func (c *PipelineConfig) qaUsername() string {
	if c.QACredentials == nil {
		return ""
	}
	for _, key := range []string{
		"ADMIN_USERNAME", "ADMIN_USER", "USERNAME",
		"DEFAULT_USER", "TEST_USERNAME",
	} {
		if v := c.QACredentials[key]; v != "" {
			return v
		}
	}
	return ""
}

// qaPassword returns the admin password from QA credentials.
func (c *PipelineConfig) qaPassword() string {
	if c.QACredentials == nil {
		return ""
	}
	for _, key := range []string{
		"ADMIN_PASSWORD", "ADMIN_PASS", "PASSWORD",
		"DEFAULT_PASSWORD", "TEST_PASSWORD",
	} {
		if v := c.QACredentials[key]; v != "" {
			return v
		}
	}
	return ""
}

// SessionPipeline orchestrates the four-phase autonomous QA
// pipeline: learn, plan, execute, analyze.
type SessionPipeline struct {
	config   *PipelineConfig
	provider llm.Provider
	store    *memory.Store
	// kbContext holds a summary of the Learn phase knowledge
	// base, injected into navigation prompts so the LLM
	// knows app-specific details (credentials, screens, etc.)
	// without hardcoding them in the prompt templates.
	kbContext string
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

	// ── Phase 0: Vision pool setup ───────────────────────
	// Create a VisionPool with one dedicated slot per
	// platform/device. Each slot serializes its own vision
	// calls so platforms don't contend with each other.
	var visionPool *visionremote.VisionPool
	if sp.config.VisionHost != "" {
		fmt.Printf(
			"[pipeline] Phase 0: Vision pool on %s "+
				"(%d platforms)\n",
			sp.config.VisionHost,
			len(sp.config.Platforms),
		)
		poolCfg := visionremote.PoolConfig{
			Host:   sp.config.VisionHost,
			User:   sp.config.VisionUser,
			Model:  sp.config.VisionModel,
			Shared: true,
		}

		// Use llama.cpp backend when configured — provides
		// true multi-instance with one llama-server per
		// platform/device for zero contention.
		if sp.config.UseLlamaCpp {
			poolCfg.InferenceBackend = visionremote.BackendLlamaCpp
			poolCfg.Shared = false // dedicated instance per slot
			poolCfg.BasePort = 8090
			poolCfg.LlamaCpp = &visionremote.LlamaCppConfig{
				Host:       sp.config.VisionHost,
				User:       sp.config.VisionUser,
				RepoDir:    "~/llama.cpp",
				ModelPath:  sp.config.LlamaCppModelPath,
				MMProjPath: sp.config.LlamaCppMMProjPath,
				BasePort:   8090,
				GPULayers:  -1,
				ContextSize: 8192,
			}
			fmt.Printf(
				"[pipeline] Using llama.cpp backend "+
					"(dedicated instances)\n",
			)
		}

		visionPool = visionremote.NewVisionPool(poolCfg)

		// Free GPU by stopping Ollama if configured.
		// This allows MiniCPM-V to use the full GPU.
		if sp.config.LlamaCppFreeGPU &&
			sp.config.UseLlamaCpp &&
			poolCfg.LlamaCpp != nil {
			deployer := visionremote.NewLlamaCppDeployer(
				*poolCfg.LlamaCpp,
			)
			deployer.FreeGPU(ctx)
		}

		if err := visionPool.EnsureReady(ctx); err != nil {
			fmt.Printf(
				"[pipeline] warning: vision pool "+
					"failed: %v (continuing without)\n",
				err,
			)
			visionPool = nil
		} else {
			// Build slot targets — one per device for Android,
			// one per non-Android platform.
			var targets []visionremote.SlotTarget
			for _, platform := range sp.config.Platforms {
				if (platform == "android" ||
					platform == "androidtv") &&
					len(sp.config.AndroidDevices) > 0 {
					// One slot per Android device.
					for _, dev := range sp.config.AndroidDevices {
						targets = append(targets,
							visionremote.SlotTarget{
								Platform: platform,
								Device:   dev,
							},
						)
					}
					continue
				}
				device := ""
				if platform == "android" ||
					platform == "androidtv" {
					device = sp.config.AndroidDevice
				} else if platform == "web" {
					device = sp.config.WebURL
				} else if platform == "api" {
					device = "api"
				}
				targets = append(targets,
					visionremote.SlotTarget{
						Platform: platform,
						Device:   device,
					},
				)
			}
			visionPool.AssignSlots(targets)
			fmt.Printf(
				"[pipeline] %d vision slots assigned\n",
				visionPool.Size(),
			)
		}
	}

	// ── Phase 0b: ADB reverse proxy for ALL Android devices
	// Ensure every connected device can reach the API at
	// localhost:8080 via ADB reverse proxy.
	allDevices := sp.config.AndroidDevices
	if len(allDevices) == 0 && sp.config.AndroidDevice != "" {
		allDevices = []string{sp.config.AndroidDevice}
	}
	for _, device := range allDevices {
		revCtx, revCancel := context.WithTimeout(
			ctx, 10*time.Second,
		)
		out, err := osexec.CommandContext(
			revCtx, "adb", "-s", device,
			"reverse", "tcp:8080", "tcp:8080",
		).CombinedOutput()
		revCancel()
		if err != nil {
			fmt.Printf(
				"[pipeline] warning: ADB reverse "+
					"on %s failed: %v (%s)\n",
				device, err, string(out),
			)
		} else {
			fmt.Printf(
				"[pipeline] ADB reverse proxy "+
					"set on %s\n",
				device,
			)
		}
		// Also launch the app on this device.
		if sp.config.AndroidPackage != "" {
			launchCtx, lc := context.WithTimeout(
				ctx, 10*time.Second,
			)
			// Launch with QA credentials via intent extras
			// so the app auto-logs in (bypasses keyboard).
			args := []string{
				"-s", device, "shell", "am", "start",
				"-n", sp.config.AndroidPackage +
					"/.ui.MainActivity",
			}
			// Inject credentials from KB if available.
			user := sp.config.qaUsername()
			pass := sp.config.qaPassword()
			if user != "" && pass != "" {
				args = append(args,
					"--es", "qa_username", user,
					"--es", "qa_password", pass,
				)
				fmt.Printf(
					"[pipeline] launching %s on %s "+
						"with QA auto-login\n",
					sp.config.AndroidPackage, device,
				)
			}
			_, _ = osexec.CommandContext(
				launchCtx, "adb", args...,
			).CombinedOutput()
			lc()
		}
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

	// Build knowledge context for navigation prompts.
	// This injects project-specific details (credentials,
	// screens, constraints) discovered by the Learn phase
	// into the generic navigation prompts.
	var kbParts []string
	// Credentials from .env — most important for login.
	if len(kb.Credentials) > 0 {
		kbParts = append(kbParts,
			"LOGIN CREDENTIALS (from project .env):")
		for k, v := range kb.Credentials {
			kbParts = append(kbParts,
				fmt.Sprintf("  %s = %s", k, v))
		}
		// Make it explicit for the LLM.
		user := kb.Credentials["ADMIN_USERNAME"]
		if user == "" {
			user = kb.Credentials["USERNAME"]
		}
		pass := kb.Credentials["ADMIN_PASSWORD"]
		if pass == "" {
			pass = kb.Credentials["PASSWORD"]
		}
		if user != "" && pass != "" {
			kbParts = append(kbParts,
				fmt.Sprintf(
					"USE THESE CREDENTIALS: "+
						"username='%s' password='%s'",
					user, pass))
		}
	}
	if len(kb.Constraints) > 0 {
		kbParts = append(kbParts,
			"PROJECT CONSTRAINTS:")
		for _, c := range kb.Constraints {
			kbParts = append(kbParts, "- "+c)
		}
	}
	if len(kb.Screens) > 0 {
		var screenNames []string
		for _, s := range kb.Screens {
			screenNames = append(screenNames, s.Name)
		}
		kbParts = append(kbParts,
			"KNOWN SCREENS: "+strings.Join(
				screenNames, ", "))
	}
	sp.kbContext = strings.Join(kbParts, "\n")
	// Store credentials in config for app auto-login.
	if len(kb.Credentials) > 0 {
		sp.config.QACredentials = kb.Credentials
	}
	if sp.kbContext != "" {
		fmt.Printf(
			"[pipeline]   KB context: %d chars\n",
			len(sp.kbContext),
		)
	}

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

		// Build list of curiosity targets — one entry per
		// device for Android, one per non-Android platform.
		type curiosityTarget struct {
			platform string
			device   string
		}
		var curTargets []curiosityTarget
		for _, platform := range sp.config.Platforms {
			if (platform == "android" ||
				platform == "androidtv") &&
				len(sp.config.AndroidDevices) > 0 {
				for _, dev := range sp.config.AndroidDevices {
					curTargets = append(curTargets,
						curiosityTarget{platform, dev},
					)
				}
			} else {
				dev := sp.config.AndroidDevice
				if platform == "web" {
					dev = sp.config.WebURL
				} else if platform == "api" {
					dev = "api"
				}
				curTargets = append(curTargets,
					curiosityTarget{platform, dev},
				)
			}
		}

		fmt.Printf(
			"  [curiosity] %d targets: ",
			len(curTargets),
		)
		for _, ct := range curTargets {
			fmt.Printf("%s(%s) ", ct.platform, ct.device)
		}
		fmt.Println()

		// Launch app on all Android devices with auto-login.
		for _, ct := range curTargets {
			if (ct.platform == "android" ||
				ct.platform == "androidtv") &&
				sp.config.AndroidPackage != "" {
				launchCtx, lc := context.WithTimeout(
					ctx, 10*time.Second,
				)
				args := []string{
					"-s", ct.device, "shell", "am", "start",
					"-n", sp.config.AndroidPackage +
						"/.ui.MainActivity",
				}
				user := sp.config.qaUsername()
				pass := sp.config.qaPassword()
				if user != "" && pass != "" {
					args = append(args,
						"--es", "qa_username", user,
						"--es", "qa_password", pass,
					)
				}
				_, _ = osexec.CommandContext(
					launchCtx, "adb", args...,
				).CombinedOutput()
				lc()
				time.Sleep(5 * time.Second)
				fmt.Printf(
					"  [curiosity] launched %s on %s\n",
					sp.config.AndroidPackage, ct.device,
				)
			}
		}

		// Run curiosity on each target sequentially.
		// (Parallel would overload single llama-server.)
		for _, ct := range curTargets {
			platform := ct.platform
			device := ct.device

			// Create executor for this specific device.
			var executor navigator.ActionExecutor
			var err error
			if (platform == "android" ||
				platform == "androidtv") && device != "" {
				executor = navigator.NewADBExecutor(
					device,
					detector.NewExecRunner(),
				)
			} else {
				executor, err = execFactory.Create(platform)
				if err != nil {
					continue
				}
			}

			// Per-target vision provider. Uses the shared
			// AdaptiveProvider (which includes Ollama with
			// GPU support) by default. Only overrides with
			// llama-server when explicitly configured.
			platformProvider := sp.provider
			if visionPool != nil && sp.config.UseLlamaCpp {
				slot := visionPool.GetSlot(
					platform, device,
				)
				if slot != nil && slot.Endpoint != "" {
					slotProvider := llm.NewOpenAIProvider(
						llm.ProviderConfig{
							Name:    "llamacpp-" + slot.ID,
							BaseURL: slot.Endpoint,
							Model:   "llava",
						},
					)
					platformProvider = slotProvider
					fmt.Printf(
						"  [curiosity %s] using "+
							"dedicated vision: %s\n",
						platform, slot.Endpoint,
					)
				}
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
				if !platformProvider.SupportsVision() {
					// No vision provider available —
					// skip this step entirely. HelixQA
					// is fully autonomous and MUST NOT
					// use hardcoded navigation. Without
					// vision, curiosity cannot proceed.
					fmt.Printf(
						"  [curiosity %s #%d] "+
							"no vision provider — "+
							"skipping\n",
						platform, i+1,
					)
					break
				}

				// Resize before sending to LLM to
				// reduce latency and token cost.
				resized := resizeScreenshot(screenshot)

				// Acquire the platform's dedicated
				// vision slot to prevent contention
				// with other platforms' calls.
				var slot *visionremote.VisionSlot
				if visionPool != nil {
					slot = visionPool.GetSlot(
						platform,
						sp.config.AndroidDevice,
					)
					if slot != nil {
						slot.Lock()
					}
				}
				visionStart := time.Now()
				actions := sp.llmNavigate(
					curiosityCtx,
					resized,
					platform,
					i+1,
					stepHistory,
					platformProvider,
				)

				// If llama-server crashed (connection
				// refused), try to restart it via the pool.
				if len(actions) == 0 && visionPool != nil &&
					slot != nil {
					health, _ := http.Get(
						slot.Endpoint + "/health",
					)
					if health == nil ||
						health.StatusCode != 200 {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"vision server down,"+
								" restarting\n",
							platform, i+1,
						)
						if visionPool != nil &&
							sp.config.UseLlamaCpp {
							// Attempt restart via deployer
							cfg := sp.config
							deployer := visionremote.NewLlamaCppDeployer(
								visionremote.LlamaCppConfig{
									Host:        cfg.VisionHost,
									User:        cfg.VisionUser,
									RepoDir:     "~/llama.cpp",
									ModelPath:   cfg.LlamaCppModelPath,
									MMProjPath:  cfg.LlamaCppMMProjPath,
									BasePort:    slot.Port,
									ContextSize: 8192,
								},
							)
							deployer.StartInstance(
								curiosityCtx, slot.Port,
							)
							time.Sleep(10 * time.Second)
						}
					}
					if health != nil {
						health.Body.Close()
					}
				}
				if slot != nil {
					slot.RecordCall(
						time.Since(visionStart),
						nil,
					)
					slot.Unlock()
				}

				// Step 3: Execute LLM-suggested actions.
				// If the LLM returned no actions (parse
				// error), retry the vision call once.
				// HelixQA is fully autonomous — NO
				// hardcoded fallback navigation.
				if len(actions) == 0 {
					time.Sleep(2 * time.Second)
					retryShot, _ := executor.Screenshot(
						curiosityCtx,
					)
					if len(retryShot) > 0 {
						actions = sp.llmNavigate(
							curiosityCtx,
							resizeScreenshot(retryShot),
							platform,
							i+1,
							stepHistory,
							platformProvider,
						)
					}
					if len(actions) == 0 {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"LLM returned no "+
								"actions after retry"+
								" — waiting\n",
							platform, i+1,
						)
						time.Sleep(3 * time.Second)
						continue
					}
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

		// Validate that on-screen data matches API data.
		apiFindings := sp.validateAPIData(ctx)
		if len(apiFindings) > 0 {
			allFindings = append(
				allFindings, apiFindings...,
			)
			fmt.Printf(
				"  [data-validation] %d issues found\n",
				len(apiFindings),
			)
		}
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

	// ── Shutdown vision pool ────────────────────────────
	if visionPool != nil {
		visionPool.Shutdown(ctx)
	}
	// Restore Ollama if we stopped it for GPU access.
	if sp.config.LlamaCppFreeGPU && sp.config.UseLlamaCpp {
		deployer := visionremote.NewLlamaCppDeployer(
			visionremote.LlamaCppConfig{
				Host: sp.config.VisionHost,
				User: sp.config.VisionUser,
			},
		)
		deployer.RestoreOllama(ctx)
	}

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

// apiDataTimeout limits individual HTTP requests during
// API data validation so a slow or unreachable backend
// does not stall the pipeline.
const apiDataTimeout = 10 * time.Second

// validateAPIData makes HTTP requests to the catalog-api
// to verify that backend data is available and consistent
// with what should appear on screen. It returns findings
// for any errors or empty responses that indicate a data
// mismatch between the API and the UI.
func (sp *SessionPipeline) validateAPIData(
	ctx context.Context,
) []analysis.AnalysisFinding {
	baseURL := "http://localhost:8080"
	if sp.config.WebURL != "" {
		baseURL = strings.TrimRight(
			sp.config.WebURL, "/",
		)
	}

	fmt.Printf(
		"[data-validation] Validating API data "+
			"at %s\n",
		baseURL,
	)

	client := &http.Client{Timeout: apiDataTimeout}
	var findings []analysis.AnalysisFinding

	// ── 0. Login first to get auth token ────────────
	var authToken string
	loginURL := baseURL + "/api/v1/auth/login"
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	loginReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, loginURL,
		bytes.NewReader(loginBody),
	)
	if err == nil {
		loginReq.Header.Set(
			"Content-Type", "application/json",
		)
		resp, err := client.Do(loginReq)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var loginResp struct {
					SessionToken string `json:"session_token"`
				}
				if jErr := json.Unmarshal(
					body, &loginResp,
				); jErr == nil && loginResp.SessionToken != "" {
					authToken = loginResp.SessionToken
					fmt.Println(
						"[data-validation] login OK " +
							"(admin/admin123)",
					)
				}
			} else {
				fmt.Printf(
					"[data-validation] login failed "+
						"with status %d\n",
					resp.StatusCode,
				)
				findings = append(findings,
					analysis.AnalysisFinding{
						Category: analysis.CategoryFunctional,
						Severity: analysis.SeverityHigh,
						Title: fmt.Sprintf(
							"API login failed with "+
								"status %d",
							resp.StatusCode,
						),
						Description: string(body),
						Platform:    "api",
					},
				)
			}
		} else {
			fmt.Printf(
				"[data-validation] login request "+
					"failed: %v\n", err,
			)
		}
	}

	// ── 1. Entity stats ─────────────────────────────
	statsURL := baseURL + "/api/v1/entities/stats"
	statsReq, err := http.NewRequestWithContext(
		ctx, http.MethodGet, statsURL, nil,
	)
	if err == nil {
		if authToken != "" {
			statsReq.Header.Set(
				"Authorization", "Bearer "+authToken,
			)
		}
		resp, err := client.Do(statsReq)
		if err != nil {
			fmt.Printf(
				"[data-validation] entities/stats "+
					"request failed: %v\n",
				err,
			)
			findings = append(findings,
				analysis.AnalysisFinding{
					Category: analysis.CategoryFunctional,
					Severity: analysis.SeverityHigh,
					Title: "API unreachable: " +
						"entities/stats",
					Description: fmt.Sprintf(
						"GET %s failed: %v",
						statsURL, err,
					),
					Platform: "api",
				},
			)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf(
					"[data-validation] entities/stats "+
						"returned %d\n",
					resp.StatusCode,
				)
				findings = append(findings,
					analysis.AnalysisFinding{
						Category: analysis.CategoryFunctional,
						Severity: analysis.SeverityHigh,
						Title: fmt.Sprintf(
							"API error: entities/stats "+
								"returned %d",
							resp.StatusCode,
						),
						Description: string(body),
						Platform:    "api",
					},
				)
			} else {
				var statsResp struct {
					Total   int            `json:"total_entities"`
					ByType  map[string]int `json:"by_type"`
				}
				if jErr := json.Unmarshal(
					body, &statsResp,
				); jErr == nil {
					fmt.Printf(
						"[data-validation] API has "+
							"%d entities",
						statsResp.Total,
					)
					if len(statsResp.ByType) > 0 {
						var parts []string
						for k, v := range statsResp.ByType {
							parts = append(parts,
								fmt.Sprintf("%s=%d", k, v),
							)
						}
						fmt.Printf(
							" (%s)",
							strings.Join(parts, ", "),
						)
					}
					fmt.Println()

					if statsResp.Total == 0 {
						findings = append(findings,
							analysis.AnalysisFinding{
								Category: analysis.CategoryFunctional,
								Severity: analysis.SeverityHigh,
								Title: "API returned zero " +
									"entities",
								Description: "entities/stats " +
									"reports total=0; the UI " +
									"should show data if the " +
									"backend has been populated",
								Platform: "api",
							},
						)
					}
				} else {
					fmt.Printf(
						"[data-validation] entities/stats "+
							"JSON parse failed: %v\n",
						jErr,
					)
				}
			}
		}
	}

	// ── 2. Media search (authenticated) ────────────
	searchURL := baseURL +
		"/api/v1/media/search?limit=5"
	searchReq, err := http.NewRequestWithContext(
		ctx, http.MethodGet, searchURL, nil,
	)
	if err == nil {
		if authToken != "" {
			searchReq.Header.Set(
				"Authorization", "Bearer "+authToken,
			)
		}
		resp, err := client.Do(searchReq)
		if err != nil {
			fmt.Printf(
				"[data-validation] media/search "+
					"request failed: %v\n",
				err,
			)
			findings = append(findings,
				analysis.AnalysisFinding{
					Category: analysis.CategoryFunctional,
					Severity: analysis.SeverityHigh,
					Title: "API unreachable: " +
						"media/search",
					Description: fmt.Sprintf(
						"GET %s failed: %v",
						searchURL, err,
					),
					Platform: "api",
				},
			)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf(
					"[data-validation] media/search "+
						"returned %d\n",
					resp.StatusCode,
				)
				findings = append(findings,
					analysis.AnalysisFinding{
						Category: analysis.CategoryFunctional,
						Severity: analysis.SeverityHigh,
						Title: fmt.Sprintf(
							"API error: media/search "+
								"returned %d",
							resp.StatusCode,
						),
						Description: string(body),
						Platform:    "api",
					},
				)
			} else {
				var searchResp struct {
					Items []json.RawMessage `json:"items"`
					Total int               `json:"total"`
				}
				if jErr := json.Unmarshal(
					body, &searchResp,
				); jErr == nil {
					fmt.Printf(
						"[data-validation] search "+
							"returned %d items "+
							"(total %d)\n",
						len(searchResp.Items),
						searchResp.Total,
					)
					if len(searchResp.Items) == 0 &&
						searchResp.Total == 0 {
						findings = append(findings,
							analysis.AnalysisFinding{
								Category: analysis.CategoryFunctional,
								Severity: analysis.SeverityHigh,
								Title: "API search returned " +
									"zero results",
								Description: "media/search " +
									"returned no items; if " +
									"the backend is populated " +
									"this indicates a data " +
									"pipeline issue",
								Platform: "api",
							},
						)
					}
				} else {
					fmt.Printf(
						"[data-validation] media/search "+
							"JSON parse failed: %v\n",
						jErr,
					)
				}
			}
		}
	}

	if len(findings) == 0 {
		fmt.Println(
			"[data-validation] all API checks passed",
		)
	}

	return findings
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

	// Create/update "latest" symlink in the parent of the
	// session directory so users can always find the most
	// recent results at qa-results/latest/.
	parentDir := filepath.Dir(sp.config.OutputDir)
	latestLink := filepath.Join(parentDir, "latest")
	_ = os.Remove(latestLink)
	sessionDir := filepath.Base(sp.config.OutputDir)
	if err := os.Symlink(sessionDir, latestLink); err != nil {
		fmt.Printf(
			"[pipeline] warning: could not create "+
				"latest symlink: %v\n", err,
		)
	} else {
		fmt.Printf(
			"[pipeline] Updated latest -> %s\n",
			sessionDir,
		)
	}

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

// navigationPromptTemplate is the generic system prompt for
// Android/Android TV QA. It contains NO project-specific
// information — all app context comes from the screenshot
// and the LLM's visual analysis. HelixQA is decoupled and
// works with ANY app on any supported platform.
const navigationPromptTemplate = `You are an expert QA tester performing a FULL autonomous QA session on an Android TV application. You must test EVERY feature like a real human QA tester would.

Look at the screenshot carefully and determine:
1. What screen am I on? (login, home, settings, detail, search, error, etc.)
2. What is the next logical QA action?

ANDROID TV NAVIGATION RULES:
- Navigation is D-pad ONLY. NO touch input.
- dpad_up/down/left/right — move focus between UI elements
- dpad_center — select/activate the focused element
- To type in a text field: first dpad_center to activate it, then use "type"
- tab — move between form fields (moves focus AND text cursor)
- key KEYCODE_ENTER — submit forms
- back — go back to previous screen
- clear — delete text in the active field

LOGIN FLOW (when you see a login screen):
1. Navigate to the username field (dpad_up/down to reach it)
2. dpad_center to activate it
3. clear any pre-filled text
4. type the username
5. tab to move to password field
6. clear any pre-filled text
7. type the password
8. Navigate to Sign In button: dpad_down until Sign In is focused
9. dpad_center to click Sign In
IMPORTANT: Do NOT use "key KEYCODE_ENTER" to submit — navigate to the Sign In button and press dpad_center instead.

Look for credentials in the screenshot. Common defaults: admin/admin, admin/admin123, admin/password.

AFTER LOGIN — explore ALL features:
- Browse content on the home screen (dpad_down, dpad_right)
- Open items to view details (dpad_center)
- Test favorites, search, settings, collections
- Test back navigation from every screen
- Look for bugs: empty screens, broken layouts, missing data

RESPONSE FORMAT:
Return ONLY a JSON array of 1-5 actions. No other text.
Each action: {"type":"...", "value":"..." (optional), "reason":"..."}
Types: dpad_up, dpad_down, dpad_left, dpad_right, dpad_center, type, tab, key, back, clear, wait

Think step by step. Respond with ONLY the JSON array.`

// webNavigationPromptTemplate is the prompt for web browser
// QA sessions. Uses mouse clicks and keyboard input instead of
// DPAD navigation.
// webNavigationPromptTemplate is the generic prompt for web
// browser QA. No project-specific information — the LLM
// analyzes the screenshot to determine context.
const webNavigationPromptTemplate = `You are an expert QA tester performing a FULL autonomous QA session on a web application in a headless browser (1920x1080 viewport). Test EVERY feature like a real human QA tester would.

Look at the screenshot carefully and determine:
1. What page am I on? (login, dashboard, list, detail, settings, error, etc.)
2. What is the next logical QA action?

WEB INTERACTION RULES:
- click with x,y pixel coordinates to click elements
- type to enter text (click the input field first!)
- scroll_down/scroll_up to scroll the page
- key with standard names: Enter, Escape, Tab, Backspace
- back for browser back navigation
- wait pauses for 3 seconds (use after form submission)

LOGIN FLOW (when you see a login form):
1. Click the username/email field
2. Type the username (look for hints on the page, try "admin")
3. Click the password field
4. Type the password (try "admin123" or "password")
5. Click the Sign In / Login button

AFTER LOGIN — explore ALL features:
- Browse the dashboard, check stats and data
- Navigate through menu/sidebar items
- Open detail pages for items
- Test search, favorites, settings
- Test back navigation
- Look for bugs: empty screens, broken layouts, errors

RESPONSE FORMAT:
Return ONLY a JSON array of 1-5 actions. No other text.
Each action: {"type":"...", "value":"..." (optional), "reason":"..."}
Types: click, type, scroll_down, scroll_up, key, back, wait
For click: value = "x,y" coordinates. For type: value = text.

Think step by step. Respond with ONLY the JSON array.`

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
const llmNavigateTimeout = 180 * time.Second

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
	visionProvider ...llm.Provider,
) []llmAction {
	// Select the right prompt for the platform.
	var prompt string
	switch platform {
	case "web":
		prompt = webNavigationPromptTemplate
	default:
		prompt = navigationPromptTemplate
	}
	// Inject knowledge base context (credentials, screens,
	// constraints discovered during Learn phase).
	if sp.kbContext != "" {
		prompt += "\n\n" + sp.kbContext
	}
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

	// Use the per-platform provider if given, otherwise
	// fall back to the shared pipeline provider.
	vp := sp.provider
	if len(visionProvider) > 0 && visionProvider[0] != nil {
		vp = visionProvider[0]
	}

	visionStart := time.Now()
	resp, err := vp.Vision(
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
	jsonStr := content[start : end+1]
	// Repair common LLM JSON quirks before parsing.
	jsonStr = repairLLMJSON(jsonStr)
	if err := json.Unmarshal(
		[]byte(jsonStr), &actions,
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
	case "swipe_up", "scroll_up":
		return exec.Scroll(ctx, "up", 400)
	case "swipe_down", "scroll_down":
		return exec.Scroll(ctx, "down", 400)
	case "swipe_left", "scroll_left":
		return exec.Scroll(ctx, "left", 400)
	case "swipe_right", "scroll_right":
		return exec.Scroll(ctx, "right", 400)
	case "type":
		if action.Value == "" {
			return nil
		}
		return exec.Type(ctx, action.Value)
	case "key":
		keyCode := action.Value
		if keyCode == "" {
			// Infer key from reason — LLMs often omit the
			// value but describe the intent in the reason.
			reason := strings.ToLower(action.Reason)
			if strings.Contains(reason, "submit") ||
				strings.Contains(reason, "login") ||
				strings.Contains(reason, "enter") ||
				strings.Contains(reason, "confirm") {
				keyCode = "KEYCODE_ENTER"
			} else {
				keyCode = "KEYCODE_ENTER"
			}
		}
		return exec.KeyPress(ctx, keyCode)
	case "wait":
		// Allow the LLM to insert deliberate pauses for
		// screen transitions, login processing, etc.
		time.Sleep(3 * time.Second)
		return nil
	case "clear":
		// Select all text in the active field and delete it.
		// Uses Ctrl+A (select all) then Delete to clear any
		// pre-filled text before typing new content.
		_ = exec.KeyPress(ctx, "KEYCODE_MOVE_END")
		time.Sleep(200 * time.Millisecond)
		// Delete up to 30 characters to clear the field
		for i := 0; i < 30; i++ {
			_ = exec.KeyPress(ctx, "KEYCODE_DEL")
		}
		time.Sleep(300 * time.Millisecond)
		return nil
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// repairLLMJSON fixes common JSON formatting issues from LLM
// vision models (especially LLaVA) that return almost-valid
// JSON. Handles: trailing commas, single quotes, missing
// commas between objects, and bare string values.
func repairLLMJSON(s string) string {
	// Remove literal newlines inside string values.
	// LLaVA sometimes puts \n inside JSON strings which
	// breaks the parser.
	var result strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			result.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			result.WriteByte(c)
			continue
		}
		if c == '"' {
			inString = !inString
		}
		if c == '\n' && inString {
			result.WriteString("\\n")
			continue
		}
		result.WriteByte(c)
	}
	s = result.String()

	// Replace single quotes with double quotes (but not
	// within already double-quoted strings).
	if !strings.Contains(s, `"`) && strings.Contains(s, `'`) {
		s = strings.ReplaceAll(s, `'`, `"`)
	}

	// Remove trailing commas before ] or }.
	for _, pair := range [][2]string{
		{",]", "]"}, {",}", "}"},
		{", ]", "]"}, {", }", "}"},
	} {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}

	// Fix missing comma between adjacent objects: }{ → },{
	s = strings.ReplaceAll(s, "}{", "},{")
	s = strings.ReplaceAll(s, "}\n{", "},\n{")
	s = strings.ReplaceAll(s, "} {", "}, {")

	return s
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
