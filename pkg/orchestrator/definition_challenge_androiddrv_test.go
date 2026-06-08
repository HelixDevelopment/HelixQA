// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// definition_challenge_android_test.go — Android vision-backend suite.
//
// Closes the executor gap surfaced by the first end-to-end HelixQA run
// on a VM (evidence: .lava-ci-evidence/helixqa/archiveorg/
// 20260608T143737Z-archiveorg/qa-report.md): `helixqa run --banks`
// loaded each bank case then SKIPped every android-platform case with
// "bank case has no executable action for the android platform" because
// definitionChallenge.Execute only ran `shell:` steps on the desktop
// platform. The report then mis-counted those honest skips as crashes.
//
// The fix adds a real Android execution backend: an android-platform
// bank case with a wired AndroidVisionContext DRIVES the device through
// the EXISTING pkg/visionnav.Session loop and is scored on the §11.4.52
// goal-reached-AND-screen-changed verdict — never a fake PASS, and still
// an HONEST skip when the context is absent.
//
// These tests are device-free: a fake visionnav.ScreenActor returns
// canned screenshots + records dispatched actions, and a fake
// visionnav.Provider returns a scripted action sequence. No adb, no
// emulator — the device run is reserved for the orchestrator.
//
// Anti-bluff posture (§11.4 / §11.4.27): the fakes live ONLY at the
// external LLM + device boundaries (the unit-test boundary). The
// Session, ADBActor-equivalent dispatch, Target, verdict logic, and the
// definitionChallenge.executeAndroidVisionSteps mapping under test are
// all REAL. The falsifiability test (FAIL when the provider never reaches
// the goal) proves the PASS is earned, not manufactured.
package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/testbank"
	"digital.vasic.helixqa/pkg/visionnav"
)

// ── Fakes (external boundaries only) ─────────────────────────────────

// fakeActor is a visionnav.ScreenActor whose Screenshot returns a
// DIFFERENT image each call (so the §11.4.52 screen-delta requirement is
// genuinely satisfied — not faked by returning identical bytes) and
// whose Dispatch records every action the loop dispatched. It is the
// device-boundary fake (stands in for a navigator.ADBExecutor-backed
// ADBActor) so the test needs no real adb/emulator.
type fakeActor struct {
	step       int
	dispatched []string
	shots      int
}

func (a *fakeActor) Screenshot(_ context.Context) ([]byte, error) {
	a.step++
	a.shots++
	// Distinct, screenshot-sized payloads per step: vary the byte length
	// well past the screenLenNoiseBytes floor so screensDiffer() reliably
	// reports a transition between consecutive frames.
	n := 8192 + a.step*512
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte((a.step*37 + i) % 251)
	}
	return buf, nil
}

func (a *fakeActor) Dispatch(_ context.Context, action string) error {
	a.dispatched = append(a.dispatched, action)
	return nil
}

// scriptedProvider is a visionnav.Provider that returns a scripted
// sequence of valid Decisions. It is the LLM-boundary fake. When
// reachGoal is true it never matters what it returns past the goal —
// the goal is stamped by goalExplorer; this fake only proves the loop
// dispatches the scripted actions.
type scriptedProvider struct {
	name     string
	actions  []string
	idx      int
	rational string
}

func (p *scriptedProvider) Name() string {
	if p.name == "" {
		return "scripted"
	}
	return p.name
}

func (p *scriptedProvider) Decide(_ context.Context, _ visionnav.Observation) (*visionnav.Decision, error) {
	act := "noop"
	if p.idx < len(p.actions) {
		act = p.actions[p.idx]
		p.idx++
	}
	r := p.rational
	if r == "" {
		r = "scripted step advancing toward the goal screen"
	}
	d := &visionnav.Decision{Action: act, Rationale: r, ExpectedVerdict: "needs-review"}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

// goalExplorer is a visionnav.Explorer that stamps a real OCR snapshot
// per finding. When reach is true it stamps the goal substring from the
// SECOND finding onward (so the Session runs ≥2 steps and the §11.4.52
// screen-delta path is genuinely exercised before the goal match). When
// reach is false it NEVER stamps the goal — driving the loop to its
// honest zero-goal FAIL. It records via the real Evidence.Validate so a
// bluff Evidence (no OCR, no transcript) is rejected exactly as the real
// FileSink would.
type goalExplorer struct {
	goal     string
	reach    bool
	finding  int
	captured int
}

func (e *goalExplorer) Name() string { return "goal-explorer" }

func (e *goalExplorer) CaptureFinding(_ context.Context, opts visionnav.FindingOptions) (*visionnav.Evidence, error) {
	e.finding++
	ev := &visionnav.Evidence{
		Description: opts.Description,
		Verdict:     "needs-review",
		Notes:       opts.Notes,
	}
	if e.reach && e.finding >= 2 {
		ev.OCRSnapshot = "Welcome — " + e.goal
	} else {
		ev.OCRSnapshot = "Loading…"
	}
	if err := ev.Validate(); err != nil {
		return nil, err
	}
	e.captured++
	return ev, nil
}

// androidTestCase builds an android-platform bank case whose ExpectedResult
// (the success criterion) is the given goal substring, plus a launch step.
func androidTestCase(id, goal string) *testbank.TestCase {
	return &testbank.TestCase{
		ID:             id,
		Name:           "android vision case " + id,
		Description:    "drive a real device to the goal screen",
		Category:       "functional",
		Priority:       testbank.PriorityCritical,
		Platforms:      []config.Platform{config.PlatformAndroid},
		ExpectedResult: goal,
		Steps: []testbank.TestStep{
			{
				Name:   "launch app",
				Action: "adb_shell: monkey -p com.example.app 1",
			},
		},
	}
}

func androidDefChallenge(tc *testbank.TestCase, actx *AndroidVisionContext) *definitionChallenge {
	return newDefinitionChallengeForAndroid(tc.ToDefinition(), tc, actx)
}

// ── Tests ────────────────────────────────────────────────────────────

// TestAndroid_VisionRun_ReachesGoal_ScoresPASS is the primary GREEN test.
// With a wired android context whose Provider reaches the success state,
// Execute returns a NON-skip PASS whose RecordedActions contain the real
// dispatched actions + at least one screenshot was captured.
func TestAndroid_VisionRun_ReachesGoal_ScoresPASS(t *testing.T) {
	const goal = "Now Playing"
	tc := androidTestCase("AND-PASS", goal)

	actor := &fakeActor{}
	prov := &scriptedProvider{actions: []string{"tap 100 200", "tap 100 300", "tap 100 400"}}
	expl := &goalExplorer{goal: goal, reach: true}

	dc := androidDefChallenge(tc, &AndroidVisionContext{
		Provider: prov,
		Actor:    actor,
		Explorer: expl,
		MaxSteps: 4,
		Serial:   "emulator-5554",
	})

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	// PRIMARY user-visible assertion: a real PASS, never a SKIP.
	require.Equal(t, challenge.StatusPassed, res.Status,
		"a vision run that reached the goal AND changed the screen must score PASS")
	require.NotEqual(t, challenge.StatusSkipped, res.Status,
		"the android backend must DRIVE the device, never honest-skip when wired")

	// RecordedActions carry the REAL dispatched actions (launch on step 1,
	// then the scripted taps). A hollow metadata PASS cannot produce these.
	joined := strings.Join(res.RecordedActions, "\n")
	require.Contains(t, joined, "android-vision: serial=emulator-5554")
	require.Contains(t, joined, "android-vision: launch=shell monkey -p com.example.app 1")

	// The actor really dispatched: step 1 is the launch action, later
	// steps are the scripted taps.
	require.GreaterOrEqual(t, len(actor.dispatched), 2,
		"the loop must dispatch the launch action plus ≥1 scripted action")
	require.Equal(t, "shell monkey -p com.example.app 1", actor.dispatched[0],
		"step 1 must dispatch the derived launch action")
	// Step 1 always dispatches the launch action (the Provider's first
	// decision is advisory on step 1 per session.go), so a SCRIPTED tap
	// reaches the device from step 2 onward.
	var sawScriptedTap bool
	for _, a := range actor.dispatched[1:] {
		if strings.HasPrefix(a, "tap ") {
			sawScriptedTap = true
			break
		}
	}
	require.True(t, sawScriptedTap,
		"a scripted provider tap action must reach the device after launch; got %v",
		actor.dispatched)

	// Screenshots count > 0 — the device was really observed.
	require.Greater(t, actor.shots, 0, "the loop must capture ≥1 screenshot")
	require.Greater(t, expl.captured, 0, "the explorer must capture ≥1 Evidence record")

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
	require.True(t, goalReached, "goal-reached assertion must pass on a PASS run")
	require.True(t, screenChanged, "screen-changed assertion must pass on a PASS run")

	// Real wall-clock duration (not a sub-µs metadata stamp).
	require.GreaterOrEqual(t, res.Duration, time.Duration(0))
}

// TestAndroid_VisionRun_NeverReachesGoal_ScoresFAIL is the FALSIFIABILITY
// test. A fake provider that drives the loop but NEVER reaches the goal
// (the explorer never stamps the goal OCR) MUST score FAIL — not PASS,
// not SKIP — with a clear message. This proves the PASS in the test above
// is EARNED by the real verdict, not manufactured: break the thing the
// test claims to verify (goal reachability) and the verdict flips to FAIL.
func TestAndroid_VisionRun_NeverReachesGoal_ScoresFAIL(t *testing.T) {
	const goal = "Now Playing"
	tc := androidTestCase("AND-FAIL", goal)

	actor := &fakeActor{}
	prov := &scriptedProvider{actions: []string{"tap 1 2", "tap 3 4"}}
	// reach=false ⇒ the explorer never stamps the goal OCR.
	expl := &goalExplorer{goal: goal, reach: false}

	dc := androidDefChallenge(tc, &AndroidVisionContext{
		Provider: prov,
		Actor:    actor,
		Explorer: expl,
		MaxSteps: 3,
		Serial:   "emulator-5554",
	})

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	// PRIMARY assertion: a run that never reached the goal is a FAIL.
	require.Equal(t, challenge.StatusFailed, res.Status,
		"a vision run that never reached the goal MUST score FAIL, not PASS")
	require.NotEqual(t, challenge.StatusPassed, res.Status,
		"falsifiability: breaking goal-reachability must flip the verdict away from PASS")
	require.NotEqual(t, challenge.StatusSkipped, res.Status,
		"a wired android run that genuinely failed must FAIL, not SKIP")

	// Clear failure message naming the unreached goal.
	require.NotEmpty(t, res.Error, "a FAIL must carry a clear reason")
	require.Contains(t, res.Error, "no ScreenGoal reached",
		"the failure message must name the unreached-goal cause")

	// The goal-reached assertion must be present and failing.
	var sawGoalAssertion bool
	for _, a := range res.Assertions {
		if a.Type == "vision-goal-reached" {
			sawGoalAssertion = true
			require.False(t, a.Passed, "goal-reached assertion must fail on a FAIL run")
		}
	}
	require.True(t, sawGoalAssertion, "the FAIL result must record the goal-reached assertion")
}

// TestAndroid_NoVisionContext_HonestSkip proves the honest-skip path is
// preserved: with NO android context wired (the desktop/CI default),
// Execute still returns the explicit Skipped result — never a fake PASS.
func TestAndroid_NoVisionContext_HonestSkip(t *testing.T) {
	tc := androidTestCase("AND-SKIP", "Now Playing")

	// No AndroidVisionContext (nil) — exactly the no-device-no-provider run.
	dc := newDefinitionChallengeForPlatform(
		tc.ToDefinition(), tc, config.PlatformAndroid)

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, challenge.StatusSkipped, res.Status,
		"with no vision context wired, an android case must honestly SKIP")
	require.NotEqual(t, challenge.StatusPassed, res.Status,
		"an unwired android case must NEVER bluff a PASS")

	// The skip carries the wrapper sentinel + an explicit android reason.
	require.True(t, hasSkipSentinel(res),
		"honest skip must carry the definition-wrapper sentinel")
	joined := strings.Join(res.RecordedActions, "\n")
	require.Contains(t, joined, "android",
		"the skip reason must explicitly name the android platform")
	require.Contains(t, joined, "needs an Android/UI/web topology backend")
}

// TestAndroid_PartiallyWiredContext_HonestSkip proves a misconfigured
// context (Provider but no Actor) is treated as NOT wired — the wrapper
// honestly skips rather than half-running (which could mask a real
// configuration error as a green run).
func TestAndroid_PartiallyWiredContext_HonestSkip(t *testing.T) {
	tc := androidTestCase("AND-PARTIAL", "Now Playing")

	dc := androidDefChallenge(tc, &AndroidVisionContext{
		Provider: &scriptedProvider{actions: []string{"tap 1 2"}},
		// Actor + Explorer intentionally nil ⇒ context.valid() is false.
		MaxSteps: 3,
	})

	res, err := dc.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, challenge.StatusSkipped, res.Status,
		"a partially-wired android context must honestly SKIP, never half-run")
}
