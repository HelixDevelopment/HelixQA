package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "pending"
	PayoutStatusInTransit  PayoutStatus = "in_transit"
	PayoutStatusPaid       PayoutStatus = "paid"
	PayoutStatusFailed     PayoutStatus = "failed"
	PayoutStatusCancelled  PayoutStatus = "cancelled"
)

type PayoutMethod string

const (
	PayoutMethodStandard PayoutMethod = "standard"
	PayoutMethodInstant  PayoutMethod = "instant"
)

type Payout struct {
	ID               uuid.UUID      `json:"id"`
	MerchantID       uuid.UUID      `json:"merchant_id"`
	Provider         string         `json:"provider"`
	ProviderPayoutID string         `json:"provider_payout_id"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	Status           PayoutStatus   `json:"status"`
	Method           PayoutMethod   `json:"method"`
	ArrivalDate      *time.Time     `json:"arrival_date"`
	FeeAmount        int64          `json:"fee_amount"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}
