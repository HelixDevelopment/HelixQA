// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"errors"
	"image"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/agent/action"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

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

// scriptedResolver returns the given sequence of actions in order;
// once the script is exhausted it returns the errOutOfScript.
type scriptedResolver struct {
	actions        []action.Action
	calls          int
	errOutOfScript error
	err            error
}

func (r *scriptedResolver) Resolve(ctx context.Context, img image.Image, goal string) (action.Action, error) {
	if err := ctx.Err(); err != nil {
		return action.Action{}, err
	}
	if r.err != nil {
		return action.Action{}, r.err
	}
	if r.calls >= len(r.actions) {
		if r.errOutOfScript != nil {
			return action.Action{}, r.errOutOfScript
		}
		return action.Action{}, errors.New("resolver out of scripted actions")
	}
	a := r.actions[r.calls]
	r.calls++
	return a, nil
}

type recordingExecutor struct {
	executed []action.Action
	err      error
	errAt    int
	calls    int
}

func (e *recordingExecutor) Execute(ctx context.Context, a action.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.calls++
	if e.err != nil && e.calls == e.errAt {
		return e.err
	}
	e.executed = append(e.executed, a)
	return nil
}

func tinyImg() image.Image { return image.NewRGBA(image.Rect(0, 0, 8, 8)) }

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestRun_TerminatesOnDoneAction(t *testing.T) {
	script := []action.Action{
		{Kind: action.KindClick, X: 10, Y: 20, Reason: "step 1"},
		{Kind: action.KindType, Text: "admin", Reason: "step 2"},
		{Kind: action.KindDone, Reason: "logged in"},
	}
	ex := &recordingExecutor{}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      ex,
	}
	res, err := r.Run(context.Background(), "Log in")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Done || res.Stuck {
		t.Fatalf("flags = (Done=%v, Stuck=%v), want (true, false)", res.Done, res.Stuck)
	}
	if res.Steps != 3 || len(res.History) != 3 {
		t.Fatalf("Steps=%d, History=%d, want 3, 3", res.Steps, len(res.History))
	}
	// Done is recorded but not executed — so the executor saw 2
	// actions (click + type), not 3.
	if len(ex.executed) != 2 {
		t.Fatalf("executor saw %d actions, want 2 (click + type, Done excluded)", len(ex.executed))
	}
}

func TestRun_OnStepHookFiresPerStep(t *testing.T) {
	script := []action.Action{
		{Kind: action.KindClick, X: 10, Y: 20},
		{Kind: action.KindDone},
	}
	var calls int32
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		OnStep:        func(step int, a action.Action) { atomic.AddInt32(&calls, 1) },
	}
	_, err := r.Run(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("OnStep fired %d times, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// Termination — MaxSteps
// ---------------------------------------------------------------------------

func TestRun_TerminatesOnMaxSteps(t *testing.T) {
	// Script that never emits Done — will exhaust MaxSteps.
	script := make([]action.Action, 10)
	for i := range script {
		script[i] = action.Action{Kind: action.KindClick, X: i, Y: i}
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		MaxSteps:      5,
	}
	res, err := r.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Done || res.Stuck {
		t.Fatalf("flags = (Done=%v, Stuck=%v), want both false", res.Done, res.Stuck)
	}
	if res.Steps != 5 || len(res.History) != 5 {
		t.Fatalf("Steps=%d, History=%d, want 5, 5", res.Steps, len(res.History))
	}
}

func TestRun_DefaultMaxStepsIs50(t *testing.T) {
	// Script of 60 identical-but-distinct (X differs) click actions.
	// Default MaxSteps=50 should terminate at step 50.
	script := make([]action.Action, 60)
	for i := range script {
		script[i] = action.Action{Kind: action.KindClick, X: i, Y: 0}
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
	}
	res, _ := r.Run(context.Background(), "go")
	if res.Steps != 50 {
		t.Fatalf("Steps = %d, want 50 (default MaxSteps)", res.Steps)
	}
}

// ---------------------------------------------------------------------------
// Termination — Stuck detection
// ---------------------------------------------------------------------------

func TestRun_StuckDetectionFires(t *testing.T) {
	// All three scripted actions are identical in the Kind+coords
	// sense (the Reason differs but sameCore ignores it). With
	// StuckDetect=3 the runner aborts on the 3rd call.
	script := []action.Action{
		{Kind: action.KindClick, X: 100, Y: 200, Reason: "first try"},
		{Kind: action.KindClick, X: 100, Y: 200, Reason: "second try"},
		{Kind: action.KindClick, X: 100, Y: 200, Reason: "third try"},
		{Kind: action.KindDone}, // never reached
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		StuckDetect:   3,
	}
	res, err := r.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Stuck || res.Done {
		t.Fatalf("flags = (Done=%v, Stuck=%v), want (false, true)", res.Done, res.Stuck)
	}
	if res.Steps != 3 {
		t.Fatalf("Steps = %d, want 3 (aborted on 3rd identical action)", res.Steps)
	}
}

func TestRun_StuckDetectionIgnoredWhenZero(t *testing.T) {
	script := []action.Action{
		{Kind: action.KindClick, X: 100, Y: 200},
		{Kind: action.KindClick, X: 100, Y: 200},
		{Kind: action.KindClick, X: 100, Y: 200},
		{Kind: action.KindDone},
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		// StuckDetect = 0 → never fires, runs to Done.
	}
	res, _ := r.Run(context.Background(), "go")
	if !res.Done {
		t.Fatalf("StuckDetect=0 should not trigger Stuck: Done=%v Stuck=%v", res.Done, res.Stuck)
	}
}

func TestRun_StuckDetectionOneMeansNeverFires(t *testing.T) {
	// StuckDetect=1 would imply "a single action is stuck" which is
	// nonsensical; isStuck returns false for n<2 by design.
	script := []action.Action{
		{Kind: action.KindClick, X: 10, Y: 20},
		{Kind: action.KindDone},
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		StuckDetect:   1,
	}
	res, err := r.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stuck {
		t.Fatal("StuckDetect=1 should never trigger Stuck")
	}
}

func TestRun_StuckNotFiredWhenActionsDiffer(t *testing.T) {
	// Three different clicks — stuck never fires.
	script := []action.Action{
		{Kind: action.KindClick, X: 100, Y: 200},
		{Kind: action.KindClick, X: 150, Y: 250},
		{Kind: action.KindClick, X: 200, Y: 300},
		{Kind: action.KindDone},
	}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		StuckDetect:   3,
	}
	res, _ := r.Run(context.Background(), "go")
	if res.Stuck {
		t.Fatal("distinct actions should not trigger Stuck")
	}
	if !res.Done {
		t.Fatal("should terminate on Done")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestRun_NilScreenshotterError(t *testing.T) {
	r := &Runner{
		Resolver: &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
		Executor: &recordingExecutor{},
	}
	if _, err := r.Run(context.Background(), "go"); !errors.Is(err, ErrNoScreenshotter) {
		t.Fatalf("nil Screenshotter: %v, want ErrNoScreenshotter", err)
	}
}

func TestRun_NilResolverError(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Executor:      &recordingExecutor{},
	}
	if _, err := r.Run(context.Background(), "go"); !errors.Is(err, ErrNoResolver) {
		t.Fatalf("nil Resolver: %v, want ErrNoResolver", err)
	}
}

func TestRun_NilExecutorError(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
	}
	if _, err := r.Run(context.Background(), "go"); !errors.Is(err, ErrNoExecutor) {
		t.Fatalf("nil Executor: %v, want ErrNoExecutor", err)
	}
}

func TestRun_EmptyGoalError(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
		Executor:      &recordingExecutor{},
	}
	if _, err := r.Run(context.Background(), ""); !errors.Is(err, ErrEmptyGoal) {
		t.Fatalf("empty goal: %v, want ErrEmptyGoal", err)
	}
}

func TestRun_ScreenshotErrorWrapsWithStep(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{err: errors.New("capture down")},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
		Executor:      &recordingExecutor{},
	}
	_, err := r.Run(context.Background(), "go")
	if err == nil || !strings.Contains(err.Error(), "step 1") || !strings.Contains(err.Error(), "Screenshot") {
		t.Fatalf("Screenshot error should wrap with step + operation: %v", err)
	}
}

func TestRun_ResolveErrorWrapsWithStep(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{err: errors.New("vlm down")},
		Executor:      &recordingExecutor{},
	}
	_, err := r.Run(context.Background(), "go")
	if err == nil || !strings.Contains(err.Error(), "Resolve") {
		t.Fatalf("Resolve error should wrap: %v", err)
	}
}

func TestRun_ExecuteErrorWrapsWithStep(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindClick, X: 10, Y: 20}}},
		Executor:      &recordingExecutor{err: errors.New("adb offline"), errAt: 1},
	}
	_, err := r.Run(context.Background(), "go")
	if err == nil || !strings.Contains(err.Error(), "Execute") {
		t.Fatalf("Execute error should wrap: %v", err)
	}
}

func TestRun_ContextCanceledBeforeFirstStep(t *testing.T) {
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
		Executor:      &recordingExecutor{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Run(ctx, "go")
	if err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// StepTimeout
// ---------------------------------------------------------------------------

// slowScreenshotter blocks for 200ms — enough to blow a 10ms
// StepTimeout and confirm the per-step context is honored.
type slowScreenshotter struct{}

func (slowScreenshotter) Screenshot(ctx context.Context) (image.Image, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	}
}

func TestRun_StepTimeoutCancelsSlowSteps(t *testing.T) {
	r := &Runner{
		Screenshotter: slowScreenshotter{},
		Resolver:      &scriptedResolver{actions: []action.Action{{Kind: action.KindDone}}},
		Executor:      &recordingExecutor{},
		StepTimeout:   10 * time.Millisecond,
	}
	start := time.Now()
	_, err := r.Run(context.Background(), "go")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("slow step should trigger StepTimeout")
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("StepTimeout didn't cancel fast enough: %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestIsStuck_EmptyHistory(t *testing.T) {
	if isStuck(nil, 3) {
		t.Fatal("empty history should not be stuck")
	}
}

func TestIsStuck_NTooSmall(t *testing.T) {
	h := []action.Action{{Kind: action.KindClick}}
	if isStuck(h, 1) {
		t.Fatal("n<2 should disable stuck detection")
	}
	if isStuck(h, 0) {
		t.Fatal("n=0 should disable stuck detection")
	}
}

func TestIsStuck_HistoryShorterThanN(t *testing.T) {
	h := []action.Action{
		{Kind: action.KindClick, X: 1},
		{Kind: action.KindClick, X: 1},
	}
	if isStuck(h, 3) {
		t.Fatal("history shorter than n should not be stuck")
	}
}

func TestSameCore_IgnoresReason(t *testing.T) {
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20, Reason: "first"}
	b := action.Action{Kind: action.KindClick, X: 10, Y: 20, Reason: "second (different)"}
	if !sameCore(a, b) {
		t.Fatal("sameCore should ignore Reason")
	}
}

func TestSameCore_DistinguishesCoords(t *testing.T) {
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20}
	b := action.Action{Kind: action.KindClick, X: 15, Y: 20}
	if sameCore(a, b) {
		t.Fatal("different coords should be different core")
	}
}

func TestResolverFunc_Adapter(t *testing.T) {
	var called bool
	rf := ResolverFunc(func(ctx context.Context, img image.Image, goal string) (action.Action, error) {
		called = true
		return action.Action{Kind: action.KindDone}, nil
	})
	_, err := rf.Resolve(context.Background(), tinyImg(), "go")
	if err != nil || !called {
		t.Fatalf("ResolverFunc didn't dispatch: called=%v err=%v", called, err)
	}
}

// ---------------------------------------------------------------------------
// UnwrapMaxStepsOrStuck
// ---------------------------------------------------------------------------

func TestUnwrapMaxStepsOrStuck_Done(t *testing.T) {
	if err := UnwrapMaxStepsOrStuck(Result{Done: true}); err != nil {
		t.Fatalf("Done = %v, want nil", err)
	}
}

func TestUnwrapMaxStepsOrStuck_Stuck(t *testing.T) {
	if err := UnwrapMaxStepsOrStuck(Result{Stuck: true}); !errors.Is(err, ErrStuck) {
		t.Fatalf("Stuck = %v, want ErrStuck", err)
	}
}

func TestUnwrapMaxStepsOrStuck_MaxSteps(t *testing.T) {
	if err := UnwrapMaxStepsOrStuck(Result{}); !errors.Is(err, ErrMaxStepsReached) {
		t.Fatalf("bare = %v, want ErrMaxStepsReached", err)
	}
}

// ---------------------------------------------------------------------------
// MaxSteps = -1 (disabled) — must still terminate on Done.
// ---------------------------------------------------------------------------

func TestRun_MaxStepsNegativeAllowsUnboundedLoopUntilDone(t *testing.T) {
	script := make([]action.Action, 100)
	for i := range script[:99] {
		script[i] = action.Action{Kind: action.KindClick, X: i, Y: 0}
	}
	script[99] = action.Action{Kind: action.KindDone}
	r := &Runner{
		Screenshotter: &staticScreenshotter{img: tinyImg()},
		Resolver:      &scriptedResolver{actions: script},
		Executor:      &recordingExecutor{},
		MaxSteps:      -1,
	}
	res, err := r.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Done {
		t.Fatal("MaxSteps=-1 should allow the loop to reach Done")
	}
	if res.Steps != 100 {
		t.Fatalf("Steps = %d, want 100", res.Steps)
	}
}
