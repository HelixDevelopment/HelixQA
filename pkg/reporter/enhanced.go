// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"digital.vasic.helixqa/pkg/i18n"
)

// reporterT resolves a user-facing string for the executive
// summary / report surface through the package-level translator
// seam in helix_qa/pkg/i18n (CONST-046). Callers pass the
// namespaced messageID (`helixqa_report_*`), the pre-migration
// English fallback, and optional template data for placeholder
// substitution. The seam-installed backend (LLM-driven or
// YAML-bundle-backed) returns the localised string; the
// NoopTranslator default echoes the messageID, in which case the
// fallback is used so a backend that has not been wired cannot
// produce empty stakeholder-facing report text. Errors from the
// backend are likewise resolved to the fallback.
func reporterT(
	ctx context.Context,
	messageID, fallback string,
	data map[string]any,
) string {
	out, err := i18n.ActiveTranslator().T(ctx, messageID, data)
	if err != nil || out == "" || out == messageID {
		return fallback
	}
	return out
}

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
// deterministic summary from report data. All user-facing
// strings are resolved through the i18n seam (CONST-046).
func GenerateExecutiveSummary(
	qa *QAReport,
	agent LLMSummarizer,
) (*ExecutiveSummary, error) {
	if qa == nil {
		return nil, fmt.Errorf("QA report is nil")
	}

	ctx := context.Background()
	summary := &ExecutiveSummary{}

	// Determine overall status.
	if qa.TotalCrashes > 0 {
		summary.OverallStatus = reporterT(ctx,
			"helixqa_report_status_critical",
			"Critical Issues Found", nil)
	} else if qa.FailedChallenges > 0 {
		summary.OverallStatus = reporterT(ctx,
			"helixqa_report_status_at_risk", "At Risk", nil)
	} else {
		summary.OverallStatus = reporterT(ctx,
			"helixqa_report_status_stable", "Stable", nil)
	}

	// Determine risk assessment.
	if qa.TotalCrashes > 0 || qa.TotalANRs > 0 {
		summary.RiskAssessment = reporterT(ctx,
			"helixqa_report_risk_high",
			fmt.Sprintf(
				"High risk: %d crashes, %d ANRs detected",
				qa.TotalCrashes, qa.TotalANRs,
			),
			map[string]any{
				"Crashes": qa.TotalCrashes,
				"ANRs":    qa.TotalANRs,
			})
	} else if qa.FailedChallenges > 0 {
		summary.RiskAssessment = reporterT(ctx,
			"helixqa_report_risk_medium",
			fmt.Sprintf(
				"Medium risk: %d of %d challenges failed",
				qa.FailedChallenges, qa.TotalChallenges,
			),
			map[string]any{
				"Failed": qa.FailedChallenges,
				"Total":  qa.TotalChallenges,
			})
	} else {
		summary.RiskAssessment = reporterT(ctx,
			"helixqa_report_risk_low",
			"Low risk: all checks passed", nil)
	}

	// Collect top issues from platform results.
	for _, pr := range qa.PlatformResults {
		platform := strings.ToUpper(string(pr.Platform))
		if pr.CrashCount > 0 {
			summary.TopIssues = append(
				summary.TopIssues,
				reporterT(ctx, "helixqa_report_issue_crashes",
					fmt.Sprintf("%d crash(es) on %s",
						pr.CrashCount, platform),
					map[string]any{
						"Count":    pr.CrashCount,
						"Platform": platform,
					}),
			)
		}
		if pr.ANRCount > 0 {
			summary.TopIssues = append(
				summary.TopIssues,
				reporterT(ctx, "helixqa_report_issue_anrs",
					fmt.Sprintf("%d ANR(s) on %s",
						pr.ANRCount, platform),
					map[string]any{
						"Count":    pr.ANRCount,
						"Platform": platform,
					}),
			)
		}
	}

	// Generate coverage highlights.
	if qa.TotalChallenges > 0 {
		pct := float64(qa.PassedChallenges) /
			float64(qa.TotalChallenges) * 100
		summary.CoverageHighlights = reporterT(ctx,
			"helixqa_report_coverage_highlights",
			fmt.Sprintf(
				"%.0f%% pass rate (%d/%d challenges) across %d platform(s)",
				pct, qa.PassedChallenges, qa.TotalChallenges,
				len(qa.PlatformResults),
			),
			map[string]any{
				"Percent":   fmt.Sprintf("%.0f", pct),
				"Passed":    qa.PassedChallenges,
				"Total":     qa.TotalChallenges,
				"Platforms": len(qa.PlatformResults),
			})
	} else {
		summary.CoverageHighlights = reporterT(ctx,
			"helixqa_report_coverage_none",
			"No challenges executed", nil)
	}

	// Generate recommendations.
	if qa.TotalCrashes > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			reporterT(ctx, "helixqa_report_rec_crashes",
				"Investigate and fix crash root causes before release",
				nil),
		)
	}
	if qa.TotalANRs > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			reporterT(ctx, "helixqa_report_rec_anrs",
				"Address ANR issues to improve responsiveness",
				nil),
		)
	}
	if qa.FailedChallenges > 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			reporterT(ctx, "helixqa_report_rec_failed",
				fmt.Sprintf("Fix %d failing challenge(s)",
					qa.FailedChallenges),
				map[string]any{"Failed": qa.FailedChallenges}),
		)
	}
	if len(summary.Recommendations) == 0 {
		summary.Recommendations = append(
			summary.Recommendations,
			reporterT(ctx, "helixqa_report_rec_all_passed",
				"All checks passed — ready for release consideration",
				nil),
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

// RenderMarkdown renders the ExecutiveSummary as Markdown. All
// section headings and field labels are resolved through the
// i18n seam (CONST-046).
func (es *ExecutiveSummary) RenderMarkdown() string {
	var buf bytes.Buffer
	ctx := context.Background()

	fmt.Fprintf(&buf, "## %s\n", reporterT(ctx,
		"helixqa_report_heading_executive_summary",
		"Executive Summary", nil))
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "**%s** %s\n\n", reporterT(ctx,
		"helixqa_report_label_status", "Status:", nil),
		es.OverallStatus)
	fmt.Fprintf(&buf, "**%s** %s\n\n", reporterT(ctx,
		"helixqa_report_label_risk", "Risk:", nil),
		es.RiskAssessment)

	if es.CoverageHighlights != "" {
		fmt.Fprintf(&buf, "**%s** %s\n\n", reporterT(ctx,
			"helixqa_report_label_coverage", "Coverage:", nil),
			es.CoverageHighlights,
		)
	}

	if len(es.TopIssues) > 0 {
		fmt.Fprintf(&buf, "### %s\n", reporterT(ctx,
			"helixqa_report_heading_top_issues",
			"Top Issues", nil))
		fmt.Fprintln(&buf)
		for _, issue := range es.TopIssues {
			fmt.Fprintf(&buf, "- %s\n", issue)
		}
		fmt.Fprintln(&buf)
	}

	if len(es.Recommendations) > 0 {
		fmt.Fprintf(&buf, "### %s\n", reporterT(ctx,
			"helixqa_report_heading_recommendations",
			"Recommendations", nil))
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

	fmt.Fprintf(&buf, "## %s\n", reporterT(context.Background(),
		"helixqa_report_heading_navigation_map",
		"Navigation Map", nil))
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
