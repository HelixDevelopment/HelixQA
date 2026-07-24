package helixsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID         uuid.UUID `json:"id"`
	MerchantID uuid.UUID `json:"merchant_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Amount     int64     `json:"amount"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Provider   string    `json:"provider"`
	CreatedAt  time.Time `json:"created_at"`
}

type ProcessPaymentRequest struct {
	CustomerID      string `json:"customer_id"`
	PaymentMethodID string `json:"payment_method_id"`
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	IdempotencyKey  string `json:"idempotency_key"`
}

func (c *Client) ProcessPayment(ctx context.Context, merchantID uuid.UUID, req *ProcessPaymentRequest) (*Transaction, error) {
	data, err := c.do(ctx, "POST", fmt.Sprintf("/api/v1/merchants/%s/transactions", merchantID), req)
	if err != nil {
		return nil, err
	}
	var tx Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &tx, nil
}

func (c *Client) GetTransaction(ctx context.Context, merchantID, transactionID uuid.UUID) (*Transaction, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s/transactions/%s", merchantID, transactionID), nil)
	if err != nil {
		return nil, err
	}
	var tx Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &tx, nil
}

func (c *Client) ListTransactions(ctx context.Context, merchantID uuid.UUID) ([]Transaction, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s/transactions", merchantID), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Transactions []Transaction `json:"transactions"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result.Transactions, nil
}
