package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type ProviderHandler struct {
	providerRepo *repository.ProviderConfigRepo
}

func NewProviderHandler(providerRepo *repository.ProviderConfigRepo) *ProviderHandler {
	return &ProviderHandler{providerRepo: providerRepo}
}

// POST /merchants/:merchantId/providers
func (h *ProviderHandler) CreateProvider(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	var req struct {
		Provider    string          `json:"provider" binding:"required"`
		Config      json.RawMessage `json:"config"`
		IsActive    bool            `json:"is_active"`
		FallbackOrder int16         `json:"fallback_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pc := &model.ProviderConfig{
		ID:            uuid.New(),
		MerchantID:    merchantID,
		Provider:      req.Provider,
		IsActive:      req.IsActive,
		Config:        req.Config,
		FallbackOrder: req.FallbackOrder,
		HealthStatus:  model.HealthStatusHealthy,
	}
	if err := h.providerRepo.Create(c.Request.Context(), pc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create provider config"})
		return
	}
	c.JSON(http.StatusCreated, pc)
}

// GET /merchants/:merchantId/providers
func (h *ProviderHandler) ListProviders(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	providers, err := h.providerRepo.ListByMerchant(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list providers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// GET /merchants/:merchantId/providers/:providerId
func (h *ProviderHandler) GetProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("providerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}
	provider, err := h.providerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// PUT /merchants/:merchantId/providers/:providerId
func (h *ProviderHandler) UpdateProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("providerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}
	provider, err := h.providerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	var req struct {
		Config        json.RawMessage `json:"config"`
		IsActive      *bool           `json:"is_active"`
		FallbackOrder *int16          `json:"fallback_order"`
		HealthStatus  *string         `json:"health_status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Config != nil {
		provider.Config = req.Config
	}
	if req.IsActive != nil {
		provider.IsActive = *req.IsActive
	}
	if req.FallbackOrder != nil {
		provider.FallbackOrder = *req.FallbackOrder
	}
	if req.HealthStatus != nil {
		provider.HealthStatus = model.HealthStatus(*req.HealthStatus)
	}
	if err := h.providerRepo.Update(c.Request.Context(), provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update provider"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// DELETE /merchants/:merchantId/providers/:providerId
func (h *ProviderHandler) DeleteProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("providerId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider id"})
		return
	}
	if err := h.providerRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete provider"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "provider deleted"})
}
