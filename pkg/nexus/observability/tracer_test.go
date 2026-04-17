package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryTracer_StartSetEnd(t *testing.T) {
	tr := NewInMemoryTracer()
	_, sp := tr.Start(context.Background(), "flow.step")
	sp.SetAttribute("user", "alice")
	sp.AddEvent("checkpoint", map[string]any{"k": 1})
	sp.End()

	fin := tr.Finished()
	if len(fin) != 1 {
		t.Fatalf("finished = %d", len(fin))
	}
	if fin[0].Name != "flow.step" {
		t.Errorf("name = %q", fin[0].Name)
	}
	if fin[0].Attributes["user"] != "alice" {
		t.Error("attribute not recorded")
	}
	if len(fin[0].Events) != 1 || fin[0].Events[0].Name != "checkpoint" {
		t.Errorf("events = %+v", fin[0].Events)
	}
	if fin[0].Duration() < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestInMemoryTracer_EndIdempotent(t *testing.T) {
	tr := NewInMemoryTracer()
	_, sp := tr.Start(context.Background(), "x")
	sp.End()
	sp.End()
	if len(tr.Finished()) != 1 {
		t.Error("End should be idempotent")
	}
}

func TestInMemoryTracer_MutationAfterEndIgnored(t *testing.T) {
	tr := NewInMemoryTracer()
	_, sp := tr.Start(context.Background(), "x")
	sp.End()
	sp.SetAttribute("late", true)
	sp.AddEvent("late", nil)
	sp.SetError(errors.New("late"))
	if fin := tr.Finished(); fin[0].Attributes["late"] == true || fin[0].Err != nil {
		t.Error("post-End mutations must not alter the snapshot")
	}
}

func TestNoopTracer_DoesNothing(t *testing.T) {
	_, sp := NoopTracer{}.Start(context.Background(), "ignored")
	sp.SetAttribute("k", "v")
	sp.SetError(errors.New("ignored"))
	sp.End()
	if len(NoopTracer{}.Finished()) != 0 {
		t.Error("noop tracer must never record anything")
	}
}

func TestDefault_SwapAndRestore(t *testing.T) {
	tr := NewInMemoryTracer()
	SetDefault(tr)
	defer SetDefault(nil) // restore noop

	if _, ok := Default().(*InMemoryTracer); !ok {
		t.Fatal("Default did not return injected tracer")
	}

	SetDefault(nil)
	if _, ok := Default().(NoopTracer); !ok {
		t.Error("nil should reset to NoopTracer")
	}
}

func TestInstrument_CapturesError(t *testing.T) {
	tr := NewInMemoryTracer()
	SetDefault(tr)
	defer SetDefault(nil)

	err := Instrument(context.Background(), "risky", func(_ context.Context, _ Span) error {
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("Instrument should propagate the error")
	}
	fin := tr.Finished()
	if len(fin) != 1 || fin[0].Err == nil {
		t.Errorf("Instrument did not record error: %+v", fin)
	}
}

func TestInstrument_SuccessPath(t *testing.T) {
	tr := NewInMemoryTracer()
	SetDefault(tr)
	defer SetDefault(nil)

	var ran bool
	err := Instrument(context.Background(), "ok", func(_ context.Context, sp Span) error {
		sp.SetAttribute("ran", true)
		ran = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("fn not executed")
	}
	if got := tr.Finished()[0].Attributes["ran"]; got != true {
		t.Errorf("attribute not recorded: %v", got)
	}
	if tr.Finished()[0].End.IsZero() || tr.Finished()[0].Start.After(tr.Finished()[0].End) {
		t.Error("span times look wrong")
	}
}

func TestTimerOrdering(t *testing.T) {
	tr := NewInMemoryTracer()
	SetDefault(tr)
	defer SetDefault(nil)

	start := time.Now()
	_ = Instrument(context.Background(), "timed", func(_ context.Context, _ Span) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	fin := tr.Finished()
	if len(fin) != 1 || fin[0].Start.Before(start) || fin[0].End.Before(fin[0].Start) {
		t.Errorf("bad span times: %+v", fin)
	}
}
