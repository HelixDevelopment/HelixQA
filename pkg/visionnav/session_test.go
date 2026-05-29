// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionnav

import (
	"context"
	"testing"
)

// --- fakes (allowed in UNIT tests per §11.4.27) ---

// fakeActor returns scripted screenshots + records dispatched actions.
// shots[i] is returned on the (i+1)-th Screenshot call; the last shot
// repeats once the script is exhausted.
type fakeActor struct {
	shots     [][]byte
	calls     int
	dispatched []string
}

func (f *fakeActor) Screenshot(_ context.Context) ([]byte, error) {
	i := f.calls
	f.calls++
	if i >= len(f.shots) {
		i = len(f.shots) - 1
	}
	return f.shots[i], nil
}

func (f *fakeActor) Dispatch(_ context.Context, action string) error {
	f.dispatched = append(f.dispatched, action)
	return nil
}

// fakeProvider always returns the same valid Decision.
type fakeProvider struct{}

func (fakeProvider) Name() string { return "fake" }
func (fakeProvider) Decide(_ context.Context, _ Observation) (*Decision, error) {
	return &Decision{Action: "tap_next", Rationale: "explore forward"}, nil
}

// fakeExplorer produces Evidence whose OCRSnapshot is taken from a
// per-step script, so a test controls exactly when a goal "appears".
type fakeExplorer struct {
	ocrByStep []string // ocrByStep[i] used on (i+1)-th CaptureFinding
	calls     int
}

func (e *fakeExplorer) Name() string { return "fake-explorer" }
func (e *fakeExplorer) CaptureFinding(_ context.Context, opts FindingOptions) (*Evidence, error) {
	i := e.calls
	e.calls++
	ocr := ""
	if i < len(e.ocrByStep) {
		ocr = e.ocrByStep[i]
	}
	return &Evidence{
		Description: opts.Description,
		Verdict:     opts.Verdict,
		OCRSnapshot: ocr,
		Notes:       opts.Notes,
	}, nil
}

// distinctShots returns n screenshots that differ from each other
// (length + sampled bytes), so screensDiffer reports change.
func distinctShots(n int) [][]byte {
	out := make([][]byte, n)
	for i := 0; i < n; i++ {
		// 6000 bytes + i extra so lengths differ beyond the noise floor,
		// and a varying payload byte at a sampled offset.
		b := make([]byte, 6000+(i+1)*128)
		for j := range b {
			b[j] = byte((i*7 + j) % 251)
		}
		out[i] = b
	}
	return out
}

// identicalShots returns n copies of the same screenshot (zero delta).
func identicalShots(n int) [][]byte {
	base := make([]byte, 6000)
	for j := range base {
		base[j] = byte(j % 251)
	}
	out := make([][]byte, n)
	for i := range out {
		cp := make([]byte, len(base))
		copy(cp, base)
		out[i] = cp
	}
	return out
}

func mustTarget(t *testing.T) Target {
	t.Helper()
	ResetTargetRegistry()
	t.Cleanup(ResetTargetRegistry)
	tgt := Target{Name: "demo", LaunchAction: "launch demo", ScreenGoals: []string{"GOAL_SCREEN"}}
	if err := Register(tgt); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, _ := Get("demo")
	return got
}

func TestSession_HappyPath_GoalReached_PASS(t *testing.T) {
	tgt := mustTarget(t)
	actor := &fakeActor{shots: distinctShots(3)}
	// Goal text appears on step 3.
	expl := &fakeExplorer{ocrByStep: []string{"Loading", "Menu", "Now on GOAL_SCREEN here"}}

	sess, err := NewSession(SessionConfig{
		Provider: fakeProvider{}, Actor: actor, Explorer: expl, Target: tgt, MaxSteps: 5,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	res, err := sess.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected PASS, got %+v", res)
	}
	if !res.GoalReached || !res.ScreenChanged {
		t.Fatalf("PASS requires GoalReached && ScreenChanged, got %+v", res)
	}
	if res.Steps != 3 {
		t.Fatalf("expected loop to stop at step 3 on goal, got %d", res.Steps)
	}
	// Step 1 dispatched the LaunchAction.
	if len(actor.dispatched) == 0 || actor.dispatched[0] != "launch demo" {
		t.Fatalf("step 1 should dispatch LaunchAction, got %v", actor.dispatched)
	}
}

func TestSession_MaxStepWithoutGoal_FAIL(t *testing.T) {
	tgt := mustTarget(t)
	actor := &fakeActor{shots: distinctShots(4)}
	// Goal never appears.
	expl := &fakeExplorer{ocrByStep: []string{"a", "b", "c", "d"}}

	sess, _ := NewSession(SessionConfig{
		Provider: fakeProvider{}, Actor: actor, Explorer: expl, Target: tgt, MaxSteps: 3,
	})
	res, err := sess.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected FAIL (no goal), got %+v", res)
	}
	if res.GoalReached {
		t.Fatalf("GoalReached must be false")
	}
	if res.Steps != 3 {
		t.Fatalf("expected MaxSteps=3 iterations, got %d", res.Steps)
	}
}

func TestSession_ZeroScreenDelta_AutoFAIL(t *testing.T) {
	tgt := mustTarget(t)
	// Screen never changes — every screenshot identical.
	actor := &fakeActor{shots: identicalShots(3)}
	// Goal text IS present every step — but the unchanged screen must
	// still force auto-FAIL per §11.4.52(b).
	expl := &fakeExplorer{ocrByStep: []string{"GOAL_SCREEN", "GOAL_SCREEN", "GOAL_SCREEN"}}

	sess, _ := NewSession(SessionConfig{
		Provider: fakeProvider{}, Actor: actor, Explorer: expl, Target: tgt, MaxSteps: 3,
	})
	res, err := sess.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ScreenChanged {
		t.Fatalf("identical screenshots must report ScreenChanged=false")
	}
	if res.Passed {
		t.Fatalf("zero-screen-delta MUST auto-FAIL even with goal text present, got %+v", res)
	}
	// The goal text matched, but the verdict is still FAIL.
	if !res.GoalReached {
		t.Fatalf("goal text present should set GoalReached=true (proving the auto-FAIL is the delta, not the goal)")
	}
}

func TestSession_RejectsInvalidConfig(t *testing.T) {
	tgt := mustTarget(t)
	cases := []SessionConfig{
		{Provider: nil, Actor: &fakeActor{shots: distinctShots(1)}, Explorer: &fakeExplorer{}, Target: tgt, MaxSteps: 1},
		{Provider: fakeProvider{}, Actor: nil, Explorer: &fakeExplorer{}, Target: tgt, MaxSteps: 1},
		{Provider: fakeProvider{}, Actor: &fakeActor{shots: distinctShots(1)}, Explorer: nil, Target: tgt, MaxSteps: 1},
		{Provider: fakeProvider{}, Actor: &fakeActor{shots: distinctShots(1)}, Explorer: &fakeExplorer{}, Target: tgt, MaxSteps: 0},
		{Provider: fakeProvider{}, Actor: &fakeActor{shots: distinctShots(1)}, Explorer: &fakeExplorer{}, Target: Target{}, MaxSteps: 1},
	}
	for i, c := range cases {
		if _, err := NewSession(c); err == nil {
			t.Fatalf("case %d: NewSession should reject invalid config", i)
		}
	}
}
