package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type PayoutRepo struct {
	db *pgxpool.Pool
}

func NewPayoutRepo(db *pgxpool.Pool) *PayoutRepo {
	return &PayoutRepo{db: db}
}

func (r *PayoutRepo) Create(ctx context.Context, p *model.Payout) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO payouts (id, merchant_id, provider, provider_payout_id, amount, currency, status, method, arrival_date, fee_amount, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())`,
		p.ID, p.MerchantID, p.Provider, p.ProviderPayoutID, p.Amount, p.Currency, p.Status, p.Method,
		p.ArrivalDate, p.FeeAmount, p.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert payout: %w", err)
	}
	return nil
}

func (r *PayoutRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Payout, error) {
	p := &model.Payout{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, provider, provider_payout_id, amount, currency, status, method, arrival_date, fee_amount, metadata, created_at, updated_at
		 FROM payouts WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.MerchantID, &p.Provider, &p.ProviderPayoutID, &p.Amount, &p.Currency, &p.Status, &p.Method,
		&p.ArrivalDate, &p.FeeAmount, &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query payout by id: %w", err)
	}
	return p, nil
}

func (r *PayoutRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Payout, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM payouts WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count payouts: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, provider, provider_payout_id, amount, currency, status, method, arrival_date, fee_amount, metadata, created_at, updated_at
		 FROM payouts WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list payouts: %w", err)
	}
	defer rows.Close()

	var payouts []*model.Payout
	for rows.Next() {
		p := &model.Payout{}
		if err := rows.Scan(
			&p.ID, &p.MerchantID, &p.Provider, &p.ProviderPayoutID, &p.Amount, &p.Currency, &p.Status, &p.Method,
			&p.ArrivalDate, &p.FeeAmount, &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan payout: %w", err)
		}
		payouts = append(payouts, p)
	}

	return payouts, total, nil
}

func (r *PayoutRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PayoutStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE payouts SET status = $2, updated_at = NOW() WHERE id = $1`, id, status,
	)
	if err != nil {
		return fmt.Errorf("update payout status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
