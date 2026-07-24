package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MerchantStatus string

const (
	MerchantStatusActive              MerchantStatus = "active"
	MerchantStatusSuspended           MerchantStatus = "suspended"
	MerchantStatusPendingVerification MerchantStatus = "pending_verification"
	MerchantStatusPending             MerchantStatus = "pending"
)

type KycStatus string

const (
	KycStatusPending    KycStatus = "pending"
	KycStatusVerified   KycStatus = "verified"
	KycStatusRejected   KycStatus = "rejected"
	KycStatusInProgress KycStatus = "in_progress"
)

type Merchant struct {
	ID              uuid.UUID       `json:"id"`
	LegalName       string          `json:"legal_name"`
	TradeName       string          `json:"trade_name"`
	Name            string          `json:"name"`
	Email           string          `json:"email"`
	Phone           string          `json:"phone"`
	Country         string          `json:"country"`
	Currency        string          `json:"currency"`
	Slug            string          `json:"slug"`
	Status          MerchantStatus  `json:"status"`
	KycStatus       KycStatus       `json:"kyc_status"`
	DefaultCurrency string          `json:"default_currency"`
	Timezone        string          `json:"timezone"`
	Branding        json.RawMessage `json:"branding"`
	Settings        json.RawMessage `json:"settings"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty"`
}
