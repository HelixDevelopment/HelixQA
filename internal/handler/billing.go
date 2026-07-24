package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/service"
)

type BillingHandler struct {
	billingSvc *service.BillingService
}

func NewBillingHandler(billingSvc *service.BillingService) *BillingHandler {
	return &BillingHandler{billingSvc: billingSvc}
}

func (h *BillingHandler) GetFees(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' date"})
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' date"})
		return
	}

	period, err := h.billingSvc.CalculateFees(c.Request.Context(), merchantID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to calculate fees"})
		return
	}

	c.JSON(http.StatusOK, period)
}

func (h *BillingHandler) GetBillingInvoices(c *gin.Context) {
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

	invoices, total, err := h.billingSvc.ListBillingInvoices(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list billing invoices"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"merchant_id": merchantID,
		"invoices":    invoices,
		"total":       total,
	})
}
