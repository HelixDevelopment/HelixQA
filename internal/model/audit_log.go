package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ActorType string

const (
	ActorTypeRootAdmin ActorType = "root_admin"
	ActorTypeAccountAdmin ActorType = "account_admin"
	ActorTypeUser      ActorType = "user"
	ActorTypeSystem    ActorType = "system"
	ActorTypeAPIKey    ActorType = "api_key"
)

type AuditLog struct {
	ID           uuid.UUID       `json:"id"`
	MerchantID   uuid.UUID       `json:"merchant_id"`
	ActorID      uuid.UUID       `json:"actor_id"`
	ActorType    ActorType       `json:"actor_type"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Changes      json.RawMessage `json:"changes"`
	IPAddress    string          `json:"ip_address"`
	UserAgent    string          `json:"user_agent"`
	CreatedAt    time.Time       `json:"created_at"`
}
