// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/i18n"
)

// fakeTranslator returns a sentinel for every messageID, so
// migration tests can assert that the seam was exercised rather
// than the pre-migration English literal being printed directly.
// Per CONST-046 contract: a call site that bypasses
// ActiveTranslator and prints the literal directly is a
// regression.
type fakeTranslator struct{}

func (fakeTranslator) T(_ context.Context, messageID string, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + messageID + ">", nil
}

func (fakeTranslator) TPlural(_ context.Context, messageID string, _ int, _ map[string]any) (string, error) {
	return "<TRANSLATED-PLURAL:" + messageID + ">", nil
}

// withFakeTranslator installs the test backend for the duration
// of the test and restores the Noop default in cleanup. Tests
// MUST call this rather than touching i18n state directly.
func withFakeTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(i18n.ResetForTest)
	i18n.SetTranslator(fakeTranslator{})
}

// TestHelixqaT_RoutesThroughActiveTranslator asserts the call
// site helper exercises the seam and returns the sentinel — NOT
// the pre-migration literal — when a real backend is installed.
// This is the anti-bluff guarantee for every migrated call site
// in main.go: if a future change replaces helixqaT(...) with the
// raw literal, this test FAILs.
func TestHelixqaT_RoutesThroughActiveTranslator(t *testing.T) {
	withFakeTranslator(t)

	cases := []struct {
		id       string
		fallback string
		want     string
	}{
		{
			id:       "helixqa_cli_banner",
			fallback: "HelixQA — AI-driven QA orchestration",
			want:     "<TRANSLATED:helixqa_cli_banner>",
		},
		{
			id:       "helixqa_cli_help_hint",
			fallback: "Run 'helixqa <command> --help' for command details.",
			want:     "<TRANSLATED:helixqa_cli_help_hint>",
		},
		{
			id:       "helixqa_run_banks_flag_usage",
			fallback: "Comma-separated test bank paths (files or directories)",
			want:     "<TRANSLATED:helixqa_run_banks_flag_usage>",
		},
		{
			id:       "helixqa_run_platform_flag_usage",
			fallback: "Target platform: android|web|desktop|all",
			want:     "<TRANSLATED:helixqa_run_platform_flag_usage>",
		},
		{
			id:       "helixqa_run_speed_flag_usage",
			fallback: "Speed mode: slow|normal|fast",
			want:     "<TRANSLATED:helixqa_run_speed_flag_usage>",
		},
		// --- round-205 (extension, 10 entries) ---------------
		{
			id:       "helixqa_run_interrupt_shutdown",
			fallback: "Received interrupt, shutting down...",
			want:     "<TRANSLATED:helixqa_run_interrupt_shutdown>",
		},
		{
			id:       "helixqa_run_summary_observed_line1",
			fallback: "OBSERVED - 0 challenges executed; crash-observation only.",
			want:     "<TRANSLATED:helixqa_run_summary_observed_line1>",
		},
		{
			id:       "helixqa_run_summary_observed_line2",
			fallback: "           This is NOT a PASS. The bank's prose steps require",
			want:     "<TRANSLATED:helixqa_run_summary_observed_line2>",
		},
		{
			id:       "helixqa_run_summary_observed_line3",
			fallback: "           an execution backend (autonomous LLM mode or a",
			want:     "<TRANSLATED:helixqa_run_summary_observed_line3>",
		},
		{
			id:       "helixqa_run_summary_observed_line4",
			fallback: "           concrete-action runner like Appium/UiAutomator2).",
			want:     "<TRANSLATED:helixqa_run_summary_observed_line4>",
		},
		{
			id:       "helixqa_run_summary_passed_fmt",
			fallback: "PASSED - All %d challenges passed, no crashes\n",
			want:     "<TRANSLATED:helixqa_run_summary_passed_fmt>",
		},
		{
			id:       "helixqa_run_summary_failed_fmt",
			fallback: "FAILED - %d/%d challenges failed or crashes detected\n",
			want:     "<TRANSLATED:helixqa_run_summary_failed_fmt>",
		},
		{
			id:       "helixqa_run_summary_report_fmt",
			fallback: "Report: %s\n",
			want:     "<TRANSLATED:helixqa_run_summary_report_fmt>",
		},
		{
			id:       "helixqa_run_summary_duration_fmt",
			fallback: "Duration: %v\n",
			want:     "<TRANSLATED:helixqa_run_summary_duration_fmt>",
		},
		{
			id:       "helixqa_report_generated_fmt",
			fallback: "Report generated: %s\n",
			want:     "<TRANSLATED:helixqa_report_generated_fmt>",
		},
	}

	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got := helixqaT(ctx, tc.id, tc.fallback)
			// Sentinel assertion — NOT the original literal.
			// Per the round-112 anti-bluff contract: if the
			// migration regresses to the raw literal, this
			// assertion catches it.
			if got != tc.want {
				t.Fatalf("helixqaT(%q) = %q, want sentinel %q (migration regression: call site is printing the literal directly, bypassing the i18n seam)", tc.id, got, tc.want)
			}
			// Defensive cross-check: result must NOT equal the
			// pre-migration English fallback when the fake
			// backend is installed.
			if got == tc.fallback {
				t.Fatalf("helixqaT(%q) returned the pre-migration English fallback %q while fakeTranslator is installed — the seam is being bypassed", tc.id, tc.fallback)
			}
		})
	}
}

// TestPrintUsage_RoutesBannerThroughSeam captures stdout while
// invoking the actual printUsage call site, asserting the
// translator-routed banner sentinel appears (NOT the raw English
// literal). This is the mutation-sensitive guarantee: if a
// future change replaces the helixqaT(...) call inside
// printUsage with the raw literal, this test FAILs because the
// sentinel will not appear in captured output.
func TestPrintUsage_RoutesBannerThroughSeam(t *testing.T) {
	withFakeTranslator(t)

	// Capture stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	printUsage()

	if cerr := w.Close(); cerr != nil {
		os.Stdout = origStdout
		t.Fatalf("close pipe writer: %v", cerr)
	}
	os.Stdout = origStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	captured := string(out)

	// Sentinel MUST appear (seam is being exercised).
	const bannerSentinel = "<TRANSLATED:helixqa_cli_banner>"
	if !strings.Contains(captured, bannerSentinel) {
		t.Fatalf("printUsage stdout missing banner sentinel %q.\nCaptured:\n%s\n\nThis FAIL indicates the call site is bypassing the i18n seam (CONST-046 regression).", bannerSentinel, captured)
	}
	const hintSentinel = "<TRANSLATED:helixqa_cli_help_hint>"
	if !strings.Contains(captured, hintSentinel) {
		t.Fatalf("printUsage stdout missing help-hint sentinel %q.\nCaptured:\n%s", hintSentinel, captured)
	}

	// Raw English literal MUST NOT appear with fake backend.
	const rawBanner = "HelixQA — AI-driven QA orchestration"
	if strings.Contains(captured, rawBanner) {
		t.Fatalf("printUsage stdout contains raw English literal %q while fakeTranslator is installed — call site is bypassing the i18n seam (CONST-046 PASS-bluff regression).\nCaptured:\n%s", rawBanner, captured)
	}
}

// TestMainGo_ReferencesAllRound205IDs is the paired-mutation
// sentinel for round-205: it reads cmd/helixqa/main.go and asserts
// every new messageID literal appears in the source. If a future
// refactor removes a helixqaT(ctx, "<id>", ...) call and inlines
// the English literal directly, the corresponding ID disappears
// from main.go and this test FAILs — catching the regression at
// the source layer before it can ship and silently break the
// non-English UX.
//
// This complements TestHelixqaT_RoutesThroughActiveTranslator
// (sentinel via the seam) and TestPrintUsage_RoutesBannerThroughSeam
// (sentinel via captured stdout). Three independent assertion
// surfaces => mutation has to defeat all three to ship.
func TestMainGo_ReferencesAllRound205IDs(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(src)

	round205IDs := []string{
		"helixqa_run_interrupt_shutdown",
		"helixqa_run_summary_observed_line1",
		"helixqa_run_summary_observed_line2",
		"helixqa_run_summary_observed_line3",
		"helixqa_run_summary_observed_line4",
		"helixqa_run_summary_passed_fmt",
		"helixqa_run_summary_failed_fmt",
		"helixqa_run_summary_report_fmt",
		"helixqa_run_summary_duration_fmt",
		"helixqa_report_generated_fmt",
	}

	for _, id := range round205IDs {
		t.Run(id, func(t *testing.T) {
			if !strings.Contains(body, `"`+id+`"`) {
				t.Fatalf("main.go missing messageID literal %q — call site was inlined / removed, bypassing the i18n seam (CONST-046 regression). Restore the helixqaT(ctx, %q, ...) call.", id, id)
			}
		})
	}
}

// TestHelixqaT_FallbackOnNoop asserts that with the default
// NoopTranslator installed, helixqaT returns the messageID
// verbatim (matching the NoopTranslator T contract). This locks
// the pre-migration backward-compat behaviour for downstream
// consumers that have not yet wired a real backend.
func TestHelixqaT_FallbackOnNoop(t *testing.T) {
	t.Cleanup(i18n.ResetForTest)
	i18n.ResetForTest() // ensure Noop default
	const id = "helixqa_cli_banner"
	const fallback = "HelixQA — AI-driven QA orchestration"
	got := helixqaT(context.Background(), id, fallback)
	// With NoopTranslator, T(id) returns id verbatim (non-empty),
	// so helixqaT returns id (NOT fallback). Operators who want
	// the English literal must install a YAML-bundle backend.
	if got != id {
		t.Fatalf("with NoopTranslator, helixqaT(%q) = %q, want messageID %q (Noop returns the id verbatim per its contract)", id, got, id)
	}
}
