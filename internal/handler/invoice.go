package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/service"
)

type InvoiceHandler struct {
	invoiceSvc *service.InvoiceService
}

func NewInvoiceHandler(invoiceSvc *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceSvc: invoiceSvc}
}

func (h *InvoiceHandler) CreateInvoice(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		CustomerID     string `json:"customer_id" binding:"required"`
		SubscriptionID *string `json:"subscription_id"`
		Amount         int64  `json:"amount" binding:"required,gt=0"`
		Currency       string `json:"currency" binding:"required"`
		Provider       string `json:"provider"`
		DueDate        string `json:"due_date" binding:"required"`
		PeriodStart    string `json:"period_start" binding:"required"`
		PeriodEnd      string `json:"period_end" binding:"required"`
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

	var subscriptionID *uuid.UUID
	if req.SubscriptionID != nil {
		sid, err := uuid.Parse(*req.SubscriptionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
			return
		}
		subscriptionID = &sid
	}

	periodStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_start"})
		return
	}
	periodEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period_end"})
		return
	}

	dueDate, err := time.Parse(time.RFC3339, req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid due_date"})
		return
	}

	inv, err := h.invoiceSvc.CreateInvoice(c.Request.Context(), merchantID, customerID, subscriptionID, req.Amount, req.Currency, req.Provider, dueDate, periodStart, periodEnd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invoice"})
		return
	}
	c.JSON(http.StatusCreated, inv)
}

func (h *InvoiceHandler) ListInvoices(c *gin.Context) {
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

	invoices, total, err := h.invoiceSvc.ListInvoices(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list invoices"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invoices": invoices, "total": total})
}

func (h *InvoiceHandler) GetInvoice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("invoiceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice id"})
		return
	}

	inv, err := h.invoiceSvc.GetInvoice(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
		return
	}
	c.JSON(http.StatusOK, inv)
}
