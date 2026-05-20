// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/i18n"
)

// sentinelTranslator is the paired-mutation backend for the
// CONST-046 round-447 reporter migration. It returns a sentinel
// string that wraps the messageID so a test can prove the
// migrated executive-summary / report call sites genuinely route
// through the i18n seam. If any call site is re-inlined back to a
// hardcoded English literal, the sentinel will NOT appear in the
// output and the assertions below FAIL — that is the paired
// mutation guarding the migration.
type sentinelTranslator struct{}

const i18nSentinelPrefix = "XLATED::"

func (sentinelTranslator) T(
	_ context.Context,
	messageID string,
	_ map[string]any,
) (string, error) {
	return i18nSentinelPrefix + messageID, nil
}

func (sentinelTranslator) TPlural(
	_ context.Context,
	messageID string,
	_ int,
	_ map[string]any,
) (string, error) {
	return i18nSentinelPrefix + messageID, nil
}

// TestReporterT_RoutesThroughSeam proves the reporterT helper
// consults the package-level i18n translator and surfaces the
// backend's output (not the fallback) when the backend returns a
// non-empty, non-echo string.
func TestReporterT_RoutesThroughSeam(t *testing.T) {
	i18n.SetTranslator(sentinelTranslator{})
	t.Cleanup(i18n.ResetForTest)

	got := reporterT(context.Background(),
		"helixqa_report_status_stable", "Stable", nil)
	want := i18nSentinelPrefix + "helixqa_report_status_stable"
	if got != want {
		t.Fatalf("reporterT did not route through seam: got %q, want %q",
			got, want)
	}
}

// TestReporterT_FallbackOnNoop verifies that with the default
// NoopTranslator (which echoes the messageID) the helper returns
// the English fallback — a backend that has not been wired MUST
// NOT leak the raw messageID into stakeholder-facing report text.
func TestReporterT_FallbackOnNoop(t *testing.T) {
	i18n.ResetForTest()
	t.Cleanup(i18n.ResetForTest)

	got := reporterT(context.Background(),
		"helixqa_report_status_stable", "Stable", nil)
	if got != "Stable" {
		t.Fatalf("reporterT with Noop = %q, want fallback %q",
			got, "Stable")
	}
}

// TestGenerateExecutiveSummary_RoutesThroughSeam is the paired
// mutation for GenerateExecutiveSummary: with the sentinel
// translator installed, every migrated status / risk / coverage /
// recommendation string MUST carry the sentinel prefix. Re-inlining
// any literal makes the corresponding assertion FAIL.
func TestGenerateExecutiveSummary_RoutesThroughSeam(t *testing.T) {
	i18n.SetTranslator(sentinelTranslator{})
	t.Cleanup(i18n.ResetForTest)

	// Crash path: critical status + high risk + crash rec.
	pr := &PlatformResult{Platform: "android", CrashCount: 2, ANRCount: 1}
	qa := makeQAReport(2, 1, 3, 1, pr)
	es, err := GenerateExecutiveSummary(qa, nil)
	if err != nil {
		t.Fatalf("GenerateExecutiveSummary error: %v", err)
	}
	if !strings.HasPrefix(es.OverallStatus, i18nSentinelPrefix) {
		t.Errorf("OverallStatus not routed through seam: %q",
			es.OverallStatus)
	}
	if !strings.HasPrefix(es.RiskAssessment, i18nSentinelPrefix) {
		t.Errorf("RiskAssessment not routed through seam: %q",
			es.RiskAssessment)
	}
	if !strings.HasPrefix(es.CoverageHighlights, i18nSentinelPrefix) {
		t.Errorf("CoverageHighlights not routed through seam: %q",
			es.CoverageHighlights)
	}
	for i, rec := range es.Recommendations {
		if !strings.HasPrefix(rec, i18nSentinelPrefix) {
			t.Errorf("Recommendation[%d] not routed through seam: %q",
				i, rec)
		}
	}
	for i, issue := range es.TopIssues {
		if !strings.HasPrefix(issue, i18nSentinelPrefix) {
			t.Errorf("TopIssue[%d] not routed through seam: %q",
				i, issue)
		}
	}

	// Stable path: stable status + low risk + all-passed rec.
	qaOK, _ := GenerateExecutiveSummary(makeQAReport(0, 0, 5, 0), nil)
	if !strings.HasPrefix(qaOK.OverallStatus, i18nSentinelPrefix) {
		t.Errorf("stable OverallStatus not routed: %q",
			qaOK.OverallStatus)
	}
	if !strings.HasPrefix(qaOK.RiskAssessment, i18nSentinelPrefix) {
		t.Errorf("low RiskAssessment not routed: %q",
			qaOK.RiskAssessment)
	}
}

// TestRenderMarkdown_HeadingsRouteThroughSeam proves the executive
// summary and navigation-map Markdown section headings are resolved
// via the i18n seam. Re-inlining "## Executive Summary" etc. fails
// these assertions.
func TestRenderMarkdown_HeadingsRouteThroughSeam(t *testing.T) {
	i18n.SetTranslator(sentinelTranslator{})
	t.Cleanup(i18n.ResetForTest)

	es := &ExecutiveSummary{
		OverallStatus:   "ok",
		RiskAssessment:  "low",
		TopIssues:       []string{"issue"},
		Recommendations: []string{"rec"},
	}
	md := es.RenderMarkdown()
	for _, id := range []string{
		"helixqa_report_heading_executive_summary",
		"helixqa_report_heading_top_issues",
		"helixqa_report_heading_recommendations",
		"helixqa_report_label_status",
		"helixqa_report_label_risk",
	} {
		if !strings.Contains(md, i18nSentinelPrefix+id) {
			t.Errorf("RenderMarkdown missing seam-routed %q\n--- output ---\n%s",
				id, md)
		}
	}

	nm := &NavigationMapEmbed{Format: "mermaid", Content: "graph TD"}
	nmMD := nm.RenderMarkdown()
	if !strings.Contains(nmMD,
		i18nSentinelPrefix+"helixqa_report_heading_navigation_map") {
		t.Errorf("NavigationMapEmbed heading not routed through seam:\n%s",
			nmMD)
	}
}
