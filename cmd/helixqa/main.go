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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"digital.vasic.challenges/pkg/logging"

	"digital.vasic.helixqa/pkg/config"
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
	fmt.Println("  run      Execute QA pipeline across platforms")
	fmt.Println("  list     List test cases from banks")
	fmt.Println("  report   Generate report from existing results")
	fmt.Println("  version  Print version information")
	fmt.Println("  help     Show this help")
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
