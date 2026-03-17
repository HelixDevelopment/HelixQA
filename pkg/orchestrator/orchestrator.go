// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package orchestrator ties together the detector, validator,
// and reporter into a complete QA execution pipeline. It loads
// test banks via the Challenges framework, runs challenges per
// platform, validates each step, and produces a combined QA
// report.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.challenges/pkg/bank"
	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/logging"
	"digital.vasic.challenges/pkg/runner"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/reporter"
	"digital.vasic.helixqa/pkg/validator"
)

// Result captures the complete outcome of a HelixQA run.
type Result struct {
	// Report is the generated QA report.
	Report *reporter.QAReport `json:"report"`

	// ReportPath is where the report was written.
	ReportPath string `json:"report_path"`

	// Success is true if no crashes or test failures occurred.
	Success bool `json:"success"`

	// StartTime is when the run began.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the run completed.
	EndTime time.Time `json:"end_time"`

	// Duration is the total run time.
	Duration time.Duration `json:"duration"`
}

// Orchestrator is the main QA execution engine.
type Orchestrator struct {
	config   *config.Config
	detector *detector.Detector
	val      *validator.Validator
	reporter *reporter.Reporter
	logger   logging.Logger
	runner   runner.Runner
	bank     *bank.Bank
}

// Option configures an Orchestrator.
type Option func(*Orchestrator)

// WithLogger sets the logger.
func WithLogger(logger logging.Logger) Option {
	return func(o *Orchestrator) {
		o.logger = logger
	}
}

// WithRunner sets a custom challenge runner.
func WithRunner(r runner.Runner) Option {
	return func(o *Orchestrator) {
		o.runner = r
	}
}

// WithDetector sets a custom detector.
func WithDetector(d *detector.Detector) Option {
	return func(o *Orchestrator) {
		o.detector = d
	}
}

// WithValidator sets a custom validator.
func WithValidator(v *validator.Validator) Option {
	return func(o *Orchestrator) {
		o.val = v
	}
}

// WithReporter sets a custom reporter.
func WithReporter(r *reporter.Reporter) Option {
	return func(o *Orchestrator) {
		o.reporter = r
	}
}

// WithBank sets a pre-loaded test bank.
func WithBank(b *bank.Bank) Option {
	return func(o *Orchestrator) {
		o.bank = b
	}
}

// New creates an Orchestrator with the given configuration
// and options.
func New(cfg *config.Config, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		config: cfg,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Run executes the complete QA pipeline:
// 1. Load test banks
// 2. For each platform, run challenges with validation
// 3. Generate combined report
func (o *Orchestrator) Run(
	ctx context.Context,
) (*Result, error) {
	result := &Result{
		StartTime: time.Now(),
	}

	o.log("Starting HelixQA run")

	// 1. Load test banks.
	if err := o.loadBanks(); err != nil {
		return nil, fmt.Errorf("load banks: %w", err)
	}

	definitions := o.bank.All()
	o.log("Loaded %d challenge definitions from %d sources",
		len(definitions), len(o.bank.Sources()))

	// 2. Create output directory.
	if err := os.MkdirAll(o.config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// 3. Run challenges for each platform.
	platforms := o.config.ExpandedPlatforms()
	var platformResults []*reporter.PlatformResult

	for _, platform := range platforms {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		o.log("Testing platform: %s", platform)

		pr, err := o.runPlatform(ctx, platform, definitions)
		if err != nil {
			o.logError("Platform %s failed: %v",
				platform, err)
			// Continue with other platforms.
			pr = &reporter.PlatformResult{
				Platform:  platform,
				StartTime: time.Now(),
				EndTime:   time.Now(),
			}
		}
		platformResults = append(platformResults, pr)
	}

	// 4. Generate combined report.
	rep := o.getReporter()
	qaReport, err := rep.GenerateQAReport(platformResults)
	if err != nil {
		return nil, fmt.Errorf("generate report: %w", err)
	}

	reportPath, err := rep.WriteReport(
		qaReport, o.config.OutputDir,
	)
	if err != nil {
		return nil, fmt.Errorf("write report: %w", err)
	}

	result.Report = qaReport
	result.ReportPath = reportPath
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = qaReport.FailedChallenges == 0 &&
		qaReport.TotalCrashes == 0 &&
		qaReport.TotalANRs == 0

	o.log("HelixQA run complete: %d/%d passed, %d crashes, "+
		"%d ANRs, report at %s",
		qaReport.PassedChallenges,
		qaReport.TotalChallenges,
		qaReport.TotalCrashes,
		qaReport.TotalANRs,
		reportPath,
	)

	return result, nil
}

// runPlatform executes all challenges for a single platform.
func (o *Orchestrator) runPlatform(
	ctx context.Context,
	platform config.Platform,
	definitions []*challenge.Definition,
) (*reporter.PlatformResult, error) {
	pr := &reporter.PlatformResult{
		Platform:  platform,
		StartTime: time.Now(),
	}

	evidenceDir := filepath.Join(
		o.config.OutputDir,
		"evidence",
		string(platform),
	)
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf(
			"create evidence dir: %w", err,
		)
	}
	pr.EvidenceDir = evidenceDir

	// Create platform-specific detector if validation enabled.
	var val *validator.Validator
	if o.config.ValidateSteps {
		det := o.getDetector(platform, evidenceDir)
		val = validator.New(
			det,
			validator.WithEvidenceDir(evidenceDir),
		)
	}

	// Execute each challenge definition.
	for _, def := range definitions {
		select {
		case <-ctx.Done():
			pr.EndTime = time.Now()
			pr.Duration = pr.EndTime.Sub(pr.StartTime)
			return pr, ctx.Err()
		default:
		}

		// Create challenge config.
		cfg := challenge.NewConfig(def.ID)
		cfg.Verbose = o.config.Verbose
		cfg.Timeout = o.config.StepTimeout
		cfg.ResultsDir = filepath.Join(
			o.config.OutputDir,
			"results",
			string(platform),
			string(def.ID),
		)

		// Run challenge if runner is available.
		if o.runner != nil {
			challengeResult, err := o.runner.Run(
				ctx, def.ID, cfg,
			)
			if err != nil {
				o.logError("Challenge %s failed: %v",
					def.ID, err)
				challengeResult = &challenge.Result{
					ChallengeID:   def.ID,
					ChallengeName: def.Name,
					Status:        challenge.StatusError,
					Error:         err.Error(),
					StartTime:     time.Now(),
					EndTime:       time.Now(),
				}
			}
			pr.ChallengeResults = append(
				pr.ChallengeResults, challengeResult,
			)
		}

		// Validate step if enabled.
		if val != nil {
			stepResult, err := val.ValidateStep(
				ctx,
				string(def.ID),
				platform,
			)
			if err != nil {
				o.logError("Validation failed for %s: %v",
					def.ID, err)
			}
			if stepResult != nil {
				pr.StepResults = append(
					pr.StepResults, stepResult,
				)
				if stepResult.Detection != nil {
					if stepResult.Detection.HasCrash {
						pr.CrashCount++
					}
					if stepResult.Detection.HasANR {
						pr.ANRCount++
					}
				}
			}
		}

		// Apply step delay based on speed mode.
		delay := o.config.StepDelay()
		if delay > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(delay):
			}
		}
	}

	pr.EndTime = time.Now()
	pr.Duration = pr.EndTime.Sub(pr.StartTime)
	return pr, nil
}

// loadBanks loads test banks from configured paths.
func (o *Orchestrator) loadBanks() error {
	if o.bank != nil {
		return nil // Already loaded.
	}
	if len(o.config.Banks) == 0 {
		return fmt.Errorf("no test bank paths configured")
	}
	o.bank = bank.New()

	for _, path := range o.config.Banks {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat bank %s: %w", path, err)
		}
		if info.IsDir() {
			if err := o.bank.LoadDir(path); err != nil {
				return fmt.Errorf(
					"load bank dir %s: %w", path, err,
				)
			}
		} else {
			if err := o.bank.LoadFile(path); err != nil {
				return fmt.Errorf(
					"load bank file %s: %w", path, err,
				)
			}
		}
	}
	return nil
}

// getDetector returns the configured detector or creates one.
func (o *Orchestrator) getDetector(
	platform config.Platform,
	evidenceDir string,
) *detector.Detector {
	if o.detector != nil {
		return o.detector
	}

	opts := []detector.Option{
		detector.WithEvidenceDir(evidenceDir),
	}

	switch platform {
	case config.PlatformAndroid:
		opts = append(opts,
			detector.WithDevice(o.config.Device),
			detector.WithPackageName(o.config.PackageName),
		)
	case config.PlatformWeb:
		opts = append(opts,
			detector.WithBrowserURL(o.config.BrowserURL),
		)
	case config.PlatformDesktop:
		opts = append(opts,
			detector.WithProcessName(
				o.config.DesktopProcess,
			),
			detector.WithProcessPID(o.config.DesktopPID),
		)
	}

	return detector.New(platform, opts...)
}

// getReporter returns the configured reporter or creates one.
func (o *Orchestrator) getReporter() *reporter.Reporter {
	if o.reporter != nil {
		return o.reporter
	}
	return reporter.New(
		reporter.WithOutputDir(o.config.OutputDir),
		reporter.WithReportFormat(o.config.ReportFormat),
	)
}

// log writes an info-level log message.
func (o *Orchestrator) log(format string, args ...any) {
	if o.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	o.logger.Info(msg)
}

// logError writes an error-level log message.
func (o *Orchestrator) logError(format string, args ...any) {
	if o.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	o.logger.Error(msg)
}
