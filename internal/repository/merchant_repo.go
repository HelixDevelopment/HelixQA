package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type MerchantRepo struct {
	db *pgxpool.Pool
}

func NewMerchantRepo(db *pgxpool.Pool) *MerchantRepo {
	return &MerchantRepo{db: db}
}

func (r *MerchantRepo) Create(ctx context.Context, m *model.Merchant) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO merchants (id, name, legal_name, trade_name, email, phone, country, currency, slug, status, kyc_status, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())`,
		m.ID, m.Name, m.LegalName, m.TradeName, m.Email, m.Phone, m.Country, m.Currency, m.Slug, m.Status, m.KycStatus, m.Settings,
	)
	if err != nil {
		return fmt.Errorf("create merchant: %w", err)
	}
	return nil
}

func (r *MerchantRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Merchant, error) {
	m := &model.Merchant{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, legal_name, trade_name, email, phone, country, currency, slug, status, kyc_status, settings, created_at, updated_at
		 FROM merchants WHERE id = $1`, id,
	).Scan(
		&m.ID, &m.Name, &m.LegalName, &m.TradeName, &m.Email, &m.Phone, &m.Country,
		&m.Currency, &m.Slug, &m.Status, &m.KycStatus, &m.Settings, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query merchant by id: %w", err)
	}
	return m, nil
}

func (r *MerchantRepo) List(ctx context.Context, page, pageSize int) ([]*model.Merchant, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM merchants`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count merchants: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, name, legal_name, trade_name, email, phone, country, currency, slug, status, kyc_status, settings, created_at, updated_at
		 FROM merchants ORDER BY created_at DESC LIMIT $1 OFFSET $2`, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list merchants: %w", err)
	}
	defer rows.Close()

	var merchants []*model.Merchant
	for rows.Next() {
		m := &model.Merchant{}
		if err := rows.Scan(
			&m.ID, &m.Name, &m.LegalName, &m.TradeName, &m.Email, &m.Phone, &m.Country,
			&m.Currency, &m.Slug, &m.Status, &m.KycStatus, &m.Settings, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan merchant: %w", err)
		}
		merchants = append(merchants, m)
	}
	return merchants, total, nil
}

func (r *MerchantRepo) Update(ctx context.Context, m *model.Merchant) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE merchants SET name=$2, legal_name=$3, trade_name=$4, email=$5, phone=$6, country=$7, currency=$8, slug=$9, status=$10, kyc_status=$11, settings=$12, updated_at=NOW()
		 WHERE id=$1`,
		m.ID, m.Name, m.LegalName, m.TradeName, m.Email, m.Phone, m.Country, m.Currency, m.Slug, m.Status, m.KycStatus, m.Settings,
	)
	if err != nil {
		return fmt.Errorf("update merchant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
