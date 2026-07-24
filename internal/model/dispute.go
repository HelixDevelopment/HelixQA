package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DisputeStatus string

const (
	DisputeStatusWarningNeedsResponse DisputeStatus = "warning_needs_response"
	DisputeStatusUnderReview          DisputeStatus = "under_review"
	DisputeStatusLost                 DisputeStatus = "lost"
	DisputeStatusWon                  DisputeStatus = "won"
	DisputeStatusClosed               DisputeStatus = "closed"
)

type Dispute struct {
	ID                    uuid.UUID      `json:"id"`
	TransactionID         uuid.UUID      `json:"transaction_id"`
	MerchantID            uuid.UUID      `json:"merchant_id"`
	Provider              string         `json:"provider"`
	ProviderDisputeID     string         `json:"provider_dispute_id"`
	Reason                string         `json:"reason"`
	Status                DisputeStatus  `json:"status"`
	Amount                int64          `json:"amount"`
	EvidenceDeadline      *time.Time     `json:"evidence_deadline"`
	EvidenceSubmittedAt   *time.Time     `json:"evidence_submitted_at"`
	Resolution            string         `json:"resolution"`
	Metadata              json.RawMessage `json:"metadata"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}
