package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ExchangeRateService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
	client *http.Client
}

func NewExchangeRateService(db *pgxpool.Pool, logger *zap.Logger) *ExchangeRateService {
	return &ExchangeRateService{
		db:     db,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type frankfurterResponse struct {
	Rates map[string]float64 `json:"rates"`
}

func (s *ExchangeRateService) GetRate(ctx context.Context, from, to string) (float64, error) {
	var rate float64
	err := s.db.QueryRow(ctx,
		`SELECT rate FROM exchange_rates WHERE base_currency = $1 AND target_currency = $2 AND fetched_at > NOW() - INTERVAL '1 hour'`,
		from, to,
	).Scan(&rate)
	if err == nil {
		return rate, nil
	}

	url := fmt.Sprintf("https://api.frankfurter.app/latest?from=%s&to=%s", from, to)
	resp, err := s.client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		var fr frankfurterResponse
		if json.NewDecoder(resp.Body).Decode(&fr) == nil {
			if r, ok := fr.Rates[to]; ok {
				s.db.Exec(ctx,
					`INSERT INTO exchange_rates (id, base_currency, target_currency, rate, source, fetched_at, created_at)
					 VALUES (gen_random_uuid(), $1, $2, $3, 'frankfurter', NOW(), NOW())`,
					from, to, r,
				)
				return r, nil
			}
		}
	}

	err = s.db.QueryRow(ctx,
		`SELECT rate FROM exchange_rates WHERE base_currency = $1 AND target_currency = $2 ORDER BY fetched_at DESC LIMIT 1`,
		from, to,
	).Scan(&rate)
	if err != nil {
		return 0, fmt.Errorf("no exchange rate available for %s/%s", from, to)
	}
	return rate, nil
}

func (s *ExchangeRateService) Convert(ctx context.Context, amount int64, from, to string) (int64, float64, error) {
	if from == to {
		return amount, 1.0, nil
	}
	rate, err := s.GetRate(ctx, from, to)
	if err != nil {
		return 0, 0, err
	}
	converted := float64(amount) * rate
	return int64(converted), rate, nil
}
