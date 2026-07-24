package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleRootAdmin    UserRole = "root_admin"
	RoleAccountAdmin UserRole = "account_admin"
	RoleUser         UserRole = "user"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name"`
	Role         UserRole   `json:"role"`
	MerchantID   uuid.UUID  `json:"merchant_id"`
	IsActive     bool       `json:"is_active"`
	MfaEnabled   bool       `json:"mfa_enabled"`
	MfaSecret    *string    `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ApiKey struct {
	ID         uuid.UUID  `json:"id"`
	MerchantID uuid.UUID  `json:"merchant_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Scopes     []string   `json:"scopes"`
	RateLimit  int        `json:"rate_limit"`
	IsActive   bool       `json:"is_active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
