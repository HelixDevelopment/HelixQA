package observability

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestInitOTel(t *testing.T) {
	logger := zap.NewNop()
	cfg := OTelConfig{
		ServiceName:    "helix-seller-test",
		ServiceVersion: "0.0.1-test",
		Environment:    "test",
	}

	shutdown, err := InitOTel(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}
	defer shutdown()

	if RequestDuration == nil {
		t.Error("RequestDuration not initialized after InitOTel")
	}
	if RequestCounter == nil {
		t.Error("RequestCounter not initialized after InitOTel")
	}
	if TransactionCounter == nil {
		t.Error("TransactionCounter not initialized after InitOTel")
	}
}

func TestInitOTel_CalledTwice(t *testing.T) {
	logger := zap.NewNop()
	cfg := OTelConfig{
		ServiceName:    "helix-seller-test-2",
		ServiceVersion: "0.0.1-test",
		Environment:    "test",
	}

	shutdown1, err := InitOTel(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("InitOTel first call failed: %v", err)
	}
	defer shutdown1()

	shutdown2, err := InitOTel(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("InitOTel second call failed: %v", err)
	}
	defer shutdown2()
}

func TestInitOTel_ShutdownSafe(t *testing.T) {
	logger := zap.NewNop()
	cfg := OTelConfig{
		ServiceName:    "helix-seller-test-shutdown",
		ServiceVersion: "0.0.1-test",
		Environment:    "test",
	}

	shutdown, err := InitOTel(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("InitOTel failed: %v", err)
	}

	shutdown()

	// Second shutdown should not panic
	shutdown()
}

func TestOTelConfig_Fields(t *testing.T) {
	cfg := OTelConfig{
		ServiceName:    "test-svc",
		ServiceVersion: "1.0.0",
		Environment:    "production",
	}

	if cfg.ServiceName != "test-svc" {
		t.Errorf("ServiceName = %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "1.0.0" {
		t.Errorf("ServiceVersion = %q", cfg.ServiceVersion)
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q", cfg.Environment)
	}
}
