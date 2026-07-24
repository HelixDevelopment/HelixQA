package service

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

type payoutTaskHandler struct {
	payoutSvc *PayoutService
	logger    *zap.Logger
}

func NewPayoutTaskHandler(payoutSvc *PayoutService, logger *zap.Logger) TaskHandler {
	return &payoutTaskHandler{payoutSvc: payoutSvc, logger: logger}
}

func (h *payoutTaskHandler) Type() string { return "payout" }

func (h *payoutTaskHandler) HandleTask(ctx context.Context, payload []byte) error {
	if h.payoutSvc == nil {
		return fmt.Errorf("payout service not initialized")
	}
	var req struct {
		PayoutID string `json:"payout_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("payout task: unmarshal payload: %w", err)
	}
	h.logger.Info("processing payout task", zap.String("payout_id", req.PayoutID))
	return nil
}

type reconciliationTaskHandler struct {
	logger *zap.Logger
}

func NewReconciliationTaskHandler(_ interface{}, logger *zap.Logger) TaskHandler {
	return &reconciliationTaskHandler{logger: logger}
}

func (h *reconciliationTaskHandler) Type() string { return "reconciliation" }

func (h *reconciliationTaskHandler) HandleTask(ctx context.Context, payload []byte) error {
	h.logger.Info("processing reconciliation task")
	return nil
}

type invoiceTaskHandler struct {
	invoiceSvc *InvoiceService
	logger     *zap.Logger
}

func NewInvoiceTaskHandler(invoiceSvc *InvoiceService, logger *zap.Logger) TaskHandler {
	return &invoiceTaskHandler{invoiceSvc: invoiceSvc, logger: logger}
}

func (h *invoiceTaskHandler) Type() string { return "invoice" }

func (h *invoiceTaskHandler) HandleTask(ctx context.Context, payload []byte) error {
	if h.invoiceSvc == nil {
		return fmt.Errorf("invoice service not initialized")
	}
	var req struct {
		InvoiceID string `json:"invoice_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("invoice task: unmarshal payload: %w", err)
	}
	h.logger.Info("processing invoice task", zap.String("invoice_id", req.InvoiceID))
	return nil
}

type webhookDeliveryTaskHandler struct {
	webhookSvc *WebhookService
	logger     *zap.Logger
}

func NewWebhookDeliveryTaskHandler(webhookSvc *WebhookService, logger *zap.Logger) TaskHandler {
	return &webhookDeliveryTaskHandler{webhookSvc: webhookSvc, logger: logger}
}

func (h *webhookDeliveryTaskHandler) Type() string { return "webhook_delivery" }

func (h *webhookDeliveryTaskHandler) HandleTask(ctx context.Context, payload []byte) error {
	if h.webhookSvc == nil {
		return fmt.Errorf("webhook service not initialized")
	}
	var req struct {
		WebhookID string `json:"webhook_id"`
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("webhook delivery task: unmarshal payload: %w", err)
	}
	h.logger.Info("processing webhook delivery task",
		zap.String("webhook_id", req.WebhookID),
		zap.String("event_type", req.EventType),
	)
	return nil
}
