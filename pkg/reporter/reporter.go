// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package reporter provides evidence collection and QA report
// generation. It reuses the Challenges framework's report
// package for formatting individual challenge results and adds
// QA-specific reporting (platform results, evidence, step
// validation).
package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/report"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/validator"
)

// PlatformResult captures the QA results for a single platform.
type PlatformResult struct {
	// Platform identifies which platform was tested.
	Platform config.Platform `json:"platform"`

	// ChallengeResults holds the Challenges framework results.
	ChallengeResults []*challenge.Result `json:"challenge_results"`

	// StepResults holds the step validation results.
	StepResults []*validator.StepResult `json:"step_results"`

	// StartTime is when platform testing began.
	StartTime time.Time `json:"start_time"`

	// EndTime is when platform testing completed.
	EndTime time.Time `json:"end_time"`

	// Duration is the total testing time for this platform.
	Duration time.Duration `json:"duration"`

	// CrashCount is the number of crashes detected.
	CrashCount int `json:"crash_count"`

	// ANRCount is the number of ANRs detected.
	ANRCount int `json:"anr_count"`

	// EvidenceDir is the path to the platform's evidence.
	EvidenceDir string `json:"evidence_dir"`
}

// QAReport is the top-level report for a complete HelixQA run.
type QAReport struct {
	// Title is the report title.
	Title string `json:"title"`

	// GeneratedAt is when the report was created.
	GeneratedAt time.Time `json:"generated_at"`

	// PlatformResults holds results for each tested platform.
	PlatformResults []*PlatformResult `json:"platform_results"`

	// TotalChallenges is the total number of challenges run.
	TotalChallenges int `json:"total_challenges"`

	// PassedChallenges is the number that passed.
	PassedChallenges int `json:"passed_challenges"`

	// FailedChallenges is the number that failed.
	FailedChallenges int `json:"failed_challenges"`

	// TotalCrashes is the total crashes across all platforms.
	TotalCrashes int `json:"total_crashes"`

	// TotalANRs is the total ANRs across all platforms.
	TotalANRs int `json:"total_anrs"`

	// TotalDuration is the wall-clock time for the entire run.
	TotalDuration time.Duration `json:"total_duration"`

	// OutputDir is where reports and evidence are stored.
	OutputDir string `json:"output_dir"`
}

// Reporter generates QA reports with evidence collection.
type Reporter struct {
	challengeReporter report.Reporter
	outputDir         string
	reportFormat      config.ReportFormat
}

// Option configures a Reporter.
type Option func(*Reporter)

// WithOutputDir sets the output directory.
func WithOutputDir(dir string) Option {
	return func(r *Reporter) {
		r.outputDir = dir
	}
}

// WithReportFormat sets the report format.
func WithReportFormat(format config.ReportFormat) Option {
	return func(r *Reporter) {
		r.reportFormat = format
	}
}

// WithChallengeReporter sets a custom Challenges reporter.
func WithChallengeReporter(cr report.Reporter) Option {
	return func(r *Reporter) {
		r.challengeReporter = cr
	}
}

// New creates a Reporter with the given options.
func New(opts ...Option) *Reporter {
	r := &Reporter{
		outputDir:    "qa-results",
		reportFormat: config.ReportMarkdown,
	}
	for _, opt := range opts {
		opt(r)
	}
	// Default: use Challenges MarkdownReporter.
	if r.challengeReporter == nil {
		r.challengeReporter = report.NewMarkdownReporter(
			r.outputDir,
		)
	}
	return r
}

// GenerateQAReport creates a QAReport from platform results.
func (r *Reporter) GenerateQAReport(
	results []*PlatformResult,
) (*QAReport, error) {
	qa := &QAReport{
		Title:           "HelixQA Test Report",
		GeneratedAt:     time.Now(),
		PlatformResults: results,
		OutputDir:       r.outputDir,
	}

	for _, pr := range results {
		qa.TotalCrashes += pr.CrashCount
		qa.TotalANRs += pr.ANRCount
		qa.TotalDuration += pr.Duration

		for _, cr := range pr.ChallengeResults {
			qa.TotalChallenges++
			switch cr.Status {
			case challenge.StatusPassed:
				qa.PassedChallenges++
			case challenge.StatusFailed, challenge.StatusError,
				challenge.StatusTimedOut, challenge.StatusStuck:
				qa.FailedChallenges++
			}
		}
	}

	return qa, nil
}

// WriteMarkdown writes the QA report as Markdown.
func (r *Reporter) WriteMarkdown(
	qa *QAReport,
	path string,
) error {
	var buf bytes.Buffer
	r.writeMarkdownReport(&buf, qa)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// WriteJSON writes the QA report as JSON.
func (r *Reporter) WriteJSON(
	qa *QAReport,
	path string,
) error {
	data, err := json.MarshalIndent(qa, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// WriteReport writes the QA report in the configured format.
func (r *Reporter) WriteReport(
	qa *QAReport,
	baseDir string,
) (string, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("create base dir: %w", err)
	}

	switch r.reportFormat {
	case config.ReportJSON:
		path := filepath.Join(baseDir, "qa-report.json")
		return path, r.WriteJSON(qa, path)
	case config.ReportHTML:
		// Delegate to challenges HTML reporter for individual
		// results, then write a summary.
		path := filepath.Join(baseDir, "qa-report.html")
		return path, r.writeHTMLSummary(qa, path)
	default:
		path := filepath.Join(baseDir, "qa-report.md")
		return path, r.WriteMarkdown(qa, path)
	}
}

// GenerateChallengeReport generates a report for a single
// challenge result using the underlying Challenges reporter.
func (r *Reporter) GenerateChallengeReport(
	result *challenge.Result,
) ([]byte, error) {
	return r.challengeReporter.GenerateReport(result)
}

// writeMarkdownReport writes a complete Markdown QA report.
func (r *Reporter) writeMarkdownReport(
	buf *bytes.Buffer,
	qa *QAReport,
) {
	fmt.Fprintln(buf, "# HelixQA Test Report")
	fmt.Fprintln(buf)
	fmt.Fprintf(
		buf, "**Generated:** %s\n\n",
		qa.GeneratedAt.Format(time.RFC3339),
	)

	// Overview table.
	fmt.Fprintln(buf, "## Overview")
	fmt.Fprintln(buf)
	fmt.Fprintln(buf, "| Metric | Value |")
	fmt.Fprintln(buf, "|--------|-------|")
	fmt.Fprintf(
		buf, "| Total Challenges | %d |\n",
		qa.TotalChallenges,
	)
	fmt.Fprintf(
		buf, "| Passed | %d |\n", qa.PassedChallenges,
	)
	fmt.Fprintf(
		buf, "| Failed | %d |\n", qa.FailedChallenges,
	)
	if qa.TotalChallenges > 0 {
		pct := float64(qa.PassedChallenges) /
			float64(qa.TotalChallenges) * 100
		fmt.Fprintf(buf, "| Pass Rate | %.0f%% |\n", pct)
	}
	fmt.Fprintf(
		buf, "| Total Crashes | %d |\n", qa.TotalCrashes,
	)
	fmt.Fprintf(
		buf, "| Total ANRs | %d |\n", qa.TotalANRs,
	)
	fmt.Fprintf(
		buf, "| Total Duration | %v |\n", qa.TotalDuration,
	)
	fmt.Fprintf(
		buf, "| Platforms Tested | %d |\n",
		len(qa.PlatformResults),
	)

	// Per-platform details.
	for _, pr := range qa.PlatformResults {
		r.writePlatformSection(buf, pr)
	}

	fmt.Fprintln(buf)
	fmt.Fprintln(buf, "---")
	fmt.Fprintln(buf)
	fmt.Fprintln(buf, "*Generated by HelixQA*")
}

// writePlatformSection writes a platform-specific section.
func (r *Reporter) writePlatformSection(
	buf *bytes.Buffer,
	pr *PlatformResult,
) {
	fmt.Fprintln(buf)
	fmt.Fprintf(
		buf, "## Platform: %s\n\n",
		strings.ToUpper(string(pr.Platform)),
	)
	fmt.Fprintf(buf, "- **Duration:** %v\n", pr.Duration)
	fmt.Fprintf(buf, "- **Crashes:** %d\n", pr.CrashCount)
	fmt.Fprintf(buf, "- **ANRs:** %d\n", pr.ANRCount)
	fmt.Fprintf(
		buf, "- **Challenges:** %d\n",
		len(pr.ChallengeResults),
	)

	if len(pr.ChallengeResults) > 0 {
		fmt.Fprintln(buf)
		fmt.Fprintln(
			buf,
			"| Challenge | Status | Duration |",
		)
		fmt.Fprintln(
			buf,
			"|-----------|--------|----------|",
		)
		for _, cr := range pr.ChallengeResults {
			fmt.Fprintf(
				buf, "| %s | %s | %v |\n",
				cr.ChallengeName,
				strings.ToUpper(cr.Status),
				cr.Duration,
			)
		}
	}

	if len(pr.StepResults) > 0 {
		fmt.Fprintln(buf)
		fmt.Fprintln(buf, "### Step Validation")
		fmt.Fprintln(buf)
		fmt.Fprintln(
			buf,
			"| Step | Status | Duration | Error |",
		)
		fmt.Fprintln(
			buf,
			"|------|--------|----------|-------|",
		)

		// Sort steps by start time.
		sorted := make(
			[]*validator.StepResult, len(pr.StepResults),
		)
		copy(sorted, pr.StepResults)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].StartTime.Before(
				sorted[j].StartTime,
			)
		})

		for _, sr := range sorted {
			errMsg := sr.Error
			if errMsg == "" {
				errMsg = "-"
			}
			fmt.Fprintf(
				buf, "| %s | %s | %v | %s |\n",
				sr.StepName,
				strings.ToUpper(string(sr.Status)),
				sr.Duration,
				errMsg,
			)
		}
	}
}

// writeHTMLSummary writes an HTML summary report.
func (r *Reporter) writeHTMLSummary(
	qa *QAReport,
	path string,
) error {
	var buf bytes.Buffer

	// Simple HTML wrapper around Markdown content.
	fmt.Fprintln(&buf, "<!DOCTYPE html>")
	fmt.Fprintln(&buf, "<html><head>")
	fmt.Fprintln(&buf, "<meta charset=\"utf-8\">")
	fmt.Fprintln(
		&buf,
		"<title>HelixQA Test Report</title>",
	)
	fmt.Fprintln(&buf, "<style>")
	fmt.Fprintln(&buf, "body { font-family: sans-serif; "+
		"max-width: 960px; margin: 0 auto; padding: 2em; }")
	fmt.Fprintln(&buf, "table { border-collapse: collapse; "+
		"width: 100%; margin: 1em 0; }")
	fmt.Fprintln(&buf, "th, td { border: 1px solid #ddd; "+
		"padding: 8px; text-align: left; }")
	fmt.Fprintln(&buf, "th { background: #f5f5f5; }")
	fmt.Fprintln(&buf, ".passed { color: #2e7d32; }")
	fmt.Fprintln(&buf, ".failed { color: #c62828; }")
	fmt.Fprintln(&buf, "</style>")
	fmt.Fprintln(&buf, "</head><body>")

	fmt.Fprintln(&buf, "<h1>HelixQA Test Report</h1>")
	fmt.Fprintf(
		&buf, "<p>Generated: %s</p>\n",
		qa.GeneratedAt.Format(time.RFC3339),
	)

	fmt.Fprintln(&buf, "<h2>Overview</h2>")
	fmt.Fprintln(&buf, "<table>")
	fmt.Fprintln(&buf, "<tr><th>Metric</th><th>Value</th></tr>")
	fmt.Fprintf(
		&buf,
		"<tr><td>Total Challenges</td><td>%d</td></tr>\n",
		qa.TotalChallenges,
	)
	fmt.Fprintf(
		&buf,
		"<tr><td>Passed</td><td class=\"passed\">%d</td></tr>\n",
		qa.PassedChallenges,
	)
	fmt.Fprintf(
		&buf,
		"<tr><td>Failed</td><td class=\"failed\">%d</td></tr>\n",
		qa.FailedChallenges,
	)
	fmt.Fprintf(
		&buf,
		"<tr><td>Duration</td><td>%v</td></tr>\n",
		qa.TotalDuration,
	)
	fmt.Fprintln(&buf, "</table>")

	fmt.Fprintln(&buf, "</body></html>")

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}
