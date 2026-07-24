package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft        InvoiceStatus = "draft"
	InvoiceStatusOpen         InvoiceStatus = "open"
	InvoiceStatusPaid         InvoiceStatus = "paid"
	InvoiceStatusVoid         InvoiceStatus = "void"
	InvoiceStatusUncollectible InvoiceStatus = "uncollectible"
)

type Invoice struct {
	ID                uuid.UUID      `json:"id"`
	MerchantID        uuid.UUID      `json:"merchant_id"`
	CustomerID        uuid.UUID      `json:"customer_id"`
	SubscriptionID    *uuid.UUID     `json:"subscription_id"`
	Provider          string         `json:"provider"`
	ProviderInvoiceID string         `json:"provider_invoice_id"`
	Amount            int64          `json:"amount"`
	Currency          string         `json:"currency"`
	Status            InvoiceStatus  `json:"status"`
	DueDate           time.Time      `json:"due_date"`
	PaidAt            *time.Time     `json:"paid_at"`
	PeriodStart       time.Time      `json:"period_start"`
	PeriodEnd         time.Time      `json:"period_end"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}
