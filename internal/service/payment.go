package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/eventbus"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type PaymentService struct {
	txRepo       *repository.TransactionRepo
	pmRepo       *repository.PaymentMethodRepo
	eventBus     eventbus.EventBus
	logger       *zap.Logger
	idempotencyRepo *repository.IdempotencyRepo
}

func NewPaymentService(txRepo *repository.TransactionRepo, pmRepo *repository.PaymentMethodRepo, eventBus eventbus.EventBus, logger *zap.Logger) *PaymentService {
	var idempotencyRepo *repository.IdempotencyRepo
	if txRepo != nil {
		idempotencyRepo = repository.NewIdempotencyRepo(txRepo.DB())
	}
	return &PaymentService{
		txRepo:          txRepo,
		pmRepo:          pmRepo,
		eventBus:        eventBus,
		logger:          logger,
		idempotencyRepo: idempotencyRepo,
	}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, merchantID, customerID, paymentMethodID uuid.UUID, amount int64, currency, idempotencyKey string) (*model.Transaction, error) {
	if idempotencyKey != "" {
		existing, found, err := s.idempotencyRepo.CheckAndSave(ctx, idempotencyKey, merchantID.String())
		if err != nil {
			s.logger.Error("idempotency check failed", zap.Error(err))
			return nil, fmt.Errorf("idempotency check error: %w", err)
		}
		if found {
			s.logger.Info("duplicate request detected via idempotency key, returning existing transaction",
				zap.String("idempotency_key", idempotencyKey),
				zap.String("existing_tx_id", existing.ID.String()),
			)
			return existing, nil
		}
	}

	if _, err := s.pmRepo.GetByID(ctx, paymentMethodID); err != nil {
		return nil, err
	}

	tx := &model.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		CustomerID:      customerID,
		PaymentMethodID: paymentMethodID,
		Amount:          amount,
		Currency:        currency,
		Status:          model.TransactionStatusPending,
		Type:            model.TransactionTypeCharge,
		IdempotencyKey:  idempotencyKey,
	}
	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.payment.initiated", &eventbus.Event{
		Type:   "payment.initiated",
		Source: "helix-seller",
		Data:   tx,
	}); err != nil {
		s.logger.Error("failed to publish payment.initiated event", zap.Error(err))
	}

	return tx, nil
}

func (s *PaymentService) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	return s.txRepo.GetByID(ctx, id)
}

func (s *PaymentService) ListTransactions(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Transaction, int64, error) {
	return s.txRepo.ListByMerchant(ctx, merchantID, page, pageSize)
}

func (s *PaymentService) Refund(ctx context.Context, transactionID uuid.UUID, amount int64, reason string) (*model.Transaction, error) {
	orig, err := s.txRepo.GetByID(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	refund := &model.Transaction{
		ID:              uuid.New(),
		MerchantID:      orig.MerchantID,
		CustomerID:      orig.CustomerID,
		PaymentMethodID: orig.PaymentMethodID,
		Amount:          amount,
		Currency:        orig.Currency,
		Status:          model.TransactionStatusReversed,
		Type:            model.TransactionTypeRefund,
		Provider:        orig.Provider,
		Description:     reason,
	}
	if err := s.txRepo.Create(ctx, refund); err != nil {
		return nil, err
	}

	if err := s.eventBus.Publish(ctx, "events.payment.refunded", &eventbus.Event{
		Type:   "payment.refunded",
		Source: "helix-seller",
		Data:   refund,
	}); err != nil {
		s.logger.Error("failed to publish payment.refunded event", zap.Error(err))
	}

	return refund, nil
}
