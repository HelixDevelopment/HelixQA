package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/service"
)

type SubscriptionHandler struct {
	subscriptionSvc *service.SubscriptionService
}

func NewSubscriptionHandler(subscriptionSvc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subscriptionSvc: subscriptionSvc}
}

func (h *SubscriptionHandler) CreateSubscription(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		CustomerID    string `json:"customer_id" binding:"required"`
		Amount        int64  `json:"amount" binding:"required,gt=0"`
		Currency      string `json:"currency" binding:"required"`
		Interval      string `json:"interval" binding:"required"`
		IntervalCount int16  `json:"interval_count"`
		PlanID        string `json:"plan_id"`
		Provider      string `json:"provider"`
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

	if req.IntervalCount <= 0 {
		req.IntervalCount = 1
	}

	interval := model.SubscriptionInterval(req.Interval)
	switch interval {
	case model.SubscriptionIntervalDay, model.SubscriptionIntervalWeek, model.SubscriptionIntervalMonth, model.SubscriptionIntervalYear:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval"})
		return
	}

	sub, err := h.subscriptionSvc.CreateSubscription(c.Request.Context(), merchantID, customerID, req.Amount, req.Currency, interval, req.IntervalCount, req.PlanID, req.Provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create subscription"})
		return
	}
	c.JSON(http.StatusCreated, sub)
}

func (h *SubscriptionHandler) ListSubscriptions(c *gin.Context) {
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

	subs, total, err := h.subscriptionSvc.ListSubscriptions(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list subscriptions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs, "total": total})
}

func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	sub, err := h.subscriptionSvc.GetSubscription(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) UpdateSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	var req struct {
		Amount        *int64  `json:"amount"`
		Currency      *string `json:"currency"`
		Interval      *string `json:"interval"`
		IntervalCount *int16  `json:"interval_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var interval *model.SubscriptionInterval
	if req.Interval != nil {
		v := model.SubscriptionInterval(*req.Interval)
		switch v {
		case model.SubscriptionIntervalDay, model.SubscriptionIntervalWeek, model.SubscriptionIntervalMonth, model.SubscriptionIntervalYear:
			interval = &v
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval"})
			return
		}
	}

	sub, err := h.subscriptionSvc.UpdateSubscription(c.Request.Context(), id, req.Amount, req.Currency, interval, req.IntervalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update subscription"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) CancelSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	sub, err := h.subscriptionSvc.CancelSubscription(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel subscription"})
		return
	}
	c.JSON(http.StatusOK, sub)
}
