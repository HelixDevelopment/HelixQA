package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/service"
)

type AnalyticsHandler struct {
	analyticsSvc *service.AnalyticsService
}

func NewAnalyticsHandler(analyticsSvc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsSvc: analyticsSvc}
}

// GET /merchants/:merchantId/analytics/summary
func (h *AnalyticsHandler) GetSummary(c *gin.Context) {
	merchantID, _ := uuid.Parse(c.Param("merchantId"))
	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	to = to.Add(24*time.Hour - time.Second) // end of day

	summary, err := h.analyticsSvc.GetSummary(c.Request.Context(), merchantID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get summary"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GET /merchants/:merchantId/analytics/transactions
func (h *AnalyticsHandler) GetTransactionAnalytics(c *gin.Context) {
	merchantID, _ := uuid.Parse(c.Param("merchantId"))
	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	groupBy := c.DefaultQuery("group_by", "day")
	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	to = to.Add(24*time.Hour - time.Second)

	analytics, err := h.analyticsSvc.GetTransactionAnalytics(c.Request.Context(), merchantID, from, to, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"analytics": analytics})
}

// GET /merchants/:merchantId/analytics/export
func (h *AnalyticsHandler) ExportTransactions(c *gin.Context) {
	merchantID, _ := uuid.Parse(c.Param("merchantId"))
	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	toStr := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	to = to.Add(24*time.Hour - time.Second)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=transactions.csv")
	if err := h.analyticsSvc.ExportTransactions(c.Request.Context(), merchantID, from, to, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
	}
}
