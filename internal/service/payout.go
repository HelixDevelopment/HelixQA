package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/eventbus"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type PayoutService struct {
	payoutRepo *repository.PayoutRepo
	eventBus   eventbus.EventBus
	logger     *zap.Logger
}

func NewPayoutService(payoutRepo *repository.PayoutRepo, eventBus eventbus.EventBus, logger *zap.Logger) *PayoutService {
	return &PayoutService{payoutRepo: payoutRepo, eventBus: eventBus, logger: logger}
}

func (s *PayoutService) CreatePayout(ctx context.Context, merchantID uuid.UUID, provider, currency string, amount int64, method model.PayoutMethod) (*model.Payout, error) {
	p := &model.Payout{
		ID:         uuid.New(),
		MerchantID: merchantID,
		Provider:   provider,
		Amount:     amount,
		Currency:   currency,
		Status:     model.PayoutStatusPending,
		Method:     method,
	}
	if err := s.payoutRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.payout.created", &eventbus.Event{
		Type:   "payout.created",
		Source: "helix-seller",
		Data:   p,
	}); err != nil {
		s.logger.Error("failed to publish payout.created event", zap.Error(err))
	}

	return p, nil
}

func (s *PayoutService) GetPayout(ctx context.Context, id uuid.UUID) (*model.Payout, error) {
	return s.payoutRepo.GetByID(ctx, id)
}

func (s *PayoutService) ListPayouts(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Payout, int64, error) {
	return s.payoutRepo.ListByMerchant(ctx, merchantID, page, pageSize)
}
