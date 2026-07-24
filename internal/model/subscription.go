package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusUnpaid   SubscriptionStatus = "unpaid"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
)

type SubscriptionInterval string

const (
	SubscriptionIntervalDay   SubscriptionInterval = "day"
	SubscriptionIntervalWeek  SubscriptionInterval = "week"
	SubscriptionIntervalMonth SubscriptionInterval = "month"
	SubscriptionIntervalYear  SubscriptionInterval = "year"
)

type Subscription struct {
	ID                      uuid.UUID            `json:"id"`
	MerchantID              uuid.UUID            `json:"merchant_id"`
	CustomerID              uuid.UUID            `json:"customer_id"`
	Provider                string               `json:"provider"`
	ProviderSubscriptionID  string               `json:"provider_subscription_id"`
	PlanID                  string               `json:"plan_id"`
	Status                  SubscriptionStatus   `json:"status"`
	Amount                  int64                `json:"amount"`
	Currency                string               `json:"currency"`
	Interval                SubscriptionInterval `json:"interval"`
	IntervalCount           int16                `json:"interval_count"`
	CurrentPeriodStart      time.Time            `json:"current_period_start"`
	CurrentPeriodEnd        time.Time            `json:"current_period_end"`
	CancelAt               *time.Time           `json:"cancel_at"`
	CancelledAt            *time.Time           `json:"cancelled_at"`
	TrialStart             *time.Time           `json:"trial_start"`
	TrialEnd               *time.Time           `json:"trial_end"`
	Metadata               json.RawMessage      `json:"metadata"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}
