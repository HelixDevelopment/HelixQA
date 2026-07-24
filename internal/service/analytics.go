package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnalyticsService struct {
	db *pgxpool.Pool
}

func NewAnalyticsService(db *pgxpool.Pool) *AnalyticsService {
	return &AnalyticsService{db: db}
}

type AnalyticsSummary struct {
	TotalRevenue           int64   `json:"total_revenue"`
	TotalTransactions      int64   `json:"total_transactions"`
	SuccessfulTransactions int64   `json:"successful_transactions"`
	FailedTransactions     int64   `json:"failed_transactions"`
	AverageTransactionSize float64 `json:"average_transaction_size"`
	RefundAmount           int64   `json:"refund_amount"`
	Period                 string  `json:"period"`
}

func (s *AnalyticsService) GetSummary(ctx context.Context, merchantID uuid.UUID, from, to time.Time) (*AnalyticsSummary, error) {
	summary := &AnalyticsSummary{Period: fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02"))}

	row := s.db.QueryRow(ctx,
		`SELECT 
			COALESCE(SUM(CASE WHEN status = 'completed' THEN amount ELSE 0 END), 0),
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(CASE WHEN status = 'completed' THEN amount END), 0),
			COALESCE(SUM(CASE WHEN status = 'refunded' THEN amount ELSE 0 END), 0)
		 FROM transactions
		 WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3`,
		merchantID, from, to,
	)
	err := row.Scan(&summary.TotalRevenue, &summary.TotalTransactions, &summary.SuccessfulTransactions,
		&summary.FailedTransactions, &summary.AverageTransactionSize, &summary.RefundAmount)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *AnalyticsService) GetTransactionAnalytics(ctx context.Context, merchantID uuid.UUID, from, to time.Time, groupBy string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`SELECT DATE_TRUNC('%s', created_at) as period, 
		COUNT(*) as count,
		COALESCE(SUM(CASE WHEN status = 'completed' THEN amount ELSE 0 END), 0) as revenue
		FROM transactions
		WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY period ORDER BY period`, groupBy)

	rows, err := s.db.Query(ctx, query, merchantID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var period time.Time
		var count int64
		var revenue int64
		if err := rows.Scan(&period, &count, &revenue); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"period":  period.Format(time.RFC3339),
			"count":   count,
			"revenue": revenue,
		})
	}
	return results, nil
}

func (s *AnalyticsService) ExportTransactions(ctx context.Context, merchantID uuid.UUID, from, to time.Time, w io.Writer) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, amount, currency, status, provider, created_at
		 FROM transactions WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3
		 ORDER BY created_at DESC`, merchantID, from, to,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Amount", "Currency", "Status", "Provider", "Created At"})

	for rows.Next() {
		var id uuid.UUID
		var amount int64
		var currency, status, provider string
		var createdAt time.Time
		if err := rows.Scan(&id, &amount, &currency, &status, &provider, &createdAt); err != nil {
			return err
		}
		writer.Write([]string{
			id.String(),
			fmt.Sprintf("%d", amount),
			currency,
			status,
			provider,
			createdAt.Format(time.RFC3339),
		})
	}
	writer.Flush()
	return writer.Error()
}
