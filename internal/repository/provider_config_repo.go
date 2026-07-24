package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type ProviderConfigRepo struct {
	db *pgxpool.Pool
}

func NewProviderConfigRepo(db *pgxpool.Pool) *ProviderConfigRepo {
	return &ProviderConfigRepo{db: db}
}

func (r *ProviderConfigRepo) Create(ctx context.Context, pc *model.ProviderConfig) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO provider_configs (id, merchant_id, provider, is_active, config, fallback_order, health_status, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		pc.ID, pc.MerchantID, pc.Provider, pc.IsActive, pc.Config, pc.FallbackOrder, pc.HealthStatus, pc.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert provider config: %w", err)
	}
	return nil
}

func (r *ProviderConfigRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.ProviderConfig, error) {
	pc := &model.ProviderConfig{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, provider, is_active, config, fallback_order, health_status, last_health_check, metadata, created_at, updated_at
		 FROM provider_configs WHERE id = $1`, id,
	).Scan(
		&pc.ID, &pc.MerchantID, &pc.Provider, &pc.IsActive, &pc.Config, &pc.FallbackOrder,
		&pc.HealthStatus, &pc.LastHealthCheck, &pc.Metadata, &pc.CreatedAt, &pc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query provider config by id: %w", err)
	}
	return pc, nil
}

func (r *ProviderConfigRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]*model.ProviderConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, provider, is_active, config, fallback_order, health_status, last_health_check, metadata, created_at, updated_at
		 FROM provider_configs WHERE merchant_id = $1
		 ORDER BY fallback_order ASC, created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider configs: %w", err)
	}
	defer rows.Close()

	var configs []*model.ProviderConfig
	for rows.Next() {
		pc := &model.ProviderConfig{}
		if err := rows.Scan(
			&pc.ID, &pc.MerchantID, &pc.Provider, &pc.IsActive, &pc.Config, &pc.FallbackOrder,
			&pc.HealthStatus, &pc.LastHealthCheck, &pc.Metadata, &pc.CreatedAt, &pc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provider config: %w", err)
		}
		configs = append(configs, pc)
	}
	return configs, nil
}

func (r *ProviderConfigRepo) Update(ctx context.Context, pc *model.ProviderConfig) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE provider_configs SET is_active=$2, config=$3, fallback_order=$4, health_status=$5, last_health_check=$6, metadata=$7, updated_at=NOW()
		 WHERE id=$1`,
		pc.ID, pc.IsActive, pc.Config, pc.FallbackOrder, pc.HealthStatus, pc.LastHealthCheck, pc.Metadata,
	)
	if err != nil {
		return fmt.Errorf("update provider config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *ProviderConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM provider_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete provider config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
