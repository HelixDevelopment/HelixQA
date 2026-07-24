package service

import (
	"testing"

	"go.uber.org/zap"
)

func TestPaymentService_Constructor(t *testing.T) {
	svc := NewPaymentService(nil, nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil PaymentService")
	}
}

func TestSubscriptionService_Constructor(t *testing.T) {
	svc := NewSubscriptionService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil SubscriptionService")
	}
}

func TestInvoiceService_Constructor(t *testing.T) {
	svc := NewInvoiceService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil InvoiceService")
	}
}

func TestPayoutService_Constructor(t *testing.T) {
	svc := NewPayoutService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil PayoutService")
	}
}

func TestDisputeService_Constructor(t *testing.T) {
	svc := NewDisputeService(nil, nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil DisputeService")
	}
}

func TestExchangeRateService_Constructor(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil ExchangeRateService")
	}
	if svc.client == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestAnalyticsService_Constructor(t *testing.T) {
	svc := NewAnalyticsService(nil)
	if svc == nil {
		t.Fatal("expected non-nil AnalyticsService")
	}
}

func TestBackgroundService_Constructor(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 4, 5000000000)
	if svc == nil {
		t.Fatal("expected non-nil BackgroundService")
	}
	if svc.workers != 4 {
		t.Errorf("workers = %d, want 4", svc.workers)
	}
}

func TestAnalyticsSummary_Fields(t *testing.T) {
	s := &AnalyticsSummary{
		TotalRevenue:           100000,
		TotalTransactions:      500,
		SuccessfulTransactions: 480,
		FailedTransactions:     20,
		AverageTransactionSize: 200.0,
		RefundAmount:           5000,
		Period:                 "2026-01-01 to 2026-01-31",
	}

	if s.TotalRevenue != 100000 {
		t.Errorf("TotalRevenue = %d", s.TotalRevenue)
	}
	if s.TotalTransactions != 500 {
		t.Errorf("TotalTransactions = %d", s.TotalTransactions)
	}
	if s.SuccessfulTransactions != 480 {
		t.Errorf("SuccessfulTransactions = %d", s.SuccessfulTransactions)
	}
	if s.FailedTransactions != 20 {
		t.Errorf("FailedTransactions = %d", s.FailedTransactions)
	}
	if s.AverageTransactionSize != 200.0 {
		t.Errorf("AverageTransactionSize = %f", s.AverageTransactionSize)
	}
	if s.RefundAmount != 5000 {
		t.Errorf("RefundAmount = %d", s.RefundAmount)
	}
}

func TestFeeStructure_Fields(t *testing.T) {
	f := &FeeStructure{
		PercentageFee: 0.01,
		FixedFee:      10,
	}

	if f.PercentageFee != 0.01 {
		t.Errorf("PercentageFee = %f", f.PercentageFee)
	}
	if f.FixedFee != 10 {
		t.Errorf("FixedFee = %d", f.FixedFee)
	}
}

func TestBackgroundTaskRow_Fields(t *testing.T) {
	task := &backgroundTaskRow{
		Type:     "reconciliation",
		Priority: 5,
		Attempts: 2,
	}

	if task.Type != "reconciliation" {
		t.Errorf("Type = %q", task.Type)
	}
	if task.Priority != 5 {
		t.Errorf("Priority = %d", task.Priority)
	}
	if task.Attempts != 2 {
		t.Errorf("Attempts = %d", task.Attempts)
	}
}

func TestFrankfurterResponse_Fields(t *testing.T) {
	fr := frankfurterResponse{
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
		},
	}

	if fr.Rates["EUR"] != 0.85 {
		t.Errorf("EUR rate = %f", fr.Rates["EUR"])
	}
	if fr.Rates["GBP"] != 0.73 {
		t.Errorf("GBP rate = %f", fr.Rates["GBP"])
	}
}
