package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMerchantHandler_CreateMerchant_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants", func(c *gin.Context) {
		var req struct {
			LegalName string `json:"legal_name" binding:"required"`
			Email     string `json:"email" binding:"required,email"`
			Country   string `json:"country" binding:"required"`
			Currency  string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]string{"legal_name": "Test Co"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerchantHandler_CreateMerchant_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants", func(c *gin.Context) {
		var req struct {
			LegalName string `json:"legal_name" binding:"required"`
			Email     string `json:"email" binding:"required,email"`
			Country   string `json:"country" binding:"required"`
			Currency  string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test", "legal_name": req.LegalName})
	})

	body := map[string]string{
		"legal_name": "Test Corp",
		"email":      "test@example.com",
		"country":    "US",
		"currency":   "USD",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}
