package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type WebhookConfig struct {
	ID        uuid.UUID       `json:"id"`
	MerchantID uuid.UUID      `json:"merchant_id"`
	URL       string          `json:"url"`
	Secret    string          `json:"secret"`
	Events    []string        `json:"events"`
	IsActive  bool            `json:"is_active"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
