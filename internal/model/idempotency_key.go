package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type IdempotencyKey struct {
	ID         int32           `json:"id"`
	KeyHash    string          `json:"key_hash"`
	Response   json.RawMessage `json:"response"`
	StatusCode int16           `json:"status_code"`
	MerchantID uuid.UUID       `json:"merchant_id"`
	CreatedAt  time.Time       `json:"created_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
}
