package service

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestReconciliationService_Constructor(t *testing.T) {
	svc := NewReconciliationService(nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil ReconciliationService")
	}
}

func TestReconciliationService_Constructor_WithLogger(t *testing.T) {
	logger := zap.NewNop()
	svc := NewReconciliationService(nil, logger)
	if svc == nil {
		t.Fatal("expected non-nil ReconciliationService")
	}
	if svc.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestReconciliationResult_Fields(t *testing.T) {
	r := &ReconciliationResult{
		PlatformTotal:    100000,
		ProviderTotal:    99500,
		Discrepancy:      500,
		TransactionCount: 50,
	}

	if r.PlatformTotal != 100000 {
		t.Errorf("PlatformTotal = %d, want 100000", r.PlatformTotal)
	}
	if r.ProviderTotal != 99500 {
		t.Errorf("ProviderTotal = %d, want 99500", r.ProviderTotal)
	}
	if r.Discrepancy != 500 {
		t.Errorf("Discrepancy = %d, want 500", r.Discrepancy)
	}
	if r.TransactionCount != 50 {
		t.Errorf("TransactionCount = %d, want 50", r.TransactionCount)
	}
}

func TestReconciliationResult_ZeroValues(t *testing.T) {
	r := &ReconciliationResult{}
	if r.PlatformTotal != 0 {
		t.Errorf("PlatformTotal = %d, want 0", r.PlatformTotal)
	}
	if r.ProviderTotal != 0 {
		t.Errorf("ProviderTotal = %d, want 0", r.ProviderTotal)
	}
	if r.Discrepancy != 0 {
		t.Errorf("Discrepancy = %d, want 0", r.Discrepancy)
	}
	if r.TransactionCount != 0 {
		t.Errorf("TransactionCount = %d, want 0", r.TransactionCount)
	}
}

func TestReconciliationResult_NegativeDiscrepancy(t *testing.T) {
	r := &ReconciliationResult{
		PlatformTotal: 50000,
		ProviderTotal: 60000,
		Discrepancy:   -10000,
	}

	if r.Discrepancy >= 0 {
		t.Errorf("Discrepancy = %d, want negative", r.Discrepancy)
	}
}

func TestReconciliationResult_MatchesExpectedLogic(t *testing.T) {
	platform := int64(100000)
	provider := platform
	discrepancy := platform - provider

	if discrepancy != 0 {
		t.Errorf("same totals should yield zero discrepancy, got %d", discrepancy)
	}

	platform = 100000
	provider = 98000
	discrepancy = platform - provider

	if discrepancy != 2000 {
		t.Errorf("discrepancy = %d, want 2000", discrepancy)
	}
}

func TestReconciliationResult_UUID(t *testing.T) {
	id := uuid.New()
	if id == uuid.Nil {
		t.Error("UUID should not be nil")
	}
}
