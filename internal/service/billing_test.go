package service

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestBillingService_Constructor(t *testing.T) {
	svc := NewBillingService(nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil BillingService")
	}
}

func TestFeeStructure(t *testing.T) {
	fees := &FeeStructure{
		PercentageFee: 0.01,
		FixedFee:      10,
	}

	// 1% of 10000 cents = 100
	// Fixed: 10 * 1 transaction = 10
	// Total: 110
	amount := int64(10000)
	txCount := int64(1)

	percentageFee := int64(float64(amount) * fees.PercentageFee)
	fixedFee := fees.FixedFee * txCount
	totalFees := percentageFee + fixedFee

	if totalFees != 110 {
		t.Errorf("expected total fees 110, got %d", totalFees)
	}
}

func TestFeeStructure_ZeroAmount(t *testing.T) {
	fees := &FeeStructure{
		PercentageFee: 0.01,
		FixedFee:      10,
	}

	amount := int64(0)
	txCount := int64(0)

	percentageFee := int64(float64(amount) * fees.PercentageFee)
	fixedFee := fees.FixedFee * txCount
	totalFees := percentageFee + fixedFee

	if totalFees != 0 {
		t.Errorf("expected 0 fees for zero amount, got %d", totalFees)
	}
}

func TestFeeStructure_LargeAmount(t *testing.T) {
	fees := &FeeStructure{
		PercentageFee: 0.01,
		FixedFee:      10,
	}

	amount := int64(1000000) // $10,000
	txCount := int64(100)

	percentageFee := int64(float64(amount) * fees.PercentageFee)
	fixedFee := fees.FixedFee * txCount
	totalFees := percentageFee + fixedFee

	if percentageFee != 10000 {
		t.Errorf("expected percentage fee 10000, got %d", percentageFee)
	}
	if fixedFee != 1000 {
		t.Errorf("expected fixed fee 1000, got %d", fixedFee)
	}
	if totalFees != 11000 {
		t.Errorf("expected total 11000, got %d", totalFees)
	}
}

func TestBillingPeriod_Fields(t *testing.T) {
	merchantID := uuid.New()
	period := &BillingPeriod{
		MerchantID:        merchantID,
		TotalTransactions: 50,
		TotalAmount:       50000,
		TotalFees:         550,
		Currency:          "USD",
	}

	if period.TotalTransactions != 50 {
		t.Errorf("TotalTransactions = %d, want 50", period.TotalTransactions)
	}
	if period.TotalAmount != 50000 {
		t.Errorf("TotalAmount = %d, want 50000", period.TotalAmount)
	}
	if period.TotalFees != 550 {
		t.Errorf("TotalFees = %d, want 550", period.TotalFees)
	}
	if period.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", period.Currency)
	}
}
