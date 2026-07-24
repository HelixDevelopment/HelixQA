package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/service"
)

type WebhookHandler struct {
	webhookSvc *service.WebhookService
}

func NewWebhookHandler(webhookSvc *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookSvc: webhookSvc}
}

func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		URL      string   `json:"url" binding:"required"`
		Secret   string   `json:"secret"`
		Events   []string `json:"events" binding:"required"`
		IsActive bool     `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &model.WebhookConfig{
		MerchantID: merchantID,
		URL:        req.URL,
		Secret:     req.Secret,
		Events:     req.Events,
		IsActive:   req.IsActive,
	}

	if err := h.webhookSvc.CreateWebhook(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}
	c.JSON(http.StatusCreated, config)
}

func (h *WebhookHandler) GetWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("webhookId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	config, err := h.webhookSvc.GetWebhook(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

func (h *WebhookHandler) ListWebhooks(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	configs, err := h.webhookSvc.ListWebhooks(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": configs, "total": len(configs)})
}

func (h *WebhookHandler) UpdateWebhook(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	webhookID, err := uuid.Parse(c.Param("webhookId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	existing, err := h.webhookSvc.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	if existing.MerchantID != merchantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	var req struct {
		URL      string   `json:"url"`
		Secret   string   `json:"secret"`
		Events   []string `json:"events"`
		IsActive *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.URL != "" {
		existing.URL = req.URL
	}
	if req.Secret != "" {
		existing.Secret = req.Secret
	}
	if req.Events != nil {
		existing.Events = req.Events
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := h.webhookSvc.UpdateWebhook(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	webhookID, err := uuid.Parse(c.Param("webhookId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	existing, err := h.webhookSvc.GetWebhook(c.Request.Context(), webhookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	if existing.MerchantID != merchantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	if err := h.webhookSvc.DeleteWebhook(c.Request.Context(), webhookID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted"})
}
