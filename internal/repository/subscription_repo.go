package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type SubscriptionRepo struct {
	db *pgxpool.Pool
}

func NewSubscriptionRepo(db *pgxpool.Pool) *SubscriptionRepo {
	return &SubscriptionRepo{db: db}
}

func (r *SubscriptionRepo) Create(ctx context.Context, s *model.Subscription) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO subscriptions (id, merchant_id, customer_id, provider, provider_subscription_id, plan_id, status, amount, currency, interval, interval_count, current_period_start, current_period_end, cancel_at, cancelled_at, trial_start, trial_end, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW())`,
		s.ID, s.MerchantID, s.CustomerID, s.Provider, s.ProviderSubscriptionID, s.PlanID,
		s.Status, s.Amount, s.Currency, s.Interval, s.IntervalCount,
		s.CurrentPeriodStart, s.CurrentPeriodEnd, s.CancelAt, s.CancelledAt,
		s.TrialStart, s.TrialEnd, s.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert subscription: %w", err)
	}
	return nil
}

func (r *SubscriptionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	s := &model.Subscription{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, customer_id, provider, provider_subscription_id, plan_id, status, amount, currency, interval, interval_count, current_period_start, current_period_end, cancel_at, cancelled_at, trial_start, trial_end, metadata, created_at, updated_at
		 FROM subscriptions WHERE id = $1`, id,
	).Scan(
		&s.ID, &s.MerchantID, &s.CustomerID, &s.Provider, &s.ProviderSubscriptionID, &s.PlanID,
		&s.Status, &s.Amount, &s.Currency, &s.Interval, &s.IntervalCount,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAt, &s.CancelledAt,
		&s.TrialStart, &s.TrialEnd, &s.Metadata,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query subscription by id: %w", err)
	}
	return s, nil
}

func (r *SubscriptionRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Subscription, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM subscriptions WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, customer_id, provider, provider_subscription_id, plan_id, status, amount, currency, interval, interval_count, current_period_start, current_period_end, cancel_at, cancelled_at, trial_start, trial_end, metadata, created_at, updated_at
		 FROM subscriptions WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*model.Subscription
	for rows.Next() {
		s := &model.Subscription{}
		if err := rows.Scan(
			&s.ID, &s.MerchantID, &s.CustomerID, &s.Provider, &s.ProviderSubscriptionID, &s.PlanID,
			&s.Status, &s.Amount, &s.Currency, &s.Interval, &s.IntervalCount,
			&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAt, &s.CancelledAt,
			&s.TrialStart, &s.TrialEnd, &s.Metadata,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, total, nil
}

func (r *SubscriptionRepo) Update(ctx context.Context, s *model.Subscription) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET provider=$2, provider_subscription_id=$3, plan_id=$4, status=$5, amount=$6, currency=$7, interval=$8, interval_count=$9, current_period_start=$10, current_period_end=$11, cancel_at=$12, cancelled_at=$13, trial_start=$14, trial_end=$15, metadata=$16, updated_at=NOW() WHERE id=$1`,
		s.ID, s.Provider, s.ProviderSubscriptionID, s.PlanID, s.Status, s.Amount, s.Currency,
		s.Interval, s.IntervalCount, s.CurrentPeriodStart, s.CurrentPeriodEnd,
		s.CancelAt, s.CancelledAt, s.TrialStart, s.TrialEnd, s.Metadata,
	)
	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *SubscriptionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET status = $2, updated_at = NOW() WHERE id = $1`, id, status,
	)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *SubscriptionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET status = $2, updated_at = NOW() WHERE id = $1`, id, model.SubscriptionStatusCancelled,
	)
	if err != nil {
		return fmt.Errorf("soft delete subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
