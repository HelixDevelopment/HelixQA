package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type PaymentMethodHandler struct {
	pmRepo *repository.PaymentMethodRepo
}

func NewPaymentMethodHandler(pmRepo *repository.PaymentMethodRepo) *PaymentMethodHandler {
	return &PaymentMethodHandler{pmRepo: pmRepo}
}

// POST /merchants/:merchantId/payment-methods
func (h *PaymentMethodHandler) CreatePaymentMethod(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	var req struct {
		CustomerID   string          `json:"customer_id" binding:"required"`
		Type         string          `json:"type" binding:"required"`
		Provider     string          `json:"provider" binding:"required"`
		ProviderToken string         `json:"provider_token" binding:"required"`
		Last4        string          `json:"last4"`
		Fingerprint  string          `json:"fingerprint"`
		Brand        string          `json:"brand"`
		ExpMonth     int16           `json:"exp_month"`
		ExpYear      int16           `json:"exp_year"`
		IsDefault    bool            `json:"is_default"`
		Metadata     json.RawMessage `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	pm := &model.PaymentMethod{
		ID:             uuid.New(),
		CustomerID:     customerID,
		MerchantID:     merchantID,
		Type:           model.PaymentMethodType(req.Type),
		Provider:       req.Provider,
		ProviderToken:  req.ProviderToken,
		Last4:          req.Last4,
		Fingerprint:    req.Fingerprint,
		Brand:          req.Brand,
		ExpMonth:       req.ExpMonth,
		ExpYear:        req.ExpYear,
		IsDefault:      req.IsDefault,
		Metadata:       req.Metadata,
	}
	if err := h.pmRepo.Create(c.Request.Context(), pm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment method"})
		return
	}
	c.JSON(http.StatusCreated, pm)
}

// GET /merchants/:merchantId/payment-methods
func (h *PaymentMethodHandler) ListPaymentMethods(c *gin.Context) {
	customerIDStr := c.Query("customer_id")
	if customerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id query param is required"})
		return
	}
	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	methods, err := h.pmRepo.ListByCustomer(c.Request.Context(), customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payment methods"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment_methods": methods})
}

// GET /merchants/:merchantId/payment-methods/:paymentMethodId
func (h *PaymentMethodHandler) GetPaymentMethod(c *gin.Context) {
	id, err := uuid.Parse(c.Param("paymentMethodId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method id"})
		return
	}
	pm, err := h.pmRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment method not found"})
		return
	}
	c.JSON(http.StatusOK, pm)
}

// DELETE /merchants/:merchantId/payment-methods/:paymentMethodId
func (h *PaymentMethodHandler) DeletePaymentMethod(c *gin.Context) {
	id, err := uuid.Parse(c.Param("paymentMethodId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method id"})
		return
	}
	if err := h.pmRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete payment method"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "payment method deleted"})
}
