// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"digital.vasic.challenges/pkg/userflow"
)

// PlaywrightExecutor wraps userflow.PlaywrightCLIAdapter for use
// from the StructuredTestExecutor's performAction switch. It is
// the structured-bank-step counterpart of HTTPExecutor: a thin
// dispatcher that parses an ActionTypePlaywright step's value
// ("verb selector|url[ value]") and delegates to the underlying
// CDP-driven adapter.
//
// Added 2026-04-29 to close the runtime gap that caused
// ActionTypePlaywright steps to SKIP with
// PLAYWRIGHT-RUNTIME-PENDING. With a Playwright container running
// (CDP exposed via ws://host:9222) and PipelineConfig.PlaywrightCDPURL
// set, ActionTypePlaywright bank steps now execute against a real
// browser instead of skipping.
//
// Construction is lazy and gated on configuration: when no CDP URL
// is supplied, ActionTypePlaywright steps continue to SKIP with the
// pending-runtime ticket (Article XI §11.2.2 — no silent PASS).
type PlaywrightExecutor struct {
	cdpURL  string
	adapter *userflow.PlaywrightCLIAdapter

	mu       sync.Mutex
	prepared bool
}

// NewPlaywrightExecutor builds a PlaywrightExecutor pointing at the
// given CDP URL. The underlying adapter is constructed eagerly but
// the browser context is only created on first action (Initialize
// is called inside ensurePrepared).
func NewPlaywrightExecutor(cdpURL string) *PlaywrightExecutor {
	return &PlaywrightExecutor{
		cdpURL:  cdpURL,
		adapter: userflow.NewPlaywrightCLIAdapter(cdpURL),
	}
}

// Execute parses a Playwright action value of the form
//
//	"<verb> <selector|url>[ <value>]"
//
// and dispatches to the matching adapter method. Recognized verbs:
//
//	navigate <url>            — page.goto
//	click    <selector>       — page.click
//	fill     <selector> <txt> — page.fill
//	type     <selector> <txt> — alias for fill
//	waitFor  <selector>       — page.waitForSelector
//	assertVisible    <selector> — page.isVisible (must be true)
//	assertNotVisible <selector> — page.isVisible (must be false)
//	press    <key>            — page.keyboard.press
//
// Returns ActionResult so the dispatch in performAction can use it
// the same way other executors do.
func (p *PlaywrightExecutor) Execute(
	ctx context.Context, value string,
) ActionResult {
	if p.adapter == nil {
		return ActionResult{Success: false, Message: "playwright: executor not initialized"}
	}
	verb, rest := splitVerb(value)
	if verb == "" {
		return ActionResult{Success: false, Message: "playwright: empty action value (expected '<verb> <selector|url>[ <value>]')"}
	}

	if err := p.ensurePrepared(ctx); err != nil {
		return ActionResult{Success: false, Message: fmt.Sprintf("playwright: initialize failed: %v", err)}
	}

	switch verb {
	case "navigate":
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright navigate: URL missing"}
		}
		if err := p.adapter.Navigate(ctx, rest); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright navigate %s: %v", rest, err)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("navigated to %s", rest)}

	case "click":
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright click: selector missing"}
		}
		if err := p.adapter.Click(ctx, rest); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright click %s: %v", rest, err)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("clicked %s", rest)}

	case "fill", "type":
		// rest == "<selector> <text>" — the <text> may contain spaces;
		// split only on the first run of whitespace.
		sel, text := splitSelectorValue(rest)
		if sel == "" {
			return ActionResult{Success: false, Message: "playwright fill: selector missing"}
		}
		if err := p.adapter.Fill(ctx, sel, text); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright fill %s: %v", sel, err)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("filled %s with %d chars", sel, len(text))}

	case "waitfor":
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright waitFor: selector missing"}
		}
		// 30 s default — bank steps that need different timeouts can use the
		// step's Timeout field; we honor that via the parent ctx upstream.
		if err := p.adapter.WaitForSelector(ctx, rest, 30*time.Second); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright waitFor %s: %v", rest, err)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("waited for %s", rest)}

	case "assertvisible":
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright assertVisible: selector missing"}
		}
		visible, err := p.adapter.IsVisible(ctx, rest)
		if err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright assertVisible %s: %v", rest, err)}
		}
		if !visible {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright assertVisible %s: NOT visible", rest)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("visible: %s", rest)}

	case "assertnotvisible":
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright assertNotVisible: selector missing"}
		}
		visible, err := p.adapter.IsVisible(ctx, rest)
		if err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright assertNotVisible %s: %v", rest, err)}
		}
		if visible {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright assertNotVisible %s: IS visible (expected hidden)", rest)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("not visible: %s", rest)}

	case "press":
		// Treat as keypress on the focused element via JS. The
		// adapter doesn't expose a dedicated keyboard API, so we
		// route through EvaluateJS with a tight script.
		if rest == "" {
			return ActionResult{Success: false, Message: "playwright press: key missing"}
		}
		js := fmt.Sprintf(`
const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.connectOverCDP('%s');
  const page = browser.contexts()[0].pages()[0];
  await page.keyboard.press('%s');
  console.log('pressed');
})();
`, p.cdpURL, rest)
		if _, err := p.adapter.EvaluateJS(ctx, js); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("playwright press %s: %v", rest, err)}
		}
		return ActionResult{Success: true, Message: fmt.Sprintf("pressed %s", rest)}

	default:
		return ActionResult{Success: false, Message: fmt.Sprintf("playwright: unknown verb %q (expected navigate/click/fill/waitFor/assertVisible/assertNotVisible/press)", verb)}
	}
}

// Close releases browser resources. Caller responsibility; the
// dispatcher creates one PlaywrightExecutor per session and
// reuses it across all bank steps.
func (p *PlaywrightExecutor) Close(ctx context.Context) error {
	if p.adapter == nil {
		return nil
	}
	return p.adapter.Close(ctx)
}

func (p *PlaywrightExecutor) ensurePrepared(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.prepared {
		return nil
	}
	cfg := userflow.BrowserConfig{
		BrowserType: "chromium",
		Headless:    true,
		WindowSize:  [2]int{1920, 1080},
	}
	if err := p.adapter.Initialize(ctx, cfg); err != nil {
		return err
	}
	p.prepared = true
	return nil
}

// splitVerb extracts the first whitespace-delimited word
// (lower-cased) and returns the remainder of the string.
func splitVerb(s string) (verb, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return strings.ToLower(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return strings.ToLower(s), ""
}

// splitSelectorValue handles the two-arg case for fill: "<selector> <value>".
// Selectors may contain spaces only inside CSS attribute brackets — for our
// converter output, selectors are simple (text=Foo, input[name=bar]) and
// don't contain unbracketed spaces, so a single split is safe.
func splitSelectorValue(s string) (selector, value string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}
