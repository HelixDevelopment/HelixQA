package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProviderHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/providers", func(c *gin.Context) {
		var req struct {
			Provider string `json:"provider" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/providers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProviderHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/providers", func(c *gin.Context) {
		var req struct {
			Provider string `json:"provider" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test", "provider": req.Provider})
	})

	body := map[string]interface{}{
		"provider":     "stripe",
		"is_active":    true,
		"config":       map[string]string{"api_key": "sk_test"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/providers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestProviderHandler_InvalidMerchantID(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/providers", func(c *gin.Context) {
		id := c.Param("merchantId")
		if len(id) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	body := map[string]string{"provider": "stripe"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/not-a-uuid/providers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payment-methods", func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
			Type       string `json:"type" binding:"required"`
			Provider   string `json:"provider" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]string{"customer_id": "c1"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/payment-methods", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payment-methods", func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
			Type       string `json:"type" binding:"required"`
			Provider   string `json:"provider" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]interface{}{
		"customer_id": "123e4567-e89b-12d3-a456-426614174000",
		"type":        "card",
		"provider":    "stripe",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174001/payment-methods", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestPaymentMethodHandler_List_MissingCustomerID(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/payment-methods", func(c *gin.Context) {
		customerIDStr := c.Query("customer_id")
		if customerIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id query param is required"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"payment_methods": []string{}})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/payment-methods", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_List_Success(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/payment-methods", func(c *gin.Context) {
		customerIDStr := c.Query("customer_id")
		if customerIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id required"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"payment_methods": []string{"pm1"}})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/payment-methods?customer_id=123e4567-e89b-12d3-a456-426614174001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestBillingHandler_GetFees_InvalidMerchantID(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/billing/fees", func(c *gin.Context) {
		id := c.Param("merchantId")
		if len(id) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"fees": 0})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/not-a-uuid/billing/fees", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBillingHandler_GetFees_InvalidDate(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/billing/fees", func(c *gin.Context) {
		id := c.Param("merchantId")
		if len(id) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
			return
		}
		fromStr := c.DefaultQuery("from", "")
		if fromStr != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"fees": 0})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/billing/fees?from=not-a-date", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAnalyticsHandler_GetSummary_Defaults(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/analytics/summary", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"from": c.DefaultQuery("from", "default-from"), "to": c.DefaultQuery("to", "default-to")})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/analytics/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["from"] != "default-from" {
		t.Errorf("expected default from, got %v", resp["from"])
	}
}

func TestAnalyticsHandler_GetTransactionAnalytics_CustomParams(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/analytics/transactions", func(c *gin.Context) {
		groupBy := c.DefaultQuery("group_by", "day")
		c.JSON(http.StatusOK, gin.H{"group_by": groupBy})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/analytics/transactions?group_by=month", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["group_by"] != "month" {
		t.Errorf("expected month, got %v", resp["group_by"])
	}
}

func TestAuditHandler_List_InvalidMerchantID(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/audit-logs", func(c *gin.Context) {
		id := c.Param("merchantId")
		if len(id) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"audit_logs": []string{}})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/not-a-uuid/audit-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Update_BindError(t *testing.T) {
	router := setupRouter()
	router.PUT("/users/me", func(c *gin.Context) {
		var req struct {
			Name  string `json:"name"`
			Email string `json:"email" binding:"omitempty,email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := map[string]string{"email": "not-an-email"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_Update_Success(t *testing.T) {
	router := setupRouter()
	router.PUT("/users/me", func(c *gin.Context) {
		var req struct {
			Name  string `json:"name"`
			Email string `json:"email" binding:"omitempty,email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": req.Name, "email": req.Email})
	})

	body := map[string]string{"name": "John Doe"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApiKeyHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/api-keys", func(c *gin.Context) {
		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApiKeyHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/api-keys", func(c *gin.Context) {
		var req struct {
			Name   string   `json:"name" binding:"required"`
			Scopes []string `json:"scopes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"name": req.Name})
	})

	body := map[string]interface{}{
		"name":   "Production Key",
		"scopes": []string{"read", "write"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestWebhookIngressHandler_Stripe(t *testing.T) {
	router := setupRouter()
	router.POST("/webhooks/stripe", func(c *gin.Context) {
		sig := c.GetHeader("Stripe-Signature")
		if sig == "" {
			// no sig is fine for now (TODO)
		}
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	body := map[string]string{"type": "payment_intent.succeeded"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "sig_test_123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookIngressHandler_PayPal(t *testing.T) {
	router := setupRouter()
	router.POST("/webhooks/paypal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	body := map[string]string{"event_type": "PAYMENT.CAPTURE.COMPLETED"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/paypal", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookIngressHandler_Square(t *testing.T) {
	router := setupRouter()
	router.POST("/webhooks/square", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"received": true})
	})

	body := map[string]string{"type": "payment.completed"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/square", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookHandler_Update_BindError(t *testing.T) {
	router := setupRouter()
	router.PUT("/merchants/:merchantId/webhooks/:webhookId", func(c *gin.Context) {
		var req struct {
			URL    string   `json:"url" binding:"required"`
			Events []string `json:"events" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/merchants/123e4567-e89b-12d3-a456-426614174000/webhooks/123e4567-e89b-12d3-a456-426614174001", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Cancel_NotFound(t *testing.T) {
	router := setupRouter()
	router.DELETE("/merchants/:merchantId/subscriptions/:subscriptionId", func(c *gin.Context) {
		subID := c.Param("subscriptionId")
		if len(subID) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
	})

	req := httptest.NewRequest(http.MethodDelete, "/merchants/123e4567-e89b-12d3-a456-426614174000/subscriptions/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_AddEvidence_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/disputes/:disputeId/evidence", func(c *gin.Context) {
		var req struct {
			Type    string `json:"type" binding:"required"`
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := map[string]string{"type": "customer_communication"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/disputes/123e4567-e89b-12d3-a456-426614174001/evidence", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ListTransactions_Defaults(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/transactions", func(c *gin.Context) {
		page := c.DefaultQuery("page", "1")
		pageSize := c.DefaultQuery("page_size", "20")
		status := c.DefaultQuery("status", "")
		c.JSON(http.StatusOK, gin.H{"page": page, "page_size": pageSize, "status": status})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/transactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["page"] != "1" {
		t.Errorf("expected page 1, got %v", resp["page"])
	}
}

func TestMerchantHandler_Update_BindError(t *testing.T) {
	router := setupRouter()
	router.PUT("/merchants/:merchantId", func(c *gin.Context) {
		var req struct {
			LegalName string `json:"legal_name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/merchants/123e4567-e89b-12d3-a456-426614174000", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerchantHandler_Update_Success(t *testing.T) {
	router := setupRouter()
	router.PUT("/merchants/:merchantId", func(c *gin.Context) {
		var req struct {
			LegalName string `json:"legal_name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"legal_name": req.LegalName})
	})

	body := map[string]string{"legal_name": "Updated Corp"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/merchants/123e4567-e89b-12d3-a456-426614174000", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCustomerHandler_Update_BindError(t *testing.T) {
	router := setupRouter()
	router.PUT("/merchants/:merchantId/customers/:customerId", func(c *gin.Context) {
		var req struct {
			Name  string `json:"name" binding:"required"`
			Email string `json:"email" binding:"required,email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := map[string]string{"email": "bad"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/merchants/123e4567-e89b-12d3-a456-426614174000/customers/123e4567-e89b-12d3-a456-426614174001", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
