package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type ProviderConfig struct {
	ID               uuid.UUID       `json:"id"`
	MerchantID       uuid.UUID       `json:"merchant_id"`
	Provider         string          `json:"provider"`
	IsActive         bool            `json:"is_active"`
	Config           json.RawMessage `json:"config"`
	FallbackOrder    int16           `json:"fallback_order"`
	HealthStatus     HealthStatus    `json:"health_status"`
	LastHealthCheck  *time.Time      `json:"last_health_check"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
