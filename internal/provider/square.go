package provider

import (
	"context"
	"fmt"

	"github.com/helix-seller/helix-seller/internal/model"
)

type SquareProvider struct{}

func NewSquareProvider(accessToken, applicationID, webhookSigKey string) *SquareProvider {
	return &SquareProvider{}
}

func (p *SquareProvider) Name() string { return "square" }

func (p *SquareProvider) Charge(ctx context.Context, req *ChargeRequest) (*model.Transaction, error) {
	return nil, fmt.Errorf("square provider not yet configured")
}

func (p *SquareProvider) Refund(ctx context.Context, req *RefundRequest) (*model.Transaction, error) {
	return nil, fmt.Errorf("square provider not yet configured")
}

func (p *SquareProvider) VerifyWebhookSignature(payload []byte, sigHeader string, secret string) (bool, error) {
	return false, fmt.Errorf("square provider not yet configured")
}
