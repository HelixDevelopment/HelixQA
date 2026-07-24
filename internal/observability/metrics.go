package observability

import (
	"go.opentelemetry.io/otel/metric"
)

var (
	// Request metrics
	RequestDuration  metric.Float64Histogram
	RequestCounter   metric.Int64Counter
	RequestErrors    metric.Int64Counter

	// Business metrics
	TransactionCounter metric.Int64Counter
	TransactionAmount  metric.Float64Counter
	PaymentSuccessRate metric.Float64Gauge

	// Provider metrics
	ProviderLatency      metric.Float64Histogram
	ProviderErrors       metric.Int64Counter
	ProviderCircuitBreak metric.Int64Gauge

	// Webhook metrics
	WebhookDeliveryCount  metric.Int64Counter
	WebhookDeliveryErrors metric.Int64Counter
	WebhookLatency        metric.Float64Histogram

	// Active merchants
	ActiveMerchants metric.Int64Gauge
)

func InitMetrics(meter metric.Meter) error {
	var err error

	RequestDuration, err = meter.Float64Histogram("http.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	RequestCounter, err = meter.Int64Counter("http.request.total",
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		return err
	}

	RequestErrors, err = meter.Int64Counter("http.request.errors.total",
		metric.WithDescription("Total HTTP request errors"),
	)
	if err != nil {
		return err
	}

	TransactionCounter, err = meter.Int64Counter("transaction.total",
		metric.WithDescription("Total transactions"),
	)
	if err != nil {
		return err
	}

	TransactionAmount, err = meter.Float64Counter("transaction.amount.total",
		metric.WithDescription("Total transaction amount"),
	)
	if err != nil {
		return err
	}

	PaymentSuccessRate, err = meter.Float64Gauge("payment.success.rate",
		metric.WithDescription("Payment success rate"),
	)
	if err != nil {
		return err
	}

	ProviderLatency, err = meter.Float64Histogram("provider.latency",
		metric.WithDescription("Provider API latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	ProviderErrors, err = meter.Int64Counter("provider.errors.total",
		metric.WithDescription("Total provider errors"),
	)
	if err != nil {
		return err
	}

	ProviderCircuitBreak, err = meter.Int64Gauge("provider.circuit_breaker",
		metric.WithDescription("Provider circuit breaker status (0=closed, 1=open)"),
	)
	if err != nil {
		return err
	}

	WebhookDeliveryCount, err = meter.Int64Counter("webhook.delivery.total",
		metric.WithDescription("Total webhook deliveries"),
	)
	if err != nil {
		return err
	}

	WebhookDeliveryErrors, err = meter.Int64Counter("webhook.delivery.errors.total",
		metric.WithDescription("Total webhook delivery errors"),
	)
	if err != nil {
		return err
	}

	WebhookLatency, err = meter.Float64Histogram("webhook.latency",
		metric.WithDescription("Webhook delivery latency in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	ActiveMerchants, err = meter.Int64Gauge("merchant.active.count",
		metric.WithDescription("Number of active merchants"),
	)
	if err != nil {
		return err
	}

	return nil
}
