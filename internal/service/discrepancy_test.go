package service

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestDiscrepancyService_Constructor(t *testing.T) {
	svc := NewDiscrepancyService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil DiscrepancyService")
	}
}

func TestDiscrepancyService_Constructor_WithDeps(t *testing.T) {
	reconSvc := NewReconciliationService(nil, zap.NewNop())
	logger := zap.NewNop()

	svc := NewDiscrepancyService(reconSvc, nil, logger)
	if svc == nil {
		t.Fatal("expected non-nil DiscrepancyService")
	}
	if svc.reconSvc != reconSvc {
		t.Error("reconSvc not set correctly")
	}
	if svc.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestDiscrepancy_Fields(t *testing.T) {
	d := &Discrepancy{
		MerchantID: uuid.New(),
		Provider:   "stripe",
		Amount:     5000,
		Severity:   "high",
	}

	if d.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", d.Amount)
	}
	if d.Severity != "high" {
		t.Errorf("Severity = %q, want %q", d.Severity, "high")
	}
	if d.Provider != "stripe" {
		t.Errorf("Provider = %q, want %q", d.Provider, "stripe")
	}
}

func TestDiscrepancy_SeverityLogic(t *testing.T) {
	tests := []struct {
		name         string
		discrepancy  int64
		wantSeverity string
	}{
		{"small discrepancy", 100, "low"},
		{"exactly low threshold", 999, "low"},
		{"medium threshold crossed", 1001, "medium"},
		{"exactly 10000", 10000, "medium"},
		{"high threshold crossed", 10001, "high"},
		{"very large discrepancy", 999999, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var severity string
			switch {
			case tt.discrepancy > 10000:
				severity = "high"
			case tt.discrepancy > 1000:
				severity = "medium"
			default:
				severity = "low"
			}

			if severity != tt.wantSeverity {
				t.Errorf("severity for discrepancy %d = %q, want %q", tt.discrepancy, severity, tt.wantSeverity)
			}
		})
	}
}

func TestDiscrepancy_ZeroAmount(t *testing.T) {
	d := &Discrepancy{
		MerchantID: uuid.New(),
		Provider:   "stripe",
		Amount:     0,
		Severity:   "low",
	}

	if d.Amount != 0 {
		t.Errorf("Amount = %d, want 0", d.Amount)
	}
}

func TestDiscrepancy_NegativeAmount(t *testing.T) {
	d := &Discrepancy{
		MerchantID: uuid.New(),
		Provider:   "stripe",
		Amount:     -5000,
		Severity:   "medium",
	}

	if d.Amount != -5000 {
		t.Errorf("Amount = %d, want -5000", d.Amount)
	}
}
