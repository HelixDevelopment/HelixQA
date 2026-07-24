package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/helix-seller/helix-seller/internal/websocket"
	"go.uber.org/zap"
)

func TestNewRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	hub := websocket.NewHub(logger)
	wsHandler := websocket.NewWSHandler(hub, logger)

	r := NewRouter(
		logger,
		nil, nil, nil, 0,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil,
		wsHandler,
	)
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}

	routes := r.Routes()
	if len(routes) == 0 {
		t.Fatal("expected routes to be registered")
	}
}

func TestNewHealthHandler(t *testing.T) {
	h := NewHealthHandler(nil, nil, zap.NewNop())
	if h == nil {
		t.Fatal("NewHealthHandler returned nil")
	}
}

func TestRouter_HealthAndEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	hub := websocket.NewHub(logger)
	wsHandler := websocket.NewWSHandler(hub, logger)

	r := NewRouter(
		logger,
		func(c *gin.Context) { c.Next() }, nil, nil, 100,
		&AuthHandler{}, &UserHandler{}, &ApiKeyHandler{}, &MerchantHandler{},
		&PaymentHandler{}, &CustomerHandler{}, &SubscriptionHandler{},
		&InvoiceHandler{}, &PayoutHandler{}, &DisputeHandler{}, &WebhookHandler{},
		&AnalyticsHandler{}, &ProviderHandler{}, &PaymentMethodHandler{},
		&ExchangeRateHandler{}, &AuditHandler{}, &WebhookIngressHandler{},
		&BillingHandler{},
		&HealthHandler{
			db:     &mockDB{pingErr: nil},
			redis:  &mockRedis{pingErr: nil},
			logger: zap.NewNop(),
		},
		wsHandler,
	)

	tests := []struct {
		method string
		path   string
		code   int
	}{
		{"GET", "/health", http.StatusOK},
		{"GET", "/health/ready", http.StatusOK},
		{"GET", "/health/live", http.StatusOK},
		{"GET", "/metrics", http.StatusOK},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != tt.code {
			t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, w.Code, tt.code)
		}
	}
}

func TestBillingHandler_GetBillingInvoices_PaginationClamping(t *testing.T) {
	h := &BillingHandler{}
	mID := validUserID()
	tests := []struct {
		name   string
		query  string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max_page_size", "?page=1&page_size=200"},
		{"custom_valid", "?page=5&page_size=50"},
		{"defaults", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/invoices"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() {
				recover()
			}()
			h.GetBillingInvoices(c)
		})
	}
}

func TestBillingHandler_GetFees_DefaultDates(t *testing.T) {
	h := &BillingHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/fees", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.GetFees(c)
}

func TestPaymentHandler_ListTransactions_PaginationClamping(t *testing.T) {
	h := &PaymentHandler{}
	mID := validUserID()
	tests := []struct {
		name  string
		query string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max", "?page=1&page_size=200"},
		{"custom", "?page=5&page_size=50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/transactions"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() { recover() }()
			h.ListTransactions(c)
		})
	}
}

func TestPayoutHandler_ListPayouts_PaginationClamping(t *testing.T) {
	h := &PayoutHandler{}
	mID := validUserID()
	tests := []struct {
		name  string
		query string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max", "?page=1&page_size=200"},
		{"custom", "?page=3&page_size=10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/payouts"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() { recover() }()
			h.ListPayouts(c)
		})
	}
}

func TestDisputeHandler_ListDisputes_PaginationClamping(t *testing.T) {
	h := &DisputeHandler{}
	mID := validUserID()
	tests := []struct {
		name  string
		query string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max", "?page=1&page_size=200"},
		{"custom", "?page=2&page_size=15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/disputes"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() { recover() }()
			h.ListDisputes(c)
		})
	}
}

func TestSubscriptionHandler_ListSubscriptions_PaginationClamping(t *testing.T) {
	h := &SubscriptionHandler{}
	mID := validUserID()
	tests := []struct {
		name  string
		query string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max", "?page=1&page_size=200"},
		{"custom", "?page=2&page_size=15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/subs"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() { recover() }()
			h.ListSubscriptions(c)
		})
	}
}

func TestInvoiceHandler_ListInvoices_PaginationClamping(t *testing.T) {
	h := &InvoiceHandler{}
	mID := validUserID()
	tests := []struct {
		name  string
		query string
	}{
		{"page_below_one", "?page=0&page_size=0"},
		{"exceeds_max", "?page=1&page_size=200"},
		{"custom", "?page=3&page_size=10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestGinContext("GET", "/inv"+tt.query, nil,
				gin.Params{{Key: "merchantId", Value: mID}})
			defer func() { recover() }()
			h.ListInvoices(c)
		})
	}
}

func TestSubscriptionHandler_UpdateSubscription_AllIntervals(t *testing.T) {
	for _, interval := range []string{"day", "week", "month", "year"} {
		h := &SubscriptionHandler{}
		subID := validUserID()
		body, _ := json.Marshal(map[string]interface{}{
			"interval": interval,
		})
		c, _ := newTestGinContext("PATCH", "/sub/"+subID, body,
			gin.Params{{Key: "subscriptionId", Value: subID}})
		defer func() { recover() }()
		h.UpdateSubscription(c)
	}
}

func TestSubscriptionHandler_UpdateSubscription_NilInterval(t *testing.T) {
	h := &SubscriptionHandler{}
	subID := validUserID()
	body, _ := json.Marshal(map[string]interface{}{
		"amount": 2000,
	})
	c, _ := newTestGinContext("PATCH", "/sub/"+subID, body,
		gin.Params{{Key: "subscriptionId", Value: subID}})
	defer func() { recover() }()
	h.UpdateSubscription(c)
}

func TestPaymentMethodHandler_ListPaymentMethods_EmptyQuery(t *testing.T) {
	h := &PaymentMethodHandler{}
	mID := validUserID()
	c, w := newTestGinContext("GET", "/pm", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.ListPaymentMethods(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_ListPM_BadCustID(t *testing.T) {
	h := &PaymentMethodHandler{}
	mID := validUserID()
	c, w := newTestGinContext("GET", "/pm?customer_id=bad", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.ListPaymentMethods(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_CreatePM_BadCustID(t *testing.T) {
	h := &PaymentMethodHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    "not-a-uuid",
		"type":           "card",
		"provider":       "stripe",
		"provider_token": "tok_test",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/pm", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreatePaymentMethod(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Register_PwdTooShort(t *testing.T) {
	h := &AuthHandler{}
	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "short",
		"name":     "Test",
	})
	c, w := newTestGinContext("POST", "/auth/register", body, nil)
	h.Register(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	h := &AuthHandler{}
	c, w := newTestGinContext("POST", "/auth/login",
		[]byte(`{"email":"test@example.com"}`), nil)
	h.Login(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Refresh_EmptyReqBody(t *testing.T) {
	h := &AuthHandler{}
	c, w := newTestGinContext("POST", "/auth/refresh", []byte(`{}`), nil)
	h.Refresh(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_VerifyMFA_EmptyPayload(t *testing.T) {
	h := &AuthHandler{}
	c, w := newTestGinContext("POST", "/auth/mfa/verify", []byte(`{}`), nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerchantHandler_CreateMerchant_AllFields(t *testing.T) {
	h := &MerchantHandler{}
	body, _ := json.Marshal(map[string]string{
		"legal_name": "Full Corp",
		"trade_name": "Full",
		"email":      "full@corp.com",
		"phone":      "+1234567890",
		"country":    "US",
		"currency":   "USD",
	})
	c, _ := newTestGinContext("POST", "/merchants", body, nil)
	defer func() { recover() }()
	h.CreateMerchant(c)
}

func TestCustomerHandler_CreateCustomer_AllFields(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{
		"name":  "Full Customer",
		"email": "full@customer.com",
		"phone": "+1234567890",
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/merchants/"+mID+"/customers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreateCustomer(c)
}

func TestCustomerHandler_UpdateCustomer_AllFields(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{
		"name":  "Updated Name",
		"email": "updated@example.com",
		"phone": "+9876543210",
	})
	cID := validUserID()
	c, _ := newTestGinContext("PUT", "/customers/"+cID, body,
		gin.Params{{Key: "customerId", Value: cID}})
	defer func() { recover() }()
	h.UpdateCustomer(c)
}

func TestCustomerHandler_UpdateCustomer_EmptyFields(t *testing.T) {
	h := &CustomerHandler{}
	cID := validUserID()
	c, _ := newTestGinContext("PUT", "/customers/"+cID, []byte(`{}`),
		gin.Params{{Key: "customerId", Value: cID}})
	defer func() { recover() }()
	h.UpdateCustomer(c)
}

func TestMerchantHandler_UpdateMerchant_AllFields(t *testing.T) {
	h := &MerchantHandler{}
	body, _ := json.Marshal(map[string]string{
		"legal_name": "Updated Legal",
		"trade_name": "Updated Trade",
		"email":      "updated@corp.com",
		"phone":      "+1111111111",
	})
	mID := validUserID()
	c, _ := newTestGinContext("PUT", "/merchants/"+mID, body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.UpdateMerchant(c)
}

func TestMerchantHandler_UpdateMerchant_EmptyFields(t *testing.T) {
	h := &MerchantHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("PUT", "/merchants/"+mID, []byte(`{}`),
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.UpdateMerchant(c)
}

func TestUserHandler_UpdateUser_EmptyFields(t *testing.T) {
	h := &UserHandler{}
	cID := validUserID()
	c, _ := newTestGinContext("PUT", "/users/me", []byte(`{}`), nil)
	c.Set("user_id", cID)
	defer func() { recover() }()
	h.UpdateUser(c)
}

func TestWebhookHandler_CreateWebhook_AllFields(t *testing.T) {
	h := &WebhookHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"url":      "https://example.com/webhook",
		"secret":   "whsec_test123",
		"events":   []string{"payment.succeeded", "payment.failed"},
		"is_active": true,
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/webhooks", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreateWebhook(c)
}

func TestProviderHandler_CreateProvider_AllFields(t *testing.T) {
	h := &ProviderHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider":       "stripe",
		"is_active":      true,
		"fallback_order": 1,
		"config":         map[string]string{"api_key": "sk_test_123"},
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/providers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreateProvider(c)
}

func TestProviderHandler_UpdateProvider_AllOptionalFields(t *testing.T) {
	h := &ProviderHandler{}
	isActive := true
	fo := int16(2)
	hs := "degraded"
	body, _ := json.Marshal(map[string]interface{}{
		"config":         map[string]string{"api_key": "new"},
		"is_active":      &isActive,
		"fallback_order": &fo,
		"health_status":  &hs,
	})
	pID := validUserID()
	c, _ := newTestGinContext("PUT", "/provider/"+pID, body,
		gin.Params{{Key: "providerId", Value: pID}})
	defer func() { recover() }()
	h.UpdateProvider(c)
}

func TestProviderHandler_UpdateProvider_PartialFields(t *testing.T) {
	h := &ProviderHandler{}
	isActive := false
	body, _ := json.Marshal(map[string]interface{}{
		"is_active": &isActive,
	})
	pID := validUserID()
	c, _ := newTestGinContext("PUT", "/provider/"+pID, body,
		gin.Params{{Key: "providerId", Value: pID}})
	defer func() { recover() }()
	h.UpdateProvider(c)
}

func TestPaymentMethodHandler_CreatePM_AllFields(t *testing.T) {
	h := &PaymentMethodHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    validUserID(),
		"type":           "card",
		"provider":       "stripe",
		"provider_token": "tok_visa_4242",
		"last4":          "4242",
		"fingerprint":    "fp_test",
		"brand":          "visa",
		"exp_month":      12,
		"exp_year":       2027,
		"is_default":     true,
		"metadata":       map[string]string{"label": "primary"},
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/pm", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreatePaymentMethod(c)
}

func TestApiKeyHandler_CreateApiKey_DefaultRateLimit_NoRate(t *testing.T) {
	h := &ApiKeyHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Test Key",
		"scopes": []string{"read"},
	})
	c, _ := newTestGinContext("POST", "/api-keys", body, nil)
	c.Set("merchant_id", validUserID())
	c.Set("user_id", validUserID())
	defer func() { recover() }()
	h.CreateApiKey(c)
}

func TestApiKeyHandler_CreateApiKey_CustomRateLimit(t *testing.T) {
	h := &ApiKeyHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"name":       "Test Key",
		"scopes":     []string{"read", "write"},
		"rate_limit": 500,
	})
	c, _ := newTestGinContext("POST", "/api-keys", body, nil)
	c.Set("merchant_id", validUserID())
	c.Set("user_id", validUserID())
	defer func() { recover() }()
	h.CreateApiKey(c)
}

func TestApiKeyHandler_ListApiKeys_NilService(t *testing.T) {
	h := &ApiKeyHandler{}
	c, _ := newTestGinContext("GET", "/api-keys", nil, nil)
	c.Set("merchant_id", validUserID())
	defer func() { recover() }()
	h.ListApiKeys(c)
}

func TestApiKeyHandler_RevokeApiKey_ValidID(t *testing.T) {
	h := &ApiKeyHandler{}
	kID := validUserID()
	c, _ := newTestGinContext("DELETE", "/api-keys/"+kID, nil,
		gin.Params{{Key: "keyId", Value: kID}})
	defer func() { recover() }()
	h.RevokeApiKey(c)
}

func TestWebhookHandler_DeleteWebhook_ValidIDs_NilService(t *testing.T) {
	h := &WebhookHandler{}
	mID := validUserID()
	whID := validUserID()
	c, _ := newTestGinContext("DELETE", "/webhook/"+whID, nil,
		gin.Params{{Key: "merchantId", Value: mID}, {Key: "webhookId", Value: whID}})
	defer func() { recover() }()
	h.DeleteWebhook(c)
}

func TestWebhookHandler_UpdateWebhook_ValidIDs_NilService(t *testing.T) {
	h := &WebhookHandler{}
	mID := validUserID()
	whID := validUserID()
	body, _ := json.Marshal(map[string]interface{}{
		"url":    "https://new.example.com",
		"secret": "new_secret",
		"events": []string{"payment.succeeded"},
	})
	c, _ := newTestGinContext("PUT", "/webhook/"+whID, body,
		gin.Params{{Key: "merchantId", Value: mID}, {Key: "webhookId", Value: whID}})
	defer func() { recover() }()
	h.UpdateWebhook(c)
}

func TestExchangeRateHandler_GetExchangeRate_ValidParams_NilService(t *testing.T) {
	h := &ExchangeRateHandler{}
	c, _ := newTestGinContext("GET", "/rates?from=USD&to=EUR", nil, nil)
	defer func() { recover() }()
	h.GetExchangeRate(c)
}

func TestPayoutHandler_CreatePayout_NormalMethod(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreatePayout(c)
}

func TestPayoutHandler_CreatePayout_FastMethod(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
		"method":   "instant",
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreatePayout(c)
}

func TestDisputeHandler_AddEvidence_ValidBody(t *testing.T) {
	h := &DisputeHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"evidence_url": "https://example.com/evidence.pdf",
	})
	dID := validUserID()
	c, _ := newTestGinContext("POST", "/evidence", body,
		gin.Params{{Key: "disputeId", Value: dID}})
	defer func() { recover() }()
	h.AddEvidence(c)
}

func TestAnalyticsHandler_GetSummary_CustomDates(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/summary?from=2026-01-01&to=2026-06-30", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.GetSummary(c)
}

func TestAnalyticsHandler_GetTransactionAnalytics_CustomGroupBy(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/transactions?group_by=month&from=2026-01-01&to=2026-06-30", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.GetTransactionAnalytics(c)
}

func TestAnalyticsHandler_ExportTransactions_CustomDates(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/export?from=2026-01-01&to=2026-01-31", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.ExportTransactions(c)
}

func TestInvoiceHandler_CreateInvoice_HasDueDate(t *testing.T) {
	h := &InvoiceHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":  validUserID(),
		"amount":       5000,
		"currency":     "USD",
		"due_date":     "2026-02-15T00:00:00Z",
		"period_start": "2026-01-01T00:00:00Z",
		"period_end":   "2026-01-31T23:59:59Z",
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreateInvoice(c)
}

func TestInvoiceHandler_CreateInvoice_HasSubID(t *testing.T) {
	h := &InvoiceHandler{}
	subID := validUserID()
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    validUserID(),
		"subscription_id": subID,
		"amount":         5000,
		"currency":       "USD",
		"period_start":   "2026-01-01T00:00:00Z",
		"period_end":     "2026-01-31T23:59:59Z",
	})
	mID := validUserID()
	c, _ := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() { recover() }()
	h.CreateInvoice(c)
}

func TestInvoiceHandler_CreateInvoice_BadDueDate(t *testing.T) {
	h := &InvoiceHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":  validUserID(),
		"amount":       5000,
		"currency":     "USD",
		"due_date":     "bad-date",
		"period_start": "2026-01-01T00:00:00Z",
		"period_end":   "2026-01-31T23:59:59Z",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateInvoice(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ProcessPayment_BadCustID(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":      "not-a-uuid",
		"payment_method_id": validUserID(),
		"amount":           1000,
		"currency":         "USD",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payments", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.ProcessPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ProcessPayment_BadPMID(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":      validUserID(),
		"payment_method_id": "not-a-uuid",
		"amount":           1000,
		"currency":         "USD",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payments", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.ProcessPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_CreateRefund_BadTxID(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": "not-a-uuid",
		"amount":         500,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/refunds", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateRefund(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_CreateDispute_BadTxID(t *testing.T) {
	h := &DisputeHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": "not-a-uuid",
		"provider":       "stripe",
		"reason":         "fraud",
		"amount":         500,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/disputes", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateDispute(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_CreateSubscription_BadInterval(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id": validUserID(),
		"amount":      1000,
		"currency":    "USD",
		"interval":    "invalid-interval",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/subs", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateSubscription(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_CreateSubscription_BadCustID(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id": "not-a-uuid",
		"amount":      1000,
		"currency":    "USD",
		"interval":    "month",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/subs", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateSubscription(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_UpdateWebhook_BindError(t *testing.T) {
	h := &WebhookHandler{}
	mID := validUserID()
	whID := validUserID()
	c, w := newTestGinContext("PUT", "/webhook/"+whID, []byte(`not json`),
		gin.Params{{Key: "merchantId", Value: mID}, {Key: "webhookId", Value: whID}})
	defer func() { recover() }()
	h.UpdateWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_AddEvidence_NoBody(t *testing.T) {
	h := &DisputeHandler{}
	dID := validUserID()
	c, w := newTestGinContext("POST", "/evidence", []byte(`{}`),
		gin.Params{{Key: "disputeId", Value: dID}})
	defer func() { recover() }()
	h.AddEvidence(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerchantHandler_CreateMerchant_BadEmail(t *testing.T) {
	h := &MerchantHandler{}
	body, _ := json.Marshal(map[string]string{
		"legal_name": "Test",
		"email":      "bad",
		"country":    "US",
		"currency":   "USD",
	})
	c, w := newTestGinContext("POST", "/merchants", body, nil)
	h.CreateMerchant(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCustomerHandler_CreateCustomer_BadEmail(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{
		"name":  "John",
		"email": "bad",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/merchants/"+mID+"/customers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateCustomer(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandler_UpdateUser_BadEmail(t *testing.T) {
	h := &UserHandler{}
	body, _ := json.Marshal(map[string]string{"email": "not-valid"})
	c, w := newTestGinContext("PUT", "/users/me", body, nil)
	c.Set("user_id", "invalid")
	h.UpdateUser(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ProcessPayment_ZeroAmount(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":      validUserID(),
		"payment_method_id": validUserID(),
		"amount":           0,
		"currency":         "USD",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payments", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.ProcessPayment(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_CreateSubscription_AmtZero(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id": validUserID(),
		"amount":      0,
		"currency":    "USD",
		"interval":    "month",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/subs", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateSubscription(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPayoutHandler_CreatePayout_AmtZero(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   0,
		"currency": "USD",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreatePayout(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_CreateDispute_AmtZero(t *testing.T) {
	h := &DisputeHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": validUserID(),
		"provider":       "stripe",
		"reason":         "fraud",
		"amount":         0,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/disputes", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateDispute(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_CreateInvoice_AmtZero(t *testing.T) {
	h := &InvoiceHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":  validUserID(),
		"amount":       0,
		"currency":     "USD",
		"period_start": "2026-01-01T00:00:00Z",
		"period_end":   "2026-01-31T23:59:59Z",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	h.CreateInvoice(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
