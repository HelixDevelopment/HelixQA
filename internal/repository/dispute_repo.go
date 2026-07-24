package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type DisputeRepo struct {
	db *pgxpool.Pool
}

func NewDisputeRepo(db *pgxpool.Pool) *DisputeRepo {
	return &DisputeRepo{db: db}
}

func (r *DisputeRepo) Create(ctx context.Context, d *model.Dispute) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO disputes (id, transaction_id, merchant_id, provider, provider_dispute_id, reason, status, amount, evidence_deadline, evidence_submitted_at, resolution, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())`,
		d.ID, d.TransactionID, d.MerchantID, d.Provider, d.ProviderDisputeID, d.Reason, d.Status, d.Amount,
		d.EvidenceDeadline, d.EvidenceSubmittedAt, d.Resolution, d.Metadata,
	)
	if err != nil {
		return fmt.Errorf("insert dispute: %w", err)
	}
	return nil
}

func (r *DisputeRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Dispute, error) {
	d := &model.Dispute{}
	err := r.db.QueryRow(ctx,
		`SELECT id, transaction_id, merchant_id, provider, provider_dispute_id, reason, status, amount, evidence_deadline, evidence_submitted_at, resolution, metadata, created_at, updated_at
		 FROM disputes WHERE id = $1`, id,
	).Scan(
		&d.ID, &d.TransactionID, &d.MerchantID, &d.Provider, &d.ProviderDisputeID, &d.Reason, &d.Status, &d.Amount,
		&d.EvidenceDeadline, &d.EvidenceSubmittedAt, &d.Resolution, &d.Metadata, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query dispute by id: %w", err)
	}
	return d, nil
}

func (r *DisputeRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Dispute, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM disputes WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count disputes: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, merchant_id, provider, provider_dispute_id, reason, status, amount, evidence_deadline, evidence_submitted_at, resolution, metadata, created_at, updated_at
		 FROM disputes WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list disputes: %w", err)
	}
	defer rows.Close()

	var disputes []*model.Dispute
	for rows.Next() {
		d := &model.Dispute{}
		if err := rows.Scan(
			&d.ID, &d.TransactionID, &d.MerchantID, &d.Provider, &d.ProviderDisputeID, &d.Reason, &d.Status, &d.Amount,
			&d.EvidenceDeadline, &d.EvidenceSubmittedAt, &d.Resolution, &d.Metadata, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan dispute: %w", err)
		}
		disputes = append(disputes, d)
	}

	return disputes, total, nil
}

func (r *DisputeRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.DisputeStatus) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE disputes SET status = $2, updated_at = NOW() WHERE id = $1`, id, status,
	)
	if err != nil {
		return fmt.Errorf("update dispute status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
