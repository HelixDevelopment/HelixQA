package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOTelTracer_RecordsSpansToBothSinks(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer provider.Shutdown(context.Background())

	tr := NewOTelTracer(provider, "nexus-test")
	SetDefault(tr)
	defer SetDefault(nil)

	err := Instrument(context.Background(), "op.do", func(_ context.Context, s Span) error {
		s.SetAttribute("kind", "click")
		s.AddEvent("checkpoint", map[string]any{"i": 1})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("otel recorded %d spans, want 1", len(ended))
	}
	if ended[0].Name() != "op.do" {
		t.Errorf("span name = %q", ended[0].Name())
	}
	if len(tr.Finished()) != 1 {
		t.Errorf("mirror missing span: %+v", tr.Finished())
	}
}

func TestOTelTracer_RecordsErrorsOnBothSinks(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer provider.Shutdown(context.Background())

	tr := NewOTelTracer(provider, "nexus-test")
	SetDefault(tr)
	defer SetDefault(nil)

	_ = Instrument(context.Background(), "op.err", func(_ context.Context, _ Span) error {
		return errors.New("boom")
	})
	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("spans = %d", len(ended))
	}
	if ended[0].Status().Code != codes.Error {
		t.Errorf("status code = %v, want codes.Error", ended[0].Status().Code)
	}
	if tr.Finished()[0].Err == nil {
		t.Error("mirror missing error")
	}
}

func TestOTelTracer_AttributeTypeCoercion(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer provider.Shutdown(context.Background())

	tr := NewOTelTracer(provider, "nexus-test")
	SetDefault(tr)
	defer SetDefault(nil)

	_ = Instrument(context.Background(), "op.attrs", func(_ context.Context, s Span) error {
		s.SetAttribute("s", "hello")
		s.SetAttribute("i", 42)
		s.SetAttribute("b", true)
		s.SetAttribute("f", 1.5)
		s.SetAttribute("unknown", struct{}{})
		return nil
	})
	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("spans = %d", len(ended))
	}
}

func TestOTelTracer_DoubleEndIdempotent(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer provider.Shutdown(context.Background())

	tr := NewOTelTracer(provider, "nexus-test")
	_, s := tr.Start(context.Background(), "dbl")
	s.End()
	s.End()
	if len(rec.Ended()) != 1 {
		t.Error("second End must not produce another exported span")
	}
}

func TestNewDefaultTracerProvider_WithoutEndpoint(t *testing.T) {
	provider, err := NewDefaultTracerProvider(context.Background(), "", "nexus")
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	// Must still be usable without an exporter.
	ctx, span := provider.Tracer("t").Start(context.Background(), "probe")
	_ = ctx
	span.End()
}
