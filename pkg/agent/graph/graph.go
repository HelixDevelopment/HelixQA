// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package graph is the stateful agent-loop runner for HelixQA Phase 3.
// It composes the three narrow contracts that together drive a
// goal-directed agent:
//
//   - Screenshotter — captures the current display (pkg/capture/* or
//     any fake that returns a static image).
//   - Resolver      — decides the next action given screenshot + goal
//     (pkg/agent/ground.Grounder is the canonical implementation; a
//     bare UI-TARS client also satisfies this via a trivial shim).
//   - Executor      — carries the action out on the device (pkg/
//     navigator/linux/uinput, pkg/bridge/scrcpy, etc.).
//
// The Runner loops: Screenshot → Resolve → Execute → record, until
// one of four termination conditions:
//
//  1. Resolver returns action.KindDone — the VLM decided the goal is
//     met.
//  2. Step count reaches MaxSteps.
//  3. Stuck detection: the last StuckDetect actions are all identical
//     (excluding Reason). The agent is cycling on the same screen and
//     should abort.
//  4. Context cancellation.
//
// LangGraph-style graphs (conditional edges, parallel branches, named
// nodes) are not modeled here — real HelixQA workloads are linear
// observe-act loops and the extra ceremony adds cost without value.
// If future needs require branching, build on top of Runner.
package graph

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	"digital.vasic.helixqa/pkg/agent/action"
)

// Screenshotter captures the current display. A zero-overhead
// abstraction so tests inject static PNG fixtures without touching
// the capture stack.
type Screenshotter interface {
	Screenshot(ctx context.Context) (image.Image, error)
}

// Resolver decides the next action. Satisfied by *ground.Grounder;
// a bare UI-TARS client satisfies it via a trivial helper
// (see ResolverFunc below) when no detector is available.
type Resolver interface {
	Resolve(ctx context.Context, screenshot image.Image, goal string) (action.Action, error)
}

// ResolverFunc adapts a plain function into a Resolver. Useful for
// the UI-TARS-only path (no detector) or for test fakes that return
// a scripted sequence of actions.
type ResolverFunc func(ctx context.Context, screenshot image.Image, goal string) (action.Action, error)

// Resolve satisfies Resolver for ResolverFunc.
func (f ResolverFunc) Resolve(ctx context.Context, screenshot image.Image, goal string) (action.Action, error) {
	return f(ctx, screenshot, goal)
}

// Executor carries an action out. Returns nil on success.
type Executor interface {
	Execute(ctx context.Context, a action.Action) error
}

// Runner drives the agent loop.
type Runner struct {
	Screenshotter Screenshotter
	Resolver      Resolver
	Executor      Executor

	// MaxSteps caps the loop length. Default 50. Set < 0 to disable
	// (dangerous — the loop can only terminate via Done / Stuck /
	// ctx then).
	MaxSteps int

	// StepTimeout applies to each iteration of Screenshot +
	// Resolve + Execute (the triple runs under a single child
	// context with this timeout). Zero disables — use the parent
	// context's deadline instead.
	StepTimeout time.Duration

	// OnStep fires after each successful action is executed. Useful
	// for session-logging hooks. Optional.
	OnStep func(step int, a action.Action)

	// StuckDetect — when > 0, the runner aborts if the last
	// StuckDetect actions are all identical (ignoring Reason). 0
	// disables stuck detection.
	StuckDetect int
}

// Sentinel errors.
var (
	ErrNoScreenshotter = errors.New("helixqa/agent/graph: Runner.Screenshotter is nil")
	ErrNoResolver      = errors.New("helixqa/agent/graph: Runner.Resolver is nil")
	ErrNoExecutor      = errors.New("helixqa/agent/graph: Runner.Executor is nil")
	ErrMaxStepsReached = errors.New("helixqa/agent/graph: max steps reached without Done")
	ErrStuck           = errors.New("helixqa/agent/graph: agent stuck — same action repeated")
	ErrEmptyGoal       = errors.New("helixqa/agent/graph: empty goal")
)

// Result is the outcome of a Run call.
type Result struct {
	// History is every action the agent emitted, in order. The last
	// entry may be the Done action (if Done=true).
	History []action.Action

	// Done is true when the agent terminated because it emitted a
	// KindDone action.
	Done bool

	// Stuck is true when the agent terminated because the last
	// StuckDetect actions were identical.
	Stuck bool

	// Steps is the total number of loop iterations that produced an
	// executed action (equivalent to len(History)).
	Steps int
}

// Run drives the agent loop toward the given goal.
//
// Returns an error only on hard failures (nil dependency, hard step
// failure, parent ctx cancellation). MaxSteps reached and Stuck
// detection return a successful (err=nil) Result with the appropriate
// flag set, so callers can choose how to react.
func (r *Runner) Run(ctx context.Context, goal string) (Result, error) {
	if r.Screenshotter == nil {
		return Result{}, ErrNoScreenshotter
	}
	if r.Resolver == nil {
		return Result{}, ErrNoResolver
	}
	if r.Executor == nil {
		return Result{}, ErrNoExecutor
	}
	if goal == "" {
		return Result{}, ErrEmptyGoal
	}

	maxSteps := r.MaxSteps
	if maxSteps == 0 {
		maxSteps = 50
	}

	var result Result
	for step := 1; maxSteps < 0 || step <= maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		if r.StepTimeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, r.StepTimeout)
		}

		a, runErr := r.stepOnce(stepCtx, goal)
		if cancel != nil {
			cancel()
		}
		if runErr != nil {
			return result, fmt.Errorf("step %d: %w", step, runErr)
		}

		result.History = append(result.History, a)
		result.Steps++
		if r.OnStep != nil {
			r.OnStep(step, a)
		}

		if a.Kind == action.KindDone {
			result.Done = true
			return result, nil
		}

		if r.StuckDetect > 0 && isStuck(result.History, r.StuckDetect) {
			result.Stuck = true
			return result, nil
		}
	}

	// MaxSteps reached without Done.
	return result, nil
}

// stepOnce performs a single screenshot → resolve → execute cycle.
// Returns the executed action on success.
func (r *Runner) stepOnce(ctx context.Context, goal string) (action.Action, error) {
	img, err := r.Screenshotter.Screenshot(ctx)
	if err != nil {
		return action.Action{}, fmt.Errorf("Screenshot: %w", err)
	}
	a, err := r.Resolver.Resolve(ctx, img, goal)
	if err != nil {
		return action.Action{}, fmt.Errorf("Resolve: %w", err)
	}
	// Done actions are recorded but never executed — the agent
	// declares the goal met and halts.
	if a.Kind == action.KindDone {
		return a, nil
	}
	if err := r.Executor.Execute(ctx, a); err != nil {
		return action.Action{}, fmt.Errorf("Execute: %w", err)
	}
	return a, nil
}

// isStuck reports whether the last n actions in history are all
// identical (ignoring Reason, which may differ across calls for the
// same logical action). Returns false when history is shorter than n.
func isStuck(history []action.Action, n int) bool {
	if n < 2 || len(history) < n {
		return false
	}
	first := history[len(history)-1]
	for i := len(history) - 2; i >= len(history)-n; i-- {
		if !sameCore(history[i], first) {
			return false
		}
	}
	return true
}

// sameCore compares two Actions ignoring their Reason field. The
// Reason is VLM-generated prose and differs across calls even when
// the underlying action is semantically identical — we want Stuck
// detection to fire on "click (100, 200)" loops regardless of the
// VLM's evolving rationalizations.
func sameCore(a, b action.Action) bool {
	a.Reason = ""
	b.Reason = ""
	return a == b
}

// Compile-time guards.
var (
	_ Resolver = ResolverFunc(nil)
)

// UnwrapMaxStepsOrStuck returns a sentinel error if the Result
// indicates a MaxSteps or Stuck termination WITHOUT Done. Callers
// that treat these as failures can wrap their Result with this:
//
//	result, err := runner.Run(ctx, "Log in")
//	if err != nil { return err }
//	if err := graph.UnwrapMaxStepsOrStuck(result); err != nil { return err }
//
// Returns nil for a clean Done result.
func UnwrapMaxStepsOrStuck(r Result) error {
	switch {
	case r.Done:
		return nil
	case r.Stuck:
		return ErrStuck
	default:
		return ErrMaxStepsReached
	}
}
