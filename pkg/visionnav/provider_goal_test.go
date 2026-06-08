// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// provider_goal_test.go — unit coverage for the OCR-host-free
// provider-vision goal source at the pkg/visionnav layer.
//
// Verifies, with fakes only at the LLM + device boundaries and NO OCR
// host anywhere:
//   - Decision.GoalReached drives Session.Run to a PASS verdict, and the
//     captured Evidence carries the Provider's rationale (not OCR);
//   - falsifiability — a Provider that never sets GoalReached yields a
//     no-goal FAIL run;
//   - parseDecision extracts GOAL_REACHED from a strict LLM reply;
//   - Evidence.Validate accepts a ProviderRationale-only record.
package visionnav

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// goalSignalProvider returns scripted Decisions and flips GoalReached on
// the step indexed by reachAt (1-based). reachAt <= 0 = never. The
// LLM-boundary fake; no OCR involved.
type goalSignalProvider struct {
	reachAt int
}

func (p *goalSignalProvider) Name() string { return "goal-signal" }

func (p *goalSignalProvider) Decide(_ context.Context, obs Observation) (*Decision, error) {
	d := &Decision{
		Action:      "tap 10 20",
		Rationale:   "vision model reasoning about the current screenshot",
		GoalReached: p.reachAt > 0 && obs.StepNumber >= p.reachAt,
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

// deltaActor returns a distinct screenshot per call so the §11.4.52
// screen-delta requirement is genuinely satisfied (device boundary fake).
type deltaActor struct {
	step       int
	dispatched []string
}

func (a *deltaActor) Screenshot(_ context.Context) ([]byte, error) {
	a.step++
	n := 8192 + a.step*512
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte((a.step*41 + i) % 251)
	}
	return buf, nil
}

func (a *deltaActor) Dispatch(_ context.Context, action string) error {
	a.dispatched = append(a.dispatched, action)
	return nil
}

func providerGoalTarget() Target {
	return Target{
		Name:         "provider-goal-target",
		LaunchAction: "launch app",
		// No OCRSnapshot will ever match this — the goal must come from
		// the Provider's Decision.GoalReached, proving the OCR-free path.
		ScreenGoals: []string{"a goal token no OCRSnapshot ever produces"},
	}
}

// TestSession_ProviderGoalReached_PASS_NoOCR: GoalReached drives PASS with
// the ProviderVisionExplorer (no OCR host).
func TestSession_ProviderGoalReached_PASS_NoOCR(t *testing.T) {
	sink, err := NewFileSink(t.TempDir())
	require.NoError(t, err)
	expl, err := NewProviderVisionExplorer("test-pg", sink)
	require.NoError(t, err)

	sess, err := NewSession(SessionConfig{
		Provider: &goalSignalProvider{reachAt: 2},
		Actor:    &deltaActor{},
		Explorer: expl,
		Target:   providerGoalTarget(),
		MaxSteps: 4,
	})
	require.NoError(t, err)

	res, err := sess.Run(context.Background())
	require.NoError(t, err)
	require.True(t, res.GoalReached,
		"the Provider's Decision.GoalReached must mark the goal reached without OCR")
	require.True(t, res.ScreenChanged, "distinct screenshots must register a screen change")
	require.True(t, res.Passed, "goal reached AND screen changed must PASS")

	// Captured evidence carries the Provider rationale, not OCR.
	require.NotEmpty(t, res.Evidence)
	last := res.Evidence[len(res.Evidence)-1]
	require.Equal(t, "", last.OCRSnapshot, "no OCR host means no OCRSnapshot")
	require.NotEmpty(t, last.ProviderRationale,
		"the provider-vision evidence source is the recorded rationale")
	require.NoError(t, last.Validate(),
		"a ProviderRationale-only Evidence must be §11.4-valid (not a bluff)")
}

// TestSession_ProviderNeverConfirms_FAIL_NoOCR is the falsifiability case.
func TestSession_ProviderNeverConfirms_FAIL_NoOCR(t *testing.T) {
	sink, err := NewFileSink(t.TempDir())
	require.NoError(t, err)
	expl, err := NewProviderVisionExplorer("test-pg-fail", sink)
	require.NoError(t, err)

	sess, err := NewSession(SessionConfig{
		Provider: &goalSignalProvider{reachAt: 0}, // never confirms
		Actor:    &deltaActor{},
		Explorer: expl,
		Target:   providerGoalTarget(),
		MaxSteps: 3,
	})
	require.NoError(t, err)

	res, err := sess.Run(context.Background())
	require.NoError(t, err)
	require.False(t, res.GoalReached,
		"a Provider that never confirms the goal must NOT mark it reached")
	require.False(t, res.Passed, "no goal reached must FAIL")
	require.Contains(t, res.Reason, "no ScreenGoal reached")
}

// TestParseDecision_GoalReached covers the LLM reply parsing of the new
// GOAL_REACHED field — affirmative yes, explicit no, and absence.
func TestParseDecision_GoalReached(t *testing.T) {
	yes, err := parseDecision("GOAL_REACHED: yes\nACTION: noop\nRATIONALE: the goal screen is visible")
	require.NoError(t, err)
	require.True(t, yes.GoalReached, "GOAL_REACHED: yes must set GoalReached")

	no, err := parseDecision("GOAL_REACHED: no\nACTION: tap 1 2\nRATIONALE: keep navigating")
	require.NoError(t, err)
	require.False(t, no.GoalReached, "GOAL_REACHED: no must leave GoalReached false")

	absent, err := parseDecision("ACTION: tap 1 2\nRATIONALE: keep navigating")
	require.NoError(t, err)
	require.False(t, absent.GoalReached, "absent GOAL_REACHED must default to false")
}

// TestEvidenceValidate_ProviderRationaleOnly proves the §11.4 evidence gate
// accepts a ProviderRationale-only record AND still rejects a truly empty one.
func TestEvidenceValidate_ProviderRationaleOnly(t *testing.T) {
	ok := &Evidence{Description: "step 1", Verdict: "needs-review", ProviderRationale: "model saw the goal screen"}
	require.NoError(t, ok.Validate(), "ProviderRationale alone must satisfy the captured-evidence rule")

	bluff := &Evidence{Description: "step 1", Verdict: "needs-review"}
	require.Error(t, bluff.Validate(),
		"an Evidence with no captured source at all must still be rejected as a bluff")
}
