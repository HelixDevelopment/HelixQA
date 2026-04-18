// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

func TestMessageManager_Defaults(t *testing.T) {
	m := NewMessageManager("SYS", MessageManagerConfig{})
	if m.cfg.TokenBudget != 8000 {
		t.Errorf("TokenBudget default = %d, want 8000", m.cfg.TokenBudget)
	}
	if m.cfg.VerbatimTurns != 4 {
		t.Errorf("VerbatimTurns default = %d, want 4", m.cfg.VerbatimTurns)
	}
	if m.cfg.Tokenizer == nil {
		t.Error("Tokenizer default must not be nil")
	}
	if m.cfg.Summariser == nil {
		t.Error("Summariser default must not be nil")
	}
}

func TestApproxTokenizer_CountTokens(t *testing.T) {
	tk := ApproxTokenizer{}
	if tk.CountTokens("") != 0 {
		t.Error("empty string should be 0 tokens")
	}
	if tk.CountTokens("hello") < 1 {
		t.Error("non-empty string must have >= 1 token")
	}
	long := strings.Repeat("x", 400)
	if tk.CountTokens(long) < 100 {
		t.Errorf("400 chars should be ~100 tokens, got %d", tk.CountTokens(long))
	}
}

func TestMessageManager_CreateStateMessages_IncludesGoalAndElements(t *testing.T) {
	m := NewMessageManager("SYS", MessageManagerConfig{})
	state := NewAgentState("click login", "sess-1")
	state.Snapshot = &nexus.Snapshot{
		Tree: "login page",
		Elements: []nexus.Element{
			{Ref: "e1", Role: "button", Name: "Sign in"},
			{Ref: "e2", Role: "textbox", Name: "Email"},
		},
	}
	state.Screenshot = []byte("png-bytes")
	msg := m.CreateStateMessages(state)
	if msg.Role != RoleUser {
		t.Errorf("role = %s, want user", msg.Role)
	}
	if !strings.Contains(msg.Content, "click login") {
		t.Error("content missing task goal")
	}
	if !strings.Contains(msg.Content, "e1") || !strings.Contains(msg.Content, "Sign in") {
		t.Error("content missing element references")
	}
	if len(msg.Image) == 0 {
		t.Error("image must be attached when screenshot present")
	}
}

func TestMessageManager_Compact_DigestsOldHistoryUnderBudget(t *testing.T) {
	// Tiny budget forces compaction after the first planning pair.
	m := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: 50, VerbatimTurns: 1})
	state := NewAgentState("goal", "s")
	for i := 0; i < 10; i++ {
		step := AgentStep{
			Evaluation: strings.Repeat("very-long-eval ", 20),
			NextGoal:   "goal step",
			Actions:    []nexus.Action{{Kind: "click", Target: "e1"}},
			Results:    []ActionResult{{Action: nexus.Action{Kind: "click", Target: "e1"}, Success: true}},
		}
		state.AppendStep(step)
	}
	m.PrepareStepState(state)
	current := Message{Role: RoleUser, Content: "current"}

	if m.WindowSize() != 20 {
		t.Fatalf("window size before compact = %d, want 20", m.WindowSize())
	}

	compacted, err := m.Compact(context.Background(), current)
	if err != nil {
		t.Fatal(err)
	}
	if !compacted {
		t.Fatal("tiny-budget fixture must trigger compaction")
	}
	if m.Digest().Content == "" {
		t.Error("digest must be populated after compaction")
	}
	// Verbatim window should be pruned below 2*VerbatimTurns.
	if m.WindowSize() > 2 {
		t.Errorf("window after compact = %d, want <= 2", m.WindowSize())
	}
}

func TestMessageManager_Messages_Ordering(t *testing.T) {
	m := NewMessageManager("SYS", MessageManagerConfig{})
	state := NewAgentState("g", "s")
	state.AppendStep(AgentStep{Evaluation: "first"})
	m.PrepareStepState(state)
	current := Message{Role: RoleUser, Content: "curr"}
	out := m.Messages(current)
	if out[0].Role != RoleSystem {
		t.Error("first message must be system")
	}
	if out[len(out)-1].Role != RoleUser || out[len(out)-1].Content != "curr" {
		t.Error("last message must be the current user turn")
	}
}

func TestMessageManager_Compact_NoopWhenUnderBudget(t *testing.T) {
	m := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: 100000, VerbatimTurns: 100})
	state := NewAgentState("g", "s")
	state.AppendStep(AgentStep{Evaluation: "only step"})
	m.PrepareStepState(state)
	compacted, err := m.Compact(context.Background(), Message{Role: RoleUser, Content: "tiny"})
	if err != nil {
		t.Fatal(err)
	}
	if compacted {
		t.Error("under-budget compact must be a no-op")
	}
	if m.Digest().Content != "" {
		t.Error("no compaction means empty digest")
	}
}

func TestMessageManager_500StepSession_StaysUnderBudget(t *testing.T) {
	// Stress: 500-step run must stay inside the budget.
	budget := 2000
	m := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: budget, VerbatimTurns: 2})
	state := NewAgentState("stress", "s")
	for i := 0; i < 500; i++ {
		state.AppendStep(AgentStep{
			Evaluation: strings.Repeat("e ", 10),
			Actions:    []nexus.Action{{Kind: "click", Target: "e1"}},
		})
	}
	m.PrepareStepState(state)
	current := Message{Role: RoleUser, Content: "now"}
	if _, err := m.Compact(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	total := m.TotalTokens(current)
	// Compaction caps at 80% budget (we set window=2 so compaction
	// shrinks the window aggressively).
	if total > budget {
		t.Errorf("total tokens = %d, budget = %d", total, budget)
	}
}

// Fuzz target guards the compaction path against pathological
// summaries + tokenizer inputs.
func FuzzMessageManager_Compact(f *testing.F) {
	f.Add("normal-sized step content", 8000, 4)
	f.Add("", 100, 1)
	f.Add("\x00embedded null", 50, 2)
	f.Add(strings.Repeat("really-long-content ", 200), 500, 1)
	f.Fuzz(func(t *testing.T, content string, budget int, verbatim int) {
		if budget < 1 {
			budget = 1
		}
		if verbatim < 1 {
			verbatim = 1
		}
		m := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: budget, VerbatimTurns: verbatim})
		state := NewAgentState("fuzz", "s")
		for i := 0; i < 6; i++ {
			state.AppendStep(AgentStep{Evaluation: content})
		}
		m.PrepareStepState(state)
		_, _ = m.Compact(context.Background(), Message{Role: RoleUser, Content: content})
	})
}

func TestDefaultSummariser_CollapsesTurns(t *testing.T) {
	s := DefaultSummariser{}
	got, err := s.Summarise(context.Background(), []Message{
		{Role: RoleAssistant, Content: "plan A"},
		{Role: RoleUser, Content: "obs A"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "plan A") || !strings.Contains(got, "obs A") {
		t.Errorf("digest missing content: %q", got)
	}
}

func BenchmarkMessageManager_Compact(b *testing.B) {
	budget := 500
	m := NewMessageManager("SYS", MessageManagerConfig{TokenBudget: budget, VerbatimTurns: 2})
	state := NewAgentState("bench", "s")
	for i := 0; i < 20; i++ {
		state.AppendStep(AgentStep{Evaluation: strings.Repeat("x", 200)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.PrepareStepState(state)
		_, _ = m.Compact(context.Background(), Message{Role: RoleUser, Content: "now"})
	}
}
