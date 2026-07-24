package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type InvoiceRepo struct {
	db *pgxpool.Pool
}

func NewInvoiceRepo(db *pgxpool.Pool) *InvoiceRepo {
	return &InvoiceRepo{db: db}
}

func (r *InvoiceRepo) Create(ctx context.Context, inv *model.Invoice) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO invoices (id, merchant_id, customer_id, subscription_id, provider, provider_invoice_id, amount, currency, status, due_date, paid_at, period_start, period_end, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())`,
		inv.ID, inv.MerchantID, inv.CustomerID, inv.SubscriptionID, inv.Provider, inv.ProviderInvoiceID,
		inv.Amount, inv.Currency, inv.Status, inv.DueDate, inv.PaidAt,
		inv.PeriodStart, inv.PeriodEnd, inv.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}
	return nil
}

func (r *InvoiceRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	inv := &model.Invoice{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, customer_id, subscription_id, provider, provider_invoice_id, amount, currency, status, due_date, paid_at, period_start, period_end, metadata, created_at, updated_at
		 FROM invoices WHERE id = $1`, id,
	).Scan(
		&inv.ID, &inv.MerchantID, &inv.CustomerID, &inv.SubscriptionID, &inv.Provider, &inv.ProviderInvoiceID,
		&inv.Amount, &inv.Currency, &inv.Status, &inv.DueDate, &inv.PaidAt,
		&inv.PeriodStart, &inv.PeriodEnd, &inv.Metadata,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query invoice by id: %w", err)
	}
	return inv, nil
}

func (r *InvoiceRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Invoice, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoices WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, customer_id, subscription_id, provider, provider_invoice_id, amount, currency, status, due_date, paid_at, period_start, period_end, metadata, created_at, updated_at
		 FROM invoices WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []*model.Invoice
	for rows.Next() {
		inv := &model.Invoice{}
		if err := rows.Scan(
			&inv.ID, &inv.MerchantID, &inv.CustomerID, &inv.SubscriptionID, &inv.Provider, &inv.ProviderInvoiceID,
			&inv.Amount, &inv.Currency, &inv.Status, &inv.DueDate, &inv.PaidAt,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.Metadata,
			&inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, nil
}

func (r *InvoiceRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.InvoiceStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE invoices SET status = $2, updated_at = NOW() WHERE id = $1`, id, status,
	)
	if err != nil {
		return fmt.Errorf("update invoice status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
