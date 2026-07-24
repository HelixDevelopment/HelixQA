package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/eventbus"
	"github.com/helix-seller/helix-seller/internal/service"
)

type WebhookIngressHandler struct {
	webhookSvc *service.WebhookService
	eventBus   eventbus.EventBus
	logger     *zap.Logger
}

func NewWebhookIngressHandler(webhookSvc *service.WebhookService, eventBus eventbus.EventBus, logger *zap.Logger) *WebhookIngressHandler {
	return &WebhookIngressHandler{webhookSvc: webhookSvc, eventBus: eventBus, logger: logger}
}

func (h *WebhookIngressHandler) HandleStripe(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	sig := c.GetHeader("Stripe-Signature")
	_ = sig // TODO: verify signature

	h.eventBus.Publish(c.Request.Context(), "events.provider.stripe", &eventbus.Event{
		Type:   "provider.webhook.received",
		Source: "stripe",
		Data:   string(body),
	})
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *WebhookIngressHandler) HandlePayPal(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	h.eventBus.Publish(c.Request.Context(), "events.provider.paypal", &eventbus.Event{
		Type:   "provider.webhook.received",
		Source: "paypal",
		Data:   string(body),
	})
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *WebhookIngressHandler) HandleSquare(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	h.eventBus.Publish(c.Request.Context(), "events.provider.square", &eventbus.Event{
		Type:   "provider.webhook.received",
		Source: "square",
		Data:   string(body),
	})
	c.JSON(http.StatusOK, gin.H{"received": true})
}
