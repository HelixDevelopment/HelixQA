package observability

import (
	"context"
	"sync"
	"time"
)

// Span is a minimal OTel-like span. Implementations record attributes
// and events in memory until End() is called. Adapters can bridge to
// real OTel by wrapping Span.
type Span interface {
	SetAttribute(key string, value any)
	AddEvent(name string, attrs map[string]any)
	SetError(err error)
	End()
}

// Tracer is the entry point for starting spans. The default in-memory
// Tracer is enough for unit tests and the CLI runner; production
// environments swap to an OTel-backed Tracer via SetDefault.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
	Finished() []FinishedSpan
}

// FinishedSpan is the snapshot an InMemoryTracer keeps after End.
type FinishedSpan struct {
	Name       string
	Start      time.Time
	End        time.Time
	Attributes map[string]any
	Events     []SpanEvent
	Err        error
}

// SpanEvent is a structured event inside a span.
type SpanEvent struct {
	Name       string
	At         time.Time
	Attributes map[string]any
}

// Duration is a convenience accessor.
func (f FinishedSpan) Duration() time.Duration { return f.End.Sub(f.Start) }

// --- In-memory default ---

// InMemoryTracer records every span so tests can assert shape.
type InMemoryTracer struct {
	mu       sync.Mutex
	finished []FinishedSpan
}

// NewInMemoryTracer returns a fresh recorder.
func NewInMemoryTracer() *InMemoryTracer { return &InMemoryTracer{} }

// Start begins a new span.
func (t *InMemoryTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	s := &inMemorySpan{
		tracer:     t,
		name:       name,
		start:      time.Now(),
		attributes: map[string]any{},
	}
	return ctx, s
}

// Finished returns a copy of every span that has ended.
func (t *InMemoryTracer) Finished() []FinishedSpan {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]FinishedSpan, len(t.finished))
	copy(out, t.finished)
	return out
}

func (t *InMemoryTracer) record(s FinishedSpan) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finished = append(t.finished, s)
}

type inMemorySpan struct {
	tracer     *InMemoryTracer
	name       string
	start      time.Time
	mu         sync.Mutex
	attributes map[string]any
	events     []SpanEvent
	err        error
	ended      bool
}

func (s *inMemorySpan) SetAttribute(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.attributes[key] = value
}

func (s *inMemorySpan) AddEvent(name string, attrs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	cp := map[string]any{}
	for k, v := range attrs {
		cp[k] = v
	}
	s.events = append(s.events, SpanEvent{Name: name, At: time.Now(), Attributes: cp})
}

func (s *inMemorySpan) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.err = err
}

func (s *inMemorySpan) End() {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	snap := FinishedSpan{
		Name: s.name, Start: s.start, End: time.Now(),
		Attributes: copyMap(s.attributes), Events: append([]SpanEvent{}, s.events...), Err: s.err,
	}
	s.mu.Unlock()
	s.tracer.record(snap)
}

func copyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// --- No-op ---

// NoopTracer is a zero-cost Tracer used when observability is disabled.
type NoopTracer struct{}

// Start returns a span that does nothing.
func (NoopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

// Finished is always empty.
func (NoopTracer) Finished() []FinishedSpan { return nil }

type noopSpan struct{}

func (noopSpan) SetAttribute(_ string, _ any)        {} // intentionally empty: noop span discards attributes
func (noopSpan) AddEvent(_ string, _ map[string]any) {} // intentionally empty: noop span discards events
func (noopSpan) SetError(_ error)                  {} // intentionally empty: noop span discards errors
func (noopSpan) End()                            {} // intentionally empty: noop span has nothing to finalize

// --- Default registration ---

var (
	defaultTracerMu sync.RWMutex
	defaultTracer   Tracer = NoopTracer{}
)

// SetDefault swaps the global Tracer used by Instrument.
func SetDefault(t Tracer) {
	defaultTracerMu.Lock()
	defer defaultTracerMu.Unlock()
	if t == nil {
		t = NoopTracer{}
	}
	defaultTracer = t
}

// Default returns the currently configured Tracer.
func Default() Tracer {
	defaultTracerMu.RLock()
	defer defaultTracerMu.RUnlock()
	return defaultTracer
}

// Instrument is a convenience wrapper: it starts a span, runs fn, and
// ends the span with any error fn returned. Call sites stay minimal.
func Instrument(ctx context.Context, name string, fn func(ctx context.Context, span Span) error) error {
	ctx, span := Default().Start(ctx, name)
	defer span.End()
	err := fn(ctx, span)
	if err != nil {
		span.SetError(err)
	}
	return err
}
