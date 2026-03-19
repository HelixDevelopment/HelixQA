// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// ExecutiveSummary provides a high-level overview of a QA run,
// suitable for stakeholder review. Generated optionally by an
// LLM summarizer.
type ExecutiveSummary struct {
	// OverallStatus is a one-line status (e.g., "Stable",
	// "At Risk", "Critical Issues Found").
	OverallStatus string `json:"overall_status"`

	// RiskAssessment describes the overall risk level.
	RiskAssessment string `json:"risk_assessment"`

	// TopIssues lists the most important issues found.
	TopIssues []string `json:"top_issues,omitempty"`

	// Recommendations lists suggested actions.
	Recommendations []string `json:"recommendations,omitempty"`

	// CoverageHighlights describes coverage achievements.
	CoverageHighlights string `json:"coverage_highlights"`
}

// NavigationMapEmbed holds a renderable navigation graph in
// a text-based graph format.
type NavigationMapEmbed struct {
	// Format is the graph format ("mermaid" or "dot").
	Format string `json:"format"`

	// Content is the graph definition text.
	Content string `json:"content"`
}

// LLMSummarizer generates text summaries using an LLM. This
// is an optional dependency -- reports work without it.
type LLMSummarizer interface {
	// Summarize condenses the given text into a concise
	// summary.
	Summarize(
		ctx context.Context,
		text string,
	) (string, error)
}

// GenerateExecutiveSummary creates an ExecutiveSummary from a
// QAReport using the optional LLM summarizer for natural
// language generation. If agent is nil, generates a
// deterministic summary from report data.
func GenerateExecutiveSummary(
	qa *QAReport,
	agent LLMSummarizer,
) (*ExecutiveSummary, error) {
	if qa == nil {
		return nil, fmt.Errorf("QA report is nil")
	}

	summary := &ExecutiveSummary{}

	// Determine overall status.
	if qa.TotalCrashes > 0 {
		summary.OverallStatus = "Critical Issues Found"
	} else if qa.FailedChallenges > 0 {
		summary.OverallStatus = "At Risk"
	} else {
		summary.OverallStatus = "Stable"
	}

	// Determine risk assessment.
	if qa.TotalCrashes > 0 || qa.TotalANRs > 0 {
		summary.RiskAssessment = fmt.Sprintf(
			"High risk: %d crashes, %d ANRs detected",
			qa.TotalCrashes, qa.TotalANRs,
		)
	} else if qa.FailedChallenges > 0 {
		summary.RiskAssessment = fmt.Sprintf(
			"Medium risk: %d of %d challenges failed",
			qa.FailedChallenges, qa.TotalChallenges,
		)
	} else {
		summary.RiskAssessment = "Low risk: all checks passed"
	}

	// Collect top issues from platform results.
	for _, pr := range qa.PlatformResults {
		if pr.CrashCount > 0 {
			summary.TopIssues = append(
				summary.TopIssues,
				fmt.Sprintf(
					"%d crash(es) on %s",
					pr.CrashCount,
					strings.ToUpper(string(pr.Platform)),
				),
			)
		}
		if pr.ANRCount > 0 {
			summary.TopIssues = append(
				summary.TopIssues,
				fmt.Sprintf(
					"%d ANR(s) on %s",
					pr.ANRCount,
					strings.ToUpper(string(pr.Platform)),
				),
			)
		}
	}

	// Generate coverage highlights.
	if qa.TotalChallenges > 0 {
		pct := float64(qa.PassedChallenges) /
			float64(qa.TotalChallenges) * 100
		summary.CoverageHighlights = fmt.Sprintf(
			"%.0f%% pass rate (%d/%d challenges) across %d platform(s)",
			pct, qa.PassedChallenges, qa.TotalChallenges,
			len(qa.PlatformResults),
		)
	} else {
		summary.CoverageHighlights =
			"No challenges executed"
	}

	// Generate recommendations.
	if qa.TotalCrashes > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			"Investigate and fix crash root causes before release",
		)
	}
	if qa.TotalANRs > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			"Address ANR issues to improve responsiveness",
		)
	}
	if qa.FailedChallenges > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			fmt.Sprintf(
				"Fix %d failing challenge(s)", qa.FailedChallenges,
			),
		)
	}
	if len(summary.Recommendations) == 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			"All checks passed — ready for release consideration",
		)
	}

	// If LLM summarizer is available, enhance the summary.
	if agent != nil {
		reportText := buildReportText(qa)
		llmSummary, err := agent.Summarize(
			context.Background(), reportText,
		)
		if err == nil && llmSummary != "" {
			summary.RiskAssessment = llmSummary
		}
		// On error, keep the deterministic summary.
	}

	return summary, nil
}

// Validate checks that the ExecutiveSummary has required fields.
func (es *ExecutiveSummary) Validate() error {
	if es.OverallStatus == "" {
		return fmt.Errorf("overall status is required")
	}
	if es.RiskAssessment == "" {
		return fmt.Errorf("risk assessment is required")
	}
	return nil
}

// Validate checks that the NavigationMapEmbed has valid fields.
func (nm *NavigationMapEmbed) Validate() error {
	if nm.Format == "" {
		return fmt.Errorf("format is required")
	}
	if nm.Format != "mermaid" && nm.Format != "dot" {
		return fmt.Errorf(
			"format must be 'mermaid' or 'dot', got %q",
			nm.Format,
		)
	}
	if nm.Content == "" {
		return fmt.Errorf("content is required")
	}
	return nil
}

// RenderMarkdown renders the ExecutiveSummary as Markdown.
func (es *ExecutiveSummary) RenderMarkdown() string {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "## Executive Summary")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "**Status:** %s\n\n", es.OverallStatus)
	fmt.Fprintf(&buf, "**Risk:** %s\n\n", es.RiskAssessment)

	if es.CoverageHighlights != "" {
		fmt.Fprintf(&buf,
			"**Coverage:** %s\n\n", es.CoverageHighlights,
		)
	}

	if len(es.TopIssues) > 0 {
		fmt.Fprintln(&buf, "### Top Issues")
		fmt.Fprintln(&buf)
		for _, issue := range es.TopIssues {
			fmt.Fprintf(&buf, "- %s\n", issue)
		}
		fmt.Fprintln(&buf)
	}

	if len(es.Recommendations) > 0 {
		fmt.Fprintln(&buf, "### Recommendations")
		fmt.Fprintln(&buf)
		for _, rec := range es.Recommendations {
			fmt.Fprintf(&buf, "- %s\n", rec)
		}
		fmt.Fprintln(&buf)
	}

	return buf.String()
}

// RenderMarkdown renders the NavigationMapEmbed as a Markdown
// code block.
func (nm *NavigationMapEmbed) RenderMarkdown() string {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "## Navigation Map")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "```%s\n", nm.Format)
	fmt.Fprintln(&buf, nm.Content)
	fmt.Fprintln(&buf, "```")

	return buf.String()
}

// buildReportText creates a text representation of the QA
// report for LLM summarization.
func buildReportText(qa *QAReport) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "QA Report: %s\n", qa.Title)
	fmt.Fprintf(&buf, "Total challenges: %d\n",
		qa.TotalChallenges)
	fmt.Fprintf(&buf, "Passed: %d, Failed: %d\n",
		qa.PassedChallenges, qa.FailedChallenges)
	fmt.Fprintf(&buf, "Crashes: %d, ANRs: %d\n",
		qa.TotalCrashes, qa.TotalANRs)
	fmt.Fprintf(&buf, "Duration: %v\n", qa.TotalDuration)
	fmt.Fprintf(&buf, "Platforms: %d\n",
		len(qa.PlatformResults))

	for _, pr := range qa.PlatformResults {
		fmt.Fprintf(&buf, "Platform %s: %d crashes, %d ANRs, "+
			"%d challenges\n",
			pr.Platform, pr.CrashCount, pr.ANRCount,
			len(pr.ChallengeResults),
		)
	}

	return buf.String()
}
