package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/service"
)

type PayoutHandler struct {
	payoutSvc *service.PayoutService
}

func NewPayoutHandler(payoutSvc *service.PayoutService) *PayoutHandler {
	return &PayoutHandler{payoutSvc: payoutSvc}
}

func (h *PayoutHandler) CreatePayout(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		Provider string            `json:"provider" binding:"required"`
		Amount   int64             `json:"amount" binding:"required,gt=0"`
		Currency string            `json:"currency" binding:"required"`
		Method   model.PayoutMethod `json:"method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Method == "" {
		req.Method = model.PayoutMethodStandard
	}

	payout, err := h.payoutSvc.CreatePayout(c.Request.Context(), merchantID, req.Provider, req.Currency, req.Amount, req.Method)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payout"})
		return
	}
	c.JSON(http.StatusCreated, payout)
}

func (h *PayoutHandler) GetPayout(c *gin.Context) {
	id, err := uuid.Parse(c.Param("payoutId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payout id"})
		return
	}

	payout, err := h.payoutSvc.GetPayout(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payout not found"})
		return
	}
	c.JSON(http.StatusOK, payout)
}

func (h *PayoutHandler) ListPayouts(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	payouts, total, err := h.payoutSvc.ListPayouts(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payouts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payouts": payouts, "total": total})
}
