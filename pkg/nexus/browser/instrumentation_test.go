package browser

import (
	"context"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/observability"
)

func TestInstrumentedEngine_EmitsMetricsAndSpans(t *testing.T) {
	tr := observability.NewInMemoryTracer()
	observability.SetDefault(tr)
	defer observability.SetDefault(nil)

	reg := observability.NewRegistry()
	metrics := observability.DefaultMetrics(reg)

	base, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	eng := Instrument(base, metrics)

	sess, err := eng.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Navigate(context.Background(), sess, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Snapshot(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	if err := eng.Do(context.Background(), sess, nexus.Action{Kind: "click", Target: "e1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Screenshot(context.Background(), sess); err != nil {
		t.Fatal(err)
	}

	if metrics.SessionOpensTotal.Value() != 1 {
		t.Errorf("SessionOpensTotal = %d", metrics.SessionOpensTotal.Value())
	}
	if metrics.BrowserActiveSessions.Value() != 1 {
		t.Errorf("BrowserActiveSessions = %d", metrics.BrowserActiveSessions.Value())
	}
	if metrics.SnapshotDurationMs.Count() != 1 {
		t.Errorf("SnapshotDurationMs count = %d", metrics.SnapshotDurationMs.Count())
	}

	_ = sess.Close()
	if metrics.BrowserActiveSessions.Value() != 0 {
		t.Errorf("after Close active = %d, want 0", metrics.BrowserActiveSessions.Value())
	}
	if metrics.SessionClosesTotal.Value() != 1 {
		t.Errorf("SessionClosesTotal = %d", metrics.SessionClosesTotal.Value())
	}

	// Tracer should have recorded at least Open + Navigate + Snapshot + Do + Screenshot.
	if len(tr.Finished()) < 5 {
		t.Errorf("expected >=5 spans, got %d", len(tr.Finished()))
	}
}

func TestInstrumentedEngine_NilMetricsStillWorks(t *testing.T) {
	// bluff-scan: nil-only-ok (regression — nil metrics path must not panic/error)
	base, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	eng := Instrument(base, nil)
	sess, err := eng.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Navigate(context.Background(), sess, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	_ = sess.Close()
}

// TestNewInstrumentedEngine_W5_RecommendedRuntimeFactory locks in W5
// from docs/nexus/remaining-work.md: the recommended runtime entry
// point must produce an InstrumentedEngine so production code never
// accidentally ships an un-instrumented browser engine.
func TestNewInstrumentedEngine_W5_RecommendedRuntimeFactory(t *testing.T) {
	reg := observability.NewRegistry()
	metrics := observability.DefaultMetrics(reg)

	eng, err := NewInstrumentedEngine(
		&mockDriver{kind: EngineChromedp},
		Config{Engine: EngineChromedp},
		metrics,
	)
	if err != nil {
		t.Fatal(err)
	}
	if eng == nil {
		t.Fatal("expected non-nil InstrumentedEngine")
	}

	sess, err := eng.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if metrics.SessionOpensTotal.Value() != 1 {
		t.Errorf("SessionOpensTotal = %d, want 1 — NewInstrumentedEngine must wire metrics",
			metrics.SessionOpensTotal.Value())
	}
	_ = sess.Close()

	// A nil metrics argument is valid — the factory should still
	// return spans-only instrumentation.
	eng2, err := NewInstrumentedEngine(
		&mockDriver{kind: EngineChromedp},
		Config{Engine: EngineChromedp},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	sess2, err := eng2.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_ = sess2.Close()
}
