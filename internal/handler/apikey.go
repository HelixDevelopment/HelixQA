package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/service"
)

type ApiKeyHandler struct {
	apiKeySvc *service.ApiKeyService
}

func NewApiKeyHandler(apiKeySvc *service.ApiKeyService) *ApiKeyHandler {
	return &ApiKeyHandler{apiKeySvc: apiKeySvc}
}

// POST /api-keys
func (h *ApiKeyHandler) CreateApiKey(c *gin.Context) {
	var req struct {
		Name      string   `json:"name" binding:"required"`
		Scopes    []string `json:"scopes"`
		RateLimit int      `json:"rate_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	merchantID, err := uuid.Parse(c.GetString("merchant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if req.RateLimit == 0 {
		req.RateLimit = 1000
	}
	fullKey, apiKey, err := h.apiKeySvc.Create(c.Request.Context(), merchantID, userID, req.Name, req.Scopes, req.RateLimit, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"key":     fullKey,
		"api_key": apiKey,
		"message": "save this key securely, it will not be shown again",
	})
}

// GET /api-keys
func (h *ApiKeyHandler) ListApiKeys(c *gin.Context) {
	merchantID, err := uuid.Parse(c.GetString("merchant_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	keys, err := h.apiKeySvc.ListByMerchant(c.Request.Context(), merchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

// DELETE /api-keys/:keyId
func (h *ApiKeyHandler) RevokeApiKey(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("keyId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}
	if err := h.apiKeySvc.Revoke(c.Request.Context(), keyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke API key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
}
