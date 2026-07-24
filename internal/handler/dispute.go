package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/service"
)

type DisputeHandler struct {
	disputeSvc *service.DisputeService
}

func NewDisputeHandler(disputeSvc *service.DisputeService) *DisputeHandler {
	return &DisputeHandler{disputeSvc: disputeSvc}
}

func (h *DisputeHandler) CreateDispute(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	var req struct {
		TransactionID string `json:"transaction_id" binding:"required"`
		Provider      string `json:"provider" binding:"required"`
		Reason        string `json:"reason" binding:"required"`
		Amount        int64  `json:"amount" binding:"required,gt=0"`
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

	dispute, err := h.disputeSvc.CreateDispute(c.Request.Context(), merchantID, txID, req.Provider, req.Reason, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dispute"})
		return
	}
	c.JSON(http.StatusCreated, dispute)
}

func (h *DisputeHandler) GetDispute(c *gin.Context) {
	id, err := uuid.Parse(c.Param("disputeId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dispute id"})
		return
	}

	dispute, err := h.disputeSvc.GetDispute(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dispute not found"})
		return
	}
	c.JSON(http.StatusOK, dispute)
}

func (h *DisputeHandler) ListDisputes(c *gin.Context) {
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

	disputes, total, err := h.disputeSvc.ListDisputes(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list disputes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"disputes": disputes, "total": total})
}

func (h *DisputeHandler) AddEvidence(c *gin.Context) {
	disputeID, err := uuid.Parse(c.Param("disputeId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dispute id"})
		return
	}

	var req struct {
		EvidenceURL string `json:"evidence_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dispute, err := h.disputeSvc.AddEvidence(c.Request.Context(), disputeID, req.EvidenceURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add evidence"})
		return
	}
	c.JSON(http.StatusOK, dispute)
}
