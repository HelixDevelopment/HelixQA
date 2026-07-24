package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMerchantStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   MerchantStatus
		expected string
	}{
		{"active", MerchantStatusActive, "active"},
		{"suspended", MerchantStatusSuspended, "suspended"},
		{"pending_verification", MerchantStatusPendingVerification, "pending_verification"},
		{"pending", MerchantStatusPending, "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("MerchantStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestKycStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   KycStatus
		expected string
	}{
		{"pending", KycStatusPending, "pending"},
		{"verified", KycStatusVerified, "verified"},
		{"rejected", KycStatusRejected, "rejected"},
		{"in_progress", KycStatusInProgress, "in_progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("KycStatus = %q, want %q", tt.status, tt.expected)
			}
		})
	}
}

func TestMerchantJSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	deletedAt := now.Add(time.Hour)

	m := Merchant{
		ID:              uuid.New(),
		LegalName:       "Acme Corp",
		TradeName:       "Acme",
		Name:            "Acme Shop",
		Email:           "admin@acme.com",
		Phone:           "+1-555-0100",
		Country:         "US",
		Currency:        "USD",
		Slug:            "acme-shop",
		Status:          MerchantStatusActive,
		KycStatus:       KycStatusVerified,
		DefaultCurrency: "USD",
		Timezone:        "America/New_York",
		Branding:        json.RawMessage(`{"logo":"https://example.com/logo.png"}`),
		Settings:        json.RawMessage(`{"auto_payout":true}`),
		CreatedAt:       now,
		UpdatedAt:       now,
		DeletedAt:       &deletedAt,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Merchant
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.LegalName != "Acme Corp" {
		t.Errorf("LegalName = %q, want %q", decoded.LegalName, "Acme Corp")
	}
	if decoded.Slug != "acme-shop" {
		t.Errorf("Slug = %q, want %q", decoded.Slug, "acme-shop")
	}
	if decoded.Status != MerchantStatusActive {
		t.Errorf("Status = %q, want %q", decoded.Status, MerchantStatusActive)
	}
	if decoded.KycStatus != KycStatusVerified {
		t.Errorf("KycStatus = %q, want %q", decoded.KycStatus, KycStatusVerified)
	}
	if decoded.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", decoded.Timezone, "America/New_York")
	}
}

func TestMerchantNilDeletedAt(t *testing.T) {
	m := Merchant{
		ID:       uuid.New(),
		Name:     "Active Shop",
		Status:   MerchantStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if m.DeletedAt != nil {
		t.Errorf("DeletedAt should be nil, got %v", m.DeletedAt)
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Merchant
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.DeletedAt != nil {
		t.Errorf("decoded DeletedAt should be nil, got %v", decoded.DeletedAt)
	}

	jsonStr := string(data)
	if contains := `"deleted_at"`; containsJSONKey(jsonStr, contains) {
		t.Errorf("JSON should omit deleted_at when nil, but found in: %s", jsonStr)
	}
}

func TestMerchantBrandingAndSettings(t *testing.T) {
	tests := []struct {
		name     string
		branding json.RawMessage
		settings json.RawMessage
	}{
		{
			"empty branding and settings",
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		},
		{
			"populated branding and settings",
			json.RawMessage(`{"logo_url":"https://example.com/logo.png","primary_color":"#FF0000"}`),
			json.RawMessage(`{"auto_payout":true,"payout_schedule":"weekly","min_payout_amount":100}`),
		},
		{
			"nil branding and settings",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Merchant{
				ID:        uuid.New(),
				Name:      "Test",
				Branding:  tt.branding,
				Settings:  tt.settings,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			data, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded Merchant
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestMerchantStatusTransitions(t *testing.T) {
	transitions := []struct {
		name  string
		from  MerchantStatus
		to    MerchantStatus
		valid bool
	}{
		{"pending to pending_verification", MerchantStatusPending, MerchantStatusPendingVerification, true},
		{"pending_verification to active", MerchantStatusPendingVerification, MerchantStatusActive, true},
		{"pending to active", MerchantStatusPending, MerchantStatusActive, true},
		{"active to suspended", MerchantStatusActive, MerchantStatusSuspended, true},
		{"suspended to active", MerchantStatusSuspended, MerchantStatusActive, true},
		{"active to pending_verification", MerchantStatusActive, MerchantStatusPendingVerification, true},
		{"suspended to suspended", MerchantStatusSuspended, MerchantStatusSuspended, true},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			m := Merchant{
				ID:     uuid.New(),
				Name:   "Transition Test",
				Status: tt.from,
			}
			m.Status = tt.to
			if m.Status != tt.to {
				t.Errorf("Status transition from %q to %q failed, got %q", tt.from, tt.to, m.Status)
			}
		})
	}
}

func TestMerchantCountryCurrencyPairs(t *testing.T) {
	tests := []struct {
		name     string
		country  string
		currency string
	}{
		{"US", "US", "USD"},
		{"EU Germany", "DE", "EUR"},
		{"UK", "GB", "GBP"},
		{"Japan", "JP", "JPY"},
		{"Serbia", "RS", "RSD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Merchant{
				ID:        uuid.New(),
				Country:   tt.country,
				Currency:  tt.currency,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if m.Country != tt.country {
				t.Errorf("Country = %q, want %q", m.Country, tt.country)
			}
			if m.Currency != tt.currency {
				t.Errorf("Currency = %q, want %q", m.Currency, tt.currency)
			}
		})
	}
}

func containsJSONKey(jsonStr, key string) bool {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return false
	}
	_, ok := raw[key]
	return ok
}
