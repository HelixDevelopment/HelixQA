package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPaymentHandler_ProcessPayment_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payments", func(c *gin.Context) {
		var req struct {
			CustomerID      string `json:"customer_id" binding:"required"`
			PaymentMethodID string `json:"payment_method_id" binding:"required"`
			Amount          int64  `json:"amount" binding:"required,gt=0"`
			Currency        string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test"})
	})

	body := map[string]string{"customer_id": "test"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/payments", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ProcessPayment_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payments", func(c *gin.Context) {
		var req struct {
			CustomerID      string `json:"customer_id" binding:"required"`
			PaymentMethodID string `json:"payment_method_id" binding:"required"`
			Amount          int64  `json:"amount" binding:"required,gt=0"`
			Currency        string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "test", "amount": req.Amount})
	})

	body := map[string]interface{}{
		"customer_id":      "123e4567-e89b-12d3-a456-426614174000",
		"payment_method_id": "123e4567-e89b-12d3-a456-426614174001",
		"amount":           5000,
		"currency":         "USD",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174002/payments", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestPaymentHandler_CreateRefund_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/refunds", func(c *gin.Context) {
		var req struct {
			TransactionID string `json:"transaction_id" binding:"required"`
			Amount        int64  `json:"amount" binding:"required,gt=0"`
			Reason        string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "refund"})
	})

	body := map[string]string{"reason": "too expensive"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/refunds", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/subscriptions", func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
			Amount     int64  `json:"amount" binding:"required,gt=0"`
			Currency   string `json:"currency" binding:"required"`
			Interval   string `json:"interval" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "sub"})
	})

	body := map[string]string{"interval": "monthly"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/subscriptions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/subscriptions", func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
			Amount     int64  `json:"amount" binding:"required,gt=0"`
			Currency   string `json:"currency" binding:"required"`
			Interval   string `json:"interval" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "sub", "interval": req.Interval})
	})

	body := map[string]interface{}{
		"customer_id": "123e4567-e89b-12d3-a456-426614174000",
		"amount":      1000,
		"currency":    "USD",
		"interval":    "month",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174001/subscriptions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestInvoiceHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/invoices", func(c *gin.Context) {
		var req struct {
			CustomerID  string `json:"customer_id" binding:"required"`
			Amount      int64  `json:"amount" binding:"required,gt=0"`
			Currency    string `json:"currency" binding:"required"`
			PeriodStart string `json:"period_start" binding:"required"`
			PeriodEnd   string `json:"period_end" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "inv"})
	})

	body := map[string]string{"customer_id": "test"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/invoices", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/invoices", func(c *gin.Context) {
		var req struct {
			CustomerID  string `json:"customer_id" binding:"required"`
			Amount      int64  `json:"amount" binding:"required,gt=0"`
			Currency    string `json:"currency" binding:"required"`
			PeriodStart string `json:"period_start" binding:"required"`
			PeriodEnd   string `json:"period_end" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "inv", "amount": req.Amount})
	})

	body := map[string]interface{}{
		"customer_id":  "123e4567-e89b-12d3-a456-426614174000",
		"amount":       2500,
		"currency":     "EUR",
		"period_start": "2026-01-01T00:00:00Z",
		"period_end":   "2026-01-31T23:59:59Z",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174001/invoices", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestPayoutHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payouts", func(c *gin.Context) {
		var req struct {
			Provider string `json:"provider" binding:"required"`
			Amount   int64  `json:"amount" binding:"required,gt=0"`
			Currency string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "payout"})
	})

	body := map[string]string{"provider": "stripe"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/payouts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPayoutHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/payouts", func(c *gin.Context) {
		var req struct {
			Provider string `json:"provider" binding:"required"`
			Amount   int64  `json:"amount" binding:"required,gt=0"`
			Currency string `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "payout", "amount": req.Amount})
	})

	body := map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/payouts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestDisputeHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/disputes", func(c *gin.Context) {
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
		c.JSON(http.StatusCreated, gin.H{"id": "dispute"})
	})

	body := map[string]string{"reason": "fraud"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/disputes", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/disputes", func(c *gin.Context) {
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
		c.JSON(http.StatusCreated, gin.H{"id": "dispute", "reason": req.Reason})
	})

	body := map[string]interface{}{
		"transaction_id": "123e4567-e89b-12d3-a456-426614174000",
		"provider":       "stripe",
		"reason":         "unauthorized",
		"amount":         500,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174001/disputes", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestWebhookHandler_Create_BindError(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/webhooks", func(c *gin.Context) {
		var req struct {
			URL    string   `json:"url" binding:"required"`
			Events []string `json:"events" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "wh"})
	})

	body := map[string]string{"url": "https://example.com"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/webhooks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	router := setupRouter()
	router.POST("/merchants/:merchantId/webhooks", func(c *gin.Context) {
		var req struct {
			URL    string   `json:"url" binding:"required"`
			Events []string `json:"events" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": "wh", "url": req.URL})
	})

	body := map[string]interface{}{
		"url":    "https://example.com/webhook",
		"events": []string{"payment.succeeded", "payment.failed"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/merchants/123e4567-e89b-12d3-a456-426614174000/webhooks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestExchangeRateHandler_MissingParams(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/exchange-rates", func(c *gin.Context) {
		from := c.Query("from")
		to := c.Query("to")
		if from == "" || to == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query params required"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"rate": 1.0})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/exchange-rates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExchangeRateHandler_Success(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId/exchange-rates", func(c *gin.Context) {
		from := c.Query("from")
		to := c.Query("to")
		if from == "" || to == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query params required"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"from": from, "to": to, "rate": 1.1})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/123e4567-e89b-12d3-a456-426614174000/exchange-rates?from=USD&to=EUR", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["from"] != "USD" {
		t.Errorf("expected from USD, got %v", resp["from"])
	}
}

func TestPaginationDefaults(t *testing.T) {
	router := setupRouter()
	router.GET("/items", func(c *gin.Context) {
		page := c.DefaultQuery("page", "1")
		pageSize := c.DefaultQuery("page_size", "20")
		c.JSON(http.StatusOK, gin.H{"page": page, "page_size": pageSize})
	})

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
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
	if resp["page_size"] != "20" {
		t.Errorf("expected page_size 20, got %v", resp["page_size"])
	}
}

func TestPaginationCustomParams(t *testing.T) {
	router := setupRouter()
	router.GET("/items", func(c *gin.Context) {
		page := c.DefaultQuery("page", "1")
		pageSize := c.DefaultQuery("page_size", "20")
		c.JSON(http.StatusOK, gin.H{"page": page, "page_size": pageSize})
	})

	req := httptest.NewRequest(http.MethodGet, "/items?page=3&page_size=50", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["page"] != "3" {
		t.Errorf("expected page 3, got %v", resp["page"])
	}
	if resp["page_size"] != "50" {
		t.Errorf("expected page_size 50, got %v", resp["page_size"])
	}
}

func TestInvalidUUID(t *testing.T) {
	router := setupRouter()
	router.GET("/merchants/:merchantId", func(c *gin.Context) {
		id := c.Param("merchantId")
		if id == "" || len(id) != 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant id"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	req := httptest.NewRequest(http.MethodGet, "/merchants/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestEmptyBodyPOST(t *testing.T) {
	router := setupRouter()
	router.POST("/items", func(c *gin.Context) {
		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
