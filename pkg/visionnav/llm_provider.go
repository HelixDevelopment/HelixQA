// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// llm_provider.go — Issues.md C6 (§GT.3) wiring: the real LLM-driven
// vision-nav Provider.
//
// provider.go defines the Provider interface and a deterministic
// NopProvider for offline plumbing tests. THIS file supplies the
// concrete LLM-backed Provider that turns a screen capture + the
// exploration goal into a Decision by calling a vision-capable LLM
// backend (pkg/llm.Provider.Vision). It is the seam §GT.3 calls for:
// "implement pkg/visionnav.Explorer (LLM vision-nav driver)".
//
// Decoupling (project-agnostic, §11.4.27 / HelixQA-must-stay-generic):
// this file imports ONLY pkg/llm (a generic LLM abstraction) — no
// project package names, no device serials, no region endpoints, no
// pkg/navigator import. The LLM backend is injected by the caller; the
// prompt is built from caller-supplied, runtime-registered Target data
// (goals), never from a hardcoded app list.
//
// Anti-bluff (§11.4.6 / §11.4): the LLM's reply is parsed into a
// Decision and run through Decision.Validate() before it can drive a
// step — a reply with no action or no rationale is rejected as a
// bluff-by-construction, never silently turned into a no-op.

package visionnav

import (
	"context"
	"fmt"
	"os"
	"strings"

	"digital.vasic.helixqa/pkg/llm"
)

// VisionDecider is the minimal slice of pkg/llm.Provider that the
// LLM-backed visionnav Provider needs. Defining it HERE (consumer side)
// keeps llm_provider.go honest about its dependency surface and lets
// tests supply a fake without constructing a full LLM backend. Any
// pkg/llm.Provider satisfies it structurally.
type VisionDecider interface {
	// Vision sends a screenshot (raw bytes) plus a text prompt and
	// returns the assistant reply.
	Vision(ctx context.Context, image []byte, prompt string) (*llm.Response, error)
	// SupportsVision reports whether this backend can process images.
	SupportsVision() bool
	// Name is the backend identifier (e.g. "anthropic", "ollama").
	Name() string
}

// LLMProvider implements visionnav.Provider by asking a vision-capable
// LLM backend what to do next at each step. It is the concrete,
// non-deterministic counterpart to NopProvider.
type LLMProvider struct {
	backend VisionDecider
	// goals is the caller-supplied exploration goal text injected into
	// the prompt so the model knows what "done" looks like. Runtime
	// data — NOT a hardcoded literal in this package.
	goals []string
	// systemHint is optional extra caller context (e.g. the action
	// grammar the executor understands). Project-agnostic free text.
	systemHint string
}

// NewLLMProvider wires an LLM backend into a visionnav.Provider.
//
// backend MUST support vision (the loop hands it screenshots). goals is
// the caller's runtime-registered goal text — typically Target.ScreenGoals
// — so the model can reason about progress; at least one non-empty goal
// is required (a Provider with no goal can only guess). systemHint is
// optional caller context describing the executor's action grammar.
func NewLLMProvider(backend VisionDecider, goals []string, systemHint string) (*LLMProvider, error) {
	if backend == nil {
		return nil, fmt.Errorf("visionnav: NewLLMProvider: backend is nil")
	}
	if !backend.SupportsVision() {
		return nil, fmt.Errorf("visionnav: NewLLMProvider: backend %q does not support vision "+
			"(the vision-nav loop hands the model screenshots)", backend.Name())
	}
	cleaned := make([]string, 0, len(goals))
	for _, g := range goals {
		if strings.TrimSpace(g) != "" {
			cleaned = append(cleaned, g)
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("visionnav: NewLLMProvider: at least one non-empty goal is required " +
			"(without a goal the model can only guess — that is the bluff §11.4 forbids)")
	}
	return &LLMProvider{backend: backend, goals: cleaned, systemHint: systemHint}, nil
}

// Name returns "llm:<backend>" so logs distinguish which backend drove a run.
func (p *LLMProvider) Name() string {
	return "llm:" + p.backend.Name()
}

// Decide reads the most recent screenshot (preferring the in-memory
// Observation.LastImageBytes the Session captures, falling back to
// reading Observation.LastImagePath), builds a goal-aware prompt, asks
// the backend, and parses the reply into a validated Decision.
//
// When no screenshot is available yet (neither bytes nor path — e.g.
// before the first frame), Decide returns a deterministic launch
// Decision: the Session dispatches Target.LaunchAction on step 1
// regardless of the action field, so the action here is advisory, but
// the Rationale is real (it documents that no observation was available
// yet).
func (p *LLMProvider) Decide(ctx context.Context, obs Observation) (*Decision, error) {
	img, err := observationImage(obs)
	if err != nil {
		return nil, fmt.Errorf("visionnav: LLMProvider: %w", err)
	}
	if len(img) == 0 {
		d := &Decision{
			Action:          "launch",
			Rationale:       "step has no prior screenshot to observe; deferring to the target launch action",
			ExpectedVerdict: "",
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		return d, nil
	}

	resp, err := p.backend.Vision(ctx, img, p.buildPrompt(obs))
	if err != nil {
		return nil, fmt.Errorf("visionnav: LLMProvider: backend %q vision call: %w", p.backend.Name(), err)
	}
	if resp == nil || !resp.HasContent() {
		return nil, fmt.Errorf("visionnav: LLMProvider: backend %q returned empty reply", p.backend.Name())
	}

	d, err := parseDecision(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("visionnav: LLMProvider: %w", err)
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

// observationImage resolves the screen frame for an Observation,
// preferring the in-memory bytes (the Session's capture form, no disk
// round-trip) and falling back to reading LastImagePath. Returns nil
// bytes (not an error) when neither is provided — Decide treats that as
// "no screenshot yet".
func observationImage(obs Observation) ([]byte, error) {
	if len(obs.LastImageBytes) > 0 {
		return obs.LastImageBytes, nil
	}
	if obs.LastImagePath == "" {
		return nil, nil
	}
	img, err := os.ReadFile(obs.LastImagePath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot %q: %w", obs.LastImagePath, err)
	}
	if len(img) == 0 {
		return nil, fmt.Errorf("screenshot %q is empty", obs.LastImagePath)
	}
	return img, nil
}

// buildPrompt assembles the per-step instruction. It embeds the
// caller's runtime goal text (never a hardcoded app/region literal) and
// asks the model to reply in a strict, machine-parseable line format so
// parseDecision can extract the Decision without an LLM-specific SDK.
func (p *LLMProvider) buildPrompt(obs Observation) string {
	var b strings.Builder
	b.WriteString("You are driving an automated UI exploration. ")
	b.WriteString("Look at the attached screenshot and decide the single next action ")
	b.WriteString("that makes progress toward the goal.\n\n")
	b.WriteString("Goal — reach a screen that shows any of:\n")
	for _, g := range p.goals {
		b.WriteString("  - ")
		b.WriteString(g)
		b.WriteString("\n")
	}
	if strings.TrimSpace(p.systemHint) != "" {
		b.WriteString("\nAvailable actions / grammar:\n")
		b.WriteString(p.systemHint)
		b.WriteString("\n")
	}
	if obs.LastEvidence != nil {
		b.WriteString("\nPrevious step note: ")
		b.WriteString(obs.LastEvidence.Description)
		b.WriteString("\n")
	}
	b.WriteString("\nReply EXACTLY in this format, one field per line:\n")
	b.WriteString("ACTION: <one action in the grammar above>\n")
	b.WriteString("RATIONALE: <one sentence: why this action makes progress>\n")
	b.WriteString("EXPECT: <pass|fail|needs-review or leave blank if unsure>\n")
	return b.String()
}

// parseDecision turns the strict ACTION/RATIONALE/EXPECT reply into a
// Decision. It is tolerant of surrounding prose and casing but requires
// the ACTION and RATIONALE fields to be present and non-empty — a reply
// missing either is rejected (caught later by Decision.Validate too, but
// rejecting here gives a precise error).
func parseDecision(content string) (*Decision, error) {
	var action, rationale, expect string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case hasPrefixFold(line, "ACTION:"):
			action = strings.TrimSpace(line[len("ACTION:"):])
		case hasPrefixFold(line, "RATIONALE:"):
			rationale = strings.TrimSpace(line[len("RATIONALE:"):])
		case hasPrefixFold(line, "EXPECT:"):
			expect = strings.ToLower(strings.TrimSpace(line[len("EXPECT:"):]))
		}
	}
	if action == "" {
		return nil, fmt.Errorf("LLM reply had no ACTION line")
	}
	if rationale == "" {
		return nil, fmt.Errorf("LLM reply had no RATIONALE line (a decision without a reason is a bluff)")
	}
	switch expect {
	case "", "pass", "fail", "needs-review":
		// ok
	default:
		// Unknown expectation — drop it rather than fail the whole step;
		// Decision treats "" as "model honestly does not know".
		expect = ""
	}
	return &Decision{Action: action, Rationale: rationale, ExpectedVerdict: expect}, nil
}

// hasPrefixFold reports whether s starts with prefix, case-insensitively
// on the ASCII letters of prefix. Kept local + tiny to avoid pulling in
// extra imports and to keep the field-tag matching obvious.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}
