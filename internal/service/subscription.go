package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/eventbus"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type SubscriptionService struct {
	subRepo  *repository.SubscriptionRepo
	eventBus eventbus.EventBus
	logger   *zap.Logger
}

func NewSubscriptionService(subRepo *repository.SubscriptionRepo, eventBus eventbus.EventBus, logger *zap.Logger) *SubscriptionService {
	return &SubscriptionService{subRepo: subRepo, eventBus: eventBus, logger: logger}
}

func (s *SubscriptionService) CreateSubscription(ctx context.Context, merchantID, customerID uuid.UUID, amount int64, currency string, interval model.SubscriptionInterval, intervalCount int16, planID, provider string) (*model.Subscription, error) {
	now := time.Now()
	sub := &model.Subscription{
		ID:             uuid.New(),
		MerchantID:     merchantID,
		CustomerID:     customerID,
		PlanID:         planID,
		Status:         model.SubscriptionStatusActive,
		Amount:         amount,
		Currency:       currency,
		Interval:       interval,
		IntervalCount:  intervalCount,
		Provider:       provider,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   calculatePeriodEnd(now, interval, intervalCount),
	}

	if err := s.subRepo.Create(ctx, sub); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.subscription.created", &eventbus.Event{
		Type:   "subscription.created",
		Source: "helix-seller",
		Data:   sub,
	}); err != nil {
		s.logger.Error("failed to publish subscription.created event", zap.Error(err))
	}

	return sub, nil
}

func (s *SubscriptionService) GetSubscription(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

func (s *SubscriptionService) ListSubscriptions(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Subscription, int64, error) {
	return s.subRepo.ListByMerchant(ctx, merchantID, page, pageSize)
}

func (s *SubscriptionService) UpdateSubscription(ctx context.Context, id uuid.UUID, amount *int64, currency *string, interval *model.SubscriptionInterval, intervalCount *int16) (*model.Subscription, error) {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if amount != nil {
		sub.Amount = *amount
	}
	if currency != nil {
		sub.Currency = *currency
	}
	if interval != nil {
		sub.Interval = *interval
	}
	if intervalCount != nil {
		sub.IntervalCount = *intervalCount
	}

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.subscription.updated", &eventbus.Event{
		Type:   "subscription.updated",
		Source: "helix-seller",
		Data:   sub,
	}); err != nil {
		s.logger.Error("failed to publish subscription.updated event", zap.Error(err))
	}

	return sub, nil
}

func (s *SubscriptionService) CancelSubscription(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	sub.Status = model.SubscriptionStatusCancelled
	sub.CancelledAt = &now

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.subscription.cancelled", &eventbus.Event{
		Type:   "subscription.cancelled",
		Source: "helix-seller",
		Data:   sub,
	}); err != nil {
		s.logger.Error("failed to publish subscription.cancelled event", zap.Error(err))
	}

	return sub, nil
}

func (s *SubscriptionService) ProcessRenewal(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if sub.Status != model.SubscriptionStatusActive {
		return nil, model.NewValidationError("subscription is not active")
	}

	sub.CurrentPeriodStart = sub.CurrentPeriodEnd
	sub.CurrentPeriodEnd = calculatePeriodEnd(sub.CurrentPeriodEnd, sub.Interval, sub.IntervalCount)

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.subscription.renewed", &eventbus.Event{
		Type:   "subscription.renewed",
		Source: "helix-seller",
		Data:   sub,
	}); err != nil {
		s.logger.Error("failed to publish subscription.renewed event", zap.Error(err))
	}

	return sub, nil
}

func calculatePeriodEnd(start time.Time, interval model.SubscriptionInterval, count int16) time.Time {
	switch interval {
	case model.SubscriptionIntervalDay:
		return start.AddDate(0, 0, int(count))
	case model.SubscriptionIntervalWeek:
		return start.AddDate(0, 0, 7*int(count))
	case model.SubscriptionIntervalMonth:
		return start.AddDate(0, int(count), 0)
	case model.SubscriptionIntervalYear:
		return start.AddDate(int(count), 0, 0)
	default:
		return start.AddDate(0, int(count), 0)
	}
}
