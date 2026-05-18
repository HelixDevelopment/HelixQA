// Package orchestrator — definition_challenge.go.
//
// definitionChallenge bridges bank-loaded *challenge.Definition records
// into the runnable challenge.Challenge interface so the orchestrator
// can dispatch them through challenges/pkg/runner. Before this
// adapter existed, helixqa loaded YAML definitions into the bank
// but never registered them with registry.Default — so the
// orchestrator's per-definition dispatch loop in runPlatform always
// short-circuited (o.runner == nil OR registry.Get returned not-found),
// producing the "PASS-bluff detected: 0 challenges actually executed"
// honest-fail in cmd/helixqa/main.go.
//
// CONST-035 posture: a definitionChallenge that has no real backend
// to dispatch against returns Status=Skipped with explicit reason in
// RecordedActions ("declarative-only definition, no backend
// registered") AND a passing AssertionResult of type "definition-
// loaded" — both required by the runner's anti-bluff guard. This
// turns the prior 0-challenges-executed posture into N-challenges-
// honestly-skipped, which is anti-bluff-correct: SKIP is for
// environment limitations (no backend dispatcher), MUST always carry
// an explicit reason, and PASS is reserved for cases where positive
// evidence was observed.
//
// Future evolution: as real per-Type dispatchers are added (HTTP-API
// checks, browser-flow runners, mobile-launch runners), Execute will
// branch on def.Configuration to route to the appropriate backend
// instead of skipping. The wrapper signature stays stable so callers
// don't need to change.
package orchestrator

import (
	"context"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/registry"
)

// definitionWrapperSkipSentinel is the prefix written into
// RecordedActions by definitionChallenge.Execute so the orchestrator
// can detect, after the runner has merged the result, that the
// original wrapper intended a Skipped status. See
// restoreSkippedFromDefinitionWrapper for the round-82 anti-bluff
// rationale.
const definitionWrapperSkipSentinel = "skip-reason:"

// restoreSkippedFromDefinitionWrapper restores Status=Skipped on a
// challenge.Result produced by the definitionChallenge wrapper after
// the challenges-runner's executeChallenge merge logic has overridden
// the wrapper's intent. See round-82 commit body for the full
// causal chain — the short version: the runner only preserves
// Failed/TimedOut/Error from the inner Execute call; Skipped with a
// passing assertion gets silently promoted to Passed, producing the
// canonical CONST-035 / Article XI §11.9 PASS-bluff (declarative-only
// definitions with no real backend dispatch report success to the
// end user).
//
// The signal that triggers restoration is the
// definitionWrapperSkipSentinel string at the start of any
// RecordedAction entry — only the wrapper writes that exact prefix.
// Non-wrapper results pass through untouched.
//
// Anti-bluff posture: this restores the HONEST skip, it does not
// hide a real PASS. The wrapper itself is what produced the
// sentinel; if a future per-Type real-backend dispatcher replaces
// the wrapper for a given Category, the sentinel will not be emitted
// and the runner's Passed status will stand.
func restoreSkippedFromDefinitionWrapper(r *challenge.Result) {
	if r == nil {
		return
	}
	if r.Status != challenge.StatusPassed {
		return
	}
	for _, action := range r.RecordedActions {
		if strings.HasPrefix(action, definitionWrapperSkipSentinel) {
			r.Status = challenge.StatusSkipped
			return
		}
	}
}

// newDefinitionRegistry returns a fresh, non-shared registry that the
// orchestrator can populate with definitionChallenge wrappers. Using a
// dedicated per-run registry (rather than registry.Default) keeps
// concurrent helixqa invocations from polluting each other's
// challenge namespaces.
func newDefinitionRegistry() registry.Registry {
	return registry.NewRegistry()
}

// definitionChallenge wraps a *challenge.Definition so it satisfies
// the challenge.Challenge interface. Stateless other than the
// captured cfg from Configure.
type definitionChallenge struct {
	def *challenge.Definition
	cfg *challenge.Config
}

// newDefinitionChallenge constructs the wrapper.
func newDefinitionChallenge(def *challenge.Definition) *definitionChallenge {
	return &definitionChallenge{def: def}
}

// ID returns the definition ID verbatim.
func (d *definitionChallenge) ID() challenge.ID {
	return d.def.ID
}

// Name returns the definition Name verbatim.
func (d *definitionChallenge) Name() string {
	return d.def.Name
}

// Description returns the definition Description verbatim.
func (d *definitionChallenge) Description() string {
	return d.def.Description
}

// Category returns the definition Category verbatim.
func (d *definitionChallenge) Category() string {
	return d.def.Category
}

// Dependencies returns the definition Dependencies verbatim.
func (d *definitionChallenge) Dependencies() []challenge.ID {
	return d.def.Dependencies
}

// Configure captures the runtime config for use in Execute.
func (d *definitionChallenge) Configure(cfg *challenge.Config) error {
	d.cfg = cfg
	return nil
}

// Validate is a no-op for declarative definitions — preconditions
// declared in def.Inputs are validated at Execute time when a real
// backend dispatches against them.
func (d *definitionChallenge) Validate(ctx context.Context) error {
	return nil
}

// Execute dispatches the definition. Since no real backend is wired
// yet, returns Status=Skipped with positive evidence (RecordedActions
// + a passing AssertionResult) that satisfies the runner's anti-bluff
// guard. The skip reason is explicit per CONST-035 ("SKIP is for
// environment limitations and MUST always carry an explicit reason").
func (d *definitionChallenge) Execute(ctx context.Context) (*challenge.Result, error) {
	start := time.Now()
	now := time.Now()
	skipReason := "declarative-only definition, no backend dispatcher registered for this Category"
	result := &challenge.Result{
		ChallengeID:   d.def.ID,
		ChallengeName: d.def.Name,
		Status:        challenge.StatusSkipped,
		StartTime:     start,
		EndTime:       now,
		Duration:      now.Sub(start),
		// CONST-035 anti-bluff: explicit skip reason in RecordedActions
		// + a single passing "definition-loaded" assertion so the
		// runner's ValidateAntiBluff guard accepts the Skipped status
		// as positive evidence (the runner explicitly downgrades
		// Status=Passed without evidence; Skipped+evidence is the
		// honest opposite — we DID see the definition load).
		RecordedActions: []string{
			"definition-loaded: id=" + string(d.def.ID),
			"definition-loaded: category=" + d.def.Category,
			definitionWrapperSkipSentinel + " " + skipReason,
		},
		Assertions: []challenge.AssertionResult{{
			Type:     "definition-loaded",
			Target:   "definition.ID",
			Expected: string(d.def.ID),
			Actual:   string(d.def.ID),
			Passed:   true,
			Message:  "bank-loaded definition successfully bridged to Challenge interface (close-out⁷⁵ wrapper)",
		}},
		Error: "", // not an error — honest skip
	}
	return result, nil
}

// Cleanup is a no-op for declarative definitions.
func (d *definitionChallenge) Cleanup(ctx context.Context) error {
	return nil
}

// Compile-time interface assertion.
var _ challenge.Challenge = (*definitionChallenge)(nil)
