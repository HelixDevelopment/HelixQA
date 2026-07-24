package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID         uuid.UUID       `json:"id"`
	MerchantID uuid.UUID       `json:"merchant_id"`
	ExternalID string          `json:"external_id"`
	Name       string          `json:"name"`
	Email      string          `json:"email"`
	Phone      string          `json:"phone"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  *time.Time      `json:"deleted_at,omitempty"`
}
