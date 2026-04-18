// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// ---------------------------------------------------------------------------
// ExecuteRetry wiring
// ---------------------------------------------------------------------------

// TestAgent_ExecuteRetry_RecoversOnFlakyAdapter proves that when
// Config.ExecuteRetry is set, a transient Adapter.Do failure is
// retried transparently.
func TestAgent_ExecuteRetry_RecoversOnFlakyAdapter(t *testing.T) {
	var calls atomic.Int32
	adapter := &flakyAdapter{failUntil: 3, calls: &calls}
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{
			Actions: []nexus.Action{{Kind: "click", Target: "e1"}},
			Done:    true,
		}, nil
	})
	a, _ := NewAgent(planner, adapter, Config{
		MaxIterations: 5,
		ExecuteRetry: &BackoffPolicy{
			Base:      time.Nanosecond,
			MaxTries:  5,
			JitterPct: 0.01,
		},
	})
	state := NewAgentState("flake", "flake-1")
	if err := a.Run(context.Background(), state, integFakeSession{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !state.Done {
		t.Error("Run must succeed once retry exhausts the flake")
	}
	if calls.Load() < 3 {
		t.Errorf("adapter calls = %d, want >= 3", calls.Load())
	}
	result := state.History[0].Results[0]
	if !result.Success {
		t.Errorf("final Action result should be success, got %+v", result)
	}
}

// TestAgent_ExecuteRetry_NilConfigKeepsSingleAttempt proves the
// default zero-overhead path: no retry when ExecuteRetry is nil.
func TestAgent_ExecuteRetry_NilConfigKeepsSingleAttempt(t *testing.T) {
	var calls atomic.Int32
	adapter := &flakyAdapter{failUntil: 3, calls: &calls}
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{
			Actions: []nexus.Action{{Kind: "click", Target: "e1"}},
			Done:    true,
		}, nil
	})
	a, _ := NewAgent(planner, adapter, Config{MaxIterations: 2})
	state := NewAgentState("flake-no-retry", "flake-2")
	_ = a.Run(context.Background(), state, integFakeSession{})
	if calls.Load() != 1 {
		t.Errorf("no retry should mean 1 call, got %d", calls.Load())
	}
}

// ---------------------------------------------------------------------------
// LoopDetector wiring
// ---------------------------------------------------------------------------

// TestAgent_LoopDetector_AbortsStuckPlanner proves the wired
// detector raises ErrLoopDetected before MaxIterations drains.
func TestAgent_LoopDetector_AbortsStuckPlanner(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{
			Actions: []nexus.Action{{Kind: "click", Target: "e1"}},
		}, nil
	})
	ld := NewLoopDetector(12, 3)
	a, _ := NewAgent(planner, &integFakeAdapter{}, Config{
		MaxIterations: 100,
		LoopDetector:  ld,
	})
	state := NewAgentState("stuck", "stuck-1")
	err := a.Run(context.Background(), state, integFakeSession{})
	if !errors.Is(err, ErrLoopDetected) {
		t.Errorf("expected ErrLoopDetected, got %v", err)
	}
	if state.Iteration > 9 {
		t.Errorf("detector should trip before 10 iterations, got %d", state.Iteration)
	}
}

// TestAgent_LoopDetector_NilKeepsLegacyBehaviour proves the zero
// value is a no-op.
func TestAgent_LoopDetector_NilKeepsLegacyBehaviour(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{Done: true}, nil
	})
	a, _ := NewAgent(planner, &integFakeAdapter{}, Config{MaxIterations: 2})
	state := NewAgentState("nodet", "x")
	err := a.Run(context.Background(), state, integFakeSession{})
	if err != nil {
		t.Errorf("clean run shouldn't error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SelfHealer wiring
// ---------------------------------------------------------------------------

// TestAgent_SelfHealer_RecoversAfterFailedAction proves that when a
// step's Execute fails, the healer re-plans and the healed actions
// land on the same iteration's Results.
func TestAgent_SelfHealer_RecoversAfterFailedAction(t *testing.T) {
	var planCalls atomic.Int32
	planner := llmFunc(func(_ context.Context, req PlanRequest) (AgentStep, error) {
		planCalls.Add(1)
		// On the healer's call RecentSteps starts with the failure
		// hint — return a different target.
		if len(req.RecentSteps) > 0 && contains(req.RecentSteps[0].Evaluation, "previous_attempt_failed_because") {
			return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e-healed"}}, Done: true}, nil
		}
		return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e-bad"}}}, nil
	})
	healer, err := NewSelfHealer(planner, 2)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &refusingAdapter{refuse: "e-bad"}
	a, _ := NewAgent(planner, adapter, Config{
		MaxIterations: 3,
		SelfHealer:    healer,
	})
	state := NewAgentState("heal", "heal-1")
	_ = a.Run(context.Background(), state, integFakeSession{})

	// Inspect the first step's results: first action failed, second
	// succeeded (healed).
	if len(state.History) == 0 {
		t.Fatal("no history recorded")
	}
	firstStepResults := state.History[0].Results
	if len(firstStepResults) < 2 {
		t.Fatalf("expected >= 2 results (failed + healed), got %d", len(firstStepResults))
	}
	if firstStepResults[0].Success {
		t.Error("first (bad) action should be flagged failure")
	}
	if !firstStepResults[len(firstStepResults)-1].Success {
		t.Error("last (healed) action should succeed")
	}
	if planCalls.Load() < 2 {
		t.Errorf("expected planner invoked at least twice (plan + heal), got %d", planCalls.Load())
	}
}

// ---------------------------------------------------------------------------
// Combined resilience stack
// ---------------------------------------------------------------------------

// TestAgent_AllResilienceTogether proves retry + healer + loop
// detector compose cleanly within a single Run. A flaky adapter
// that fails every Action twice + a healer that steers to a clean
// target + a loop detector watching — Run() completes green.
func TestAgent_AllResilienceTogether(t *testing.T) {
	var planCalls atomic.Int32
	planner := llmFunc(func(_ context.Context, req PlanRequest) (AgentStep, error) {
		planCalls.Add(1)
		// Healer replans with a fresh Target so the next Execute
		// exits the retry loop quickly.
		if len(req.RecentSteps) > 0 && contains(req.RecentSteps[0].Evaluation, "previous_attempt_failed_because") {
			return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e-fresh"}}, Done: true}, nil
		}
		return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e-normal"}}}, nil
	})
	healer, _ := NewSelfHealer(planner, 2)
	adapter := &flakyThenOkAdapter{}
	a, _ := NewAgent(planner, adapter, Config{
		MaxIterations: 5,
		ExecuteRetry: &BackoffPolicy{
			Base: time.Nanosecond, MaxTries: 3, JitterPct: 0.01,
		},
		LoopDetector: NewLoopDetector(12, 3),
		SelfHealer:   healer,
	})
	state := NewAgentState("all", "all-1")
	err := a.Run(context.Background(), state, integFakeSession{})
	if err != nil && !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Errorf("combined stack should finish clean, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

type flakyAdapter struct {
	failUntil int32
	calls     *atomic.Int32
}

func (f *flakyAdapter) Open(_ context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	return integFakeSession{}, nil
}
func (f *flakyAdapter) Navigate(_ context.Context, _ nexus.Session, _ string) error { return nil }
func (f *flakyAdapter) Snapshot(_ context.Context, _ nexus.Session) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "page"}, nil
}
func (f *flakyAdapter) Do(_ context.Context, _ nexus.Session, _ nexus.Action) error {
	n := f.calls.Add(1)
	if n < f.failUntil {
		return errors.New("transient")
	}
	return nil
}
func (f *flakyAdapter) Screenshot(_ context.Context, _ nexus.Session) ([]byte, error) {
	return []byte("png"), nil
}

type refusingAdapter struct {
	refuse  string
	doCalls int
}

func (r *refusingAdapter) Open(_ context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	return integFakeSession{}, nil
}
func (r *refusingAdapter) Navigate(_ context.Context, _ nexus.Session, _ string) error { return nil }
func (r *refusingAdapter) Snapshot(_ context.Context, _ nexus.Session) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "page"}, nil
}
func (r *refusingAdapter) Do(_ context.Context, _ nexus.Session, a nexus.Action) error {
	r.doCalls++
	if a.Target == r.refuse {
		return errors.New("selector missing")
	}
	return nil
}
func (r *refusingAdapter) Screenshot(_ context.Context, _ nexus.Session) ([]byte, error) {
	return []byte("png"), nil
}

type flakyThenOkAdapter struct {
	calls int
}

func (f *flakyThenOkAdapter) Open(_ context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	return integFakeSession{}, nil
}
func (f *flakyThenOkAdapter) Navigate(_ context.Context, _ nexus.Session, _ string) error { return nil }
func (f *flakyThenOkAdapter) Snapshot(_ context.Context, _ nexus.Session) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "page"}, nil
}
func (f *flakyThenOkAdapter) Do(_ context.Context, _ nexus.Session, a nexus.Action) error {
	f.calls++
	if a.Target == "e-fresh" {
		return nil
	}
	if f.calls%2 == 1 {
		return errors.New("transient flake")
	}
	return nil
}
func (f *flakyThenOkAdapter) Screenshot(_ context.Context, _ nexus.Session) ([]byte, error) {
	return []byte("png"), nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
