package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ReconciliationService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewReconciliationService(db *pgxpool.Pool, logger *zap.Logger) *ReconciliationService {
	return &ReconciliationService{db: db, logger: logger}
}

type ReconciliationResult struct {
	PlatformTotal    int64 `json:"platform_total"`
	ProviderTotal    int64 `json:"provider_total"`
	Discrepancy      int64 `json:"discrepancy"`
	TransactionCount int64 `json:"transaction_count"`
}

func (s *ReconciliationService) Reconcile(ctx context.Context, merchantID uuid.UUID, provider string, from, to time.Time) (*ReconciliationResult, error) {
	result := &ReconciliationResult{}

	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0), COUNT(*)
		 FROM transactions
		 WHERE merchant_id = $1 AND provider = $2 AND status = 'succeeded'
		 AND created_at BETWEEN $3 AND $4`,
		merchantID, provider, from, to,
	).Scan(&result.PlatformTotal, &result.TransactionCount)
	if err != nil {
		return nil, err
	}

	result.ProviderTotal = 0
	result.Discrepancy = result.PlatformTotal - result.ProviderTotal

	s.logger.Warn("reconciliation requires provider data to compute ProviderTotal accurately",
		zap.String("merchant_id", merchantID.String()),
		zap.String("provider", provider),
		zap.Int64("platform_total", result.PlatformTotal),
		zap.String("status", "pending"),
	)

	return result, nil
}
