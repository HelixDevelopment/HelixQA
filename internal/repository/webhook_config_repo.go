package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type WebhookConfigRepo struct {
	db *pgxpool.Pool
}

func NewWebhookConfigRepo(db *pgxpool.Pool) *WebhookConfigRepo {
	return &WebhookConfigRepo{db: db}
}

func (r *WebhookConfigRepo) Create(ctx context.Context, w *model.WebhookConfig) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO webhook_configs (id, merchant_id, url, secret, events, is_active, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
		w.ID, w.MerchantID, w.URL, w.Secret, w.Events, w.IsActive, w.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert webhook config: %w", err)
	}
	return nil
}

func (r *WebhookConfigRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.WebhookConfig, error) {
	w := &model.WebhookConfig{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, url, secret, events, is_active, metadata, created_at, updated_at
		 FROM webhook_configs WHERE id = $1`, id,
	).Scan(
		&w.ID, &w.MerchantID, &w.URL, &w.Secret, &w.Events, &w.IsActive,
		&w.Metadata, &w.CreatedAt, &w.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query webhook config by id: %w", err)
	}
	return w, nil
}

func (r *WebhookConfigRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]*model.WebhookConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, url, secret, events, is_active, metadata, created_at, updated_at
		 FROM webhook_configs WHERE merchant_id = $1
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhook configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.WebhookConfig
	for rows.Next() {
		w := &model.WebhookConfig{}
		if err := rows.Scan(
			&w.ID, &w.MerchantID, &w.URL, &w.Secret, &w.Events, &w.IsActive,
			&w.Metadata, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook config: %w", err)
		}
		configs = append(configs, w)
	}
	return configs, nil
}

func (r *WebhookConfigRepo) Update(ctx context.Context, w *model.WebhookConfig) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE webhook_configs SET url=$2, secret=$3, events=$4, is_active=$5, metadata=$6, updated_at=NOW()
		 WHERE id=$1`,
		w.ID, w.URL, w.Secret, w.Events, w.IsActive, w.Metadata,
	)
	if err != nil {
		return fmt.Errorf("update webhook config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *WebhookConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM webhook_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete webhook config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
