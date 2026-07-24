package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/helix-seller/helix-seller/internal/service"
)

type ExchangeRateHandler struct {
	exchangeSvc *service.ExchangeRateService
}

func NewExchangeRateHandler(exchangeSvc *service.ExchangeRateService) *ExchangeRateHandler {
	return &ExchangeRateHandler{exchangeSvc: exchangeSvc}
}

// GET /merchants/:merchantId/exchange-rates
func (h *ExchangeRateHandler) GetExchangeRate(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query params required"})
		return
	}
	rate, err := h.exchangeSvc.GetRate(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"from": from,
		"to":   to,
		"rate": rate,
	})
}
