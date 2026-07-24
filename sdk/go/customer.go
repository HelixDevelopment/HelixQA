package helixsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID         uuid.UUID `json:"id"`
	MerchantID uuid.UUID `json:"merchant_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

func (c *Client) CreateCustomer(ctx context.Context, merchantID uuid.UUID, req *CreateCustomerRequest) (*Customer, error) {
	data, err := c.do(ctx, "POST", fmt.Sprintf("/api/v1/merchants/%s/customers", merchantID), req)
	if err != nil {
		return nil, err
	}
	var customer Customer
	if err := json.Unmarshal(data, &customer); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &customer, nil
}

func (c *Client) GetCustomer(ctx context.Context, merchantID, customerID uuid.UUID) (*Customer, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s/customers/%s", merchantID, customerID), nil)
	if err != nil {
		return nil, err
	}
	var customer Customer
	if err := json.Unmarshal(data, &customer); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &customer, nil
}

func (c *Client) ListCustomers(ctx context.Context, merchantID uuid.UUID) ([]Customer, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v1/merchants/%s/customers", merchantID), nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Customers []Customer `json:"customers"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result.Customers, nil
}
