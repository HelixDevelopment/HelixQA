package helixsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Merchant struct {
	ID        uuid.UUID `json:"id"`
	LegalName string    `json:"legal_name"`
	TradeName string    `json:"trade_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Country   string    `json:"country"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateMerchantRequest struct {
	LegalName string `json:"legal_name"`
	TradeName string `json:"trade_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Country   string `json:"country"`
	Currency  string `json:"currency"`
}

func (c *Client) CreateMerchant(ctx context.Context, req *CreateMerchantRequest) (*Merchant, error) {
	data, err := c.do(ctx, "POST", "/api/v1/merchants", req)
	if err != nil {
		return nil, err
	}
	var merchant Merchant
	if err := json.Unmarshal(data, &merchant); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &merchant, nil
}

func (c *Client) GetMerchant(ctx context.Context, id uuid.UUID) (*Merchant, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s", id), nil)
	if err != nil {
		return nil, err
	}
	var merchant Merchant
	if err := json.Unmarshal(data, &merchant); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &merchant, nil
}

func (c *Client) ListMerchants(ctx context.Context) ([]Merchant, error) {
	data, err := c.do(ctx, "GET", "/api/v1/merchants", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Merchants []Merchant `json:"merchants"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result.Merchants, nil
}
