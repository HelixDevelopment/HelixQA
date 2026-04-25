// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func tinySolid(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// failWriter returns the given error on every Write — drives the
// io.Writer-error branch in Render.
type failWriter struct{ err error }

func (f failWriter) Write(p []byte) (int, error) { return 0, f.err }

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestReport_Render_HappyPath(t *testing.T) {
	before := tinySolid(color.RGBA{0, 0, 0, 255})
	after := tinySolid(color.RGBA{0, 0, 0, 255})
	diff, err := PixelMatch{}.Diff(before, after, DiffOptions{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	report := Report{
		Title: "Test Report",
		Sessions: []Session{
			{Name: "Login screen", Before: before, After: after, Diff: &diff, Note: "identical inputs"},
		},
	}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	// Core markers.
	for _, marker := range []string{
		"<!doctype html>",
		"<title>Test Report</title>",
		"<h1>Test Report</h1>",
		"Login screen",
		"identical inputs",
		"data:image/png;base64,",
		`class="session match"`,
		"DiffCount",
		"TotalPixels",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("output missing %q:\n%s", marker, truncateForLog(out, 500))
		}
	}
}

func TestReport_Render_DifferentSessionsMarkedCorrectly(t *testing.T) {
	match := tinySolid(color.RGBA{100, 100, 100, 255})
	differ := tinySolid(color.RGBA{200, 200, 200, 255})
	d1, _ := PixelMatch{}.Diff(match, match, DiffOptions{})
	d2, _ := PixelMatch{}.Diff(match, differ, DiffOptions{})

	report := Report{
		Sessions: []Session{
			{Name: "Same", Before: match, After: match, Diff: &d1},
			{Name: "Different", Before: match, After: differ, Diff: &d2},
		},
	}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `class="session match"`) {
		t.Error("same-session should be class=match")
	}
	if !strings.Contains(out, `class="session different"`) {
		t.Error("differ-session should be class=different")
	}
	// Summary totals reflect both.
	if !strings.Contains(out, "Matching: 1") || !strings.Contains(out, "Different: 1") {
		t.Error("summary totals wrong")
	}
}

func TestReport_Render_SessionWithoutDiffIsUnknown(t *testing.T) {
	// A session with Before/After but no Diff renders with the
	// "unknown" verdict class (no comparison was run).
	img := tinySolid(color.RGBA{100, 100, 100, 255})
	report := Report{
		Sessions: []Session{{Name: "Raw", Before: img, After: img}},
	}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `class="session unknown"`) {
		t.Error("session without Diff should be class=unknown")
	}
	// Stats table is elided.
	if strings.Contains(out, "DiffCount") {
		t.Error("stats table should be elided when Diff is nil")
	}
}

func TestReport_Render_IncludesBrandReport(t *testing.T) {
	img := tinySolid(color.RGBA{100, 100, 100, 255})
	brand := BrandComplianceReport{
		TotalPixels: 100, InRange: 90, MaxDeltaE: 3.2, MeanDeltaE: 0.5,
	}
	report := Report{
		Sessions: []Session{
			{Name: "Brand", Before: img, After: img, BrandReport: &brand},
		},
	}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	for _, marker := range []string{
		"Brand Compliance (CIEDE2000)",
		"Pass Rate",
		"Mean ΔE",
		"Max ΔE",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("output missing %q", marker)
		}
	}
}

func TestReport_Render_DefaultTitleAndTimestamp(t *testing.T) {
	img := tinySolid(color.RGBA{0, 0, 0, 255})
	report := Report{
		Sessions: []Session{{Name: "x", Before: img, After: img}},
	}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "HelixQA Visual Regression Report") {
		t.Error("default Title not applied")
	}
	// GeneratedAt defaults to time.Now — no exact match possible,
	// but the RFC3339 year marker must be present.
	if !strings.Contains(out, time.Now().Format("2006")) {
		t.Error("default GeneratedAt missing")
	}
}

func TestReport_Render_SortsSummaryByDiffPercentDescending(t *testing.T) {
	a := tinySolid(color.RGBA{0, 0, 0, 255})
	b := tinySolid(color.RGBA{255, 255, 255, 255})
	small, _ := PixelMatch{}.Diff(a, a, DiffOptions{})
	big, _ := PixelMatch{}.Diff(a, b, DiffOptions{})

	report := Report{
		Sessions: []Session{
			{Name: "alpha-small-diff", Before: a, After: a, Diff: &small},
			{Name: "zulu-big-diff", Before: a, After: b, Diff: &big},
		},
	}
	var buf bytes.Buffer
	_ = (Reporter{}).Render(context.Background(), &buf, report)
	out := buf.String()

	// Summary table: zulu (100%) should appear before alpha (0%) in
	// the sorted-by-diff-percent-descending order.
	zuluIdx := strings.Index(out, "zulu-big-diff")
	alphaIdx := strings.Index(out, "alpha-small-diff")
	// Both appear multiple times (summary table + detail section);
	// the summary table comes before the detail sections.
	if zuluIdx < 0 || alphaIdx < 0 {
		t.Fatalf("names missing from output")
	}
	if zuluIdx > alphaIdx {
		t.Error("summary should list higher-diff session first")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestReport_Render_EmptySessionsError(t *testing.T) {
	report := Report{Sessions: nil}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); !errors.Is(err, ErrNoSessions) {
		t.Fatalf("empty: %v, want ErrNoSessions", err)
	}
}

func TestReport_Render_MissingBeforeError(t *testing.T) {
	after := tinySolid(color.RGBA{0, 0, 0, 255})
	report := Report{Sessions: []Session{{Name: "missing-before", Before: nil, After: after}}}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); !errors.Is(err, ErrMissingImage) {
		t.Fatalf("missing Before: %v, want ErrMissingImage", err)
	}
}

func TestReport_Render_MissingAfterError(t *testing.T) {
	before := tinySolid(color.RGBA{0, 0, 0, 255})
	report := Report{Sessions: []Session{{Name: "missing-after", Before: before, After: nil}}}
	var buf bytes.Buffer
	if err := (Reporter{}).Render(context.Background(), &buf, report); !errors.Is(err, ErrMissingImage) {
		t.Fatalf("missing After: %v, want ErrMissingImage", err)
	}
}

func TestReport_Render_ContextCanceled(t *testing.T) {
	img := tinySolid(color.RGBA{0, 0, 0, 255})
	report := Report{Sessions: []Session{{Name: "x", Before: img, After: img}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	if err := (Reporter{}).Render(ctx, &buf, report); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestReport_Render_WriterError(t *testing.T) {
	img := tinySolid(color.RGBA{0, 0, 0, 255})
	report := Report{Sessions: []Session{{Name: "x", Before: img, After: img}}}
	err := (Reporter{}).Render(context.Background(), failWriter{err: io.ErrShortWrite}, report)
	if err == nil {
		t.Fatal("writer error should propagate")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestEncodePNGDataURL_HasCorrectPrefix(t *testing.T) {
	img := tinySolid(color.RGBA{100, 100, 100, 255})
	u, err := encodePNGDataURL(img)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Fatalf("prefix wrong: %q", u[:40])
	}
}

func TestVerdictForDiff(t *testing.T) {
	if verdictForDiff(DiffReport{DiffCount: 0}) != "match" {
		t.Error("zero diffs → match")
	}
	if verdictForDiff(DiffReport{DiffCount: 10}) != "different" {
		t.Error("any diffs → different")
	}
}

func TestSummarize_EmptySessionsHasZeroes(t *testing.T) {
	s := summarize(nil)
	if s.Total != 0 || s.MatchCount != 0 || s.DifferCount != 0 {
		t.Fatalf("empty summary = %+v", s)
	}
	if len(s.SortedByDiff) != 0 {
		t.Fatalf("empty SortedByDiff = %+v", s.SortedByDiff)
	}
}

// truncateForLog keeps error messages bounded in test output.
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
