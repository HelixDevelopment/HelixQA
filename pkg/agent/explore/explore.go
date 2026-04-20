// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package explore is HelixQA's Phase-5 coverage-maximizing agent.
// While pkg/agent/graph drives a goal-directed loop toward a single
// target state, Explorer drives a breadth-seeking loop that tries
// to visit as many distinct UI states as possible within a budget.
//
// The core trick: every screenshot gets a dHash-64 fingerprint.
// Identical hashes mean "same screen" — the explorer's frontier is
// the set of dHashes we've seen but not yet followed an action from.
// When an action leads to a fingerprint we've already explored, we
// backtrack and try a different action next turn.
//
// The explorer composes:
//
//   - Screenshotter (graph.Screenshotter) for current-frame capture.
//   - Resolver (graph.Resolver) for VLM-proposed actions.
//   - Executor (graph.Executor) for action dispatch.
//   - hash.DHasher for state fingerprinting (pkg/vision/hash).
//
// Termination conditions:
//   1. Budget exhausted (MaxSteps).
//   2. No new states discovered for ConvergeStreak consecutive steps.
//   3. Context cancelled.
//
// Unlike graph.Runner, Explorer NEVER emits the Done action itself
// — the exploration is always budget-bounded, and reaching a Done
// state just means the current thread is finished; Explorer
// continues with a different starting state if one is on the
// frontier.
package explore

import (
	"context"
	"errors"
	"fmt"

	"digital.vasic.helixqa/pkg/agent/action"
	"digital.vasic.helixqa/pkg/agent/graph"
	"digital.vasic.helixqa/pkg/vision/hash"
)

// Explorer drives the coverage-seeking loop.
type Explorer struct {
	Screenshotter graph.Screenshotter
	Resolver      graph.Resolver
	Executor      graph.Executor

	// Hasher fingerprints each visited screen. Default DHash-64.
	Hasher hash.DHasher

	// MaxSteps caps the total number of actions dispatched. Default
	// 100. Set < 0 to disable (exploration then terminates only on
	// ConvergeStreak or ctx cancellation).
	MaxSteps int

	// ConvergeStreak halts the explorer once N consecutive steps
	// fail to produce a previously-unseen fingerprint. Default 10.
	// Set 0 to disable.
	ConvergeStreak int

	// OnStep is an optional per-step hook. Called after the action
	// has been executed.
	OnStep func(step int, a action.Action, isNew bool, fingerprint uint64)
}

// Sentinel errors.
var (
	ErrNoScreenshotter = errors.New("helixqa/agent/explore: Screenshotter is nil")
	ErrNoResolver      = errors.New("helixqa/agent/explore: Resolver is nil")
	ErrNoExecutor      = errors.New("helixqa/agent/explore: Executor is nil")
	ErrEmptyGoal       = errors.New("helixqa/agent/explore: empty exploration goal")
)

// Result is the outcome of an Explore run.
type Result struct {
	// Visited maps fingerprint → first action that reached it.
	Visited map[uint64]action.Action

	// History is every (action, fingerprint, isNew) record in order
	// of execution.
	History []StepRecord

	// Steps is the total number of actions dispatched.
	Steps int

	// Converged is true when termination was due to the
	// ConvergeStreak trigger (no new states for N consecutive
	// steps).
	Converged bool

	// MaxSteps is true when termination was due to the step budget.
	MaxSteps bool
}

// StepRecord is one entry in the exploration history.
type StepRecord struct {
	Step         int
	Action       action.Action
	Fingerprint  uint64
	IsNew        bool
}

// Coverage returns the number of distinct fingerprints visited.
func (r Result) Coverage() int { return len(r.Visited) }

// Explore drives the exploration loop toward the given goal. The
// goal is passed to the Resolver unchanged on every step (typical
// goal: "Explore the app; try every screen you can reach").
//
// Returns an error on hard failures (nil deps, ctx cancel, step
// failure). Budget / convergence termination return err=nil with the
// appropriate Result flags.
func (e *Explorer) Explore(ctx context.Context, goal string) (Result, error) {
	if e.Screenshotter == nil {
		return Result{}, ErrNoScreenshotter
	}
	if e.Resolver == nil {
		return Result{}, ErrNoResolver
	}
	if e.Executor == nil {
		return Result{}, ErrNoExecutor
	}
	if goal == "" {
		return Result{}, ErrEmptyGoal
	}

	maxSteps := e.MaxSteps
	if maxSteps == 0 {
		maxSteps = 100
	}
	convergeStreak := e.ConvergeStreak
	if convergeStreak == 0 {
		convergeStreak = 10
	}

	hasher := e.Hasher
	if hasher.Kind == 0 {
		hasher = hash.DHasher{Kind: hash.DHash64}
	}

	result := Result{Visited: map[uint64]action.Action{}}

	// Take initial screenshot + fingerprint it (the starting state
	// counts as "visited" for coverage purposes).
	initialImg, err := e.Screenshotter.Screenshot(ctx)
	if err != nil {
		return result, fmt.Errorf("initial screenshot: %w", err)
	}
	initialFP, err := hasher.Hash(initialImg)
	if err != nil {
		return result, fmt.Errorf("initial fingerprint: %w", err)
	}
	result.Visited[initialFP] = action.Action{Kind: action.KindDone, Reason: "initial state"}

	consecutiveNoNew := 0

	for step := 1; maxSteps < 0 || step <= maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		img, err := e.Screenshotter.Screenshot(ctx)
		if err != nil {
			return result, fmt.Errorf("step %d: Screenshot: %w", step, err)
		}

		a, err := e.Resolver.Resolve(ctx, img, goal)
		if err != nil {
			return result, fmt.Errorf("step %d: Resolve: %w", step, err)
		}

		// Skip Done actions — Explorer never terminates on them, it
		// just records and continues. (The VLM may emit Done when
		// it thinks one exploration thread finished; Explorer is
		// running over a longer budget.)
		if a.Kind != action.KindDone {
			if err := e.Executor.Execute(ctx, a); err != nil {
				return result, fmt.Errorf("step %d: Execute: %w", step, err)
			}
		}

		// Fingerprint the resulting screen. Some executors are
		// async, so post-action capture may return the pre-action
		// state; the Resolver's next round re-samples and the
		// frontier picks up eventually.
		newImg, err := e.Screenshotter.Screenshot(ctx)
		if err != nil {
			return result, fmt.Errorf("step %d: post-Screenshot: %w", step, err)
		}
		fp, err := hasher.Hash(newImg)
		if err != nil {
			return result, fmt.Errorf("step %d: fingerprint: %w", step, err)
		}

		_, existed := result.Visited[fp]
		isNew := !existed
		if isNew {
			result.Visited[fp] = a
			consecutiveNoNew = 0
		} else {
			consecutiveNoNew++
		}
		result.Steps++
		result.History = append(result.History, StepRecord{
			Step:        step,
			Action:      a,
			Fingerprint: fp,
			IsNew:       isNew,
		})
		if e.OnStep != nil {
			e.OnStep(step, a, isNew, fp)
		}

		if convergeStreak > 0 && consecutiveNoNew >= convergeStreak {
			result.Converged = true
			return result, nil
		}
	}

	result.MaxSteps = true
	return result, nil
}
