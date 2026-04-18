package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// buildOTLPExporter constructs an OTLP gRPC exporter targeting endpoint.
// endpoint is of the form "host:port" (matches the OTel collector's
// default OTLP gRPC listener). The exporter is wired with TLS off by
// default; operators who need TLS should construct a TracerProvider
// themselves using otlptracegrpc.WithTLSCredentials(...).
func buildOTLPExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}
	return exp, nil
}
