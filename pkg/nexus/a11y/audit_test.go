package a11y

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeEval struct {
	script string
	out    string
	err    error
}

func (f *fakeEval) Eval(_ context.Context, script string) (string, error) {
	f.script = script
	return f.out, f.err
}

func TestAuditor_RunParsesReport(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAA)
	e := &fakeEval{out: sampleAxeReport}
	r, err := a.Run(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Violations) != 4 {
		t.Errorf("violations = %d", len(r.Violations))
	}
	if !strings.Contains(e.script, "axe.run") {
		t.Error("injection script not sent")
	}
}

func TestAuditor_RunPropagatesEvalError(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAA)
	e := &fakeEval{err: errors.New("browser crashed")}
	if _, err := a.Run(context.Background(), e); err == nil {
		t.Fatal("expected eval error")
	}
}

func TestAuditor_RunAndAssertFailsOnBreach(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAA)
	e := &fakeEval{out: sampleAxeReport}
	_, err := a.RunAndAssert(context.Background(), e)
	if err == nil {
		t.Fatal("AA breach should error")
	}
}

func TestAuditor_RunAndAssertCleanPass(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAAA)
	e := &fakeEval{out: `{"violations":[]}`}
	if _, err := a.RunAndAssert(context.Background(), e); err != nil {
		t.Fatalf("clean report should pass, got %v", err)
	}
}

func TestAuditor_NilEvaluatorRejected(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAA)
	if _, err := a.Run(context.Background(), nil); err == nil {
		t.Fatal("nil evaluator must error")
	}
}

func TestAuditor_LevelAccessor(t *testing.T) {
	a, _ := NewAuditor("https://x", LevelAAA)
	if a.Level() != LevelAAA {
		t.Errorf("level = %s", a.Level())
	}
}
