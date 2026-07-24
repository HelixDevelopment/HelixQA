package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type AuditLogRepo struct {
	db *pgxpool.Pool
}

func NewAuditLogRepo(db *pgxpool.Pool) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, log *model.AuditLog) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO audit_logs (id, merchant_id, actor_id, actor_type, action, resource_type, resource_id, changes, ip_address, user_agent, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())`,
		log.ID, log.MerchantID, log.ActorID, log.ActorType, log.Action, log.ResourceType, log.ResourceID, log.Changes, log.IPAddress, log.UserAgent,
	)
	return err
}

func (r *AuditLogRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.AuditLog, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE merchant_id = $1`, merchantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, actor_id, actor_type, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
		 FROM audit_logs WHERE merchant_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*model.AuditLog
	for rows.Next() {
		l := &model.AuditLog{}
		if err := rows.Scan(&l.ID, &l.MerchantID, &l.ActorID, &l.ActorType, &l.Action, &l.ResourceType, &l.ResourceID, &l.Changes, &l.IPAddress, &l.UserAgent, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}
