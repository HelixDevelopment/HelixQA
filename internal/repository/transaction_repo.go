package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type TransactionRepo struct {
	db *pgxpool.Pool
}

func NewTransactionRepo(db *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) Create(ctx context.Context, t *model.Transaction) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO transactions (id, merchant_id, customer_id, provider, provider_transaction_id, type, amount, currency, status, payment_method_id, idempotency_key, description, metadata, error_code, error_message, fee_amount, net_amount, processed_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW())`,
		t.ID, t.MerchantID, t.CustomerID, t.Provider, t.ProviderTransactionID, t.Type, t.Amount, t.Currency, t.Status, t.PaymentMethodID,
		t.IdempotencyKey, t.Description, t.Metadata, t.ErrorCode, t.ErrorMessage, t.FeeAmount, t.NetAmount, t.ProcessedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

func (r *TransactionRepo) DB() *pgxpool.Pool {
	return r.db
}

func (r *TransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	t := &model.Transaction{}
	err := r.db.QueryRow(ctx,
		 `SELECT id, merchant_id, customer_id, provider, provider_transaction_id, type, amount, currency, status, payment_method_id, idempotency_key, description, metadata, error_code, error_message, fee_amount, net_amount, processed_at, created_at, updated_at
		 FROM transactions WHERE id = $1
		 ORDER BY created_at DESC LIMIT 1`, id,
	).Scan(
		&t.ID, &t.MerchantID, &t.CustomerID, &t.Provider, &t.ProviderTransactionID, &t.Type, &t.Amount, &t.Currency, &t.Status, &t.PaymentMethodID,
		&t.IdempotencyKey, &t.Description, &t.Metadata, &t.ErrorCode, &t.ErrorMessage, &t.FeeAmount, &t.NetAmount, &t.ProcessedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query transaction by id: %w", err)
	}
	return t, nil
}

func (r *TransactionRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Transaction, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, customer_id, provider, provider_transaction_id, type, amount, currency, status, payment_method_id, idempotency_key, description, metadata, error_code, error_message, fee_amount, net_amount, processed_at, created_at, updated_at
		 FROM transactions WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		if err := rows.Scan(
			&t.ID, &t.MerchantID, &t.CustomerID, &t.Provider, &t.ProviderTransactionID, &t.Type, &t.Amount, &t.Currency, &t.Status, &t.PaymentMethodID,
			&t.IdempotencyKey, &t.Description, &t.Metadata, &t.ErrorCode, &t.ErrorMessage, &t.FeeAmount, &t.NetAmount, &t.ProcessedAt,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		txns = append(txns, t)
	}

	return txns, total, nil
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE transactions SET status = $2, updated_at = NOW() WHERE id = $1`, id, status,
	)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
