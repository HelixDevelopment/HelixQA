package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type BillingService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewBillingService(db *pgxpool.Pool, logger *zap.Logger) *BillingService {
	return &BillingService{db: db, logger: logger}
}

type FeeStructure struct {
	PercentageFee float64 `json:"percentage_fee"`
	FixedFee      int64   `json:"fixed_fee"`
}

type BillingPeriod struct {
	MerchantID        uuid.UUID `json:"merchant_id"`
	PeriodStart       time.Time `json:"period_start"`
	PeriodEnd         time.Time `json:"period_end"`
	TotalTransactions int64     `json:"total_transactions"`
	TotalAmount       int64     `json:"total_amount"`
	TotalFees         int64     `json:"total_fees"`
	Currency          string    `json:"currency"`
}

func (s *BillingService) CalculateFees(ctx context.Context, merchantID uuid.UUID, from, to time.Time) (*BillingPeriod, error) {
	period := &BillingPeriod{
		MerchantID:  merchantID,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	fees := &FeeStructure{
		PercentageFee: 0.01,
		FixedFee:      10,
	}

	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0), COUNT(*), COALESCE(currency, 'USD')
		 FROM transactions
		 WHERE merchant_id = $1 AND status = 'succeeded'
		 AND created_at BETWEEN $2 AND $3`,
		merchantID, from, to,
	).Scan(&period.TotalAmount, &period.TotalTransactions, &period.Currency)
	if err != nil {
		return nil, err
	}

	percentageFee := int64(float64(period.TotalAmount) * fees.PercentageFee)
	fixedFee := fees.FixedFee * period.TotalTransactions
	period.TotalFees = percentageFee + fixedFee

	return period, nil
}

func (s *BillingService) ListBillingInvoices(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]map[string]interface{}, int64, error) {
	offset := (page - 1) * pageSize

	var total int64
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoices WHERE merchant_id = $1`,
		merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, amount, currency, status, created_at
		 FROM invoices WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var invoices []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var amount int64
		var currency, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &amount, &currency, &status, &createdAt); err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, map[string]interface{}{
			"id":         id,
			"amount":     amount,
			"currency":   currency,
			"status":     status,
			"created_at": createdAt,
		})
	}

	return invoices, total, nil
}
