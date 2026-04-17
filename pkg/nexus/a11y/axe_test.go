package a11y

import (
	"strings"
	"testing"
)

const sampleAxeReport = `{
  "violations": [
    {"id":"color-contrast","impact":"serious","description":"Insufficient contrast","help":"Fix","helpUrl":"https://example.com","tags":["wcag2aa"], "nodes": [{"target":["#submit"], "html":"<button/>", "impact": "serious"}]},
    {"id":"aria-required","impact":"critical","description":"Missing aria","help":"Fix","helpUrl":"https://example.com","tags":["wcag2a","section508"], "nodes": []},
    {"id":"lang-missing","impact":"moderate","description":"No lang","help":"Fix","helpUrl":"https://example.com","tags":["wcag2a"], "nodes": []},
    {"id":"typography","impact":"minor","description":"Line height","help":"Fix","helpUrl":"https://example.com","tags":["wcag2aaa"], "nodes": []}
  ],
  "passes": ["button-name"],
  "incomplete": ["table-header"]
}`

func TestParse_Empty(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("empty input must error")
	}
}

func TestParse_ValidReport(t *testing.T) {
	r, err := Parse([]byte(sampleAxeReport))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Violations) != 4 {
		t.Errorf("violations = %d, want 4", len(r.Violations))
	}
	if len(r.Passes) != 1 {
		t.Errorf("passes = %d, want 1", len(r.Passes))
	}
}

func TestReport_Assert_LevelA(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	err := r.Assert(LevelA)
	if err == nil {
		t.Fatal("critical violation should breach level A")
	}
	if !strings.Contains(err.Error(), "aria-required") {
		t.Errorf("error should name aria-required, got %q", err.Error())
	}
}

func TestReport_Assert_LevelAA_FlagsSerious(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	err := r.Assert(LevelAA)
	if err == nil {
		t.Fatal("serious violation should breach level AA")
	}
	if !strings.Contains(err.Error(), "color-contrast") {
		t.Errorf("error should name color-contrast, got %q", err.Error())
	}
}

func TestReport_Assert_LevelAAA_FlagsModerate(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	err := r.Assert(LevelAAA)
	if err == nil {
		t.Fatal("moderate violation should breach level AAA")
	}
	if !strings.Contains(err.Error(), "lang-missing") {
		t.Errorf("error should name lang-missing, got %q", err.Error())
	}
}

func TestReport_Assert_UnknownLevel(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	if err := r.Assert("Z"); err == nil {
		t.Fatal("unknown level should error")
	}
}

func TestReport_Assert_CleanReport(t *testing.T) {
	r := &Report{}
	if err := r.Assert(LevelAAA); err != nil {
		t.Fatalf("empty report should pass at any level, got %v", err)
	}
}

func TestReport_Assert_NilReceiver(t *testing.T) {
	var r *Report
	if err := r.Assert(LevelAA); err == nil {
		t.Fatal("nil report must error")
	}
}

func TestReport_Section508Filter(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	s := r.Section508()
	if len(s) != 1 || s[0].ID != "aria-required" {
		t.Errorf("Section508 filter = %+v", s)
	}
}

func TestReport_Summary(t *testing.T) {
	r, _ := Parse([]byte(sampleAxeReport))
	s := r.Summary()
	if s[ImpactCritical] != 1 || s[ImpactSerious] != 1 || s[ImpactModerate] != 1 || s[ImpactMinor] != 1 {
		t.Errorf("Summary = %+v", s)
	}
}

func TestInjectionScript_IncludesVendoredPath(t *testing.T) {
	s := InjectionScript("https://hq.internal/static")
	if !strings.Contains(s, `"https://hq.internal/static/axe.min.js"`) {
		t.Errorf("script missing vendored URL: %q", s)
	}
	if !strings.Contains(s, "axe.run") {
		t.Error("script must call axe.run")
	}
}
