// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

// BenchmarkAgent_Step measures the per-step overhead of the state
// machine when the adapter + planner are effectively free. The
// number reflects the pure orchestration cost, not the model
// inference time — a real planner call dominates in production.
// P2.T15 target: keep ns/op stable across releases.
func BenchmarkAgent_Step(b *testing.B) {
	planner := llmFunc(func(_ context.Context, _ PlanRequest) (AgentStep, error) {
		return AgentStep{Actions: []nexus.Action{{Kind: "click", Target: "e1"}}}, nil
	})
	a, _ := NewAgent(planner, &fakeAdapter{}, Config{MaxIterations: 1000000})
	state := NewAgentState("bench", "bench-1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Step(context.Background(), state, fakeSession{})
	}
}

// BenchmarkParsePlannerJSON measures the parse path for a
// representative planner reply. P2.T15 target: zero allocations in
// the hot path (we accept some small constant due to json package).
func BenchmarkParsePlannerJSON(b *testing.B) {
	raw := `{"evaluation":"ok","memory":"m","next_goal":"g","done":false,"actions":[{"kind":"click","target":"e1"}]}`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ParsePlannerJSON(raw)
	}
}

// BenchmarkAgentState_RecentSteps measures the window slice bound.
func BenchmarkAgentState_RecentSteps(b *testing.B) {
	s := NewAgentState("x", "x")
	for i := 0; i < 100; i++ {
		s.AppendStep(AgentStep{})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.RecentSteps(4)
	}
}
