package observability

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
)

type OTelConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
}

func InitOTel(ctx context.Context, cfg OTelConfig, logger *zap.Logger) (func(), error) {
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)

	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
	)
	otelmeter := meterProvider.Meter(cfg.ServiceName)
	if err := InitMetrics(otelmeter); err != nil {
		meterProvider.Shutdown(ctx)
		return nil, err
	}
	otel.SetMeterProvider(meterProvider)

	shutdown := func() {
		meterProvider.Shutdown(ctx)
	}

	logger.Info("OpenTelemetry initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("environment", cfg.Environment),
	)

	return shutdown, nil
}
