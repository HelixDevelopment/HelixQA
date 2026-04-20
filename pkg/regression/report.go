// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io"
	"sort"
	"time"
)

// Session is a single before/after comparison inside a Report.
type Session struct {
	// Name is the human-visible identifier shown in the report
	// (e.g. "Login screen — light mode vs dark mode").
	Name string

	// Before / After are the two images being compared. Required;
	// nil is rejected by Report.Validate.
	Before image.Image
	After  image.Image

	// Diff — pixelmatch output for Before vs After. Optional; when
	// present the renderer embeds the Output image and the stats
	// table. Generate via PixelMatch{}.Diff.
	Diff *DiffReport

	// BrandReport — optional CIEDE2000 compliance stats. Generate
	// via CheckBrandCompliance.
	BrandReport *BrandComplianceReport

	// Note is an optional single-line annotation shown below the
	// session header.
	Note string
}

// Report is the top-level HTML reporter input.
type Report struct {
	// Title shown in <title> and <h1>. Zero → "HelixQA Visual
	// Regression Report".
	Title string

	// GeneratedAt is the report's timestamp. Zero → time.Now at
	// Render time.
	GeneratedAt time.Time

	// Sessions are rendered in the order supplied.
	Sessions []Session
}

// Reporter renders a Report as a single self-contained HTML
// document. All images are embedded as base64 PNG data URLs; the
// output is safe to ship as a standalone file and email, archive, or
// open from any browser with zero external resource fetches.
type Reporter struct{}

// Sentinel errors.
var (
	ErrNoSessions  = errors.New("helixqa/regression: Report has no sessions")
	ErrMissingImage = errors.New("helixqa/regression: session is missing Before or After")
)

// Render writes the HTML report to w. Respects ctx cancellation
// (cheap check; rendering is in-memory and fast even for dozens of
// 1080p sessions).
func (Reporter) Render(ctx context.Context, w io.Writer, r Report) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(r.Sessions) == 0 {
		return ErrNoSessions
	}
	for i, s := range r.Sessions {
		if s.Before == nil || s.After == nil {
			return fmt.Errorf("%w: session %d (%q)", ErrMissingImage, i, s.Name)
		}
	}

	// Encode images up-front so template rendering is cheap + any
	// encoding errors surface before we write anything.
	encoded := make([]encodedSession, 0, len(r.Sessions))
	for i, s := range r.Sessions {
		if err := ctx.Err(); err != nil {
			return err
		}
		es, err := encodeSession(i, s)
		if err != nil {
			return fmt.Errorf("session %d (%q): %w", i, s.Name, err)
		}
		encoded = append(encoded, es)
	}

	title := r.Title
	if title == "" {
		title = "HelixQA Visual Regression Report"
	}
	ts := r.GeneratedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	data := reportData{
		Title:       title,
		GeneratedAt: ts.Format(time.RFC3339),
		Summary:     summarize(r.Sessions),
		Sessions:    encoded,
	}

	return reportTemplate.Execute(w, data)
}

// encodedSession is the per-session template input. All images are
// already base64-encoded so the template does no work. BeforeDataURL
// / AfterDataURL / DiffDataURL use template.URL to bypass html/
// template's default data-URL sanitization (it replaces unknown
// URL schemes with #ZotmplZ; data:image/png;base64,... is safe to
// inline for our PNG payloads since the encoding is controlled by
// encodePNGDataURL).
type encodedSession struct {
	Index         int
	Name          string
	Note          string
	BeforeDataURL template.URL
	AfterDataURL  template.URL
	DiffDataURL   template.URL // empty if Diff is nil
	Diff          *DiffReport
	Brand         *BrandComplianceReport
	Verdict       string // "match" | "different" | "unknown"
	DiffPercent   float64
}

// reportSummary aggregates headline numbers across sessions.
type reportSummary struct {
	Total        int
	MatchCount   int
	DifferCount  int
	SortedByDiff []sessionSummaryRow
}

type sessionSummaryRow struct {
	Name        string
	DiffPercent float64
	Verdict     string
}

// reportData is the top-level template input.
type reportData struct {
	Title       string
	GeneratedAt string
	Summary     reportSummary
	Sessions    []encodedSession
}

// encodeSession converts a Session into its rendered form by PNG-
// encoding the three images and stamping the verdict based on the
// Diff.
func encodeSession(idx int, s Session) (encodedSession, error) {
	before, err := encodePNGDataURL(s.Before)
	if err != nil {
		return encodedSession{}, fmt.Errorf("Before: %w", err)
	}
	after, err := encodePNGDataURL(s.After)
	if err != nil {
		return encodedSession{}, fmt.Errorf("After: %w", err)
	}
	es := encodedSession{
		Index:         idx,
		Name:          s.Name,
		Note:          s.Note,
		BeforeDataURL: template.URL(before),
		AfterDataURL:  template.URL(after),
		Diff:          s.Diff,
		Brand:         s.BrandReport,
	}
	if s.Diff != nil {
		if s.Diff.Output != nil {
			diff, err := encodePNGDataURL(s.Diff.Output)
			if err != nil {
				return encodedSession{}, fmt.Errorf("Diff: %w", err)
			}
			es.DiffDataURL = template.URL(diff)
		}
		es.Verdict = verdictForDiff(*s.Diff)
		if s.Diff.TotalPixels > 0 {
			es.DiffPercent = 100 * float64(s.Diff.DiffCount) / float64(s.Diff.TotalPixels)
		}
	} else {
		es.Verdict = "unknown"
	}
	return es, nil
}

// verdictForDiff classifies a diff into match/different.
func verdictForDiff(d DiffReport) string {
	if d.DiffCount == 0 {
		return "match"
	}
	return "different"
}

// summarize aggregates session-level stats for the summary table.
func summarize(sessions []Session) reportSummary {
	sum := reportSummary{Total: len(sessions)}
	rows := make([]sessionSummaryRow, 0, len(sessions))
	for _, s := range sessions {
		row := sessionSummaryRow{Name: s.Name, Verdict: "unknown"}
		if s.Diff != nil {
			row.Verdict = verdictForDiff(*s.Diff)
			if s.Diff.TotalPixels > 0 {
				row.DiffPercent = 100 * float64(s.Diff.DiffCount) / float64(s.Diff.TotalPixels)
			}
			if row.Verdict == "match" {
				sum.MatchCount++
			} else {
				sum.DifferCount++
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].DiffPercent > rows[j].DiffPercent
	})
	sum.SortedByDiff = rows
	return sum
}

// encodePNGDataURL PNG-encodes img and wraps the base64 bytes in a
// data:image/png URL — the canonical inline-image wire format.
func encodePNGDataURL(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// reportFuncs exposes helpers to the template.
var reportFuncs = template.FuncMap{
	"mulFloat": func(a, b float64) float64 { return a * b },
}

// reportTemplate is the single all-inline HTML template. Zero
// external assets — every image is a base64 data URL, every style
// is <style> inside <head>.
var reportTemplate = template.Must(
	template.New("report").Funcs(reportFuncs).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Title}}</title>
  <style>
    body { font-family: -apple-system, Segoe UI, Roboto, sans-serif; margin: 24px; color: #222; }
    h1 { border-bottom: 2px solid #0af; padding-bottom: 8px; }
    .meta { color: #666; font-size: 14px; }
    table.summary { border-collapse: collapse; margin: 12px 0; }
    table.summary th, table.summary td { border: 1px solid #ccc; padding: 6px 10px; }
    table.summary th { background: #f4f6f8; text-align: left; }
    tr.match td:last-child { color: #080; font-weight: bold; }
    tr.different td:last-child { color: #c40; font-weight: bold; }
    tr.unknown td:last-child { color: #888; }
    section.session { border: 1px solid #ddd; border-radius: 6px; padding: 16px; margin: 20px 0; }
    section.session.match { border-left: 6px solid #080; }
    section.session.different { border-left: 6px solid #c40; }
    section.session.unknown { border-left: 6px solid #888; }
    .images { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 12px; }
    .image-col { flex: 1 1 300px; min-width: 260px; }
    .image-col h4 { margin: 0 0 4px 0; font-size: 13px; color: #444; }
    .image-col img { max-width: 100%; border: 1px solid #bbb; display: block; }
    table.stats { border-collapse: collapse; margin-top: 12px; font-size: 13px; }
    table.stats td { border: 1px solid #ddd; padding: 4px 10px; }
    table.stats td:first-child { background: #fafafa; font-weight: bold; }
  </style>
</head>
<body>
  <h1>{{.Title}}</h1>
  <p class="meta">Generated at <time>{{.GeneratedAt}}</time> · Sessions: {{.Summary.Total}} · Matching: {{.Summary.MatchCount}} · Different: {{.Summary.DifferCount}}</p>

  <h2>Summary</h2>
  <table class="summary">
    <thead><tr><th>Session</th><th>Diff</th><th>Verdict</th></tr></thead>
    <tbody>
    {{range .Summary.SortedByDiff}}
      <tr class="{{.Verdict}}"><td>{{.Name}}</td><td>{{printf "%.3f" .DiffPercent}}%</td><td>{{.Verdict}}</td></tr>
    {{end}}
    </tbody>
  </table>

  {{range .Sessions}}
  <section class="session {{.Verdict}}">
    <h3>#{{.Index}} — {{.Name}}</h3>
    {{if .Note}}<p class="note">{{.Note}}</p>{{end}}
    <div class="images">
      <div class="image-col">
        <h4>Before</h4>
        <img src="{{.BeforeDataURL}}" alt="before">
      </div>
      <div class="image-col">
        <h4>After</h4>
        <img src="{{.AfterDataURL}}" alt="after">
      </div>
      {{if .DiffDataURL}}
      <div class="image-col">
        <h4>Diff</h4>
        <img src="{{.DiffDataURL}}" alt="diff">
      </div>
      {{end}}
    </div>
    {{if .Diff}}
    <table class="stats">
      <tr><td>Verdict</td><td>{{.Verdict}}</td></tr>
      <tr><td>DiffCount</td><td>{{.Diff.DiffCount}}</td></tr>
      <tr><td>AACount</td><td>{{.Diff.AACount}}</td></tr>
      <tr><td>TotalPixels</td><td>{{.Diff.TotalPixels}}</td></tr>
      <tr><td>Diff %</td><td>{{printf "%.3f" .DiffPercent}}</td></tr>
    </table>
    {{end}}
    {{if .Brand}}
    <h4 style="margin-top:12px">Brand Compliance (CIEDE2000)</h4>
    <table class="stats">
      <tr><td>Total Pixels</td><td>{{.Brand.TotalPixels}}</td></tr>
      <tr><td>In-range Pixels</td><td>{{.Brand.InRange}}</td></tr>
      <tr><td>Pass Rate</td><td>{{printf "%.2f" (mulFloat .Brand.PassRate 100)}}%</td></tr>
      <tr><td>Mean ΔE</td><td>{{printf "%.3f" .Brand.MeanDeltaE}}</td></tr>
      <tr><td>Max ΔE</td><td>{{printf "%.3f" .Brand.MaxDeltaE}}</td></tr>
    </table>
    {{end}}
  </section>
  {{end}}
</body>
</html>
`))
