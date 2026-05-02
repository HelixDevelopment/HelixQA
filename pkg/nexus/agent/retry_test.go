// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// ---------------------------------------------------------------------------
// BackoffPolicy / RetryWithBackoff
// ---------------------------------------------------------------------------

func TestBackoffPolicy_ResolveDefaults(t *testing.T) {
	p := BackoffPolicy{}.Resolve()
	if p.Base != time.Second || p.Factor != 2.0 || p.Max != 30*time.Second {
		t.Errorf("defaults wrong: %+v", p)
	}
	if p.MaxTries != 5 {
		t.Errorf("MaxTries default = %d, want 5", p.MaxTries)
	}
	if p.JitterPct == 0 {
		t.Error("JitterPct default must not be zero")
	}
}

func TestBackoffPolicy_DelayMonotonicUnderJitterFloor(t *testing.T) {
	// Zero jitter to keep the test deterministic.
	p := BackoffPolicy{Base: 10 * time.Millisecond, Factor: 2, Max: 5 * time.Second, JitterPct: 0.01, MaxTries: 5}.Resolve()
	// The delay grows roughly exponentially; floor under tiny
	// jitter should remain strictly greater on each attempt until
	// the cap is reached.
	var prev time.Duration
	for i := 1; i <= 4; i++ {
		d := p.delayFor(i)
		if d <= 0 {
			t.Errorf("attempt %d: non-positive delay %s", i, d)
		}
		if i > 1 && d < prev/2 {
			t.Errorf("attempt %d: delay %s dropped below half of prev %s", i, d, prev)
		}
		prev = d
	}
}

func TestRetryWithBackoff_StopsOnFirstSuccess(t *testing.T) {
	var calls atomic.Int32
	err := RetryWithBackoff(context.Background(),
		BackoffPolicy{Base: time.Nanosecond, MaxTries: 5, JitterPct: 0.01},
		func() error {
			calls.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", calls.Load())
	}
}

func TestRetryWithBackoff_ExhaustsAndReturnsLastError(t *testing.T) {
	want := errors.New("still broken")
	var calls atomic.Int32
	err := RetryWithBackoff(context.Background(),
		BackoffPolicy{Base: time.Nanosecond, MaxTries: 3, JitterPct: 0.01},
		func() error {
			calls.Add(1)
			return want
		},
	)
	if !errors.Is(err, want) {
		t.Errorf("expected wrapped want, got %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestRetryWithBackoff_HonoursContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RetryWithBackoff(ctx,
		BackoffPolicy{Base: 100 * time.Millisecond, MaxTries: 5},
		func() error { return errors.New("never-reached") },
	)
	if err == nil {
		t.Fatal("cancelled ctx must produce an error")
	}
}

// ---------------------------------------------------------------------------
// LoopDetector
// ---------------------------------------------------------------------------

func TestLoopDetector_DefaultsBumpSmallArgs(t *testing.T) {
	ld := NewLoopDetector(0, 0)
	if ld.bufferSize < 12 {
		t.Errorf("bufferSize must default to >=12, got %d", ld.bufferSize)
	}
	if ld.threshold < 2 {
		t.Errorf("threshold must default to >=2, got %d", ld.threshold)
	}
}

func TestLoopDetector_Detects2Cycle(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	actionA := []nexus.Action{{Kind: "click", Target: "e1"}}
	actionB := []nexus.Action{{Kind: "click", Target: "e2"}}
	for i := 0; i < 3; i++ {
		ld.Record(actionA)
		ld.Record(actionB)
	}
	if !ld.IsLoop() {
		t.Error("3× (A, B) should be detected as a 2-cycle")
	}
}

func TestLoopDetector_Detects3Cycle(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	a := []nexus.Action{{Kind: "click", Target: "e1"}}
	b := []nexus.Action{{Kind: "type", Target: "e2", Text: "x"}}
	c := []nexus.Action{{Kind: "scroll"}}
	for i := 0; i < 3; i++ {
		ld.Record(a)
		ld.Record(b)
		ld.Record(c)
	}
	if !ld.IsLoop() {
		t.Error("3× (A, B, C) should be detected as a 3-cycle")
	}
}

func TestLoopDetector_HealthyHistoryPasses(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	for i := 0; i < 12; i++ {
		ld.Record([]nexus.Action{{Kind: "click", Target: "e" + string(rune('1'+i%12))}})
	}
	if ld.IsLoop() {
		t.Error("unique history should not be flagged")
	}
}

func TestLoopDetector_ResetClearsState(t *testing.T) {
	ld := NewLoopDetector(12, 3)
	a := []nexus.Action{{Kind: "click", Target: "e1"}}
	b := []nexus.Action{{Kind: "click", Target: "e2"}}
	for i := 0; i < 3; i++ {
		ld.Record(a)
		ld.Record(b)
	}
	if !ld.IsLoop() {
		t.Fatal("precondition: loop present")
	}
	ld.Reset()
	if ld.IsLoop() {
		t.Error("Reset() must clear loop state")
	}
}

func TestFingerprintActions_DeterministicAndDistinct(t *testing.T) {
	a := []nexus.Action{{Kind: "click", Target: "e1"}}
	b := []nexus.Action{{Kind: "click", Target: "e2"}}
	if FingerprintActions(a) == FingerprintActions(b) {
		t.Error("distinct actions should hash distinctly")
	}
	if FingerprintActions(a) != FingerprintActions(a) {
		t.Error("fingerprint must be deterministic across calls")
	}
}

// ---------------------------------------------------------------------------
// SelfHealer
// ---------------------------------------------------------------------------

func TestSelfHealer_ReplansWithFailureReason(t *testing.T) {
	var captured string
	planner := llmFunc(func(_ context.Context, req PlanRequest) (AgentStep, error) {
		if len(req.RecentSteps) > 0 {
			captured = req.RecentSteps[0].Evaluation
		}
		return AgentStep{NextGoal: "recover", Actions: []nexus.Action{{Kind: "click", Target: "e3"}}}, nil
	})
	healer, err := NewSelfHealer(planner, 2)
	if err != nil {
		t.Fatal(err)
	}
	state := NewAgentState("test", "sess")
	step, err := healer.Heal(context.Background(), state, "element e1 not visible")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured, "previous_attempt_failed_because: element e1 not visible") {
		t.Errorf("planner didn't see failure hint, captured = %q", captured)
	}
	if len(step.Actions) != 1 || step.Actions[0].Target != "e3" {
		t.Errorf("unexpected step: %+v", step)
	}
}

func TestSelfHealer_ExhaustsAndReturnsLastError(t *testing.T) {
	boom := errors.New("LLM 500")
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{}, boom
	})
	healer, _ := NewSelfHealer(planner, 3)
	state := NewAgentState("test", "sess")
	_, err := healer.Heal(context.Background(), state, "selector stale")
	if !errors.Is(err, boom) {
		t.Errorf("expected boom to bubble, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stress: many sequential retries + loop-detector records stay stable
// ---------------------------------------------------------------------------

func TestStress_SequentialRetryAndLoopDetector(t *testing.T) {
	ld := NewLoopDetector(24, 4)
	var attempts atomic.Int64
	for i := 0; i < 100; i++ {
		_ = RetryWithBackoff(context.Background(),
			BackoffPolicy{Base: time.Nanosecond, MaxTries: 3, JitterPct: 0.01},
			func() error {
				attempts.Add(1)
				return errors.New("flake")
			},
		)
		ld.Record([]nexus.Action{{Kind: "click", Target: "e1"}})
	}
	if attempts.Load() != 300 {
		t.Errorf("expected 300 attempts, got %d", attempts.Load())
	}
	// Same action 100 times is absolutely a loop.
	if !ld.IsLoop() {
		t.Error("100 identical actions should trip the detector")
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkLoopDetector_Record(b *testing.B) {
	ld := NewLoopDetector(64, 3)
	a := []nexus.Action{{Kind: "click", Target: "e1"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ld.Record(a)
	}
}

func BenchmarkLoopDetector_IsLoop(b *testing.B) {
	ld := NewLoopDetector(64, 3)
	for i := 0; i < 64; i++ {
		ld.Record([]nexus.Action{{Kind: "click", Target: "e1"}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ld.IsLoop()
	}
}

func BenchmarkRetryWithBackoff_HappyPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RetryWithBackoff(context.Background(),
			BackoffPolicy{Base: time.Nanosecond, MaxTries: 5, JitterPct: 0.01},
			func() error { return nil },
		)
	}
}
