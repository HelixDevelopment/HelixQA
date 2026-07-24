package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type PaymentMethodType string

const (
	PaymentMethodTypeCard       PaymentMethodType = "card"
	PaymentMethodTypeBankAccount PaymentMethodType = "bank_account"
	PaymentMethodTypeWallet     PaymentMethodType = "wallet"
)

type PaymentMethod struct {
	ID           uuid.UUID         `json:"id"`
	MerchantID   uuid.UUID         `json:"merchant_id"`
	CustomerID   uuid.UUID         `json:"customer_id"`
	Type         PaymentMethodType `json:"type"`
	Provider     string            `json:"provider"`
	ProviderToken string           `json:"provider_token"`
	Fingerprint  string            `json:"fingerprint"`
	Brand        string            `json:"brand"`
	Last4        string            `json:"last4"`
	ExpMonth     int16             `json:"exp_month"`
	ExpYear      int16             `json:"exp_year"`
	IsDefault    bool              `json:"is_default"`
	Metadata     json.RawMessage   `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}
