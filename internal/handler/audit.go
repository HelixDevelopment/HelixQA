package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/repository"
)

type AuditHandler struct {
	auditRepo *repository.AuditLogRepo
}

func NewAuditHandler(auditRepo *repository.AuditLogRepo) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	merchantID, err := uuid.Parse(c.Param("merchantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
		return
	}

	logs, total, err := h.auditRepo.ListByMerchant(c.Request.Context(), merchantID, 1, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"audit_logs": logs, "total": total})
}
