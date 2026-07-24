package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/service"
)

type PaymentHandler struct {
	paymentSvc *service.PaymentService
}

func NewPaymentHandler(paymentSvc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentSvc: paymentSvc}
}

func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		CustomerID      string `json:"customer_id" binding:"required"`
		PaymentMethodID string `json:"payment_method_id" binding:"required"`
		Amount          int64  `json:"amount" binding:"required,gt=0"`
		Currency        string `json:"currency" binding:"required"`
		IdempotencyKey  string `json:"idempotency_key"`
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
	pmID, err := uuid.Parse(req.PaymentMethodID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method id"})
		return
	}

	tx, err := h.paymentSvc.ProcessPayment(c.Request.Context(), merchantID, customerID, pmID, req.Amount, req.Currency, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment failed"})
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *PaymentHandler) ListTransactions(c *gin.Context) {
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

	txns, total, err := h.paymentSvc.ListTransactions(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txns, "total": total})
}

func (h *PaymentHandler) GetTransaction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("transactionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	tx, err := h.paymentSvc.GetTransaction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}
	c.JSON(http.StatusOK, tx)
}

func (h *PaymentHandler) CreateRefund(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}
	_ = merchantID

	var req struct {
		TransactionID string `json:"transaction_id" binding:"required"`
		Amount        int64  `json:"amount" binding:"required,gt=0"`
		Reason        string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	refund, err := h.paymentSvc.Refund(c.Request.Context(), txID, req.Amount, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refund failed"})
		return
	}
	c.JSON(http.StatusCreated, refund)
}
