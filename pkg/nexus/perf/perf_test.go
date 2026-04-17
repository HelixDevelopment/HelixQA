package perf

import (
	"strings"
	"testing"
)

func TestAssert_PassesUnderThresholds(t *testing.T) {
	m := Metrics{LCP: 1000, INP: 100, CLS: 0.05, FCP: 800, TTFB: 200}
	if err := m.Assert(DefaultThresholds()); err != nil {
		t.Fatal(err)
	}
}

func TestAssert_FailsOnBreach(t *testing.T) {
	m := Metrics{LCP: 3000, INP: 100, CLS: 0.05, FCP: 500, TTFB: 100}
	err := m.Assert(DefaultThresholds())
	if err == nil || !strings.Contains(err.Error(), "LCP") {
		t.Fatalf("expected LCP breach, got %v", err)
	}
}

func TestAssert_ZeroThresholdsIgnored(t *testing.T) {
	m := Metrics{LCP: 1_000_000}
	th := Thresholds{INPMax: 100}
	if err := m.Assert(th); err != nil {
		t.Errorf("zero threshold should skip LCP: %v", err)
	}
}

func TestParseK6JSON_ExtractsVitals(t *testing.T) {
	stream := `{"type":"Point","metric":"browser_web_vital_lcp","data":{"value":1450.0}}
{"type":"Point","metric":"browser_web_vital_cls","data":{"value":0.03}}
{"type":"Point","metric":"other","data":{"value":1}}
`
	m, err := ParseK6JSON([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if m.LCP != 1450 {
		t.Errorf("LCP = %f", m.LCP)
	}
	if m.CLS != 0.03 {
		t.Errorf("CLS = %f", m.CLS)
	}
}

func TestParseK6JSON_EmptyInput(t *testing.T) {
	if _, err := ParseK6JSON(nil); err == nil {
		t.Fatal("empty input should error")
	}
}

func TestGenerateScript_RejectsMissingURL(t *testing.T) {
	if _, err := GenerateScript(Scenario{}); err == nil {
		t.Fatal("missing URL must error")
	}
}

func TestGenerateScript_HappyPath(t *testing.T) {
	s, err := GenerateScript(Scenario{
		URL: "https://example.com", WaitSelector: "body", ClickSelector: "#go",
		VUs: 2, Iterations: 5, Duration: "1m",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `"https://example.com"`) {
		t.Error("URL missing")
	}
	if !strings.Contains(s, "page.waitForSelector(") {
		t.Error("wait selector missing")
	}
	if !strings.Contains(s, "page.locator(") {
		t.Error("locator missing")
	}
	if !strings.Contains(s, "browser_web_vital_lcp") {
		t.Error("thresholds missing")
	}
}
