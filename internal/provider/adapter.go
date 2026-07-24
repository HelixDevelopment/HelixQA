package provider

import (
	"context"

	"github.com/helix-seller/helix-seller/internal/model"
)

type PaymentProvider interface {
	Name() string
	Charge(ctx context.Context, req *ChargeRequest) (*model.Transaction, error)
	Refund(ctx context.Context, req *RefundRequest) (*model.Transaction, error)
	VerifyWebhookSignature(payload []byte, sigHeader string, secret string) (bool, error)
}

type ChargeRequest struct {
	Amount         int64
	Currency       string
	Source         string
	Description    string
	IdempotencyKey string
	Metadata       map[string]string
}

type RefundRequest struct {
	TransactionID  string
	Amount         *int64
	Reason         string
	IdempotencyKey string
	Metadata       map[string]string
}
