package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
)

type StripeProvider struct {
	apiKey      string
	webhookSecret string
	httpClient  *http.Client
}

func NewStripeProvider(apiKey, webhookSecret string) *StripeProvider {
	return &StripeProvider{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *StripeProvider) Name() string { return "stripe" }

type stripeChargeRequest struct {
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	Source      string            `json:"source"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

type stripeChargeResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Amount int64  `json:"amount"`
}

func (p *StripeProvider) Charge(ctx context.Context, req *ChargeRequest) (*model.Transaction, error) {
	body := stripeChargeRequest{
		Amount:      req.Amount,
		Currency:    req.Currency,
		Source:      req.Source,
		Description: req.Description,
		Metadata:    req.Metadata,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("stripe: marshal charge request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/charges", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("stripe: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stripe: charge request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stripe: charge failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var stripeResp stripeChargeResponse
	if err := json.Unmarshal(respBody, &stripeResp); err != nil {
		return nil, fmt.Errorf("stripe: parse response: %w", err)
	}

	status := model.TransactionStatusSucceeded
	if stripeResp.Status != "succeeded" {
		status = model.TransactionStatusFailed
	}

	now := time.Now()
	return &model.Transaction{
		ID:                    uuid.New(),
		Provider:              "stripe",
		ProviderTransactionID: stripeResp.ID,
		Type:                  model.TransactionTypeCharge,
		Amount:                stripeResp.Amount,
		Currency:              req.Currency,
		Status:                status,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil
}

type stripeRefundRequest struct {
	Charge     string `json:"charge"`
	Amount     *int64 `json:"amount,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type stripeRefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (p *StripeProvider) Refund(ctx context.Context, req *RefundRequest) (*model.Transaction, error) {
	body := stripeRefundRequest{
		Charge:   req.TransactionID,
		Amount:   req.Amount,
		Reason:   req.Reason,
		Metadata: req.Metadata,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("stripe: marshal refund request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.stripe.com/v1/refunds", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("stripe: create refund request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stripe: refund request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe: read refund response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stripe: refund failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var stripeResp stripeRefundResponse
	if err := json.Unmarshal(respBody, &stripeResp); err != nil {
		return nil, fmt.Errorf("stripe: parse refund response: %w", err)
	}

	status := model.TransactionStatusSucceeded
	if stripeResp.Status != "succeeded" {
		status = model.TransactionStatusFailed
	}

	now := time.Now()
	return &model.Transaction{
		ID:                    uuid.New(),
		Provider:              "stripe",
		ProviderTransactionID: stripeResp.ID,
		Type:                  model.TransactionTypeRefund,
		Status:                status,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil
}

func (p *StripeProvider) VerifyWebhookSignature(payload []byte, sigHeader string, secret string) (bool, error) {
	expectedSig := computeStripeSignature(payload, secret)
	return hmac.Equal([]byte(sigHeader), []byte(expectedSig)), nil
}

func computeStripeSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
