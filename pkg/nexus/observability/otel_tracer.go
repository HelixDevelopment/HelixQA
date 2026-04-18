package observability

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// B7 fix (docs/nexus/remaining-work.md): compile-time check that the
// semconv package we imported still exposes the constants we rely on.
// If the upstream schema renames ServiceName or SchemaURL the build
// fails here instead of in a hard-to-trace runtime panic. Any semconv
// version migration must update this gate.
var (
	_ string             = semconv.SchemaURL
	_ attribute.KeyValue = semconv.ServiceName("compile-time-check")
)

// OTelTracer adapts an OpenTelemetry TracerProvider into our Tracer
// interface so Nexus spans reach Jaeger / Tempo / OpenSearch / any
// OTLP-capable backend without callers having to know about OTel.
//
// Operators build a TracerProvider externally (usually pointing at an
// OTLP gRPC exporter) and pass it here. The bridge keeps a fallback
// InMemoryTracer for tests + Finished() introspection.
type OTelTracer struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
	mirror   *InMemoryTracer
}

// NewOTelTracer returns a Tracer driven by an OpenTelemetry SDK
// TracerProvider. The supplied provider is not closed by Nexus; the
// operator owns its lifecycle so shared exporters are not torn down
// by component shutdown.
func NewOTelTracer(provider *sdktrace.TracerProvider, serviceName string) *OTelTracer {
	if serviceName == "" {
		serviceName = "helix-nexus"
	}
	return &OTelTracer{
		provider: provider,
		tracer:   provider.Tracer(serviceName),
		mirror:   NewInMemoryTracer(),
	}
}

// Start begins a new span. The returned span writes to OTel AND
// records into the in-memory mirror so tests + Finished() keep
// working without reaching into OTel.
func (t *OTelTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	ctx, otelSpan := t.tracer.Start(ctx, name, oteltrace.WithTimestamp(time.Now()))
	mirrorCtx, mirror := t.mirror.Start(ctx, name)
	_ = mirrorCtx
	return ctx, &otelSpanWrapper{otel: otelSpan, mirror: mirror}
}

// Finished returns the spans recorded in the mirror. OTel itself does
// not expose a Finished() concept because exporters own the output.
func (t *OTelTracer) Finished() []FinishedSpan { return t.mirror.Finished() }

// NewDefaultTracerProvider constructs a TracerProvider with a batch
// OTLP gRPC exporter pointing at endpoint (e.g. "jaeger.local:4317").
// When endpoint is empty the returned provider exports to a local
// noop destination so operators can develop without running a
// collector.
func NewDefaultTracerProvider(ctx context.Context, endpoint, serviceName string) (*sdktrace.TracerProvider, error) {
	if serviceName == "" {
		serviceName = "helix-nexus"
	}
	res, _ := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	))
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	if endpoint != "" {
		exp, err := buildOTLPExporter(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

// otelSpanWrapper satisfies Span by fanning out to both OTel and the
// in-memory mirror.
type otelSpanWrapper struct {
	otel   oteltrace.Span
	mirror Span

	mu    sync.Mutex
	ended bool
}

func (s *otelSpanWrapper) SetAttribute(key string, value any) {
	s.otel.SetAttributes(attributeFor(key, value))
	s.mirror.SetAttribute(key, value)
}

func (s *otelSpanWrapper) AddEvent(name string, attrs map[string]any) {
	kv := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kv = append(kv, attributeFor(k, v))
	}
	s.otel.AddEvent(name, oteltrace.WithAttributes(kv...))
	s.mirror.AddEvent(name, attrs)
}

func (s *otelSpanWrapper) SetError(err error) {
	if err == nil {
		return
	}
	s.otel.RecordError(err)
	s.otel.SetStatus(codes.Error, err.Error())
	s.mirror.SetError(err)
}

func (s *otelSpanWrapper) End() {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	s.mu.Unlock()
	s.otel.End()
	s.mirror.End()
}

func attributeFor(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	}
	return attribute.String(key, "")
}
