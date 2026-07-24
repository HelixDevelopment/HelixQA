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

type DisputeService struct {
	disputeRepo *repository.DisputeRepo
	eventBus    eventbus.EventBus
	logger      *zap.Logger
}

func NewDisputeService(disputeRepo *repository.DisputeRepo, eventBus eventbus.EventBus, logger *zap.Logger) *DisputeService {
	return &DisputeService{disputeRepo: disputeRepo, eventBus: eventBus, logger: logger}
}

func (s *DisputeService) CreateDispute(ctx context.Context, merchantID, transactionID uuid.UUID, provider, reason string, amount int64) (*model.Dispute, error) {
	d := &model.Dispute{
		ID:            uuid.New(),
		TransactionID: transactionID,
		MerchantID:    merchantID,
		Provider:      provider,
		Reason:        reason,
		Status:        model.DisputeStatusWarningNeedsResponse,
		Amount:        amount,
	}
	if err := s.disputeRepo.Create(ctx, d); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.dispute.created", &eventbus.Event{
		Type:   "dispute.created",
		Source: "helix-seller",
		Data:   d,
	}); err != nil {
		s.logger.Error("failed to publish dispute.created event", zap.Error(err))
	}

	return d, nil
}

func (s *DisputeService) GetDispute(ctx context.Context, id uuid.UUID) (*model.Dispute, error) {
	return s.disputeRepo.GetByID(ctx, id)
}

func (s *DisputeService) ListDisputes(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Dispute, int64, error) {
	return s.disputeRepo.ListByMerchant(ctx, merchantID, page, pageSize)
}

func (s *DisputeService) AddEvidence(ctx context.Context, disputeID uuid.UUID, evidenceURL string) (*model.Dispute, error) {
	d, err := s.disputeRepo.GetByID(ctx, disputeID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	d.EvidenceSubmittedAt = &now
	if err := s.disputeRepo.UpdateStatus(ctx, disputeID, model.DisputeStatusUnderReview); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.dispute.evidence_submitted", &eventbus.Event{
		Type:   "dispute.evidence_submitted",
		Source: "helix-seller",
		Data:   d,
	}); err != nil {
		s.logger.Error("failed to publish dispute.evidence_submitted event", zap.Error(err))
	}

	d.Status = model.DisputeStatusUnderReview
	return d, nil
}
