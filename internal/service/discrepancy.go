package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/eventbus"
)

type DiscrepancyService struct {
	reconSvc *ReconciliationService
	eventBus eventbus.EventBus
	logger   *zap.Logger
}

func NewDiscrepancyService(reconSvc *ReconciliationService, eventBus eventbus.EventBus, logger *zap.Logger) *DiscrepancyService {
	return &DiscrepancyService{reconSvc: reconSvc, eventBus: eventBus, logger: logger}
}

type Discrepancy struct {
	MerchantID uuid.UUID `json:"merchant_id"`
	Provider   string    `json:"provider"`
	Amount     int64     `json:"amount"`
	Severity   string    `json:"severity"`
	DetectedAt time.Time `json:"detected_at"`
}

func (s *DiscrepancyService) CheckDiscrepancies(ctx context.Context, merchantID uuid.UUID, provider string, from, to time.Time) (*Discrepancy, error) {
	result, err := s.reconSvc.Reconcile(ctx, merchantID, provider, from, to)
	if err != nil {
		return nil, err
	}

	if result.Discrepancy == 0 {
		return nil, nil
	}

	severity := "low"
	if result.Discrepancy > 10000 {
		severity = "high"
	} else if result.Discrepancy > 1000 {
		severity = "medium"
	}

	discrepancy := &Discrepancy{
		MerchantID: merchantID,
		Provider:   provider,
		Amount:     result.Discrepancy,
		Severity:   severity,
		DetectedAt: time.Now().UTC(),
	}

	s.eventBus.Publish(ctx, "events.reconciliation.discrepancy", &eventbus.Event{
		Type:   "reconciliation.discrepancy",
		Source: "helix-seller",
		Data:   discrepancy,
	})

	return discrepancy, nil
}
