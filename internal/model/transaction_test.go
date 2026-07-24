package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTransactionTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		tt       TransactionType
		expected string
	}{
		{"charge", TransactionTypeCharge, "charge"},
		{"refund", TransactionTypeRefund, "refund"},
		{"payout", TransactionTypePayout, "payout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.tt) != tt.expected {
				t.Errorf("TransactionType = %q, want %q", tt.tt, tt.expected)
			}
		})
	}
}

func TestTransactionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   TransactionStatus
		expected string
	}{
		{"pending", TransactionStatusPending, "pending"},
		{"processing", TransactionStatusProcessing, "processing"},
		{"succeeded", TransactionStatusSucceeded, "succeeded"},
		{"failed", TransactionStatusFailed, "failed"},
		{"cancelled", TransactionStatusCancelled, "cancelled"},
		{"reversed", TransactionStatusReversed, "reversed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("TransactionStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestTransactionJSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	netAmount := int64(9500)
	processedAt := now.Add(time.Minute)

	tx := Transaction{
		ID:                    uuid.New(),
		MerchantID:            uuid.New(),
		CustomerID:            uuid.New(),
		Provider:              "stripe",
		ProviderTransactionID: "txn_123",
		Type:                  TransactionTypeCharge,
		Amount:                10000,
		Currency:              "USD",
		Status:                TransactionStatusSucceeded,
		PaymentMethodID:       uuid.New(),
		IdempotencyKey:        "idem_abc",
		Description:           "Test payment",
		Metadata:              json.RawMessage(`{"key":"value"}`),
		ErrorCode:             "",
		ErrorMessage:          "",
		FeeAmount:             500,
		NetAmount:             &netAmount,
		ProcessedAt:           &processedAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Transaction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Amount != 10000 {
		t.Errorf("Amount = %d, want 10000", decoded.Amount)
	}
	if decoded.FeeAmount != 500 {
		t.Errorf("FeeAmount = %d, want 500", decoded.FeeAmount)
	}
	if decoded.NetAmount == nil || *decoded.NetAmount != 9500 {
		t.Errorf("NetAmount = %v, want 9500", decoded.NetAmount)
	}
	if decoded.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", decoded.Currency, "USD")
	}
	if decoded.Type != TransactionTypeCharge {
		t.Errorf("Type = %q, want %q", decoded.Type, TransactionTypeCharge)
	}
	if decoded.Status != TransactionStatusSucceeded {
		t.Errorf("Status = %q, want %q", decoded.Status, TransactionStatusSucceeded)
	}
	if decoded.Provider != "stripe" {
		t.Errorf("Provider = %q, want %q", decoded.Provider, "stripe")
	}
	if decoded.IdempotencyKey != "idem_abc" {
		t.Errorf("IdempotencyKey = %q, want %q", decoded.IdempotencyKey, "idem_abc")
	}
}

func TestTransactionNilOptionalFields(t *testing.T) {
	tx := Transaction{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		Amount:     5000,
		Currency:   "EUR",
		Status:     TransactionStatusPending,
		Type:       TransactionTypeRefund,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if tx.NetAmount != nil {
		t.Errorf("NetAmount should be nil, got %v", tx.NetAmount)
	}
	if tx.ProcessedAt != nil {
		t.Errorf("ProcessedAt should be nil, got %v", tx.ProcessedAt)
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Transaction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.NetAmount != nil {
		t.Errorf("decoded NetAmount should be nil, got %v", decoded.NetAmount)
	}
	if decoded.ProcessedAt != nil {
		t.Errorf("decoded ProcessedAt should be nil, got %v", decoded.ProcessedAt)
	}
}

func TestTransactionJSONRawMessage(t *testing.T) {
	tx := Transaction{
		ID:       uuid.New(),
		Metadata: json.RawMessage(`{"order_id":"ORD-42","tags":["urgent"]}`),
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Transaction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(decoded.Metadata, &meta); err != nil {
		t.Fatalf("Metadata unmarshal failed: %v", err)
	}
	if meta["order_id"] != "ORD-42" {
		t.Errorf("Metadata order_id = %v, want %q", meta["order_id"], "ORD-42")
	}
}

func TestTransactionAmountSign(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		fee    int64
	}{
		{"positive charge", 10000, 200},
		{"zero amount", 0, 0},
		{"large amount", 999999999, 10000},
		{"refund (negative)", -5000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := Transaction{
				ID:       uuid.New(),
				Amount:   tt.amount,
				FeeAmount: tt.fee,
				Currency: "USD",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if tx.Amount != tt.amount {
				t.Errorf("Amount = %d, want %d", tx.Amount, tt.amount)
			}
			if tx.FeeAmount != tt.fee {
				t.Errorf("FeeAmount = %d, want %d", tx.FeeAmount, tt.fee)
			}
		})
	}
}
