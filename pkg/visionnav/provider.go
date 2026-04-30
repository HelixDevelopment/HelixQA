// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Phase 27.7 — Issues.md C6 stable LLM-adapter target.
//
// Defines the Provider interface that a future LLM-driven explorer
// will implement to decide what to test next. NopProvider provides a
// trivial implementation so the rest of pkg/visionnav can be tested
// end-to-end without a real LLM (deterministic, offline, fast).
//
// Constitution §11.4: Provider implementations MUST surface a
// non-empty Description in their Decision so any Evidence the
// explorer records based on it carries the §11.4-required
// captured discovery context. NopProvider satisfies this.

package visionnav

import (
	"context"
	"fmt"
)

// Decision is what a Provider returns to the explorer at each step:
// "here's what to do next, and here's why I think so".
type Decision struct {
	// Action describes the proposed next test action in the
	// explorer's grammar (e.g. "tap_back_button", "open_settings",
	// "play_video file=/sdcard/test.mp4"). Free-form for now.
	// Required.
	Action string
	// Rationale is the LLM's natural-language reason for choosing
	// this action. Carried into Evidence.Notes when the action
	// produces a finding so reviewers see the LLM's intent.
	Rationale string
	// ExpectedVerdict is what the LLM expects the action to
	// produce. Used by the explorer to detect "the LLM said this
	// would pass but it failed" (a high-signal divergence worth
	// flagging for human review).
	ExpectedVerdict string
}

// Validate returns an error if the Decision lacks the captured-
// evidence fields a downstream Evidence record will need. This is
// the §11.4 enforcement at the Provider interface boundary.
func (d *Decision) Validate() error {
	if d == nil {
		return fmt.Errorf("visionnav: nil Decision")
	}
	if d.Action == "" {
		return fmt.Errorf("visionnav: Decision.Action is empty (no action proposed)")
	}
	if d.Rationale == "" {
		return fmt.Errorf("visionnav: Decision.Rationale is empty " +
			"(LLM provided no reason — bluff-by-construction)")
	}
	switch d.ExpectedVerdict {
	case "", "pass", "fail", "needs-review":
		// "" allowed when LLM honestly doesn't know what to expect
	default:
		return fmt.Errorf("visionnav: Decision.ExpectedVerdict %q invalid", d.ExpectedVerdict)
	}
	return nil
}

// Provider is the LLM (or scripted, or human-in-loop) thing that
// decides the explorer's next move. Implementations MUST be safe
// for concurrent calls if used by multiple explorers; a single
// LLM session is typically serialized internally.
type Provider interface {
	// Name identifies the provider (e.g. "anthropic-claude-opus-4",
	// "scripted-bank-yaml", "nop").
	Name() string
	// Decide returns the next action given the current observation
	// (most recent screen capture path + audio path). Returns error
	// if the provider can't decide (network failure, rate limit, etc.).
	// Validate() is called on the returned Decision before it's
	// returned — invalid Decisions surface as errors.
	Decide(ctx context.Context, obs Observation) (*Decision, error)
}

// Observation is what the explorer hands to the Provider at each
// step. Mirrors the inputs the Provider needs to make a decision.
type Observation struct {
	// StepNumber is 1-based; useful for max-step caps.
	StepNumber int
	// LastImagePath is the path to the most recent screen frame.
	// May be empty on step 1.
	LastImagePath string
	// LastAudioPath is the path to the most recent audio clip.
	// May be empty on step 1 or when the explorer disables audio.
	LastAudioPath string
	// LastEvidence is the previous step's Evidence record (if any).
	// Lets the Provider detect "the previous action's actual verdict
	// diverged from my expected verdict" patterns.
	LastEvidence *Evidence
}

// NopProvider is a deterministic test-only Provider. Returns the
// same canned Decision every step. Useful for:
//   - End-to-end tests of the explorer plumbing without a real LLM
//   - Operator smoke-testing the visionnav harness offline
//   - Captured-evidence integration where no autonomy is desired
//     (the explorer just records pre-decided steps)
//
// Constitution §11.4: NopProvider's canned Decision IS valid per
// Decision.Validate() — it has a non-empty Action AND non-empty
// Rationale. A "Provider returned bluff Decision" regression would
// be caught by the Validate call inside its Decide().
type NopProvider struct {
	canned Decision
}

// NewNopProvider returns a NopProvider whose Decide() always
// returns the supplied Decision. The Decision is Validate()d at
// construction time so a bluff Decision can't be smuggled in.
func NewNopProvider(d Decision) (*NopProvider, error) {
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("visionnav: NopProvider needs valid Decision: %w", err)
	}
	return &NopProvider{canned: d}, nil
}

// Name returns "nop".
func (p *NopProvider) Name() string { return "nop" }

// Decide returns a copy of the canned Decision (defensive copy so
// the caller can't mutate the shared template).
func (p *NopProvider) Decide(ctx context.Context, obs Observation) (*Decision, error) {
	d := p.canned
	if err := d.Validate(); err != nil {
		// Defence in depth — should never trip given construction-
		// time validation, but this is the §11.4 backstop.
		return nil, fmt.Errorf("visionnav: NopProvider canned Decision became invalid: %w", err)
	}
	return &d, nil
}
