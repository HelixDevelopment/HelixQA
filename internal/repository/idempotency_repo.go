package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type IdempotencyRepo struct {
	db *pgxpool.Pool
}

func NewIdempotencyRepo(db *pgxpool.Pool) *IdempotencyRepo {
	return &IdempotencyRepo{db: db}
}

func (r *IdempotencyRepo) CheckAndSave(ctx context.Context, key, merchantID string) (*model.Transaction, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("idempotency: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	existing := &model.Transaction{}
	err = tx.QueryRow(ctx,
		`SELECT id, merchant_id, customer_id, provider, provider_transaction_id, type, amount,
		        currency, status, payment_method_id, idempotency_key, description, metadata,
		        error_code, error_message, fee_amount, net_amount, processed_at, created_at, updated_at
		 FROM transactions
		 WHERE idempotency_key = $1 AND merchant_id = $2
		 LIMIT 1`,
		key, merchantID,
	).Scan(
		&existing.ID, &existing.MerchantID, &existing.CustomerID, &existing.Provider,
		&existing.ProviderTransactionID, &existing.Type, &existing.Amount, &existing.Currency,
		&existing.Status, &existing.PaymentMethodID, &existing.IdempotencyKey, &existing.Description,
		&existing.Metadata, &existing.ErrorCode, &existing.ErrorMessage, &existing.FeeAmount,
		&existing.NetAmount, &existing.ProcessedAt, &existing.CreatedAt, &existing.UpdatedAt,
	)
	if err == nil {
		return existing, true, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return nil, false, fmt.Errorf("idempotency: query: %w", err)
	}

	return nil, false, tx.Commit(ctx)
}
