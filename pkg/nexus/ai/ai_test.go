package ai

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

// fakeLLM is a controllable LLMClient used by every ai test.
type fakeLLM struct {
	mu       sync.Mutex
	reply    string
	costUSD  float64
	tokensIn int
	tokensOut int
	err      error
	calls    int
	lastReq  ChatRequest
}

func (f *fakeLLM) Chat(_ context.Context, req ChatRequest) (ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return ChatResponse{}, f.err
	}
	return ChatResponse{
		Text:     f.reply,
		Provider: "fake",
		Model:    req.Model,
		TokensIn: f.tokensIn, TokensOut: f.tokensOut,
		CostUSD: f.costUSD,
	}, nil
}

// --- CostTracker ---

func TestCostTracker_ReserveWithinBudget(t *testing.T) {
	ct := NewCostTracker(1.0) // $1
	if err := ct.Reserve(0.25); err != nil {
		t.Fatal(err)
	}
	if err := ct.Reserve(0.5); err != nil {
		t.Fatal(err)
	}
	if got := ct.SpentUSD(); got < 0.74 || got > 0.76 {
		t.Errorf("SpentUSD = %f, want ~0.75", got)
	}
}

func TestCostTracker_BudgetBreachRefuses(t *testing.T) {
	ct := NewCostTracker(0.50)
	if err := ct.Reserve(0.60); !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestCostTracker_DisableAllowsAnything(t *testing.T) {
	ct := NewCostTracker(0.01)
	ct.Disable()
	if err := ct.Reserve(99); err != nil {
		t.Fatal(err)
	}
}

func TestCostTracker_UnlimitedWhenBudgetZero(t *testing.T) {
	ct := NewCostTracker(0)
	if err := ct.Reserve(10); err != nil {
		t.Fatal(err)
	}
}

func TestCostTracker_RecordEntries(t *testing.T) {
	ct := NewCostTracker(1)
	ct.Record(Entry{Provider: "p", Model: "m", TokensIn: 1, TokensOut: 2, CostCents: 3})
	if len(ct.Entries()) != 1 {
		t.Errorf("entries = %d", len(ct.Entries()))
	}
}

// --- Navigator ---

func TestNavigator_DecideParsesJSON(t *testing.T) {
	llm := &fakeLLM{reply: `{"kind":"click","target":"e5","reasoning":"test","confidence":0.9}`, costUSD: 0.01}
	nav := NewNavigator(llm, NewCostTracker(1), "test-model")
	a, err := nav.Decide(context.Background(), VisualContext{Goal: "click save", Tree: "<x/>"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "click" || a.Target != "e5" {
		t.Errorf("unexpected action %+v", a)
	}
}

func TestNavigator_DecideStripsCodeFences(t *testing.T) {
	llm := &fakeLLM{reply: "```json\n{\"kind\":\"type\",\"target\":\"e2\",\"text\":\"hi\",\"reasoning\":\"\",\"confidence\":1}\n```"}
	nav := NewNavigator(llm, NewCostTracker(0), "")
	a, err := nav.Decide(context.Background(), VisualContext{Goal: "type"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "type" {
		t.Errorf("kind = %s", a.Kind)
	}
}

func TestNavigator_BudgetBreachReturnsError(t *testing.T) {
	llm := &fakeLLM{reply: `{"kind":"click","target":"e1","reasoning":"","confidence":1}`, costUSD: 2}
	nav := NewNavigator(llm, NewCostTracker(0.5), "")
	_, err := nav.Decide(context.Background(), VisualContext{})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected budget error, got %v", err)
	}
}

func TestNavigator_DecidePropagatesLLMError(t *testing.T) {
	llm := &fakeLLM{err: errors.New("upstream down")}
	nav := NewNavigator(llm, NewCostTracker(0), "")
	_, err := nav.Decide(context.Background(), VisualContext{Platform: nexus.PlatformWebChromedp})
	if err == nil || !strings.Contains(err.Error(), "upstream down") {
		t.Errorf("expected upstream error, got %v", err)
	}
}

func TestNavigator_ParseActionRejectsEmpty(t *testing.T) {
	if _, err := parseAction(`{"target":"x"}`); err == nil {
		t.Fatal("empty kind must be rejected")
	}
}

func TestNavigator_PreviousActionsSerialised(t *testing.T) {
	llm := &fakeLLM{reply: `{"kind":"done","target":"","reasoning":"","confidence":1}`}
	nav := NewNavigator(llm, NewCostTracker(0), "")
	_, _ = nav.Decide(context.Background(), VisualContext{
		PreviousActions: []NavigationAction{
			{Kind: "click", Target: "e1"}, {Kind: "type", Target: "e2", Text: "hi"},
		},
	})
	if !strings.Contains(llm.lastReq.UserPrompt, "click e1") {
		t.Errorf("prev not serialised: %q", llm.lastReq.UserPrompt)
	}
}

// --- Healer ---

func TestHealer_HealReturnsSelector(t *testing.T) {
	llm := &fakeLLM{reply: "button[aria-label=Save]", costUSD: 0.001}
	h := NewHealer(llm, NewCostTracker(0.1), "")
	sel, err := h.Heal(context.Background(), "#oldSave", "Save button", "<html/>")
	if err != nil {
		t.Fatal(err)
	}
	if sel != "button[aria-label=Save]" {
		t.Errorf("sel = %q", sel)
	}
}

func TestHealer_HealReturnsEmptyWhenLLMHasNoAnswer(t *testing.T) {
	llm := &fakeLLM{reply: "\n  \n"}
	h := NewHealer(llm, nil, "")
	sel, err := h.Heal(context.Background(), "#x", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if sel != "" {
		t.Errorf("empty answer must be empty, got %q", sel)
	}
}

func TestHealer_PropagatesError(t *testing.T) {
	llm := &fakeLLM{err: errors.New("busy")}
	h := NewHealer(llm, nil, "")
	_, err := h.Heal(context.Background(), "x", "y", "z")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Generator ---

func TestGenerator_GenerateEmitsValidYAML(t *testing.T) {
	llm := &fakeLLM{reply: `id: NX-GEN-demo
name: Demo
steps:
  - name: Do it
    action: click
    expected: ok`}
	g := NewGenerator(llm, nil, "")
	out, err := g.Generate(context.Background(), "Write a test", "web")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "NX-GEN-demo") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestGenerator_RejectsEmptyStory(t *testing.T) {
	g := NewGenerator(&fakeLLM{reply: ""}, nil, "")
	if _, err := g.Generate(context.Background(), "", "web"); err == nil {
		t.Fatal("empty story must fail")
	}
}

func TestGenerator_RejectsInvalidYAML(t *testing.T) {
	llm := &fakeLLM{reply: "not: : : yaml"}
	g := NewGenerator(llm, nil, "")
	_, err := g.Generate(context.Background(), "story", "web")
	if err == nil {
		t.Fatal("malformed yaml should surface error")
	}
}

func TestGenerator_RequiresIDNameSteps(t *testing.T) {
	llm := &fakeLLM{reply: "id: x\nname: y"}
	g := NewGenerator(llm, nil, "")
	_, err := g.Generate(context.Background(), "story", "web")
	if err == nil || !strings.Contains(err.Error(), "steps") {
		t.Errorf("missing steps must be flagged, got %v", err)
	}
}

// --- Predictor ---

func TestPredictor_ProbabilityInRange(t *testing.T) {
	p := NewPredictor()
	for _, s := range []FlakeSample{
		{Retries: 0, DurationS: 1, HourOfDay: 10},
		{Retries: 5, DurationS: 60, HourOfDay: 2},
		{Retries: 1, DurationS: 100, HourOfDay: 23, RunnerRSS: 8 * 1073741824},
	} {
		got := p.Probability(s)
		if got < 0 || got > 1 {
			t.Errorf("probability out of range: %f", got)
		}
	}
}

func TestPredictor_DecideThreshold(t *testing.T) {
	p := NewPredictor()
	hot := FlakeSample{Retries: 9, DurationS: 240, HourOfDay: 3, RunnerRSS: 8 * 1073741824}
	if !p.Decide(hot, 0.5) {
		t.Error("obvious flake should trigger Decide(true)")
	}
	cool := FlakeSample{Retries: 0, DurationS: 1, HourOfDay: 12}
	if p.Decide(cool, 0.9) {
		t.Error("clean sample should not trigger high threshold")
	}
}

func TestPredictor_ObserveShiftsBias(t *testing.T) {
	p := NewPredictor()
	base := p.bias
	for i := 0; i < 10; i++ {
		p.Observe(FlakeSample{Pass: false})
	}
	if p.bias <= base {
		t.Error("bias should increase after failures")
	}
}

func TestPredictor_HistoryCopy(t *testing.T) {
	p := NewPredictor()
	p.Observe(FlakeSample{TestID: "t1", Pass: true})
	hist := p.History()
	if len(hist) != 1 {
		t.Fatalf("history = %d", len(hist))
	}
	hist[0].TestID = "mutated"
	if p.History()[0].TestID != "t1" {
		t.Error("History should return a copy")
	}
}
