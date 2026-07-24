package observability

import (
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestInitMetrics(t *testing.T) {
	meterProvider := sdkmetric.NewMeterProvider()
	meter := meterProvider.Meter("test")

	err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics failed: %v", err)
	}

	if RequestDuration == nil {
		t.Error("RequestDuration not initialized")
	}
	if RequestCounter == nil {
		t.Error("RequestCounter not initialized")
	}
	if RequestErrors == nil {
		t.Error("RequestErrors not initialized")
	}
	if TransactionCounter == nil {
		t.Error("TransactionCounter not initialized")
	}
	if TransactionAmount == nil {
		t.Error("TransactionAmount not initialized")
	}
	if PaymentSuccessRate == nil {
		t.Error("PaymentSuccessRate not initialized")
	}
	if ProviderLatency == nil {
		t.Error("ProviderLatency not initialized")
	}
	if ProviderErrors == nil {
		t.Error("ProviderErrors not initialized")
	}
	if ProviderCircuitBreak == nil {
		t.Error("ProviderCircuitBreak not initialized")
	}
	if WebhookDeliveryCount == nil {
		t.Error("WebhookDeliveryCount not initialized")
	}
	if WebhookDeliveryErrors == nil {
		t.Error("WebhookDeliveryErrors not initialized")
	}
	if WebhookLatency == nil {
		t.Error("WebhookLatency not initialized")
	}
	if ActiveMerchants == nil {
		t.Error("ActiveMerchants not initialized")
	}
}
