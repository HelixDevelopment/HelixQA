package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TransactionTypeCharge TransactionType = "charge"
	TransactionTypeRefund TransactionType = "refund"
	TransactionTypePayout TransactionType = "payout"
)

type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusProcessing TransactionStatus = "processing"
	TransactionStatusSucceeded  TransactionStatus = "succeeded"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusCancelled  TransactionStatus = "cancelled"
	TransactionStatusReversed   TransactionStatus = "reversed"
)

type Transaction struct {
	ID                    uuid.UUID         `json:"id"`
	MerchantID            uuid.UUID         `json:"merchant_id"`
	CustomerID            uuid.UUID         `json:"customer_id"`
	Provider              string            `json:"provider"`
	ProviderTransactionID string            `json:"provider_transaction_id"`
	Type                  TransactionType   `json:"type"`
	Amount                int64             `json:"amount"`
	Currency              string            `json:"currency"`
	Status                TransactionStatus `json:"status"`
	PaymentMethodID       uuid.UUID         `json:"payment_method_id"`
	IdempotencyKey        string            `json:"idempotency_key"`
	Description           string            `json:"description"`
	Metadata              json.RawMessage   `json:"metadata"`
	ErrorCode             string            `json:"error_code"`
	ErrorMessage          string            `json:"error_message"`
	FeeAmount             int64             `json:"fee_amount"`
	NetAmount             *int64            `json:"net_amount"`
	ProcessedAt           *time.Time        `json:"processed_at"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}
