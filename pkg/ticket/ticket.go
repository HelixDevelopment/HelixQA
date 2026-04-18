// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ticket generates detailed Markdown issue tickets
// from QA test failures. Each ticket includes severity,
// platform, reproduction steps, evidence (screenshots, logs,
// stack traces), and documentation references. Tickets are
// designed to feed directly into AI fix pipelines.
package ticket

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
	"digital.vasic.helixqa/pkg/validator"
)

// VideoReference links a ticket to a specific timestamp in a
// recorded video. This is an optional field -- tickets work
// without it.
type VideoReference struct {
	// VideoPath is the path to the video file.
	VideoPath string `json:"video_path"`

	// Timestamp is the offset into the video where the issue
	// is visible.
	Timestamp time.Duration `json:"timestamp"`

	// Description explains what is visible at this timestamp.
	Description string `json:"description"`
}

// LLMSuggestedFix holds an LLM-generated fix suggestion for
// the issue described in a ticket. This is an optional field.
type LLMSuggestedFix struct {
	// Description explains the suggested fix.
	Description string `json:"description"`

	// CodeSnippet contains example code for the fix.
	CodeSnippet string `json:"code_snippet,omitempty"`

	// Confidence is the LLM's confidence in this fix
	// (0.0 - 1.0).
	Confidence float64 `json:"confidence"`

	// AffectedFiles lists source files that need changes.
	AffectedFiles []string `json:"affected_files,omitempty"`
}

// Validate checks that the VideoReference has required fields.
func (vr *VideoReference) Validate() error {
	if vr.VideoPath == "" {
		return fmt.Errorf("video path is required")
	}
	if vr.Timestamp < 0 {
		return fmt.Errorf("timestamp must be non-negative")
	}
	return nil
}

// Validate checks that the LLMSuggestedFix has valid fields.
func (sf *LLMSuggestedFix) Validate() error {
	if sf.Description == "" {
		return fmt.Errorf("description is required")
	}
	if sf.Confidence < 0 || sf.Confidence > 1 {
		return fmt.Errorf(
			"confidence must be 0.0-1.0, got %f",
			sf.Confidence,
		)
	}
	return nil
}

// Severity levels for issue tickets.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Ticket represents a single QA issue to be fixed.
type Ticket struct {
	// ID is a unique identifier for this ticket.
	ID string `json:"id"`

	// Title is a short summary of the issue.
	Title string `json:"title"`

	// Severity indicates issue priority.
	Severity Severity `json:"severity"`

	// Platform where the issue was found.
	Platform config.Platform `json:"platform"`

	// TestCaseID links to the test case that found the issue.
	TestCaseID string `json:"test_case_id"`

	// Description provides a detailed explanation.
	Description string `json:"description"`

	// StepsToReproduce lists how to reproduce the issue.
	StepsToReproduce []string `json:"steps_to_reproduce"`

	// ExpectedBehavior describes what should happen.
	ExpectedBehavior string `json:"expected_behavior"`

	// ActualBehavior describes what actually happened.
	ActualBehavior string `json:"actual_behavior"`

	// Detection holds crash/ANR detection data.
	Detection *detector.DetectionResult `json:"detection,omitempty"`

	// StepResult holds the validation result.
	StepResult *validator.StepResult `json:"step_result,omitempty"`

	// Screenshots lists paths to relevant screenshots.
	Screenshots []string `json:"screenshots,omitempty"`

	// Logs contains relevant log entries.
	Logs []string `json:"logs,omitempty"`

	// StackTrace contains the crash stack trace.
	StackTrace string `json:"stack_trace,omitempty"`

	// CreatedAt is when the ticket was generated.
	CreatedAt time.Time `json:"created_at"`

	// Labels for categorization.
	Labels []string `json:"labels,omitempty"`

	// VideoRefs links this ticket to video timestamps.
	// Optional: nil means no video references.
	VideoRefs []*VideoReference `json:"video_refs,omitempty"`

	// SuggestedFix holds an LLM-generated fix suggestion.
	// Optional: nil means no suggestion available.
	SuggestedFix *LLMSuggestedFix `json:"suggested_fix,omitempty"`

	// ---- P7 (OpenClawing2 Phase 7) — rich evidence fields ----
	// These fields lock in the "better-documented tickets" brief
	// from the OpenClawing2 plan. Every one is optional so legacy
	// tickets stay schema-compatible; callers populate only what
	// applies to their failure scenario.

	// VideoTimestamp is an mm:ss offset into VideoRefs[0].Path where
	// the failure is visible. Empty string = no pinned timestamp.
	VideoTimestamp string `json:"video_timestamp,omitempty"`

	// BeforeScreenshotPath + AfterScreenshotPath pin the paired
	// pre/post frames for a single failed step. Reviewers open
	// these side-by-side to see exactly what changed.
	BeforeScreenshotPath string `json:"before_screenshot_path,omitempty"`
	AfterScreenshotPath  string `json:"after_screenshot_path,omitempty"`

	// LLMReasoningTranscript is the planner's "Evaluation + Memory
	// + NextGoal" trace across the steps leading up to the failure.
	// Reviewers see exactly what the model was thinking when it
	// produced the bad Action.
	LLMReasoningTranscript []string `json:"llm_reasoning_transcript,omitempty"`

	// ReproductionBank names the YAML bank entry that will
	// permanently replay this scenario — typically a
	// banks/fixes-validation-*.yaml row. The enhanced generator
	// can auto-create the stub so the ticket + regression guard
	// ship together.
	ReproductionBank string `json:"reproduction_bank,omitempty"`

	// SessionID is the Agent / nexus.Session ID that produced the
	// failure. Links the ticket back to qa-results/session-*.
	SessionID string `json:"session_id,omitempty"`

	// StepNumber is the 1-based iteration index inside the Agent
	// run that produced this failure.
	StepNumber int `json:"step_number,omitempty"`
}

// Generator creates markdown tickets from QA results.
type Generator struct {
	outputDir string
	counter   int
}

// Option configures a Generator.
type Option func(*Generator)

// WithOutputDir sets the ticket output directory.
func WithOutputDir(dir string) Option {
	return func(g *Generator) {
		g.outputDir = dir
	}
}

// New creates a Generator with the given options.
func New(opts ...Option) *Generator {
	g := &Generator{
		outputDir: "tickets",
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// GenerateFromStep creates a ticket from a failed step result.
func (g *Generator) GenerateFromStep(
	sr *validator.StepResult,
	testCaseID string,
) *Ticket {
	g.counter++
	t := &Ticket{
		ID:         fmt.Sprintf("HQA-%04d", g.counter),
		TestCaseID: testCaseID,
		Platform:   sr.Platform,
		CreatedAt:  time.Now(),
	}

	if sr.Detection != nil && sr.Detection.HasCrash {
		t.Title = fmt.Sprintf(
			"Crash detected during %q on %s",
			sr.StepName, sr.Platform,
		)
		t.Severity = SeverityCritical
		t.ActualBehavior = "Application crashed"
		t.StackTrace = sr.Detection.StackTrace
		t.Logs = sr.Detection.LogEntries
		t.Labels = []string{"crash", string(sr.Platform)}
	} else if sr.Detection != nil && sr.Detection.HasANR {
		t.Title = fmt.Sprintf(
			"ANR detected during %q on %s",
			sr.StepName, sr.Platform,
		)
		t.Severity = SeverityHigh
		t.ActualBehavior = "Application not responding"
		t.Logs = sr.Detection.LogEntries
		t.Labels = []string{"anr", string(sr.Platform)}
	} else {
		t.Title = fmt.Sprintf(
			"Step %q failed on %s",
			sr.StepName, sr.Platform,
		)
		t.Severity = SeverityMedium
		t.ActualBehavior = sr.Error
		t.Labels = []string{"failure", string(sr.Platform)}
	}

	t.Description = fmt.Sprintf(
		"Test step %q of test case %s failed on platform %s. "+
			"Status: %s. Duration: %v.",
		sr.StepName, testCaseID, sr.Platform,
		sr.Status, sr.Duration,
	)
	t.Detection = sr.Detection

	// Collect screenshot evidence.
	if sr.PreScreenshot != "" {
		t.Screenshots = append(t.Screenshots, sr.PreScreenshot)
	}
	if sr.PostScreenshot != "" {
		t.Screenshots = append(t.Screenshots, sr.PostScreenshot)
	}
	if sr.Detection != nil && sr.Detection.ScreenshotPath != "" {
		t.Screenshots = append(
			t.Screenshots,
			sr.Detection.ScreenshotPath,
		)
	}

	return t
}

// GenerateFromDetection creates a ticket from a raw detection
// result (e.g., background crash monitoring).
func (g *Generator) GenerateFromDetection(
	dr *detector.DetectionResult,
	context string,
) *Ticket {
	g.counter++
	t := &Ticket{
		ID:        fmt.Sprintf("HQA-%04d", g.counter),
		Platform:  dr.Platform,
		CreatedAt: time.Now(),
		Detection: dr,
		Logs:      dr.LogEntries,
	}

	if dr.HasCrash {
		t.Title = fmt.Sprintf(
			"Crash on %s: %s", dr.Platform, context,
		)
		t.Severity = SeverityCritical
		t.StackTrace = dr.StackTrace
		t.Labels = []string{"crash", string(dr.Platform)}
	} else if dr.HasANR {
		t.Title = fmt.Sprintf(
			"ANR on %s: %s", dr.Platform, context,
		)
		t.Severity = SeverityHigh
		t.Labels = []string{"anr", string(dr.Platform)}
	}

	t.ActualBehavior = context
	if dr.ScreenshotPath != "" {
		t.Screenshots = append(t.Screenshots, dr.ScreenshotPath)
	}

	return t
}

// WriteTicket writes a ticket as a Markdown file.
func (g *Generator) WriteTicket(t *Ticket) (string, error) {
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create ticket dir: %w", err)
	}

	filename := fmt.Sprintf("%s.md", t.ID)
	path := filepath.Join(g.outputDir, filename)

	content := g.RenderMarkdown(t)
	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("write ticket %s: %w", t.ID, err)
	}
	return path, nil
}

// WriteAll writes all tickets and returns their paths.
func (g *Generator) WriteAll(tickets []*Ticket) ([]string, error) {
	paths := make([]string, 0, len(tickets))
	for _, t := range tickets {
		path, err := g.WriteTicket(t)
		if err != nil {
			return paths, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// RenderMarkdown converts a ticket to Markdown bytes.
func (g *Generator) RenderMarkdown(t *Ticket) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "# %s\n\n", t.Title)

	// Metadata table.
	fmt.Fprintln(&buf, "| Field | Value |")
	fmt.Fprintln(&buf, "|-------|-------|")
	fmt.Fprintf(&buf, "| **ID** | %s |\n", t.ID)
	fmt.Fprintf(&buf, "| **Severity** | %s |\n",
		strings.ToUpper(string(t.Severity)))
	fmt.Fprintf(&buf, "| **Platform** | %s |\n", t.Platform)
	if t.TestCaseID != "" {
		fmt.Fprintf(&buf, "| **Test Case** | %s |\n",
			t.TestCaseID)
	}
	fmt.Fprintf(&buf, "| **Created** | %s |\n",
		t.CreatedAt.Format(time.RFC3339))
	if len(t.Labels) > 0 {
		fmt.Fprintf(&buf, "| **Labels** | %s |\n",
			strings.Join(t.Labels, ", "))
	}
	fmt.Fprintln(&buf)

	// Description.
	if t.Description != "" {
		fmt.Fprintln(&buf, "## Description")
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, t.Description)
		fmt.Fprintln(&buf)
	}

	// Steps to reproduce.
	if len(t.StepsToReproduce) > 0 {
		fmt.Fprintln(&buf, "## Steps to Reproduce")
		fmt.Fprintln(&buf)
		for i, step := range t.StepsToReproduce {
			fmt.Fprintf(&buf, "%d. %s\n", i+1, step)
		}
		fmt.Fprintln(&buf)
	}

	// Expected vs actual.
	if t.ExpectedBehavior != "" || t.ActualBehavior != "" {
		fmt.Fprintln(&buf, "## Expected vs Actual")
		fmt.Fprintln(&buf)
		if t.ExpectedBehavior != "" {
			fmt.Fprintf(&buf,
				"**Expected:** %s\n\n", t.ExpectedBehavior,
			)
		}
		if t.ActualBehavior != "" {
			fmt.Fprintf(&buf,
				"**Actual:** %s\n\n", t.ActualBehavior,
			)
		}
	}

	// Stack trace.
	if t.StackTrace != "" {
		fmt.Fprintln(&buf, "## Stack Trace")
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "```")
		fmt.Fprintln(&buf, t.StackTrace)
		fmt.Fprintln(&buf, "```")
		fmt.Fprintln(&buf)
	}

	// Log entries.
	if len(t.Logs) > 0 {
		fmt.Fprintln(&buf, "## Logs")
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "```")
		for _, line := range t.Logs {
			fmt.Fprintln(&buf, line)
		}
		fmt.Fprintln(&buf, "```")
		fmt.Fprintln(&buf)
	}

	// Evidence.
	if len(t.Screenshots) > 0 {
		fmt.Fprintln(&buf, "## Evidence")
		fmt.Fprintln(&buf)
		for _, s := range t.Screenshots {
			fmt.Fprintf(&buf, "- Screenshot: `%s`\n", s)
		}
		fmt.Fprintln(&buf)
	}

	// Video references (optional).
	if len(t.VideoRefs) > 0 {
		fmt.Fprintln(&buf, "## Video References")
		fmt.Fprintln(&buf)
		for _, vr := range t.VideoRefs {
			fmt.Fprintf(&buf, "- `%s` @ %s",
				vr.VideoPath, formatDuration(vr.Timestamp),
			)
			if vr.Description != "" {
				fmt.Fprintf(&buf, " — %s", vr.Description)
			}
			fmt.Fprintln(&buf)
		}
		fmt.Fprintln(&buf)
	}

	// LLM suggested fix (optional).
	if t.SuggestedFix != nil {
		fmt.Fprintln(&buf, "## Suggested Fix")
		fmt.Fprintln(&buf)
		fmt.Fprintf(&buf,
			"**Confidence:** %.0f%%\n\n",
			t.SuggestedFix.Confidence*100,
		)
		fmt.Fprintln(&buf, t.SuggestedFix.Description)
		fmt.Fprintln(&buf)
		if t.SuggestedFix.CodeSnippet != "" {
			fmt.Fprintln(&buf, "```")
			fmt.Fprintln(&buf, t.SuggestedFix.CodeSnippet)
			fmt.Fprintln(&buf, "```")
			fmt.Fprintln(&buf)
		}
		if len(t.SuggestedFix.AffectedFiles) > 0 {
			fmt.Fprintln(&buf, "**Affected files:**")
			fmt.Fprintln(&buf)
			for _, f := range t.SuggestedFix.AffectedFiles {
				fmt.Fprintf(&buf, "- `%s`\n", f)
			}
			fmt.Fprintln(&buf)
		}
	}

	fmt.Fprintln(&buf, "---")
	fmt.Fprintln(&buf, "*Generated by HelixQA*")

	return buf.Bytes()
}

// formatDuration formats a time.Duration as MM:SS for display.
func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	minutes := total / 60
	seconds := total % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
