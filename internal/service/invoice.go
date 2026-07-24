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

type InvoiceService struct {
	invRepo  *repository.InvoiceRepo
	eventBus eventbus.EventBus
	logger   *zap.Logger
}

func NewInvoiceService(invRepo *repository.InvoiceRepo, eventBus eventbus.EventBus, logger *zap.Logger) *InvoiceService {
	return &InvoiceService{invRepo: invRepo, eventBus: eventBus, logger: logger}
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, merchantID, customerID uuid.UUID, subscriptionID *uuid.UUID, amount int64, currency, provider string, dueDate *time.Time, periodStart, periodEnd time.Time) (*model.Invoice, error) {
	inv := &model.Invoice{
		ID:             uuid.New(),
		MerchantID:     merchantID,
		CustomerID:     customerID,
		SubscriptionID: subscriptionID,
		Provider:       provider,
		Amount:         amount,
		Currency:       currency,
		Status:         model.InvoiceStatusDraft,
		DueDate:        dueDate,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}

	if err := s.invRepo.Create(ctx, inv); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.invoice.created", &eventbus.Event{
		Type:   "invoice.created",
		Source: "helix-seller",
		Data:   inv,
	}); err != nil {
		s.logger.Error("failed to publish invoice.created event", zap.Error(err))
	}

	return inv, nil
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	return s.invRepo.GetByID(ctx, id)
}

func (s *InvoiceService) ListInvoices(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Invoice, int64, error) {
	return s.invRepo.ListByMerchant(ctx, merchantID, page, pageSize)
}

func (s *InvoiceService) SendInvoice(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	inv, err := s.invRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inv.Status != model.InvoiceStatusDraft {
		return nil, model.NewValidationError("only draft invoices can be sent")
	}

	if err := s.invRepo.UpdateStatus(ctx, id, model.InvoiceStatusOpen); err != nil {
		return nil, err
	}

	inv.Status = model.InvoiceStatusOpen

	if err := s.eventBus.Publish(ctx, "events.invoice.sent", &eventbus.Event{
		Type:   "invoice.sent",
		Source: "helix-seller",
		Data:   inv,
	}); err != nil {
		s.logger.Error("failed to publish invoice.sent event", zap.Error(err))
	}

	return inv, nil
}

func (s *InvoiceService) MarkPaid(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	inv, err := s.invRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inv.Status != model.InvoiceStatusOpen {
		return nil, model.NewValidationError("only open invoices can be marked as paid")
	}

	if err := s.invRepo.UpdateStatus(ctx, id, model.InvoiceStatusPaid); err != nil {
		return nil, err
	}

	now := time.Now()
	inv.Status = model.InvoiceStatusPaid
	inv.PaidAt = &now

	if err := s.eventBus.Publish(ctx, "events.invoice.paid", &eventbus.Event{
		Type:   "invoice.paid",
		Source: "helix-seller",
		Data:   inv,
	}); err != nil {
		s.logger.Error("failed to publish invoice.paid event", zap.Error(err))
	}

	return inv, nil
}

func (s *InvoiceService) MarkOverdue(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	inv, err := s.invRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inv.Status != model.InvoiceStatusOpen {
		return nil, model.NewValidationError("only open invoices can be marked as uncollectible")
	}

	if err := s.invRepo.UpdateStatus(ctx, id, model.InvoiceStatusUncollectible); err != nil {
		return nil, err
	}

	inv.Status = model.InvoiceStatusUncollectible

	if err := s.eventBus.Publish(ctx, "events.invoice.overdue", &eventbus.Event{
		Type:   "invoice.overdue",
		Source: "helix-seller",
		Data:   inv,
	}); err != nil {
		s.logger.Error("failed to publish invoice.overdue event", zap.Error(err))
	}

	return inv, nil
}
