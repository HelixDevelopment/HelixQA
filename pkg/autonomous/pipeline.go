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

	// VisionHosts is a comma-separated list of remote hosts
	// for distributed vision inference. When set, the
	// pipeline probes each host's hardware via SSH, selects
	// the strongest model that fits the combined resources,
	// and activates distributed RPC if needed. Takes
	// precedence over VisionHost for multi-host setups.
	VisionHosts []string

	// VisionMultiUser is the SSH user for multi-host
	// probing. Falls back to VisionUser if empty.
	VisionMultiUser string

	// LlamaCppRPCModelPath is the path to the GGUF model on
	// the master host for distributed RPC inference. When
	// empty, auto-detection will not start RPC workers.
	LlamaCppRPCModelPath string

	// ChatProviders holds provider configs for the chat model
	// used in Plan and Analyze phases. When non-empty, a
	// separate AdaptiveProvider is built for chat tasks
	// (reasoning, test planning, report generation) so it can
	// differ from the vision provider used in Execute and
	// Curiosity phases.
	ChatProviders []llm.ProviderConfig
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
	// chatProvider is used for reasoning-heavy phases (Plan,
	// Analyze report generation). When nil, falls back to the
	// shared provider.
	chatProvider llm.Provider
	// visionProvider is used for screenshot analysis phases
	// (Execute, Curiosity). When nil, falls back to the shared
	// provider.
	visionProvider llm.Provider
	store          *memory.Store
	// kbContext holds a summary of the Learn phase knowledge
	// base, injected into navigation prompts so the LLM
	// knows app-specific details (credentials, screens, etc.)
	// without hardcoding them in the prompt templates.
	kbContext string
}

// NewSessionPipeline creates a SessionPipeline with the
// given configuration, LLM provider, and memory store.
// The provider is used as the default for all phases.
// Use WithChatProvider and WithVisionProvider to override
// for specific phase types.
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

// WithChatProvider sets a dedicated provider for reasoning-heavy
// phases (Plan, Analyze). This allows using a different model
// optimized for text reasoning while the default provider handles
// vision tasks.
func (sp *SessionPipeline) WithChatProvider(p llm.Provider) *SessionPipeline {
	sp.chatProvider = p
	return sp
}

// WithVisionProvider sets a dedicated provider for screenshot
// analysis phases (Execute, Curiosity). This allows using a
// specialized vision model while the default provider handles
// chat/reasoning tasks.
func (sp *SessionPipeline) WithVisionProvider(p llm.Provider) *SessionPipeline {
	sp.visionProvider = p
	return sp
}

// getChatProvider returns the provider for reasoning tasks.
// Falls back to the shared provider if no dedicated chat
// provider is configured.
func (sp *SessionPipeline) getChatProvider() llm.Provider {
	if sp.chatProvider != nil {
		return sp.chatProvider
	}
	return sp.provider
}

// getVisionProvider returns the provider for vision tasks.
// Falls back to the shared provider if no dedicated vision
// provider is configured.
func (sp *SessionPipeline) getVisionProvider() llm.Provider {
	if sp.visionProvider != nil {
		return sp.visionProvider
	}
	return sp.provider
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

	// Track distributed RPC deployers for cleanup at
	// session end.
	var distributedDeployers []*visionremote.LlamaCppDeployer
	var distributedRPCPorts []int
	var distributedMasterDeployer *visionremote.LlamaCppDeployer
	var distributedMasterPort int

	// ── Phase 0a: Distributed vision auto-detection ─────
	// When HELIX_VISION_HOSTS is set, probe each host's
	// hardware via SSH, select the strongest model, and
	// activate distributed RPC if the model needs it.
	if len(sp.config.VisionHosts) > 0 {
		sshUser := sp.config.VisionMultiUser
		if sshUser == "" {
			sshUser = sp.config.VisionUser
		}

		fmt.Printf(
			"[pipeline] Phase 0a: Probing %d hosts "+
				"for distributed vision\n",
			len(sp.config.VisionHosts),
		)
		hwList := visionremote.ProbeHosts(
			ctx, sp.config.VisionHosts, sshUser,
		)

		if len(hwList) > 0 {
			rec := visionremote.SelectStrongestModel(
				hwList,
			)
			fmt.Printf(
				"[pipeline] Model selected: %s (%s) "+
					"across %d hosts "+
					"(GPU=%dMB, RAM=%dMB, "+
					"distributed=%v)\n",
				rec.ModelName, rec.ModelSize,
				len(rec.AllHosts),
				rec.TotalGPUMemMB,
				rec.TotalRAMMB,
				rec.NeedsDistribution,
			)

			// Override VisionModel with the auto-selected
			// model so downstream phases use it.
			sp.config.VisionModel = rec.ModelName

			if rec.NeedsDistribution &&
				sp.config.LlamaCppRPCModelPath != "" {
				// Distributed RPC mode: start rpc-server on
				// each host, then start master llama-server
				// with --rpc flag.
				distCfg := visionremote.PlanDistribution(
					hwList,
					sp.config.LlamaCppRPCModelPath,
					8090,  // server port
					50052, // RPC base port
				)
				if distCfg != nil {
					fmt.Printf(
						"[pipeline] Distributed RPC: "+
							"master=%s, %d workers\n",
						distCfg.MasterHost,
						len(distCfg.RPCWorkers),
					)

					// Start RPC workers on each host.
					rpcBasePort := 50052
					for i, h := range hwList {
						port := rpcBasePort + i
						deployer := visionremote.NewLlamaCppDeployer(
							visionremote.LlamaCppConfig{
								Host:    h.Host,
								User:    sshUser,
								RepoDir: h.LlamaCppDir,
							},
						)
						if err := deployer.StartRPCServer(
							ctx, port,
						); err != nil {
							fmt.Printf(
								"[pipeline] warning: "+
									"RPC server on %s:%d "+
									"failed: %v\n",
								h.Host, port, err,
							)
						} else {
							fmt.Printf(
								"[pipeline] RPC worker "+
									"started on %s:%d\n",
								h.Host, port,
							)
						}
						distributedDeployers = append(
							distributedDeployers, deployer,
						)
						distributedRPCPorts = append(
							distributedRPCPorts, port,
						)
					}

					// Start master llama-server with --rpc.
					masterDeployer := visionremote.NewLlamaCppDeployer(
						visionremote.LlamaCppConfig{
							Host:        distCfg.MasterHost,
							User:        sshUser,
							RepoDir:     distCfg.MasterDir,
							ModelPath:   distCfg.ModelPath,
							BasePort:    distCfg.ServerPort,
							ContextSize: distCfg.ContextSize,
						},
					)
					if err := masterDeployer.StartWithRPC(
						ctx,
						distCfg.ModelPath,
						distCfg.RPCWorkers,
						distCfg.ServerPort,
					); err != nil {
						fmt.Printf(
							"[pipeline] warning: "+
								"distributed master "+
								"failed: %v "+
								"(falling back to "+
								"single-host)\n",
							err,
						)
					} else {
						distributedMasterDeployer = masterDeployer
						distributedMasterPort = distCfg.ServerPort
						// Override VisionHost to point to
						// the distributed master.
						sp.config.VisionHost = distCfg.MasterHost
						sp.config.UseLlamaCpp = false
						// Use Ollama-compatible endpoint
						// (llama-server serves OpenAI API).
						fmt.Printf(
							"[pipeline] Distributed "+
								"inference active at "+
								"http://%s:%d\n",
							distCfg.MasterHost,
							distCfg.ServerPort,
						)
					}
				}
			} else if !rec.NeedsDistribution &&
				len(rec.GPUHosts) > 0 {
				// Single GPU host is sufficient — use Ollama
				// on the strongest GPU host for simplicity.
				sp.config.VisionHost = rec.GPUHosts[0]
				fmt.Printf(
					"[pipeline] Single-host vision: "+
						"%s (%s on %s)\n",
					rec.ModelName, rec.ModelSize,
					rec.GPUHosts[0],
				)
			} else if len(rec.AllHosts) > 0 {
				// CPU-only or single host — use first
				// reachable host.
				sp.config.VisionHost = rec.AllHosts[0]
				fmt.Printf(
					"[pipeline] Single-host vision: "+
						"%s (%s on %s)\n",
					rec.ModelName, rec.ModelSize,
					rec.AllHosts[0],
				)
			}
		} else {
			fmt.Println(
				"[pipeline] warning: all vision hosts " +
					"unreachable, falling back to " +
					"single-host config",
			)
		}
	}

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
	gen := planning.NewTestPlanGenerator(sp.getChatProvider())
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
			pkg      string // Android package name for foreground detection
		}
		var curTargets []curiosityTarget
		for _, platform := range sp.config.Platforms {
			if (platform == "android" ||
				platform == "androidtv") &&
				len(sp.config.AndroidDevices) > 0 {
				for _, dev := range sp.config.AndroidDevices {
					curTargets = append(curTargets,
						curiosityTarget{platform, dev, sp.config.AndroidPackage},
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
					curiosityTarget{platform, dev, sp.config.AndroidPackage},
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
		// Force-stop first to ensure intent extras are read.
		for _, ct := range curTargets {
			if (ct.platform == "android" ||
				ct.platform == "androidtv") &&
				sp.config.AndroidPackage != "" {
				// Force-stop so intent extras are re-read.
				stopCtx, sc := context.WithTimeout(
					ctx, 5*time.Second,
				)
				_, _ = osexec.CommandContext(
					stopCtx, "adb", "-s", ct.device,
					"shell", "am", "force-stop",
					sp.config.AndroidPackage,
				).CombinedOutput()
				sc()
				time.Sleep(1 * time.Second)

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

			// Per-target vision provider. Uses the dedicated
			// vision provider (or shared AdaptiveProvider) by
			// default. Only overrides with llama-server when
			// explicitly configured.
			platformProvider := sp.getVisionProvider()
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

			// Determine the expected package name for this target.
			// Used to detect if the app has lost focus (e.g., user
			// pressed back too many times and landed on the launcher).
			expectedPkg := ct.pkg
			if expectedPkg == "" {
				expectedPkg = sp.config.AndroidPackage
			}

			for i := 0; i < maxCuriositySteps; i++ {
				if curiosityCtx.Err() != nil {
					break
				}

				// Guard: verify the target app is still in the
				// foreground. If the LLM navigated away (e.g.,
				// pressed back to the launcher), relaunch the app.
				if (platform == "android" || platform == "androidtv") && device != "" && expectedPkg != "" {
					fgOut, _ := osexec.CommandContext(
						curiosityCtx,
						"adb", "-s", device,
						"shell", "dumpsys", "window", "windows",
					).CombinedOutput()
					fgStr := string(fgOut)
					// Check if the expected package appears in the
					// current focus window.
					if len(fgStr) > 0 && !strings.Contains(fgStr, expectedPkg) {
						fmt.Printf(
							"  [curiosity %s #%d] app not in foreground, relaunching %s\n",
							platform, i+1, expectedPkg,
						)
						launchArgs := []string{
							"-s", device, "shell", "am", "start",
							"-n", expectedPkg + "/.ui.MainActivity",
						}
						user := sp.config.qaUsername()
						pass := sp.config.qaPassword()
						if user != "" && pass != "" {
							launchArgs = append(launchArgs,
								"--es", "qa_username", user,
								"--es", "qa_password", pass,
							)
						}
						_, _ = osexec.CommandContext(
							curiosityCtx,
							"adb", launchArgs...,
						).CombinedOutput()
						time.Sleep(3 * time.Second)
					}
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
				// error or empty response), retry up to 3
				// times with a fresh screenshot each time.
				// HelixQA is fully autonomous — NO
				// hardcoded fallback navigation.
				if len(actions) == 0 {
					retried := false
					for retryN := 1; retryN <= 3; retryN++ {
						if curiosityCtx.Err() != nil {
							break
						}
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"empty actions, "+
								"retrying (%d/3)\n",
							platform, i+1, retryN,
						)
						time.Sleep(
							time.Duration(retryN) *
								time.Second,
						)
						retryShot, _ :=
							executor.Screenshot(
								curiosityCtx,
							)
						if len(retryShot) == 0 {
							continue
						}
						actions = sp.llmNavigate(
							curiosityCtx,
							resizeScreenshot(retryShot),
							platform,
							i+1,
							stepHistory,
							platformProvider,
						)
						if len(actions) > 0 {
							retried = true
							break
						}
					}
					if !retried && len(actions) == 0 {
						fmt.Printf(
							"  [curiosity %s #%d] "+
								"stuck: LLM returned "+
								"no actions after 3 "+
								"retries\n",
							platform, i+1,
						)
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
	analyzeVisionProvider := sp.getVisionProvider()
	if analyzeVisionProvider.SupportsVision() &&
		len(allScreenshots) > 0 {
		visionAnalyzer := analysis.NewVisionAnalyzer(
			analyzeVisionProvider,
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
			if analyzeVisionProvider.SupportsVision() && limit > 0 {
				va := analysis.NewVisionAnalyzer(
					analyzeVisionProvider,
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
	// Shutdown distributed RPC workers and master if active.
	if distributedMasterDeployer != nil {
		distributedMasterDeployer.StopInstance(
			ctx, distributedMasterPort,
		)
		fmt.Printf(
			"[pipeline] distributed master stopped "+
				"(port %d)\n",
			distributedMasterPort,
		)
	}
	for i, deployer := range distributedDeployers {
		if i < len(distributedRPCPorts) {
			deployer.StopRPCServer(
				ctx, distributedRPCPorts[i],
			)
		}
	}
	if len(distributedDeployers) > 0 {
		fmt.Printf(
			"[pipeline] %d distributed RPC workers "+
				"stopped\n",
			len(distributedDeployers),
		)
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
const navigationPromptTemplate = `You are an expert QA tester performing a thorough autonomous QA session on an Android TV application. You must test EVERY feature like a real human tester would — browsing content, opening details, playing media, testing all screens.

Look at the screenshot and determine:
1. What screen am I on?
2. What is the MOST VALUABLE next QA action? Prioritize UNEXPLORED features.

ANDROID TV CONTROLS:
- dpad_up/down/left/right — move focus
- dpad_center — select/activate focused element
- type — enter text (activate field with dpad_center first)
- tab — move between form fields
- back — go back
- clear — delete text in active field
- wait — pause 3 seconds

LOGIN (only when you see a login screen):
1. dpad_down to username field, dpad_center to activate
2. clear, then type the username
3. tab to password, clear, type the password
4. dpad_down to Sign In, dpad_center to click

QA TESTING PRIORITY (follow this order):
1. LOGIN first if on login screen (try admin/admin123)
2. BROWSE the home screen — scroll down and right through ALL content rows
3. OPEN detail screens — select items to see their detail/info page
4. PLAY media — find and activate play buttons for video/audio content
5. BROWSE categories — navigate to different content sections (movies, TV, music, etc.)
6. SEARCH — use search to find specific content, verify results appear
7. TEST favorites — add/remove favorites
8. EXPLORE settings — check settings/preferences screens
9. TEST collections — browse/create collections
10. NAVIGATE back — verify back button works from every screen

CRITICAL RULES:
- Do NOT stay on the same screen for more than 3 steps. MOVE to a different screen.
- NEVER type credentials into non-login fields. Understand WHICH screen you are on. If you see a search bar, type a search query relevant to the app's content. If you see login fields, type credentials.
- NEVER repeat the same action pattern 3 times in a row. If stuck, navigate somewhere NEW.
- After successful login, IMMEDIATELY explore the app — do NOT return to login.
- Read the screen carefully before acting. Different screens require different input.

TESTING PRIORITY (follow this order):
1. HAPPY PATHS FIRST — complete login, browse main content, open detail screens, interact with primary features
2. STANDARD FLOWS — use search with relevant terms from what you see on screen, browse all available sections, test navigation between screens
3. EDGE CASES — empty states, back navigation, error handling
4. Always use CONTEXT-APPROPRIATE input for each screen

If you see content items (cards, lists, grids), SELECT one to open its detail page.
If you see a play/open button, PRESS IT to test that feature.
If you see navigation elements you haven't visited, GO THERE.
For search fields: type terms based on content you've already seen in the app.

RESPONSE: Return ONLY a JSON array of 1-5 actions. No other text.
Format: [{"type":"...", "value":"...", "reason":"..."}]
Types: dpad_up, dpad_down, dpad_left, dpad_right, dpad_center, type, tab, key, back, clear, wait`

// webNavigationPromptTemplate is the prompt for web browser
// QA sessions. Uses mouse clicks and keyboard input instead of
// DPAD navigation.
// webNavigationPromptTemplate is the generic prompt for web
// browser QA. No project-specific information — the LLM
// analyzes the screenshot to determine context.
const webNavigationPromptTemplate = `You are an expert QA tester performing a FULL autonomous QA session on a web application in a headless browser (1920x1080 viewport). Test EVERY feature like a real human QA tester would.

Look at the screenshot and determine:
1. What page am I on?
2. What is the MOST VALUABLE unexplored QA action?

WEB CONTROLS:
- click — value is "x,y" pixel coordinates
- type — enter text (click input first)
- scroll_down/scroll_up — scroll page
- key — Enter, Escape, Tab, Backspace
- back — browser back
- wait — pause 3 seconds

LOGIN (only on login page):
1. Click username field, type "admin"
2. Click password field, type "admin123"
3. Click Sign In button

QA TESTING PRIORITY (follow this order):
1. LOGIN first if on login page
2. DASHBOARD — check stats, charts, activity feed
3. BROWSE MEDIA — click on media items to open details
4. PLAY CONTENT — click play buttons to test playback
5. SIDEBAR NAVIGATION — click every menu item
6. COLLECTIONS — browse and manage collections
7. FAVORITES — add/remove favorites
8. SEARCH — search for content, verify results
9. SETTINGS — check all settings pages
10. ADMIN — check admin panel if available

CRITICAL RULES:
- Do NOT stay on the same page for more than 3 steps. NAVIGATE to a different page.
- NEVER type credentials into non-login fields. Understand WHICH page you are on.
- NEVER repeat the same action pattern 3 times. If stuck, go somewhere NEW.
- After login, IMMEDIATELY explore — do NOT return to login.
- Use CONTEXT-APPROPRIATE input: search terms from visible content, not credentials.

TESTING PRIORITY:
1. HAPPY PATHS FIRST — login, explore dashboard, open detail pages, interact with features
2. STANDARD FLOWS — search with relevant terms, browse all sections, test navigation
3. EDGE CASES — empty states, back navigation, error handling

RESPONSE: Return ONLY a JSON array of 1-5 actions. No other text.
Format: [{"type":"...", "value":"...", "reason":"..."}]
Types: click, type, scroll_down, scroll_up, key, back, wait`

// llmAction is a single navigation action suggested by the LLM.
type llmAction struct {
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// llmNavigateTimeout caps a single LLM vision call during
// curiosity navigation so one slow API response cannot
// stall the exploration phase. Reduced from 180s to 60s
// so stuck calls fail faster and the retry logic gets a
// chance to recover.
// llmNavigateTimeout caps a single provider attempt. Kept short
// Parent timeout for a single LLM navigation call. Set to 90s to
// allow the adaptive provider to try multiple providers (each capped
// at 15-20s internally). Most successful calls complete in 2-10s.
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
	// fall back to the dedicated vision provider (or shared
	// pipeline provider).
	vp := sp.getVisionProvider()
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
		// No JSON array found — try to extract individual JSON objects
		// from markdown bullet points or inline backticks.
		// Pattern: *   `{"type":"...", "reason":"..."}`
		// or: * {"type":"...", "reason":"..."}
		var objects []string
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			// Strip markdown bullet prefixes
			line = strings.TrimLeft(line, "*-• ")
			line = strings.TrimSpace(line)
			// Strip inline backticks
			line = strings.Trim(line, "`")
			line = strings.TrimSpace(line)
			// Check if it looks like a JSON object
			if strings.HasPrefix(line, "{") && strings.Contains(line, "}") {
				// Extract just the JSON object
				objEnd := strings.LastIndex(line, "}")
				if objEnd >= 0 {
					objects = append(objects, line[:objEnd+1])
				}
			}
		}
		if len(objects) > 0 {
			content = "[" + strings.Join(objects, ",") + "]"
			start = 0
			end = len(content) - 1
		} else {
			fmt.Printf(
				"  [curiosity %s #%d] LLM response "+
					"not JSON array: %.80s\n",
				platform, step, content,
			)
			return nil
		}
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
		// The LLM decides when to clear — the framework does not
		// auto-clear. HelixQA constitution requires all navigation
		// decisions to come from the LLM vision analysis.
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
		// Delegate to the platform-specific Clear method which
		// uses select-all + delete (reliable regardless of
		// field content length).
		return exec.Clear(ctx)
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
