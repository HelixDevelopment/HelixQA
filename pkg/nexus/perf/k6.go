package perf

import (
	"fmt"
	"strings"
)

// Scenario describes the user-flow the generated k6 script walks.
type Scenario struct {
	URL           string
	WaitSelector  string
	ClickSelector string
	VUs           int
	Iterations    int
	Duration      string // "30s", "2m" etc.
}

// GenerateScript returns a self-contained k6 browser script matching
// scenario. The script enforces Core Web Vitals thresholds inline so
// k6's own `--threshold` reports align with Metrics.Assert().
func GenerateScript(s Scenario) (string, error) {
	if s.URL == "" {
		return "", fmt.Errorf("perf: Scenario.URL is required")
	}
	if s.VUs <= 0 {
		s.VUs = 1
	}
	if s.Iterations <= 0 {
		s.Iterations = 1
	}
	if s.Duration == "" {
		s.Duration = "30s"
	}

	var b strings.Builder
	b.WriteString("import { browser } from 'k6/experimental/browser';\n\n")
	b.WriteString("export const options = {\n")
	b.WriteString("  scenarios: {\n")
	b.WriteString("    browser: {\n")
	b.WriteString("      executor: 'shared-iterations',\n")
	fmt.Fprintf(&b, "      vus: %d,\n", s.VUs)
	fmt.Fprintf(&b, "      iterations: %d,\n", s.Iterations)
	b.WriteString("      options: { browser: { type: 'chromium' } },\n")
	b.WriteString("    },\n  },\n")
	b.WriteString("  thresholds: {\n")
	b.WriteString("    'browser_web_vital_lcp': ['p(95) < 2500'],\n")
	b.WriteString("    'browser_web_vital_inp': ['p(95) < 200'],\n")
	b.WriteString("    'browser_web_vital_cls': ['p(95) < 0.1'],\n")
	b.WriteString("  },\n};\n\n")
	b.WriteString("export default async function () {\n")
	b.WriteString("  const page = browser.newPage();\n")
	b.WriteString("  try {\n")
	fmt.Fprintf(&b, "    await page.goto(%q);\n", s.URL)
	if s.WaitSelector != "" {
		fmt.Fprintf(&b, "    await page.waitForSelector(%q);\n", s.WaitSelector)
	}
	if s.ClickSelector != "" {
		fmt.Fprintf(&b, "    await page.locator(%q).click();\n", s.ClickSelector)
		b.WriteString("    await page.waitForLoadState('networkidle');\n")
	}
	b.WriteString("  } finally {\n    await page.close();\n  }\n}\n")
	return b.String(), nil
}
