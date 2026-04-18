// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package primitives

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/agent"
)

// ---------------------------------------------------------------------------
// PromptCache
// ---------------------------------------------------------------------------

func TestPromptCache_DefaultTTL(t *testing.T) {
	c := NewPromptCache(0)
	if c.ttl != 5*time.Minute {
		t.Errorf("default TTL wrong: %s", c.ttl)
	}
}

func TestPromptCache_PutGetHit(t *testing.T) {
	c := NewPromptCache(time.Minute)
	c.Put("k1", agent.AgentStep{Evaluation: "v1"})
	got, ok := c.Get("k1")
	if !ok || got.Evaluation != "v1" {
		t.Errorf("cache miss: %v %v", got, ok)
	}
}

func TestPromptCache_ExpiredEntriesAreEvicted(t *testing.T) {
	c := NewPromptCache(10 * time.Millisecond)
	c.Put("k", agent.AgentStep{Evaluation: "v"})
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Error("expected expiry")
	}
}

func TestPromptCache_ConcurrentAccess(t *testing.T) {
	c := NewPromptCache(time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%10))
			c.Put(key, agent.AgentStep{Evaluation: key})
			_, _ = c.Get(key)
		}(i)
	}
	wg.Wait()
	if c.Size() > 10 {
		t.Errorf("unexpected size: %d", c.Size())
	}
}

func TestFingerprint_Stable(t *testing.T) {
	a := fingerprint("act", "click", "", "snap1")
	b := fingerprint("act", "click", "", "snap1")
	if a != b {
		t.Error("same inputs must yield same fingerprint")
	}
	c := fingerprint("act", "click", "", "snap2")
	if a == c {
		t.Error("distinct snapshot hash must yield distinct fingerprint")
	}
}

// ---------------------------------------------------------------------------
// NewEngine guards
// ---------------------------------------------------------------------------

func TestNewEngine_RejectsNilDeps(t *testing.T) {
	if _, err := NewEngine(nil, &fakeAdapter{}, fakeSession{}); err == nil {
		t.Error("nil LLMClient must error")
	}
	if _, err := NewEngine(fakeLLM{}, nil, fakeSession{}); err == nil {
		t.Error("nil Adapter must error")
	}
	if _, err := NewEngine(fakeLLM{}, &fakeAdapter{}, nil); err == nil {
		t.Error("nil Session must error")
	}
}

// ---------------------------------------------------------------------------
// Act
// ---------------------------------------------------------------------------

func TestEngine_Act_DispatchesPlannerAction(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e3"}}}, nil
	})
	adapter := &fakeAdapter{}
	engine, _ := NewEngine(planner, adapter, fakeSession{})
	err := engine.Act(context.Background(), "click the submit button")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.doCalls != 1 {
		t.Errorf("expected 1 Do call, got %d", adapter.doCalls)
	}
	if adapter.lastAction.Target != "e3" {
		t.Errorf("unexpected target dispatched: %q", adapter.lastAction.Target)
	}
}

func TestEngine_Act_CacheShortCircuitsOnSecondCall(t *testing.T) {
	var planCalls atomic.Int32
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		planCalls.Add(1)
		return agent.AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e3"}}}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{}, WithPromptCache(NewPromptCache(time.Minute)))
	if err := engine.Act(context.Background(), "same instruction"); err != nil {
		t.Fatal(err)
	}
	if err := engine.Act(context.Background(), "same instruction"); err != nil {
		t.Fatal(err)
	}
	if planCalls.Load() != 1 {
		t.Errorf("second Act must hit cache, got %d plan calls", planCalls.Load())
	}
}

func TestEngine_Act_SelfHealsOnDispatchFailure(t *testing.T) {
	var planCalls atomic.Int32
	planner := llmFunc(func(_ context.Context, req agent.PlanRequest) (agent.AgentStep, error) {
		planCalls.Add(1)
		target := "e1"
		if len(req.RecentSteps) > 0 && strings.Contains(req.RecentSteps[0].Evaluation, "first attempt failed") {
			target = "e2" // healed action
		}
		return agent.AgentStep{Actions: []nexus.Action{{Kind: "click", Target: target}}}, nil
	})
	adapter := &fakeAdapter{refuseTarget: "e1"}
	healer, err := agent.NewSelfHealer(planner, 2)
	if err != nil {
		t.Fatal(err)
	}
	engine, _ := NewEngine(planner, adapter, fakeSession{}, WithSelfHealer(healer))
	if err := engine.Act(context.Background(), "click submit"); err != nil {
		t.Fatalf("act with healer must recover: %v", err)
	}
	if planCalls.Load() < 2 {
		t.Errorf("healer must invoke planner at least twice, got %d", planCalls.Load())
	}
	if adapter.lastAction.Target != "e2" {
		t.Errorf("healed action not dispatched: %+v", adapter.lastAction)
	}
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func TestEngine_Extract_DecodesPlannerMemory(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{Memory: `{"title":"Hello"}`}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{})
	var out struct{ Title string }
	if err := engine.Extract(context.Background(), "page title", `{"type":"object"}`, &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != "Hello" {
		t.Errorf("decode wrong: %+v", out)
	}
}

func TestEngine_Extract_RejectsEmptyMemory(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{})
	var out map[string]any
	err := engine.Extract(context.Background(), "x", `{}`, &out)
	if err == nil || !strings.Contains(err.Error(), "empty Memory") {
		t.Errorf("expected empty-memory error, got %v", err)
	}
}

func TestEngine_Extract_RejectsNilOut(t *testing.T) {
	engine, _ := NewEngine(fakeLLM{}, &fakeAdapter{}, fakeSession{})
	err := engine.Extract(context.Background(), "x", `{}`, nil)
	if err == nil || !strings.Contains(err.Error(), "nil out") {
		t.Errorf("expected nil-out error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Observe
// ---------------------------------------------------------------------------

func TestEngine_Observe_ParsesMultipleRefs(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{Memory: "e1, e2, e5"}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{})
	refs, err := engine.Observe(context.Background(), "every button in the toolbar")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 3 || refs[0] != "e1" || refs[2] != "e5" {
		t.Errorf("refs wrong: %+v", refs)
	}
}

func TestEngine_Observe_FailsWhenPlannerReturnsNoRefs(t *testing.T) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{Memory: ""}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{})
	_, err := engine.Observe(context.Background(), "nothing matches")
	if err == nil {
		t.Error("expected no-refs error")
	}
}

// ---------------------------------------------------------------------------
// Agent (scoped autonomous mode)
// ---------------------------------------------------------------------------

func TestEngine_Agent_RunsToCompletion(t *testing.T) {
	var iter atomic.Int64
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		n := iter.Add(1)
		if n == 1 {
			return agent.AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e1"}}}, nil
		}
		return agent.AgentStep{Done: true}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{})
	state, err := engine.Agent(context.Background(), "scoped goal", agent.Config{MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Done {
		t.Error("scoped run must finish Done")
	}
}

// ---------------------------------------------------------------------------
// refsFromMemory parser
// ---------------------------------------------------------------------------

func TestRefsFromMemory_StripsBracketsAndQuotes(t *testing.T) {
	cases := map[string]int{
		`e1, e2`:       2,
		`[e1, e2, e3]`: 3,
		`"e1", "e2"`:   2,
		``:             0,
		`,`:            0,
		`  e1  `:       1,
	}
	for in, want := range cases {
		got := refsFromMemory(in)
		if len(got) != want {
			t.Errorf("refsFromMemory(%q) = %v (want %d refs)", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkPromptCache_PutGet(b *testing.B) {
	c := NewPromptCache(time.Minute)
	for i := 0; i < b.N; i++ {
		c.Put("k", agent.AgentStep{})
		_, _ = c.Get("k")
	}
}

func BenchmarkEngine_Act_Cached(b *testing.B) {
	planner := llmFunc(func(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
		return agent.AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e1"}}}, nil
	})
	engine, _ := NewEngine(planner, &fakeAdapter{}, fakeSession{}, WithPromptCache(NewPromptCache(time.Minute)))
	for i := 0; i < b.N; i++ {
		_ = engine.Act(context.Background(), "same")
	}
}

// ---------------------------------------------------------------------------
// Fuzz
// ---------------------------------------------------------------------------

func FuzzRefsFromMemory(f *testing.F) {
	f.Add("e1,e2,e3")
	f.Add("[e1, e2]")
	f.Add(`"e1"`)
	f.Add("")
	f.Add("\x00garbage\x00")
	f.Fuzz(func(t *testing.T, s string) {
		// Must never panic regardless of input.
		_ = refsFromMemory(s)
	})
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

type fakeAdapter struct {
	mu           sync.Mutex
	doCalls      int
	lastAction   nexus.Action
	refuseTarget string
}

func (f *fakeAdapter) Open(_ context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	return fakeSession{}, nil
}
func (f *fakeAdapter) Navigate(_ context.Context, _ nexus.Session, _ string) error { return nil }
func (f *fakeAdapter) Snapshot(_ context.Context, _ nexus.Session) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "page", Elements: []nexus.Element{
		{Ref: "e1", Role: "button"},
		{Ref: "e2", Role: "button"},
	}}, nil
}
func (f *fakeAdapter) Do(_ context.Context, _ nexus.Session, a nexus.Action) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.doCalls++
	f.lastAction = a
	if f.refuseTarget != "" && a.Target == f.refuseTarget {
		return errors.New("selector missing")
	}
	return nil
}
func (f *fakeAdapter) Screenshot(_ context.Context, _ nexus.Session) ([]byte, error) {
	return []byte("png"), nil
}

type fakeSession struct{}

func (fakeSession) ID() string               { return "primitive-session" }
func (fakeSession) Platform() nexus.Platform { return nexus.PlatformWebChromedp }
func (fakeSession) Close() error             { return nil }

type fakeLLM struct{}

func (fakeLLM) PlanStep(_ context.Context, _ agent.PlanRequest) (agent.AgentStep, error) {
	return agent.AgentStep{Done: true}, nil
}

// llmFunc adapts a plain function into the agent.LLMClient interface.
type llmFunc func(ctx context.Context, req agent.PlanRequest) (agent.AgentStep, error)

func (f llmFunc) PlanStep(ctx context.Context, req agent.PlanRequest) (agent.AgentStep, error) {
	return f(ctx, req)
}
