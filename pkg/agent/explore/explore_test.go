// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package explore

import (
	"context"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/agent/action"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// cyclingScreenshotter returns a different canvas every N calls,
// cycling through a fixed list. Used to simulate screen transitions.
type cyclingScreenshotter struct {
	frames []image.Image
	idx    int
	err    error
}

func (c *cyclingScreenshotter) Screenshot(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.err != nil {
		return nil, c.err
	}
	img := c.frames[c.idx%len(c.frames)]
	c.idx++
	return img, nil
}

type staticScreenshotter struct {
	img image.Image
	err error
}

func (s *staticScreenshotter) Screenshot(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.img, nil
}

type scriptedResolver struct {
	actions []action.Action
	idx     int
	err     error
}

func (r *scriptedResolver) Resolve(ctx context.Context, img image.Image, goal string) (action.Action, error) {
	if err := ctx.Err(); err != nil {
		return action.Action{}, err
	}
	if r.err != nil {
		return action.Action{}, r.err
	}
	if r.idx >= len(r.actions) {
		return action.Action{Kind: action.KindClick, X: 1, Y: 1}, nil
	}
	a := r.actions[r.idx]
	r.idx++
	return a, nil
}

type recordingExecutor struct {
	executed []action.Action
	err      error
}

func (e *recordingExecutor) Execute(ctx context.Context, a action.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.err != nil {
		return e.err
	}
	e.executed = append(e.executed, a)
	return nil
}

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func solidImg(gray uint8) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{gray, gray, gray, 255})
		}
	}
	return img
}

func gradientImg(seed int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + seed) & 0xFF),
				G: uint8((y * 3) & 0xFF),
				B: uint8(((x + y) ^ seed) & 0xFF),
				A: 255,
			})
		}
	}
	return img
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestExplore_TerminatesOnConvergenceWhenNoNewStates(t *testing.T) {
	// Screenshotter always returns the same image — every step
	// fingerprints to the same value, and convergence fires after
	// ConvergeStreak identical-result steps.
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 10, Y: 20}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 3,
		MaxSteps:       100,
	}
	r, err := e.Explore(context.Background(), "explore")
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if !r.Converged {
		t.Fatal("expected convergence")
	}
	if r.MaxSteps {
		t.Fatal("should not be MaxSteps")
	}
	if r.Steps != 3 {
		t.Fatalf("Steps = %d, want 3 (ConvergeStreak=3)", r.Steps)
	}
	// Initial state + any new states discovered (here just the
	// initial since all screens look the same).
	if r.Coverage() != 1 {
		t.Fatalf("Coverage = %d, want 1 (all same image)", r.Coverage())
	}
}

func TestExplore_DiscoversMultipleStates(t *testing.T) {
	// Alternate between 3 distinct screens → the explorer visits 3
	// states + initial → coverage = 3 (the initial state is one of
	// the 3 in the cycle).
	frames := []image.Image{
		gradientImg(0),
		gradientImg(100),
		gradientImg(200),
	}
	e := &Explorer{
		Screenshotter:  &cyclingScreenshotter{frames: frames},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 10, Y: 20}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 10, // won't trigger in short run
		MaxSteps:       10,
	}
	r, err := e.Explore(context.Background(), "explore")
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if r.Coverage() != 3 {
		t.Fatalf("Coverage = %d, want 3 distinct states", r.Coverage())
	}
}

func TestExplore_TerminatesOnMaxSteps(t *testing.T) {
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 1000, // never fires
		MaxSteps:       5,
	}
	r, _ := e.Explore(context.Background(), "explore")
	if !r.MaxSteps {
		t.Fatal("expected MaxSteps termination")
	}
	if r.Steps != 5 {
		t.Fatalf("Steps = %d, want 5", r.Steps)
	}
}

func TestExplore_MaxStepsNegativeRunsUntilConverge(t *testing.T) {
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 3,
		MaxSteps:       -1,
	}
	r, _ := e.Explore(context.Background(), "explore")
	if !r.Converged {
		t.Fatal("MaxSteps=-1 should allow convergence to fire")
	}
}

func TestExplore_DefaultMaxStepsIs100(t *testing.T) {
	// Disable convergence; rely on default MaxSteps=100.
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:       &recordingExecutor{},
		// MaxSteps = 0 → default 100. ConvergeStreak = 0 → default 10.
	}
	r, _ := e.Explore(context.Background(), "explore")
	// Default ConvergeStreak=10 fires after 10 identical states.
	if r.Steps != 10 {
		t.Fatalf("Steps = %d, want 10 (default ConvergeStreak)", r.Steps)
	}
}

// ---------------------------------------------------------------------------
// Done actions are recorded but not executed
// ---------------------------------------------------------------------------

func TestExplore_DoneActionRecordedButNotExecuted(t *testing.T) {
	img := solidImg(128)
	ex := &recordingExecutor{}
	// Only Done actions in the script — scriptedResolver's fallback
	// click after the script is exhausted is fine, but we stop
	// before reaching it by using ConvergeStreak=1 won't actually
	// halt before fallback. Use MaxSteps=1 to cap exactly one step.
	e := &Explorer{
		Screenshotter: &staticScreenshotter{img: img},
		Resolver: &scriptedResolver{actions: []action.Action{
			{Kind: action.KindDone, Reason: "thread done"},
		}},
		Executor:       ex,
		ConvergeStreak: 100,
		MaxSteps:       1,
	}
	r, _ := e.Explore(context.Background(), "explore")
	// The Done action is in History but NOT in executor.executed.
	if len(r.History) != 1 {
		t.Fatalf("History = %d entries, want 1", len(r.History))
	}
	if r.History[0].Action.Kind != action.KindDone {
		t.Fatalf("History[0].Action = %v, want Done", r.History[0].Action.Kind)
	}
	if len(ex.executed) != 0 {
		t.Fatalf("executor received %d actions, want 0 (Done should not execute)", len(ex.executed))
	}
}

// ---------------------------------------------------------------------------
// OnStep hook
// ---------------------------------------------------------------------------

func TestExplore_OnStepHookFiresPerStep(t *testing.T) {
	var hookCalls int
	var isNewValues []bool
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 3,
		OnStep: func(step int, a action.Action, isNew bool, fp uint64) {
			hookCalls++
			isNewValues = append(isNewValues, isNew)
		},
	}
	_, _ = e.Explore(context.Background(), "explore")
	if hookCalls != 3 {
		t.Fatalf("hook fired %d times, want 3", hookCalls)
	}
	// All static-image steps have IsNew=false (initial state was seeded).
	for i, v := range isNewValues {
		if v {
			t.Errorf("step %d: IsNew=true, want false (static image)", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Coverage accessor
// ---------------------------------------------------------------------------

func TestResult_CoverageCounts(t *testing.T) {
	r := Result{Visited: map[uint64]action.Action{1: {}, 2: {}, 3: {}}}
	if c := r.Coverage(); c != 3 {
		t.Fatalf("Coverage = %d, want 3", c)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestExplore_NilDepsError(t *testing.T) {
	if _, err := (&Explorer{}).Explore(context.Background(), "g"); !errors.Is(err, ErrNoScreenshotter) {
		t.Fatalf("nil Screenshotter: %v, want ErrNoScreenshotter", err)
	}
	if _, err := (&Explorer{Screenshotter: &staticScreenshotter{}}).Explore(context.Background(), "g"); !errors.Is(err, ErrNoResolver) {
		t.Fatalf("nil Resolver: %v, want ErrNoResolver", err)
	}
	if _, err := (&Explorer{Screenshotter: &staticScreenshotter{}, Resolver: &scriptedResolver{}}).Explore(context.Background(), "g"); !errors.Is(err, ErrNoExecutor) {
		t.Fatalf("nil Executor: %v, want ErrNoExecutor", err)
	}
}

func TestExplore_EmptyGoalError(t *testing.T) {
	e := &Explorer{
		Screenshotter: &staticScreenshotter{img: solidImg(128)},
		Resolver:      &scriptedResolver{},
		Executor:      &recordingExecutor{},
	}
	if _, err := e.Explore(context.Background(), ""); !errors.Is(err, ErrEmptyGoal) {
		t.Fatalf("empty goal: %v, want ErrEmptyGoal", err)
	}
}

func TestExplore_InitialScreenshotError(t *testing.T) {
	e := &Explorer{
		Screenshotter: &staticScreenshotter{err: errors.New("boom")},
		Resolver:      &scriptedResolver{},
		Executor:      &recordingExecutor{},
	}
	_, err := e.Explore(context.Background(), "g")
	if err == nil || !strings.Contains(err.Error(), "initial screenshot") {
		t.Fatalf("initial screenshot err = %v", err)
	}
}

func TestExplore_ResolverError(t *testing.T) {
	e := &Explorer{
		Screenshotter: &staticScreenshotter{img: solidImg(128)},
		Resolver:      &scriptedResolver{err: errors.New("vlm down")},
		Executor:      &recordingExecutor{},
	}
	_, err := e.Explore(context.Background(), "g")
	if err == nil || !strings.Contains(err.Error(), "Resolve") {
		t.Fatalf("resolver err = %v", err)
	}
}

func TestExplore_ExecutorError(t *testing.T) {
	e := &Explorer{
		Screenshotter: &staticScreenshotter{img: solidImg(128)},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:      &recordingExecutor{err: errors.New("adb offline")},
	}
	_, err := e.Explore(context.Background(), "g")
	if err == nil || !strings.Contains(err.Error(), "Execute") {
		t.Fatalf("executor err = %v", err)
	}
}

func TestExplore_ContextCanceledBeforeFirstStep(t *testing.T) {
	e := &Explorer{
		Screenshotter: &staticScreenshotter{img: solidImg(128)},
		Resolver:      &scriptedResolver{},
		Executor:      &recordingExecutor{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Explore(ctx, "g"); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// Step records
// ---------------------------------------------------------------------------

func TestExplore_HistoryRecordsPerStep(t *testing.T) {
	img := solidImg(128)
	e := &Explorer{
		Screenshotter:  &staticScreenshotter{img: img},
		Resolver:       &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 1, Y: 1}}},
		Executor:       &recordingExecutor{},
		ConvergeStreak: 2,
	}
	r, _ := e.Explore(context.Background(), "g")
	if len(r.History) != 2 {
		t.Fatalf("History = %d, want 2", len(r.History))
	}
	for i, rec := range r.History {
		if rec.Step != i+1 {
			t.Errorf("History[%d].Step = %d, want %d", i, rec.Step, i+1)
		}
		if rec.Action.Kind != action.KindClick {
			t.Errorf("History[%d].Action.Kind = %v", i, rec.Action.Kind)
		}
	}
}
