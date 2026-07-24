package helixsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID         uuid.UUID `json:"id"`
	MerchantID uuid.UUID `json:"merchant_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Amount     int64     `json:"amount"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Interval   string    `json:"interval"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateSubscriptionRequest struct {
	CustomerID    string `json:"customer_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Interval      string `json:"interval"`
	IntervalCount int    `json:"interval_count"`
}

func (c *Client) CreateSubscription(ctx context.Context, merchantID uuid.UUID, req *CreateSubscriptionRequest) (*Subscription, error) {
	data, err := c.do(ctx, "POST", fmt.Sprintf("/api/v1/merchants/%s/subscriptions", merchantID), req)
	if err != nil {
		return nil, err
	}
	var sub Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &sub, nil
}

func (c *Client) GetSubscription(ctx context.Context, merchantID, subscriptionID uuid.UUID) (*Subscription, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s/subscriptions/%s", merchantID, subscriptionID), nil)
	if err != nil {
		return nil, err
	}
	var sub Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &sub, nil
}

func (c *Client) CancelSubscription(ctx context.Context, merchantID, subscriptionID uuid.UUID) error {
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/api/v1/merchants/%s/subscriptions/%s", merchantID, subscriptionID), nil)
	return err
}
