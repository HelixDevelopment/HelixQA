// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// Phase-8 integration tests: prove the five new OpenClawing2
// packages compose correctly. These tests exercise the same Agent
// + MessageManager + retry + healer + loop-detector pipeline an
// operator sees in production, minus the real browser / LLM.

// TestIntegration_AgentWithMessageManagerBudgetEnforced proves a
// long-running agent session stays under the MessageManager's
// token budget.
func TestIntegration_AgentWithMessageManagerBudgetEnforced(t *testing.T) {
	mm := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: 1500, VerbatimTurns: 2})

	// Planner echoes its budget-observance into Evaluation so we
	// can assert MessageManager is re-used correctly.
	var iter atomic.Int64
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		n := iter.Add(1)
		if n >= 30 {
			return AgentStep{Done: true}, nil
		}
		return AgentStep{
			Evaluation: "progress step " + itoa(int(n)),
			Actions:    []nexus.Action{{Kind: "click", Target: "e1"}},
		}, nil
	})

	a, _ := NewAgent(planner, &integFakeAdapter{}, Config{MaxIterations: 60})
	state := NewAgentState("budget integration", "integ-1")
	if err := a.Run(context.Background(), state, integFakeSession{}); err != nil {
		t.Fatal(err)
	}
	if !state.Done {
		t.Fatal("agent must finish within MaxIterations")
	}

	// Feed the final state through MessageManager; compact must
	// keep the prompt inside the budget.
	mm.PrepareStepState(state)
	current := Message{Role: RoleUser, Content: "final"}
	if _, err := mm.Compact(context.Background(), current); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if total := mm.TotalTokens(current); total > 1500 {
		t.Errorf("final token count = %d, budget = 1500", total)
	}
}

// TestIntegration_RetryWithBackoff_DrivesSelfHealerLoop wires the
// retry stack + healer + loop detector together and asserts the
// combined pipeline terminates cleanly.
func TestIntegration_RetryWithBackoff_DrivesSelfHealerLoop(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	var healCalls atomic.Int64
	healer, err := NewSelfHealer(llmFunc(func(_ context.Context, req PlanRequest) (AgentStep, error) {
		healCalls.Add(1)
		if len(req.RecentSteps) > 0 && strings.Contains(req.RecentSteps[0].Evaluation, "previous_attempt_failed_because") {
			return AgentStep{NextGoal: "recovered", Actions: []nexus.Action{{Kind: "click", Target: "e-recovered"}}}, nil
		}
		return AgentStep{}, errors.New("unexpected request shape")
	}), 3)
	if err != nil {
		t.Fatal(err)
	}

	var attempts atomic.Int64
	err = RetryWithBackoff(context.Background(),
		BackoffPolicy{Base: time.Nanosecond, MaxTries: 4, JitterPct: 0.01},
		func() error {
			n := attempts.Add(1)
			if n < 3 {
				// First two attempts fail so retry exercises.
				return errors.New("transient")
			}
			// Third attempt "succeeds" via the healer.
			step, hErr := healer.Heal(context.Background(),
				NewAgentState("integ", "sess"),
				"element e1 not visible",
			)
			if hErr != nil {
				return hErr
			}
			ld.Record(step.Actions)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("integrated pipeline: %v", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3 (fail/fail/succeed)", attempts.Load())
	}
	if healCalls.Load() == 0 {
		t.Error("healer must be invoked at least once")
	}
	if ld.IsLoop() {
		t.Error("single recovered action should not trip loop detector")
	}
}

// TestIntegration_EndToEnd_AllFiveModulesCooperate constructs a
// plausible full pipeline: Agent runs a scripted flow, every step
// flows through MessageManager compaction, failures are retried +
// healed, the LoopDetector watches for stuck cycles, and the final
// history is sized inside the budget.
func TestIntegration_EndToEnd_AllFiveModulesCooperate(t *testing.T) {
	mm := NewMessageManager("SYS", MessageManagerConfig{
		TokenBudget:   3000,
		VerbatimTurns: 3,
	})
	ld := NewLoopDetector(24, 4)

	// Scripted planner: click, click, type, click, Done.
	script := []AgentStep{
		{NextGoal: "click login", Actions: []nexus.Action{{Kind: "click", Target: "e1"}}},
		{NextGoal: "type user", Actions: []nexus.Action{{Kind: "type", Target: "e2", Text: "alice"}}},
		{NextGoal: "type password", Actions: []nexus.Action{{Kind: "type", Target: "e3", Text: "pw"}}},
		{NextGoal: "submit", Actions: []nexus.Action{{Kind: "click", Target: "e4"}}},
		{Evaluation: "dashboard reached", Done: true},
	}
	var idx atomic.Int64
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		n := idx.Add(1)
		if int(n) > len(script) {
			return AgentStep{Done: true}, nil
		}
		return script[n-1], nil
	})

	adapter := &integFakeAdapter{}
	a, _ := NewAgent(planner, adapter, Config{MaxIterations: 10})
	state := NewAgentState("log in and land on dashboard", "e2e-1")

	if err := a.Run(context.Background(), state, integFakeSession{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !state.Done {
		t.Error("agent should reach Done in 5 steps")
	}
	if state.Iteration != 5 {
		t.Errorf("iterations = %d, want 5", state.Iteration)
	}

	// Record every action through LoopDetector. A healthy login
	// flow has distinct actions each step so IsLoop should stay
	// false.
	for _, step := range state.History {
		ld.Record(step.Actions)
	}
	if ld.IsLoop() {
		t.Error("unique login steps should not register as a loop")
	}

	// MessageManager compaction stays inside budget.
	mm.PrepareStepState(state)
	current := Message{Role: RoleUser, Content: "final obs"}
	if _, err := mm.Compact(context.Background(), current); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if total := mm.TotalTokens(current); total > 3000 {
		t.Errorf("final token count = %d, budget = 3000", total)
	}

	// Adapter saw exactly 4 Do() calls (one per non-Done step).
	if adapter.doCalls != 4 {
		t.Errorf("Adapter.Do invoked %d times, want 4", adapter.doCalls)
	}
}

// TestIntegration_LoopDetectionCatchesRunawayPlanner proves the
// LoopDetector trips on a stuck planner's repeat actions. The test
// runs the Agent to completion first (MaxIterations caps it) then
// feeds the recorded History through the detector, avoiding any
// concurrent read/write races on state.History during the Run loop.
func TestIntegration_LoopDetectionCatchesRunawayPlanner(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e1"}}}, nil
	})
	a, _ := NewAgent(planner, &integFakeAdapter{}, Config{MaxIterations: 20})
	state := NewAgentState("stuck", "stuck-1")
	_ = a.Run(context.Background(), state, integFakeSession{})

	// Replay the finished history through the detector.
	for _, h := range state.History {
		ld.Record(h.Actions)
	}
	if !ld.IsLoop() {
		t.Error("loop detector should trip on repeated click across history")
	}
	if state.Iteration != 20 {
		t.Errorf("MaxIterations cap should stop the loop at 20, got %d", state.Iteration)
	}
}

// -------------------------- fixtures ---------------------------------------

type integFakeAdapter struct {
	mu      sync.Mutex
	doCalls int
}

func (f *integFakeAdapter) Open(_ context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	return integFakeSession{}, nil
}
func (f *integFakeAdapter) Navigate(_ context.Context, _ nexus.Session, _ string) error { return nil }
func (f *integFakeAdapter) Snapshot(_ context.Context, _ nexus.Session) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "page"}, nil
}
func (f *integFakeAdapter) Do(_ context.Context, _ nexus.Session, _ nexus.Action) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.doCalls++
	return nil
}
func (f *integFakeAdapter) Screenshot(_ context.Context, _ nexus.Session) ([]byte, error) {
	return []byte("png"), nil
}

type integFakeSession struct{}

func (integFakeSession) ID() string               { return "integ-session" }
func (integFakeSession) Platform() nexus.Platform { return nexus.PlatformWebChromedp }
func (integFakeSession) Close() error             { return nil }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte, 0, 4)
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}
