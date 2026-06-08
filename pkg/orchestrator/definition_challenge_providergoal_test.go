// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// definition_challenge_providergoal_test.go — provider-vision goal source.
//
// Closes the OCR-host dependency in the Android vision-execution branch.
// Before this, goal detection routed exclusively through an
// OCR-backed Evidence.OCRSnapshot, which needed an external Tesseract host
// (HELIX_TESSERACT_URL). With no OCR host the whole vision run was an
// honest-SKIP for lack of infra.
//
// The provider-vision goal source derives "goal reached" from the vision
// Provider's OWN Decision.GoalReached — the model SEES each screenshot the
// Session feeds it (visionnav.Observation.LastImageBytes) and confirms the
// goal directly. The captured-evidence record for this path is the
// Provider's recorded rationale (visionnav.ProviderVisionExplorer), so NO
// OCR/audio host is required.
//
// These tests are device-free + OCR-free: a fake visionnav.Provider sets
// GoalReached on a scripted step, the real visionnav.Session +
// visionnav.ProviderVisionExplorer + definitionChallenge.executeAndroidVisionSteps
// run end-to-end, and the §11.4.52 verdict is scored from the real run.
//
// Anti-bluff posture (§11.4 / §11.4.27): the ONLY fakes are at the external
// LLM + device boundaries (the unit-test boundary). The Session, Target,
// verdict logic, evidence capture, and the orchestrator mapping under test
// are all REAL. The falsifiability test (a Provider that NEVER confirms the
// goal scores FAIL, not PASS) proves the PASS is earned, not manufactured.
package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/visionnav"
)

// providerGoalProvider is a visionnav.Provider that drives the loop with
// scripted actions and sets Decision.GoalReached on the step indexed by
// reachAt (1-based). When reachAt <= 0 it NEVER confirms the goal — the
// falsifiability driver. It is the LLM-boundary fake; no OCR is involved
// anywhere, mirroring a real vision Provider that decides "goal reached"
// purely from the screenshot it was shown.
type providerGoalProvider struct {
	reachAt int // 1-based step at which GoalReached flips true; <=0 = never
	calls   int
}

func (p *providerGoalProvider) Name() string { return "provider-goal-fake" }

func (p *providerGoalProvider) Decide(_ context.Context, obs visionnav.Observation) (*visionnav.Decision, error) {
	p.calls++
	d := &visionnav.Decision{
		Action:          "tap 100 200",
		Rationale:       "vision model reasoning about the screenshot it was shown this step",
		ExpectedVerdict: "needs-review",
		// The provider-vision goal signal: confirmed only at reachAt.
		GoalReached: p.reachAt > 0 && obs.StepNumber >= p.reachAt,
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

// newOCRFreeContext builds an AndroidVisionContext using the REAL
// ProviderVisionExplorer + a real in-memory FileSink — exactly the wiring
// buildAndroidVisionContext produces when HELIX_TESSERACT_URL is unset.
// No OCR host, no Tesseract, no Whisper.
func newOCRFreeContext(t *testing.T, prov visionnav.Provider, maxSteps int) *AndroidVisionContext {
	t.Helper()
	sink, err := visionnav.NewFileSink(t.TempDir())
	require.NoError(t, err)
	// This MUST succeed with no OCR/audio host — the whole point of the fix.
	expl, err := visionnav.NewProviderVisionExplorer("test-provider-goal", sink)
	require.NoError(t, err,
		"the provider-vision Explorer must construct WITHOUT any OCR host")
	return &AndroidVisionContext{
		Provider: prov,
		Actor:    &fakeActor{},
		Explorer: expl,
		MaxSteps: maxSteps,
		Serial:   "emulator-5554",
	}
}

// TestProviderVisionGoal_ReachesGoal_ScoresPASS_NoOCR is the primary GREEN
// test. A fake Provider that confirms the goal via Decision.GoalReached
// (NO OCR host anywhere) drives executeAndroidVisionSteps to a real PASS.
func TestProviderVisionGoal_ReachesGoal_ScoresPASS_NoOCR(t *testing.T) {
	tc := androidTestCase("PROV-PASS", "Now Playing")
	// Confirm the goal from step 2 so the §11.4.52 screen-delta path is
	// genuinely exercised (≥2 distinct screenshots) before the goal match.
	prov := &providerGoalProvider{reachAt: 2}

	actx := newOCRFreeContext(t, prov, 4)
	dc := androidDefChallenge(tc, actx)

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	// PRIMARY user-visible assertion: a real PASS, never a SKIP — achieved
	// WITHOUT any OCR host (HELIX_TESSERACT_URL never set in this test).
	require.Equal(t, challenge.StatusPassed, res.Status,
		"a provider-vision run that confirmed the goal AND changed the screen "+
			"must score PASS with no OCR host")
	require.NotEqual(t, challenge.StatusSkipped, res.Status)

	// The §11.4.52 verdict assertions are recorded + both true.
	var goalReached, screenChanged bool
	for _, a := range res.Assertions {
		switch a.Type {
		case "vision-goal-reached":
			goalReached = a.Passed
		case "vision-screen-changed":
			screenChanged = a.Passed
		}
	}
	require.True(t, goalReached,
		"goal-reached assertion must pass when the Provider confirmed the goal")
	require.True(t, screenChanged, "screen-changed assertion must pass on a PASS run")

	// The captured evidence is the Provider's rationale (no OCRSnapshot) —
	// the §11.4 captured-evidence requirement is met without OCR.
	joined := strings.Join(res.RecordedActions, "\n")
	require.Contains(t, joined, "rationale=",
		"each step's evidence must carry the Provider's recorded rationale")
}

// TestProviderVisionGoal_NeverConfirms_ScoresFAIL_NoOCR is the
// FALSIFIABILITY test. A Provider that drives the loop but NEVER sets
// Decision.GoalReached (and no OCR host to match against) MUST score FAIL,
// not PASS, not SKIP — proving the PASS above is earned by the real
// provider-vision verdict, not manufactured.
func TestProviderVisionGoal_NeverConfirms_ScoresFAIL_NoOCR(t *testing.T) {
	tc := androidTestCase("PROV-FAIL", "Now Playing")
	prov := &providerGoalProvider{reachAt: 0} // never confirms

	actx := newOCRFreeContext(t, prov, 3)
	dc := androidDefChallenge(tc, actx)

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	// PRIMARY assertion: a run whose Provider never confirmed the goal FAILs.
	require.Equal(t, challenge.StatusFailed, res.Status,
		"a provider-vision run that never confirmed the goal MUST score FAIL")
	require.NotEqual(t, challenge.StatusPassed, res.Status,
		"falsifiability: breaking goal confirmation must flip the verdict away from PASS")
	require.NotEqual(t, challenge.StatusSkipped, res.Status,
		"a wired (OCR-free) android run that genuinely failed must FAIL, not SKIP")

	require.NotEmpty(t, res.Error, "a FAIL must carry a clear reason")
	require.Contains(t, res.Error, "no ScreenGoal reached",
		"the failure message must name the unreached-goal cause")

	var sawGoalAssertion bool
	for _, a := range res.Assertions {
		if a.Type == "vision-goal-reached" {
			sawGoalAssertion = true
			require.False(t, a.Passed, "goal-reached assertion must fail on a FAIL run")
		}
	}
	require.True(t, sawGoalAssertion, "the FAIL result must record the goal-reached assertion")
}

// TestProviderVisionGoal_OCRFreeContextIsValid proves the no-OCR context is
// genuinely WIRED (valid()), not the partially-wired/honest-skip case. This
// is the contract buildAndroidVisionContext relies on when HELIX_TESSERACT_URL
// is unset: a ProviderVisionExplorer-backed context must drive the device,
// never SKIP.
func TestProviderVisionGoal_OCRFreeContextIsValid(t *testing.T) {
	prov := &providerGoalProvider{reachAt: 1}
	actx := newOCRFreeContext(t, prov, 2)

	require.True(t, actx.valid(),
		"an OCR-free context built with ProviderVisionExplorer must be valid() — "+
			"the vision run is WIRED, not an honest-skip for lack of OCR")

	// And end-to-end: a valid OCR-free context never produces a SKIP.
	tc := androidTestCase("PROV-VALID", "Now Playing")
	dc := androidDefChallenge(tc, actx)
	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, challenge.StatusSkipped, res.Status,
		"a valid OCR-free context must DRIVE the device, never honest-skip")
}
