package provider

import (
	"context"
	"fmt"

	"github.com/helix-seller/helix-seller/internal/model"
)

type PayPalProvider struct{}

func NewPayPalProvider(clientID, secret, webhookID string) *PayPalProvider {
	return &PayPalProvider{}
}

func (p *PayPalProvider) Name() string { return "paypal" }

func (p *PayPalProvider) Charge(ctx context.Context, req *ChargeRequest) (*model.Transaction, error) {
	return nil, fmt.Errorf("paypal provider not yet configured")
}

func (p *PayPalProvider) Refund(ctx context.Context, req *RefundRequest) (*model.Transaction, error) {
	return nil, fmt.Errorf("paypal provider not yet configured")
}

func (p *PayPalProvider) VerifyWebhookSignature(payload []byte, sigHeader string, secret string) (bool, error) {
	return false, fmt.Errorf("paypal provider not yet configured")
}
