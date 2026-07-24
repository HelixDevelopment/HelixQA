package model

import "time"

type ExchangeRate struct {
	ID            int32     `json:"id"`
	BaseCurrency  string    `json:"base_currency"`
	QuoteCurrency string    `json:"quote_currency"`
	Rate          string    `json:"rate"`
	Source        string    `json:"source"`
	FetchedAt     time.Time `json:"fetched_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}
