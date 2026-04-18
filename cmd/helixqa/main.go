// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa is the CLI entry point for the HelixQA
// testing framework. It supports subcommands for running QA
// pipelines, validating test banks, generating reports, and
// listing test cases.
//
// Usage:
//
//	helixqa run    --banks <paths> [flags]
//	helixqa list   --banks <paths> [--platform <p>]
//	helixqa report --input <dir>   [--format <fmt>]
//	helixqa version
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"digital.vasic.challenges/pkg/logging"

	"digital.vasic.helixqa/pkg/autonomous"
	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/controller"
	qainfra "digital.vasic.helixqa/pkg/infra"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
	"digital.vasic.helixqa/pkg/orchestrator"
	"digital.vasic.helixqa/pkg/reporter"
	"digital.vasic.helixqa/pkg/testbank"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "run":
		cmdRun(os.Args[2:])
	case "list":
		cmdList(os.Args[2:])
	case "report":
		cmdReport(os.Args[2:])
	case "autonomous":
		cmdAutonomous(os.Args[2:])
	case "version":
		fmt.Printf("helixqa v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr,
			"error: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("HelixQA — AI-driven QA orchestration")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  helixqa <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run         Execute QA pipeline across platforms")
	fmt.Println("  autonomous  Run autonomous LLM-driven QA session")
	fmt.Println("  list        List test cases from banks")
	fmt.Println("  report      Generate report from existing results")
	fmt.Println("  version     Print version information")
	fmt.Println("  help        Show this help")
	fmt.Println()
	fmt.Println("Run 'helixqa <command> --help' for command details.")
}

// cmdRun executes the full QA pipeline.
func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	banks := fs.String("banks", "",
		"Comma-separated test bank paths (files or directories)")
	platform := fs.String("platform", "all",
		"Target platform: android|web|desktop|all")
	device := fs.String("device", "",
		"Android device/emulator ID")
	output := fs.String("output", "qa-results",
		"Output directory for results and evidence")
	speed := fs.String("speed", "normal",
		"Speed mode: slow|normal|fast")
	reportFmt := fs.String("report", "markdown",
		"Report format: markdown|html|json")
	validate := fs.Bool("validate", true,
		"Enable step-by-step validation with crash detection")
	record := fs.Bool("record", true,
		"Enable video recording of test execution")
	verbose := fs.Bool("verbose", false,
		"Enable verbose logging")
	pkg := fs.String("package", "",
		"Android application package name")
	timeout := fs.Duration("timeout", 30*time.Minute,
		"Maximum duration for the entire run")
	browserURL := fs.String("browser-url", "",
		"URL for web platform testing")
	desktopProcess := fs.String("desktop-process", "",
		"Process name for desktop platform testing")
	tickets := fs.Bool("tickets", true,
		"Generate markdown tickets for failed tests")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *banks == "" {
		fmt.Fprintln(os.Stderr, "error: --banks is required")
		fs.Usage()
		os.Exit(1)
	}

	platforms, err := config.ParsePlatforms(*platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg := &config.Config{
		Banks:          config.ParseBanks(*banks),
		Platforms:      platforms,
		Device:         *device,
		PackageName:    *pkg,
		OutputDir:      *output,
		Speed:          config.SpeedMode(*speed),
		ReportFormat:   config.ReportFormat(*reportFmt),
		ValidateSteps:  *validate,
		Record:         *record,
		Verbose:        *verbose,
		Timeout:        *timeout,
		StepTimeout:    2 * time.Minute,
		BrowserURL:     *browserURL,
		DesktopProcess: *desktopProcess,
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.NewConsoleLogger(*verbose)
	defer logger.Close()

	orch := orchestrator.New(
		cfg,
		orchestrator.WithLogger(logger),
	)

	ctx, cancel := context.WithTimeout(
		context.Background(), cfg.Timeout,
	)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(os.Stderr,
				"\nReceived interrupt, shutting down...")
			cancel()
		case <-ctx.Done():
		}
	}()

	result, err := orch.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Print summary.
	fmt.Println()
	if result.Success {
		fmt.Println("PASSED - All tests passed, no crashes")
	} else {
		fmt.Println("FAILED - Issues detected")
	}
	fmt.Printf("Report: %s\n", result.ReportPath)
	fmt.Printf("Duration: %v\n", result.Duration)

	if *tickets && result.Report != nil {
		fmt.Printf("Tickets: %s/tickets/\n", cfg.OutputDir)
	}

	if !result.Success {
		os.Exit(1)
	}
}

// cmdList lists test cases from banks with optional filtering.
func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	banks := fs.String("banks", "",
		"Comma-separated test bank paths")
	platform := fs.String("platform", "",
		"Filter by platform: android|web|desktop")
	category := fs.String("category", "",
		"Filter by category")
	priority := fs.String("priority", "",
		"Filter by priority: critical|high|medium|low")
	tag := fs.String("tag", "",
		"Filter by tag")
	jsonOut := fs.Bool("json", false,
		"Output as JSON")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *banks == "" {
		fmt.Fprintln(os.Stderr, "error: --banks is required")
		fs.Usage()
		os.Exit(1)
	}

	mgr := testbank.NewManager()
	for _, path := range config.ParseBanks(*banks) {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			if err := mgr.LoadDir(path); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := mgr.LoadFile(path); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Apply filters.
	var cases []*testbank.TestCase
	switch {
	case *platform != "":
		p, err := config.ParsePlatforms(*platform)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cases = mgr.ForPlatform(p[0])
	case *category != "":
		cases = mgr.ByCategory(*category)
	case *priority != "":
		cases = mgr.ByPriority(testbank.Priority(*priority))
	case *tag != "":
		cases = mgr.ByTag(*tag)
	default:
		cases = mgr.All()
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(cases, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Test cases: %d\n\n", len(cases))
	fmt.Printf("%-12s %-40s %-12s %-10s %s\n",
		"ID", "NAME", "CATEGORY", "PRIORITY", "PLATFORMS")
	fmt.Println(strings.Repeat("-", 90))
	for _, tc := range cases {
		platforms := "all"
		if len(tc.Platforms) > 0 {
			ps := make([]string, len(tc.Platforms))
			for i, p := range tc.Platforms {
				ps[i] = string(p)
			}
			platforms = strings.Join(ps, ",")
		}
		fmt.Printf("%-12s %-40s %-12s %-10s %s\n",
			tc.ID, truncate(tc.Name, 40),
			tc.Category, tc.Priority, platforms,
		)
	}
}

// cmdReport generates a report from existing QA results.
func cmdReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	input := fs.String("input", "qa-results",
		"Input directory containing QA results")
	format := fs.String("format", "markdown",
		"Report format: markdown|html|json")
	output := fs.String("output", "",
		"Output file path (default: auto-generated)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Read existing JSON report if available.
	jsonPath := fmt.Sprintf("%s/qa-report.json", *input)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: cannot read %s: %v\n"+
				"Run 'helixqa run' first to generate results.\n",
			jsonPath, err,
		)
		os.Exit(1)
	}

	var qaReport reporter.QAReport
	if err := json.Unmarshal(data, &qaReport); err != nil {
		fmt.Fprintf(os.Stderr,
			"error: invalid report: %v\n", err,
		)
		os.Exit(1)
	}

	rep := reporter.New(
		reporter.WithOutputDir(*input),
		reporter.WithReportFormat(config.ReportFormat(*format)),
	)

	outPath := *output
	if outPath == "" {
		outPath = *input
	}

	path, err := rep.WriteReport(&qaReport, outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Report generated: %s\n", path)
}

// cmdAutonomous runs an autonomous LLM-driven QA session.
func cmdAutonomous(args []string) {
	fs := flag.NewFlagSet("autonomous", flag.ExitOnError)
	project := fs.String("project", ".",
		"Path to the project root")
	platforms := fs.String("platforms", "android,desktop,web",
		"Comma-separated platforms to test")
	envFile := fs.String("env", ".env",
		"Path to .env configuration file")
	timeout := fs.Duration("timeout", 2*time.Hour,
		"Maximum session duration")
	coverageTarget := fs.Float64("coverage-target", 0.9,
		"Desired feature coverage (0-1)")
	output := fs.String("output", "qa-results",
		"Output directory for results")
	reportFmts := fs.String("report", "markdown,html,json",
		"Comma-separated report formats")
	verbose := fs.Bool("verbose", false,
		"Enable verbose logging")
	curiosity := fs.Bool("curiosity", true,
		"Enable curiosity-driven exploration phase")
	curiosityTimeout := fs.Duration("curiosity-timeout",
		30*time.Minute,
		"Timeout for curiosity-driven phase")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Load .env file — sets environment variables for LLM
	// API keys and platform configuration without requiring
	// the caller to export them manually.
	if err := loadEnvFile(*envFile); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: could not load env file %s: %v\n",
			*envFile, err)
	}

	fmt.Println("HelixQA Autonomous QA Session")
	fmt.Println()
	fmt.Printf("Project:          %s\n", *project)
	fmt.Printf("Platforms:        %s\n", *platforms)
	fmt.Printf("Env file:         %s\n", *envFile)
	fmt.Printf("Timeout:          %v\n", *timeout)
	fmt.Printf("Coverage target:  %.0f%%\n",
		*coverageTarget*100)
	fmt.Printf("Output:           %s\n", *output)
	fmt.Printf("Report formats:   %s\n", *reportFmts)
	fmt.Printf("Curiosity:        %v (timeout: %v)\n",
		*curiosity, *curiosityTimeout)
	fmt.Printf("Verbose:          %v\n", *verbose)
	fmt.Println()

	// Parse platforms.
	platformList, err := config.ParsePlatforms(*platforms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	platformStrs := make([]string, len(platformList))
	for i, p := range platformList {
		platformStrs[i] = string(p)
	}

	fmt.Printf("Resolved platforms: %v\n", platformStrs)
	fmt.Println()

	// ── LLM provider setup ────────────────────────────────────────
	// Build provider configs from environment variables.
	// Configs are split into vision-capable and chat-capable
	// pools so the pipeline can use the best model for each
	// purpose without hardcoding preferences.
	var providerConfigs []llm.ProviderConfig
	var visionConfigs []llm.ProviderConfig
	var chatConfigs []llm.ProviderConfig

	// Known vision-capable providers (models that support
	// image input for screenshot analysis).
	visionProviders := map[string]bool{
		llm.ProviderAnthropic: true,
		llm.ProviderOpenAI:    true,
		llm.ProviderGoogle:    true,
		llm.ProviderOllama:    true,
		"astica":              true,
		"qwen":                true,
		"kimi":                true,
		"stepfun":             true,
		"nvidia":              true,
		"githubmodels":        true,
		"xai":                 true,
	}

	// Auto-discover all LLM providers from environment variables.
	// Supports 40+ providers via the registry in pkg/llm.
	for providerName, envKey := range llm.ProviderEnvKeys {
		val := os.Getenv(envKey)
		if val == "" {
			continue
		}
		var cfg llm.ProviderConfig
		if providerName == llm.ProviderOllama {
			// Ollama uses URL, not API key
			cfg = llm.ProviderConfig{
				Name:    providerName,
				BaseURL: val,
				Model:   os.Getenv("HELIX_OLLAMA_MODEL"),
			}
		} else {
			cfg = llm.ProviderConfig{
				Name:   providerName,
				APIKey: val,
			}
		}
		providerConfigs = append(providerConfigs, cfg)

		// Classify into vision and chat pools.
		if visionProviders[providerName] {
			visionConfigs = append(visionConfigs, cfg)
		}
		// All providers can do chat.
		chatConfigs = append(chatConfigs, cfg)
	}

	// ── Bridged CLI model discovery ──────────────────────────
	// Discover CLI-based LLM tools (claude, qwen-coder,
	// opencode) that can be used as providers with zero
	// API key cost. Each discovered CLI is wrapped in a
	// BridgedCLIProvider and added to the provider pools.
	var bridgedProviders []*llm.BridgedCLIProvider
	for _, cli := range []struct {
		name  string
		bin   string
		model string
	}{
		{"claude", "claude", ""},
		{"qwen-coder", "qwen-coder", ""},
		{"opencode", "opencode", ""},
	} {
		cliPath, lookErr := osexec.LookPath(cli.bin)
		if lookErr != nil {
			continue
		}
		bp := llm.NewBridgedCLIProvider(
			cliPath, cli.name, cli.model,
		)
		bridgedProviders = append(bridgedProviders, bp)

		// Bridged CLIs are chat-capable; only Claude
		// supports vision.
		chatConfigs = append(chatConfigs,
			llm.ProviderConfig{
				Name: "bridge-" + cli.name,
			},
		)
		if bp.SupportsVision() {
			visionConfigs = append(visionConfigs,
				llm.ProviderConfig{
					Name: "bridge-" + cli.name,
				},
			)
		}
		fmt.Printf(
			"Bridged CLI:      %s (%s)\n",
			cli.name, cliPath,
		)
	}

	if len(providerConfigs) == 0 &&
		len(bridgedProviders) == 0 {
		fmt.Fprintln(os.Stderr,
			"error: no LLM providers configured — set at least one "+
				"API key env var (e.g., ANTHROPIC_API_KEY, OPENAI_API_KEY, "+
				"OPENROUTER_API_KEY, DEEPSEEK_API_KEY, GROQ_API_KEY, etc.) "+
				"or install a CLI tool (claude, qwen-coder, opencode)")
		os.Exit(1)
	}

	// Build the shared provider from all discovered configs.
	// If only bridged CLIs are available, create a minimal
	// config list so the adaptive provider has at least one
	// entry.
	if len(providerConfigs) == 0 && len(bridgedProviders) > 0 {
		// Use a placeholder config — the bridged
		// providers will be added to allProviders directly.
		providerConfigs = append(providerConfigs,
			llm.ProviderConfig{
				Name: bridgedProviders[0].Name(),
			},
		)
	}
	// Use enhanced adaptive provider with rate limiting and prompt optimization
	provider, err := llm.NewEnhancedAdaptiveProvider(providerConfigs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: LLM setup: %v\n", err)
		os.Exit(1)
	}

	// Build dedicated vision provider from vision-capable configs.
	var visionProvider llm.Provider
	if len(visionConfigs) > 0 {
		visionProvider, _ = llm.NewEnhancedAdaptiveProvider(visionConfigs)
	}

	// Build dedicated chat provider from all configs (chat
	// models prioritize reasoning quality over vision).
	var chatProvider llm.Provider
	if len(chatConfigs) > 0 {
		chatProvider, _ = llm.NewEnhancedAdaptiveProvider(chatConfigs)
	}

	// ── Memory store setup ────────────────────────────────────────
	dbPath := filepath.Join(*project, "HelixQA", "data", "memory.db")

	store, err := memory.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: open memory store: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	latestPass, err := store.LatestPassNumber()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: query pass number: %v\n", err)
		os.Exit(1)
	}
	passNumber := latestPass + 1

	// ── Print session bootstrap info ─────────────────────────────
	fmt.Printf("Pass number:      %d\n", passNumber)
	fmt.Printf("LLM provider:     %s\n", provider.Name())
	fmt.Printf("Platforms:        %v\n", platformStrs)
	fmt.Printf("Memory DB:        %s\n", dbPath)
	fmt.Println()

	cfg := &autonomous.PipelineConfig{
		ProjectRoot: *project,
		Platforms:   platformStrs,
		OutputDir: filepath.Join(
			*output,
			fmt.Sprintf("session-%d", time.Now().Unix()),
		),
		IssuesDir: filepath.Join(
			*project, "docs", "issues",
		),
		BanksDir: filepath.Join(
			*project, "challenges", "helixqa-banks",
		),
		Timeout:              *timeout,
		PassNumber:           passNumber,
		AndroidDevice:        os.Getenv("HELIX_ANDROID_DEVICE"),
		AndroidDevices:       detectADBDevices(*project),
		AndroidPackage:       os.Getenv("HELIX_ANDROID_PACKAGE"),
		WebURL:               os.Getenv("HELIX_WEB_URL"),
		DesktopDisplay:       os.Getenv("HELIX_DESKTOP_DISPLAY"),
		FFmpegPath:           os.Getenv("HELIX_FFMPEG_PATH"),
		CuriosityEnabled:     *curiosity,
		CuriosityTimeout:     *curiosityTimeout,
		VisionHost:           os.Getenv("HELIX_VISION_HOST"),
		VisionUser:           os.Getenv("HELIX_VISION_USER"),
		VisionModel:          os.Getenv("HELIX_VISION_MODEL"),
		UseLlamaCpp:          os.Getenv("HELIX_LLAMACPP") == "true",
		LlamaCppModelPath:    os.Getenv("HELIX_LLAMACPP_MODEL"),
		LlamaCppMMProjPath:   os.Getenv("HELIX_LLAMACPP_MMPROJ"),
		LlamaCppFreeGPU:      os.Getenv("HELIX_LLAMACPP_FREE_GPU") == "true",
		VisionMultiUser:      os.Getenv("HELIX_VISION_MULTI_USER"),
		LlamaCppRPCModelPath: os.Getenv("HELIX_LLAMACPP_RPC_MODEL"),
	}
	// Parse HELIX_VISION_HOSTS (comma-separated).
	if hostsEnv := os.Getenv("HELIX_VISION_HOSTS"); hostsEnv != "" {
		for _, h := range strings.Split(hostsEnv, ",") {
			h = strings.TrimSpace(h)
			if h != "" {
				cfg.VisionHosts = append(cfg.VisionHosts, h)
			}
		}
	}
	pipeline := autonomous.NewSessionPipeline(
		cfg, provider, store,
	)
	// Wire dedicated providers for dual-model selection.
	// Vision provider handles Execute/Curiosity phases;
	// chat provider handles Plan/Analyze phases.
	if visionProvider != nil {
		pipeline.WithVisionProvider(visionProvider)
		fmt.Printf("Vision provider:  %s\n", visionProvider.Name())
	}
	if chatProvider != nil {
		pipeline.WithChatProvider(chatProvider)
		fmt.Printf("Chat provider:    %s\n", chatProvider.Name())
	}

	// Build phase-aware model selector from all discovered
	// providers. This scores each provider per phase
	// (vision, JSON, reasoning) so the pipeline
	// automatically picks the best model for each phase.
	var allProviders []llm.Provider
	if visionProvider != nil {
		// Try to get underlying providers from enhanced adaptive provider
		if eap, ok := visionProvider.(*llm.EnhancedAdaptiveProvider); ok {
			for _, p := range eap.Providers() {
				allProviders = append(allProviders, p)
			}
		} else if ap, ok := visionProvider.(*llm.AdaptiveProvider); ok {
			for _, p := range ap.Providers() {
				allProviders = append(allProviders, p)
			}
		} else {
			allProviders = append(allProviders, visionProvider)
		}
	}
	if chatProvider != nil {
		// Try to get underlying providers from enhanced adaptive provider
		if eap, ok := chatProvider.(*llm.EnhancedAdaptiveProvider); ok {
			for _, p := range eap.Providers() {
				allProviders = append(allProviders, p)
			}
		} else if ap, ok := chatProvider.(*llm.AdaptiveProvider); ok {
			for _, p := range ap.Providers() {
				allProviders = append(allProviders, p)
			}
		} else {
			allProviders = append(allProviders, chatProvider)
		}
	}
	// Add bridged CLI providers to the allProviders pool.
	for _, bp := range bridgedProviders {
		allProviders = append(allProviders, bp)
	}
	// Deduplicate — vision and chat pools may overlap.
	allProviders = deduplicateProviders(allProviders)
	if len(allProviders) > 0 {
		phaseSelector := llm.NewPhaseModelSelector(
			allProviders,
		)
		pipeline.WithPhaseSelector(phaseSelector)
		fmt.Printf(
			"Phase selector:   %d providers\n",
			len(allProviders),
		)
	}

	// Attach QA Process Controller watchdog. Monitors
	// curiosity steps and kills stuck ones that exceed
	// the stale threshold with no heartbeat.
	ctrl := controller.New(controller.DefaultConfig())
	pipeline.WithController(ctrl)
	fmt.Println("Process ctrl:     enabled (90s stale threshold)")

	// ── QA Infrastructure boot ──────────────────────────────────
	// When HELIX_INFRA_HOST is set, use the Containers module
	// to verify that backend services (database, cache, API) are
	// healthy before starting the pipeline. The API service name,
	// port, and health path are read from env so HelixQA stays
	// decoupled from any project-specific naming.
	if infraHost := os.Getenv("HELIX_INFRA_HOST"); infraHost != "" {
		fmt.Printf("Infra host:       %s\n", infraHost)
		apiName := os.Getenv("HELIX_INFRA_API_SERVICE")
		apiPort := os.Getenv("HELIX_INFRA_API_PORT")
		apiHealth := os.Getenv("HELIX_INFRA_API_HEALTH_PATH")
		infraCfg := qainfra.DefaultQAInfraConfig(infraHost, apiName, apiPort, apiHealth)
		infraMgr, infraErr := qainfra.NewQAInfraManager(infraCfg)
		if infraErr != nil {
			fmt.Fprintf(os.Stderr,
				"warning: infra manager: %v\n", infraErr)
		} else {
			infraCtx, infraCancel := context.WithTimeout(
				context.Background(), 30*time.Second,
			)
			_, infraBootErr := infraMgr.Boot(infraCtx)
			infraCancel()
			if infraBootErr != nil {
				fmt.Fprintf(os.Stderr,
					"warning: infra boot: %v\n",
					infraBootErr)
			}
		}
	}

	fmt.Println()
	result, err := pipeline.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: pipeline failed: %v\n", err)
		os.Exit(1)
	}
	if err := pipeline.WriteReport(result); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: could not write report: %v\n", err)
	}
	if result.Status == autonomous.StatusFailed {
		fmt.Fprintf(os.Stderr,
			"Session failed: %s\n", result.Error)
		os.Exit(1)
	}
}

// detectADBDevices runs `adb devices` and returns all
// connected device serials. Falls back to HELIX_ANDROID_DEVICE
// env var if no devices are detected.
// The projectRoot parameter is used to locate the .devignore file.
func detectADBDevices(projectRoot string) []string {
	out, err := osexec.Command(
		"adb", "devices",
	).Output()
	if err != nil {
		// Fall back to env var.
		if dev := os.Getenv("HELIX_ANDROID_DEVICE"); dev != "" {
			return []string{dev}
		}
		return nil
	}
	// Load device exclusions from .devignore file (project root)
	// and HELIX_ADB_EXCLUDE env var. Case-insensitive substring
	// match against `adb devices -l` output.
	var excludeModels []string

	// Read .devignore from project root (try multiple locations).
	// First try the project root, then relative paths from working directory.
	devignorePaths := []string{
		filepath.Join(projectRoot, ".devignore"),
		".devignore",
		"../.devignore",
		"../../.devignore",
	}
	for _, path := range devignorePaths {
		if data, err := os.ReadFile(path); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				excludeModels = append(excludeModels, strings.ToLower(line))
			}
			break
		}
	}

	// Also check HELIX_ADB_EXCLUDE env var.
	if excludeRaw := os.Getenv("HELIX_ADB_EXCLUDE"); excludeRaw != "" {
		for _, m := range strings.Split(excludeRaw, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				excludeModels = append(excludeModels, strings.ToLower(m))
			}
		}
	}

	// Also get detailed device info for filtering.
	detailOut, _ := osexec.Command("adb", "devices", "-l").Output()
	detailLines := make(map[string]string) // serial -> full line
	for _, line := range strings.Split(string(detailOut), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			detailLines[parts[0]] = strings.ToLower(line)
		}
	}

	var devices []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List") ||
			strings.HasPrefix(line, "*") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			serial := parts[0]
			// Check exclude list against device detail line.
			excluded := false
			if detail, ok := detailLines[serial]; ok {
				for _, exc := range excludeModels {
					if strings.Contains(detail, exc) {
						fmt.Printf("Excluding device %s (matches %q)\n", serial, exc)
						excluded = true
						break
					}
				}
			}
			if !excluded {
				devices = append(devices, serial)
			}
		}
	}
	if len(devices) > 0 {
		fmt.Printf(
			"Detected %d ADB devices: %v\n",
			len(devices), devices,
		)
	}
	return devices
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// deduplicateProviders removes duplicate providers by
// name, keeping the first occurrence.
func deduplicateProviders(
	providers []llm.Provider,
) []llm.Provider {
	seen := make(map[string]bool, len(providers))
	var unique []llm.Provider
	for _, p := range providers {
		if !seen[p.Name()] {
			seen[p.Name()] = true
			unique = append(unique, p)
		}
	}
	return unique
}

// loadEnvFile reads a .env file and sets environment variables
// for any keys not already present in the environment. Lines
// starting with # and blank lines are ignored. Supports
// KEY=VALUE format with optional quoting.
func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes if present.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// .env file values override existing environment variables
		// to ensure configuration from file takes precedence.
		os.Setenv(key, val)
	}
	return scanner.Err()
}
