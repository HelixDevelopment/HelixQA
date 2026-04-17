package browser

import (
	"context"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/observability"
)

// InstrumentedEngine wraps an Engine with observability hooks. Every
// Open / Close / Snapshot / Navigate path emits spans through the
// configured Tracer and records metrics against the supplied
// NexusMetrics (see pkg/nexus/observability.DefaultMetrics).
//
// Operators wire an InstrumentedEngine rather than the bare Engine
// whenever they want Grafana panels to populate.
type InstrumentedEngine struct {
	*Engine
	metrics *observability.NexusMetrics
}

// Instrument wraps eng with observability hooks. A nil metrics argument
// is accepted: the returned wrapper still emits spans but all metrics
// become no-ops.
func Instrument(eng *Engine, metrics *observability.NexusMetrics) *InstrumentedEngine {
	return &InstrumentedEngine{Engine: eng, metrics: metrics}
}

// NewInstrumentedEngine is the recommended constructor for runtime
// wiring: it calls NewEngine and immediately wraps the result with
// Instrument so NexusMetrics counters start populating as soon as
// the first browser session opens. Callers that do not want metrics
// yet still get spans when they pass nil for metrics.
//
// Operators should prefer this factory over NewEngine + manual
// Instrument() so no production code path accidentally ships an
// un-instrumented browser engine.
func NewInstrumentedEngine(d Driver, cfg Config, metrics *observability.NexusMetrics) (*InstrumentedEngine, error) {
	eng, err := NewEngine(d, cfg)
	if err != nil {
		return nil, err
	}
	return Instrument(eng, metrics), nil
}

// Open delegates to Engine.Open while recording a span + session-open
// counter + active-session gauge.
func (i *InstrumentedEngine) Open(ctx context.Context, opts nexus.SessionOptions) (nexus.Session, error) {
	var sess nexus.Session
	err := observability.Instrument(ctx, "nexus.browser.Open", func(sctx context.Context, span observability.Span) error {
		s, err := i.Engine.Open(sctx, opts)
		if err != nil {
			return err
		}
		sess = s
		span.SetAttribute("session.id", s.ID())
		return nil
	})
	if err != nil {
		return nil, err
	}
	if i.metrics != nil {
		i.metrics.SessionOpensTotal.Inc()
		i.metrics.BrowserActiveSessions.Add(1)
	}
	return &instrumentedSession{Session: sess, metrics: i.metrics}, nil
}

// Navigate wraps the parent call in a span for observability.
func (i *InstrumentedEngine) Navigate(ctx context.Context, s nexus.Session, target string) error {
	inner := s
	if is, ok := s.(*instrumentedSession); ok {
		inner = is.Session
	}
	return observability.Instrument(ctx, "nexus.browser.Navigate", func(sctx context.Context, span observability.Span) error {
		span.SetAttribute("target", target)
		return i.Engine.Navigate(sctx, inner, target)
	})
}

// Snapshot wraps the call with a span and records duration through
// observability.NexusMetrics.ObserveSnapshotDuration.
func (i *InstrumentedEngine) Snapshot(ctx context.Context, s nexus.Session) (*nexus.Snapshot, error) {
	inner := s
	if is, ok := s.(*instrumentedSession); ok {
		inner = is.Session
	}
	var snap *nexus.Snapshot
	start := time.Now()
	err := observability.Instrument(ctx, "nexus.browser.Snapshot", func(sctx context.Context, span observability.Span) error {
		out, err := i.Engine.Snapshot(sctx, inner)
		if err != nil {
			return err
		}
		snap = out
		span.SetAttribute("elements", len(out.Elements))
		return nil
	})
	if i.metrics != nil {
		i.metrics.ObserveSnapshotDuration(time.Since(start))
	}
	return snap, err
}

// Do wraps Engine.Do in a span so Kind + Target land on the trace.
func (i *InstrumentedEngine) Do(ctx context.Context, s nexus.Session, a nexus.Action) error {
	inner := s
	if is, ok := s.(*instrumentedSession); ok {
		inner = is.Session
	}
	return observability.Instrument(ctx, "nexus.browser.Do", func(sctx context.Context, span observability.Span) error {
		span.SetAttribute("action.kind", a.Kind)
		span.SetAttribute("action.target", a.Target)
		return i.Engine.Do(sctx, inner, a)
	})
}

// Screenshot delegates with a span.
func (i *InstrumentedEngine) Screenshot(ctx context.Context, s nexus.Session) ([]byte, error) {
	inner := s
	if is, ok := s.(*instrumentedSession); ok {
		inner = is.Session
	}
	var png []byte
	err := observability.Instrument(ctx, "nexus.browser.Screenshot", func(sctx context.Context, _ observability.Span) error {
		out, err := i.Engine.Screenshot(sctx, inner)
		if err != nil {
			return err
		}
		png = out
		return nil
	})
	return png, err
}

// instrumentedSession decrements the active-session gauge on Close.
type instrumentedSession struct {
	nexus.Session
	metrics *observability.NexusMetrics
}

func (s *instrumentedSession) Close() error {
	if s.metrics != nil {
		s.metrics.BrowserActiveSessions.Add(-1)
		s.metrics.SessionClosesTotal.Inc()
	}
	return s.Session.Close()
}

var _ nexus.Adapter = (*InstrumentedEngine)(nil)
