// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Phase 27.7 tests — Provider interface + NopProvider §11.4
// guarantees. Anti-bluff: every PASS asserts a SPECIFIC value
// (the canned action string, the canned rationale) so a
// regression returning empty Decisions is caught.

package visionnav

import (
	"context"
	"strings"
	"testing"
)

// --- Decision.Validate() ---

func TestDecision_Validate_NilRejected(t *testing.T) {
	var d *Decision
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "nil Decision") {
		t.Fatalf("nil Decision: got %v", err)
	}
}

func TestDecision_Validate_EmptyAction(t *testing.T) {
	d := &Decision{Rationale: "because"}
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "Action is empty") {
		t.Fatalf("empty Action: got %v", err)
	}
}

func TestDecision_Validate_EmptyRationale_RejectedAsBluff(t *testing.T) {
	// §11.4 enforcement at the Provider boundary — an LLM that
	// proposes an action with no reason is bluff-by-construction.
	d := &Decision{Action: "tap_back"}
	err := d.Validate()
	if err == nil {
		t.Fatal("empty Rationale: expected rejection, got nil")
	}
	if !strings.Contains(err.Error(), "bluff-by-construction") {
		t.Errorf("error %q should mention 'bluff-by-construction'", err.Error())
	}
}

func TestDecision_Validate_BogusExpectedVerdict(t *testing.T) {
	d := &Decision{Action: "x", Rationale: "y", ExpectedVerdict: "maybe"}
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "ExpectedVerdict") {
		t.Fatalf("bogus ExpectedVerdict: got %v", err)
	}
}

func TestDecision_Validate_AcceptsHonestUnknown(t *testing.T) {
	// Empty ExpectedVerdict is honest "I don't know what this will
	// produce" — accepted (the alternative is forcing the LLM to
	// guess, which is its OWN bluff).
	d := &Decision{Action: "explore_unknown_screen", Rationale: "first time seeing this", ExpectedVerdict: ""}
	if err := d.Validate(); err != nil {
		t.Fatalf("honest unknown ExpectedVerdict rejected: %v", err)
	}
}

// --- NopProvider ---

func TestNopProvider_Construction_RejectsBluffDecision(t *testing.T) {
	// Bluff: empty Rationale.
	_, err := NewNopProvider(Decision{Action: "x"})
	if err == nil {
		t.Fatal("NewNopProvider accepted bluff canned Decision")
	}
}

func TestNopProvider_Decide_ReturnsCannedDecision(t *testing.T) {
	canned := Decision{
		Action:          "settle_at_home_screen",
		Rationale:       "deterministic test fixture; no real exploration",
		ExpectedVerdict: "pass",
	}
	p, err := NewNopProvider(canned)
	if err != nil {
		t.Fatalf("NewNopProvider: %v", err)
	}
	if p.Name() != "nop" {
		t.Errorf("Name = %q, want 'nop'", p.Name())
	}

	d, err := p.Decide(context.Background(), Observation{StepNumber: 1})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	// Captured-evidence assertions — the Action and Rationale must
	// be the SPECIFIC strings we cannednot empty defaults.
	if d.Action != "settle_at_home_screen" {
		t.Errorf("Action = %q", d.Action)
	}
	if d.Rationale != "deterministic test fixture; no real exploration" {
		t.Errorf("Rationale = %q", d.Rationale)
	}
	if d.ExpectedVerdict != "pass" {
		t.Errorf("ExpectedVerdict = %q", d.ExpectedVerdict)
	}
}

func TestNopProvider_Decide_DefensiveCopy(t *testing.T) {
	// Caller mutating the returned Decision must not affect
	// subsequent Decide() calls.
	p, _ := NewNopProvider(Decision{Action: "a", Rationale: "b"})
	d1, _ := p.Decide(context.Background(), Observation{})
	d1.Action = "MUTATED"
	d2, _ := p.Decide(context.Background(), Observation{})
	if d2.Action != "a" {
		t.Errorf("second Decide saw mutation: %q", d2.Action)
	}
}

// --- Observation passthrough ---

func TestObservation_FieldsHonored(t *testing.T) {
	// Build a minimal Provider that introspects the Observation.
	// Done inline so we don't need a separate exported test fixture.
	type recorder struct {
		got Observation
	}
	p := &probeProvider{}
	canned := Decision{Action: "x", Rationale: "y"}
	obs := Observation{
		StepNumber:    7,
		LastImagePath: "/tmp/screen.png",
		LastAudioPath: "/tmp/audio.wav",
	}
	p.canned = canned
	d, err := p.Decide(context.Background(), obs)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Action != "x" {
		t.Errorf("Action regressed: %q", d.Action)
	}
	if p.lastObs.StepNumber != 7 {
		t.Errorf("StepNumber not propagated: %d", p.lastObs.StepNumber)
	}
	if p.lastObs.LastImagePath != "/tmp/screen.png" {
		t.Errorf("LastImagePath not propagated: %q", p.lastObs.LastImagePath)
	}
	_ = recorder{}
}

// probeProvider — test-only Provider that records what Observation
// it receives. Lives in _test.go so it's not part of the public
// surface.
type probeProvider struct {
	canned  Decision
	lastObs Observation
}

func (p *probeProvider) Name() string { return "probe" }
func (p *probeProvider) Decide(_ context.Context, obs Observation) (*Decision, error) {
	p.lastObs = obs
	d := p.canned
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}
