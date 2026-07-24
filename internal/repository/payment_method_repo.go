package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type PaymentMethodRepo struct {
	db *pgxpool.Pool
}

func NewPaymentMethodRepo(db *pgxpool.Pool) *PaymentMethodRepo {
	return &PaymentMethodRepo{db: db}
}

func (r *PaymentMethodRepo) Create(ctx context.Context, pm *model.PaymentMethod) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO payment_methods (id, merchant_id, customer_id, type, provider, provider_token, fingerprint, brand, last4, exp_month, exp_year, is_default, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())`,
		pm.ID, pm.MerchantID, pm.CustomerID, pm.Type, pm.Provider, pm.ProviderToken,
		pm.Fingerprint, pm.Brand, pm.Last4, pm.ExpMonth, pm.ExpYear, pm.IsDefault, pm.Metadata,
	)
	return err
}

func (r *PaymentMethodRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.PaymentMethod, error) {
	pm := &model.PaymentMethod{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, customer_id, type, provider, provider_token, fingerprint, brand, last4, exp_month, exp_year, is_default, metadata, created_at, updated_at
		 FROM payment_methods WHERE id = $1`, id,
	).Scan(&pm.ID, &pm.MerchantID, &pm.CustomerID, &pm.Type, &pm.Provider, &pm.ProviderToken,
		&pm.Fingerprint, &pm.Brand, &pm.Last4, &pm.ExpMonth, &pm.ExpYear, &pm.IsDefault, &pm.Metadata, &pm.CreatedAt, &pm.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query payment method by id: %w", err)
	}
	return pm, nil
}

func (r *PaymentMethodRepo) ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]*model.PaymentMethod, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, customer_id, type, provider, provider_token, fingerprint, brand, last4, exp_month, exp_year, is_default, metadata, created_at, updated_at
		 FROM payment_methods WHERE customer_id = $1
		 ORDER BY is_default DESC, created_at DESC`,
		customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	defer rows.Close()

	var methods []*model.PaymentMethod
	for rows.Next() {
		pm := &model.PaymentMethod{}
		if err := rows.Scan(&pm.ID, &pm.MerchantID, &pm.CustomerID, &pm.Type, &pm.Provider, &pm.ProviderToken,
			&pm.Fingerprint, &pm.Brand, &pm.Last4, &pm.ExpMonth, &pm.ExpYear, &pm.IsDefault, &pm.Metadata, &pm.CreatedAt, &pm.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan payment method: %w", err)
		}
		methods = append(methods, pm)
	}
	return methods, nil
}

func (r *PaymentMethodRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM payment_methods WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
