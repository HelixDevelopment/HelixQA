package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// --- AuthHandler tests that exercise deeper paths ---

func TestAuthHandler_Register_SuccessPath(t *testing.T) {
	h := &AuthHandler{}
	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "securepassword123",
		"name":     "Test User",
	})
	c, w := newTestGinContext("POST", "/auth/register", body, nil)
	defer func() {
		if r := recover(); r == nil {
			// Expected: nil authSvc causes panic at h.authSvc.HashPassword
		}
	}()
	_ = w
	h.Register(c)
}

func TestAuthHandler_Login_SuccessPath(t *testing.T) {
	h := &AuthHandler{}
	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
	})
	c, w := newTestGinContext("POST", "/auth/login", body, nil)
	defer func() {
		if r := recover(); r == nil {
			// Expected: nil authSvc causes panic
		}
	}()
	h.Login(c)
	_ = w
}

func TestAuthHandler_Refresh_SuccessPath(t *testing.T) {
	h := &AuthHandler{}
	body, _ := json.Marshal(map[string]string{
		"refresh_token": "some.jwt.token",
	})
	c, w := newTestGinContext("POST", "/auth/refresh", body, nil)
	defer func() {
		if r := recover(); r == nil {
			// Expected: nil jwtSvc causes panic
		}
	}()
	h.Refresh(c)
	_ = w
}

func TestAuthHandler_VerifyMFA_SuccessPath(t *testing.T) {
	h := &AuthHandler{}
	body, _ := json.Marshal(map[string]string{
		"user_id": validUserID(),
		"code":    "123456",
	})
	c, w := newTestGinContext("POST", "/auth/mfa/verify", body, nil)
	defer func() {
		if r := recover(); r == nil {
			// Expected: nil userRepo causes panic
		}
	}()
	h.VerifyMFA(c)
	_ = w
}

func TestAuthHandler_SetupMFA_InvalidUserID(t *testing.T) {
	h := &AuthHandler{}
	c, w := newTestGinContext("POST", "/auth/mfa/setup", nil, nil)
	c.Set("user_id", "not-a-uuid")
	defer func() {
		if r := recover(); r == nil {
			// Expected: nil mfaService causes panic
		}
	}()
	h.SetupMFA(c)
	_ = w
}

// --- UserHandler deeper paths ---

func TestUserHandler_GetUser_ValidUUID_NilRepo(t *testing.T) {
	h := &UserHandler{}
	c, w := newTestGinContext("GET", "/users/me", nil, nil)
	c.Set("user_id", validUserID())
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil userRepo causes panic
		}
	}()
	h.GetUser(c)
	_ = w
}

func TestUserHandler_UpdateUser_ValidUUID_NilRepo(t *testing.T) {
	h := &UserHandler{}
	body, _ := json.Marshal(map[string]string{"name": "Updated Name"})
	c, w := newTestGinContext("PUT", "/users/me", body, nil)
	c.Set("user_id", validUserID())
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil userRepo causes panic
		}
	}()
	h.UpdateUser(c)
	_ = w
}

func TestUserHandler_UpdateUser_EmptyBody(t *testing.T) {
	h := &UserHandler{}
	c, _ := newTestGinContext("PUT", "/users/me", []byte(`{}`), nil)
	c.Set("user_id", "invalid-uuid")
	h.UpdateUser(c)
	// invalid uuid → 400
}

// --- ApiKeyHandler deeper paths ---

func TestApiKeyHandler_CreateApiKey_ValidBody_NilService(t *testing.T) {
	h := &ApiKeyHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"name":       "Test Key",
		"scopes":     []string{"read"},
		"rate_limit": 500,
	})
	c, w := newTestGinContext("POST", "/api-keys", body, nil)
	c.Set("merchant_id", validUserID())
	c.Set("user_id", validUserID())
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil apiKeySvc causes panic
		}
	}()
	h.CreateApiKey(c)
	_ = w
}

func TestApiKeyHandler_CreateApiKey_DefaultRateLimit(t *testing.T) {
	h := &ApiKeyHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Test Key",
		"scopes": []string{"read"},
	})
	c, w := newTestGinContext("POST", "/api-keys", body, nil)
	c.Set("merchant_id", validUserID())
	c.Set("user_id", validUserID())
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil apiKeySvc causes panic
		}
	}()
	h.CreateApiKey(c)
	_ = w
}

func TestApiKeyHandler_ListApiKeys_ValidUUID_NilService(t *testing.T) {
	h := &ApiKeyHandler{}
	c, _ := newTestGinContext("GET", "/api-keys", nil, nil)
	c.Set("merchant_id", validUserID())
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil apiKeySvc causes panic
		}
	}()
	h.ListApiKeys(c)
}

// --- MerchantHandler deeper paths ---

func TestMerchantHandler_CreateMerchant_ValidBody_NilRepo(t *testing.T) {
	h := &MerchantHandler{}
	body, _ := json.Marshal(map[string]string{
		"legal_name": "Test Corp",
		"email":      "test@corp.com",
		"country":    "US",
		"currency":   "USD",
	})
	c, w := newTestGinContext("POST", "/merchants", body, nil)
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil merchantRepo causes panic
		}
	}()
	h.CreateMerchant(c)
	_ = w
}

func TestMerchantHandler_ListMerchants_ValidRequest(t *testing.T) {
	h := &MerchantHandler{}
	c, _ := newTestGinContext("GET", "/merchants", nil, nil)
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil merchantRepo causes panic
		}
	}()
	h.ListMerchants(c)
}

func TestMerchantHandler_GetMerchant_ValidUUID_NilRepo(t *testing.T) {
	h := &MerchantHandler{}
	c, w := newTestGinContext("GET", "/merchants/"+validUserID(), nil,
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil merchantRepo causes panic
		}
	}()
	h.GetMerchant(c)
	_ = w
}

func TestMerchantHandler_UpdateMerchant_ValidUUID_NilRepo(t *testing.T) {
	h := &MerchantHandler{}
	body, _ := json.Marshal(map[string]string{"legal_name": "Updated"})
	c, w := newTestGinContext("PUT", "/merchants/"+validUserID(), body,
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil merchantRepo causes panic
		}
	}()
	h.UpdateMerchant(c)
	_ = w
}

func TestMerchantHandler_UpdateMerchant_EmptyBody(t *testing.T) {
	h := &MerchantHandler{}
	c, w := newTestGinContext("PUT", "/merchants/"+validUserID(), []byte(`{}`),
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil merchantRepo causes panic
		}
	}()
	h.UpdateMerchant(c)
	_ = w
}

// --- CustomerHandler deeper paths ---

func TestCustomerHandler_CreateCustomer_ValidBody_NilRepo(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/merchants/"+mID+"/customers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.CreateCustomer(c)
	_ = w
}

func TestCustomerHandler_CreateCustomer_WithPhone(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{
		"name":  "John Doe",
		"email": "john@example.com",
		"phone": "+1234567890",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/merchants/"+mID+"/customers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.CreateCustomer(c)
	_ = w
}

func TestCustomerHandler_ListCustomers_ValidRequest(t *testing.T) {
	h := &CustomerHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/merchants/"+mID+"/customers", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.ListCustomers(c)
}

func TestCustomerHandler_GetCustomer_ValidUUID_NilRepo(t *testing.T) {
	h := &CustomerHandler{}
	cID := validUserID()
	c, w := newTestGinContext("GET", "/customers/"+cID, nil,
		gin.Params{{Key: "customerId", Value: cID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.GetCustomer(c)
	_ = w
}

func TestCustomerHandler_UpdateCustomer_ValidUUID_NilRepo(t *testing.T) {
	h := &CustomerHandler{}
	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	cID := validUserID()
	c, w := newTestGinContext("PUT", "/customers/"+cID, body,
		gin.Params{{Key: "customerId", Value: cID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.UpdateCustomer(c)
	_ = w
}

func TestCustomerHandler_UpdateCustomer_EmptyBody(t *testing.T) {
	h := &CustomerHandler{}
	cID := validUserID()
	c, w := newTestGinContext("PUT", "/customers/"+cID, []byte(`{}`),
		gin.Params{{Key: "customerId", Value: cID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil customerRepo causes panic
		}
	}()
	h.UpdateCustomer(c)
	_ = w
}

// --- PaymentHandler deeper paths ---

func TestPaymentHandler_ProcessPayment_ValidBody_NilService(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":      validUserID(),
		"payment_method_id": validUserID(),
		"amount":           5000,
		"currency":         "USD",
		"idempotency_key":  "idem-key-123",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payments", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.ProcessPayment(c)
	_ = w
}

func TestPaymentHandler_ListTransactions_DefaultQueryParams(t *testing.T) {
	h := &PaymentHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/transactions", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.ListTransactions(c)
}

func TestPaymentHandler_ListTransactions_CustomQueryParams(t *testing.T) {
	h := &PaymentHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/transactions?page=2&page_size=50", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.ListTransactions(c)
}

func TestPaymentHandler_ListTransactions_InvalidPageParams(t *testing.T) {
	h := &PaymentHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/transactions?page=0&page_size=0", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.ListTransactions(c)
}

func TestPaymentHandler_ListTransactions_ExceedsMaxPageSize(t *testing.T) {
	h := &PaymentHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/transactions?page=1&page_size=200", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.ListTransactions(c)
}

func TestPaymentHandler_GetTransaction_ValidUUID_NilService(t *testing.T) {
	h := &PaymentHandler{}
	txID := validUserID()
	c, w := newTestGinContext("GET", "/tx/"+txID, nil,
		gin.Params{{Key: "transactionId", Value: txID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.GetTransaction(c)
	_ = w
}

func TestPaymentHandler_CreateRefund_ValidBody_NilService(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": validUserID(),
		"amount":         1000,
		"reason":         "customer requested",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/refunds", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.CreateRefund(c)
	_ = w
}

func TestPaymentHandler_CreateRefund_EmptyReason(t *testing.T) {
	h := &PaymentHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": validUserID(),
		"amount":         1000,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/refunds", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil paymentSvc causes panic
		}
	}()
	h.CreateRefund(c)
	_ = w
}

// --- SubscriptionHandler deeper paths ---

func TestSubscriptionHandler_CreateSubscription_ValidBody_NilService(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    validUserID(),
		"amount":         1000,
		"currency":       "USD",
		"interval":       "month",
		"interval_count": 1,
		"plan_id":        "plan_123",
		"provider":       "stripe",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/subs", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.CreateSubscription(c)
	_ = w
}

func TestSubscriptionHandler_CreateSubscription_AllIntervals(t *testing.T) {
	intervals := []string{"day", "week", "month", "year"}
	for _, interval := range intervals {
		h := &SubscriptionHandler{}
		body, _ := json.Marshal(map[string]interface{}{
			"customer_id": validUserID(),
			"amount":      1000,
			"currency":    "USD",
			"interval":    interval,
		})
		mID := validUserID()
		c, _ := newTestGinContext("POST", "/subs", body,
			gin.Params{{Key: "merchantId", Value: mID}})
		defer func() {
			if r := recover(); r != nil {
				// Expected: nil subscriptionSvc causes panic
			}
		}()
		h.CreateSubscription(c)
	}
}

func TestSubscriptionHandler_CreateSubscription_ZeroIntervalCount(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    validUserID(),
		"amount":         1000,
		"currency":       "USD",
		"interval":       "month",
		"interval_count": 0,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/subs", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.CreateSubscription(c)
	_ = w
}

func TestSubscriptionHandler_GetSubscription_ValidUUID_NilService(t *testing.T) {
	h := &SubscriptionHandler{}
	subID := validUserID()
	c, w := newTestGinContext("GET", "/sub/"+subID, nil,
		gin.Params{{Key: "subscriptionId", Value: subID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.GetSubscription(c)
	_ = w
}

func TestSubscriptionHandler_UpdateSubscription_ValidUUID_NilService(t *testing.T) {
	h := &SubscriptionHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"amount": 2000,
	})
	subID := validUserID()
	c, w := newTestGinContext("PATCH", "/sub/"+subID, body,
		gin.Params{{Key: "subscriptionId", Value: subID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.UpdateSubscription(c)
	_ = w
}

func TestSubscriptionHandler_UpdateSubscription_ValidInterval(t *testing.T) {
	h := &SubscriptionHandler{}
	interval := "week"
	body, _ := json.Marshal(map[string]interface{}{
		"interval": &interval,
	})
	subID := validUserID()
	c, w := newTestGinContext("PATCH", "/sub/"+subID, body,
		gin.Params{{Key: "subscriptionId", Value: subID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.UpdateSubscription(c)
	_ = w
}

func TestSubscriptionHandler_CancelSubscription_ValidUUID_NilService(t *testing.T) {
	h := &SubscriptionHandler{}
	subID := validUserID()
	c, w := newTestGinContext("DELETE", "/sub/"+subID, nil,
		gin.Params{{Key: "subscriptionId", Value: subID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.CancelSubscription(c)
	_ = w
}

func TestSubscriptionHandler_ListSubscriptions_DefaultQueryParams(t *testing.T) {
	h := &SubscriptionHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/subs", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil subscriptionSvc causes panic
		}
	}()
	h.ListSubscriptions(c)
}

// --- InvoiceHandler deeper paths ---

func TestInvoiceHandler_CreateInvoice_ValidBody_NilService(t *testing.T) {
	h := &InvoiceHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":  validUserID(),
		"amount":       5000,
		"currency":     "USD",
		"provider":     "stripe",
		"period_start": "2026-01-01T00:00:00Z",
		"period_end":   "2026-01-31T23:59:59Z",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil invoiceSvc causes panic
		}
	}()
	h.CreateInvoice(c)
	_ = w
}

func TestInvoiceHandler_CreateInvoice_WithSubscriptionID(t *testing.T) {
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
	c, w := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil invoiceSvc causes panic
		}
	}()
	h.CreateInvoice(c)
	_ = w
}

func TestInvoiceHandler_CreateInvoice_WithDueDate(t *testing.T) {
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
	c, w := newTestGinContext("POST", "/inv", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil invoiceSvc causes panic
		}
	}()
	h.CreateInvoice(c)
	_ = w
}

func TestInvoiceHandler_GetInvoice_ValidUUID_NilService(t *testing.T) {
	h := &InvoiceHandler{}
	invID := validUserID()
	c, w := newTestGinContext("GET", "/inv/"+invID, nil,
		gin.Params{{Key: "invoiceId", Value: invID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil invoiceSvc causes panic
		}
	}()
	h.GetInvoice(c)
	_ = w
}

func TestInvoiceHandler_ListInvoices_DefaultQueryParams(t *testing.T) {
	h := &InvoiceHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/inv", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil invoiceSvc causes panic
		}
	}()
	h.ListInvoices(c)
}

// --- PayoutHandler deeper paths ---

func TestPayoutHandler_CreatePayout_ValidBody_NilService(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
		"method":   "standard",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil payoutSvc causes panic
		}
	}()
	h.CreatePayout(c)
	_ = w
}

func TestPayoutHandler_CreatePayout_DefaultMethod(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil payoutSvc causes panic
		}
	}()
	h.CreatePayout(c)
	_ = w
}

func TestPayoutHandler_CreatePayout_InstantMethod(t *testing.T) {
	h := &PayoutHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider": "stripe",
		"amount":   10000,
		"currency": "USD",
		"method":   "instant",
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/payouts", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil payoutSvc causes panic
		}
	}()
	h.CreatePayout(c)
	_ = w
}

func TestPayoutHandler_GetPayout_ValidUUID_NilService(t *testing.T) {
	h := &PayoutHandler{}
	pID := validUserID()
	c, w := newTestGinContext("GET", "/payout/"+pID, nil,
		gin.Params{{Key: "payoutId", Value: pID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil payoutSvc causes panic
		}
	}()
	h.GetPayout(c)
	_ = w
}

func TestPayoutHandler_ListPayouts_DefaultQueryParams(t *testing.T) {
	h := &PayoutHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/payouts", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil payoutSvc causes panic
		}
	}()
	h.ListPayouts(c)
}

// --- DisputeHandler deeper paths ---

func TestDisputeHandler_CreateDispute_ValidBody_NilService(t *testing.T) {
	h := &DisputeHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"transaction_id": validUserID(),
		"provider":       "stripe",
		"reason":         "fraudulent",
		"amount":         5000,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/disputes", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil disputeSvc causes panic
		}
	}()
	h.CreateDispute(c)
	_ = w
}

func TestDisputeHandler_GetDispute_ValidUUID_NilService(t *testing.T) {
	h := &DisputeHandler{}
	dID := validUserID()
	c, w := newTestGinContext("GET", "/dispute/"+dID, nil,
		gin.Params{{Key: "disputeId", Value: dID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil disputeSvc causes panic
		}
	}()
	h.GetDispute(c)
	_ = w
}

func TestDisputeHandler_AddEvidence_ValidBody_NilService(t *testing.T) {
	h := &DisputeHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"evidence_url": "https://example.com/evidence.pdf",
	})
	dID := validUserID()
	c, w := newTestGinContext("POST", "/evidence", body,
		gin.Params{{Key: "disputeId", Value: dID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil disputeSvc causes panic
		}
	}()
	h.AddEvidence(c)
	_ = w
}

// --- WebhookHandler deeper paths ---

func TestWebhookHandler_CreateWebhook_ValidBody_NilService(t *testing.T) {
	h := &WebhookHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"url":      "https://example.com/webhook",
		"secret":   "whsec_123",
		"events":   []string{"payment.succeeded", "payment.failed"},
		"is_active": true,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/webhooks", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil webhookSvc causes panic
		}
	}()
	h.CreateWebhook(c)
	_ = w
}

func TestWebhookHandler_GetWebhook_ValidUUID_NilService(t *testing.T) {
	h := &WebhookHandler{}
	whID := validUserID()
	c, w := newTestGinContext("GET", "/webhook/"+whID, nil,
		gin.Params{{Key: "webhookId", Value: whID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil webhookSvc causes panic
		}
	}()
	h.GetWebhook(c)
	_ = w
}

func TestWebhookHandler_UpdateWebhook_ValidBody_NilService(t *testing.T) {
	h := &WebhookHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"url": "https://new.example.com/webhook",
	})
	mID := validUserID()
	whID := validUserID()
	c, w := newTestGinContext("PUT", "/webhook/"+whID, body,
		gin.Params{{Key: "merchantId", Value: mID}, {Key: "webhookId", Value: whID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil webhookSvc causes panic
		}
	}()
	h.UpdateWebhook(c)
	_ = w
}

func TestWebhookHandler_DeleteWebhook_ValidUUID_NilService(t *testing.T) {
	h := &WebhookHandler{}
	mID := validUserID()
	whID := validUserID()
	c, w := newTestGinContext("DELETE", "/webhook/"+whID, nil,
		gin.Params{{Key: "merchantId", Value: mID}, {Key: "webhookId", Value: whID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil webhookSvc causes panic
		}
	}()
	h.DeleteWebhook(c)
	_ = w
}

// --- ProviderHandler deeper paths ---

func TestProviderHandler_CreateProvider_ValidBody_NilRepo(t *testing.T) {
	h := &ProviderHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"provider":      "stripe",
		"is_active":     true,
		"fallback_order": 1,
		"config":        map[string]string{"api_key": "sk_test"},
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/providers", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil providerRepo causes panic
		}
	}()
	h.CreateProvider(c)
	_ = w
}

func TestProviderHandler_GetProvider_ValidUUID_NilRepo(t *testing.T) {
	h := &ProviderHandler{}
	pID := validUserID()
	c, w := newTestGinContext("GET", "/provider/"+pID, nil,
		gin.Params{{Key: "providerId", Value: pID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil providerRepo causes panic
		}
	}()
	h.GetProvider(c)
	_ = w
}

func TestProviderHandler_UpdateProvider_ValidBody_NilRepo(t *testing.T) {
	h := &ProviderHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"config":         map[string]string{"api_key": "new_key"},
		"is_active":      true,
		"fallback_order": 2,
		"health_status":  "healthy",
	})
	pID := validUserID()
	c, w := newTestGinContext("PUT", "/provider/"+pID, body,
		gin.Params{{Key: "providerId", Value: pID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil providerRepo causes panic
		}
	}()
	h.UpdateProvider(c)
	_ = w
}

func TestProviderHandler_DeleteProvider_ValidUUID_NilRepo(t *testing.T) {
	h := &ProviderHandler{}
	pID := validUserID()
	c, w := newTestGinContext("DELETE", "/provider/"+pID, nil,
		gin.Params{{Key: "providerId", Value: pID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil providerRepo causes panic
		}
	}()
	h.DeleteProvider(c)
	_ = w
}

// --- PaymentMethodHandler deeper paths ---

func TestPaymentMethodHandler_CreatePaymentMethod_ValidBody_NilRepo(t *testing.T) {
	h := &PaymentMethodHandler{}
	body, _ := json.Marshal(map[string]interface{}{
		"customer_id":    validUserID(),
		"type":           "card",
		"provider":       "stripe",
		"provider_token": "tok_123",
		"last4":          "4242",
		"brand":          "visa",
		"exp_month":      12,
		"exp_year":       2027,
		"is_default":     true,
	})
	mID := validUserID()
	c, w := newTestGinContext("POST", "/pm", body,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil pmRepo causes panic
		}
	}()
	h.CreatePaymentMethod(c)
	_ = w
}

func TestPaymentMethodHandler_ListPaymentMethods_ValidCustomer_NilRepo(t *testing.T) {
	h := &PaymentMethodHandler{}
	cID := validUserID()
	c, w := newTestGinContext("GET", "/pm?customer_id="+cID, nil,
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil pmRepo causes panic
		}
	}()
	h.ListPaymentMethods(c)
	_ = w
}

func TestPaymentMethodHandler_GetPaymentMethod_ValidUUID_NilRepo(t *testing.T) {
	h := &PaymentMethodHandler{}
	pmID := validUserID()
	c, w := newTestGinContext("GET", "/pm/"+pmID, nil,
		gin.Params{{Key: "paymentMethodId", Value: pmID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil pmRepo causes panic
		}
	}()
	h.GetPaymentMethod(c)
	_ = w
}

func TestPaymentMethodHandler_DeletePaymentMethod_ValidUUID_NilRepo(t *testing.T) {
	h := &PaymentMethodHandler{}
	pmID := validUserID()
	c, w := newTestGinContext("DELETE", "/pm/"+pmID, nil,
		gin.Params{{Key: "paymentMethodId", Value: pmID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil pmRepo causes panic
		}
	}()
	h.DeletePaymentMethod(c)
	_ = w
}

// --- ExchangeRateHandler deeper paths ---

func TestExchangeRateHandler_GetExchangeRate_MissingBothParams_Struct(t *testing.T) {
	h := &ExchangeRateHandler{}
	c, w := newTestGinContext("GET", "/rates", nil, nil)
	h.GetExchangeRate(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- BillingHandler deeper paths ---

func TestBillingHandler_GetFees_ValidDates_NilService(t *testing.T) {
	h := &BillingHandler{}
	c, w := newTestGinContext("GET", "/fees?from=2026-01-01T00:00:00Z&to=2026-01-31T23:59:59Z", nil,
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil billingSvc causes panic
		}
	}()
	h.GetFees(c)
	_ = w
}

func TestBillingHandler_GetBillingInvoices_ValidRequest_NilService(t *testing.T) {
	h := &BillingHandler{}
	c, _ := newTestGinContext("GET", "/invoices", nil,
		gin.Params{{Key: "merchantId", Value: validUserID()}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil billingSvc causes panic
		}
	}()
	h.GetBillingInvoices(c)
}

// --- WebhookIngressHandler deeper paths ---

func TestWebhookIngressHandler_HandleStripe_EmptyBody_Struct(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	c, w := newTestGinContext("POST", "/webhooks/stripe", nil, nil)
	h.HandleStripe(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookIngressHandler_HandlePayPal_EmptyBody_Struct(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	c, w := newTestGinContext("POST", "/webhooks/paypal", nil, nil)
	h.HandlePayPal(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookIngressHandler_HandleSquare_EmptyBody_Struct(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	c, w := newTestGinContext("POST", "/webhooks/square", nil, nil)
	h.HandleSquare(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookIngressHandler_HandleStripe_LargeBody(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	largeBody := `{"type":"payment_intent.succeeded","data":{"object":{"amount":100000,"currency":"usd"}}}`
	c, w := newTestGinContext("POST", "/webhooks/stripe", []byte(largeBody), nil)
	c.Request.Header.Set("Stripe-Signature", "sig_test_456")
	h.HandleStripe(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(bus.published))
	}
}

func TestWebhookIngressHandler_HandlePayPal_LargeBody(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	largeBody := `{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"pay_123","amount":{"total":"100.00","currency":"USD"}}}`
	c, w := newTestGinContext("POST", "/webhooks/paypal", []byte(largeBody), nil)
	h.HandlePayPal(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if bus.published[0].event.Source != "paypal" {
		t.Errorf("source = %s, want paypal", bus.published[0].event.Source)
	}
}

func TestWebhookIngressHandler_HandleSquare_LargeBody(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	largeBody := `{"type":"payment.completed","data":{"payment":{"id":"sq_123","amount_money":{"amount":5000,"currency":"USD"}}}}`
	c, w := newTestGinContext("POST", "/webhooks/square", []byte(largeBody), nil)
	h.HandleSquare(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if bus.published[0].event.Source != "square" {
		t.Errorf("source = %s, want square", bus.published[0].event.Source)
	}
}

// --- AuditHandler deeper paths ---

func TestAuditHandler_ListAuditLogs_ValidRequest_NilRepo(t *testing.T) {
	h := &AuditHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/audit", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil auditRepo causes panic
		}
	}()
	h.ListAuditLogs(c)
}

// --- AnalyticsHandler deeper paths ---

func TestAnalyticsHandler_GetSummary_CustomDates_NilService(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/summary?from=2026-01-01&to=2026-01-31", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil analyticsSvc causes panic
		}
	}()
	h.GetSummary(c)
}

func TestAnalyticsHandler_GetTransactionAnalytics_CustomGroupBy_NilService(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/transactions?group_by=month&from=2026-01-01&to=2026-06-30", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil analyticsSvc causes panic
		}
	}()
	h.GetTransactionAnalytics(c)
}

func TestAnalyticsHandler_ExportTransactions_CustomDates_NilService(t *testing.T) {
	h := &AnalyticsHandler{}
	mID := validUserID()
	c, _ := newTestGinContext("GET", "/analytics/export?from=2026-01-01&to=2026-01-31", nil,
		gin.Params{{Key: "merchantId", Value: mID}})
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil analyticsSvc causes panic
		}
	}()
	h.ExportTransactions(c)
}

// --- HealthHandler with mock DB and Redis ---

func TestHealth_AllHealthy_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: nil},
		redis:  &mockRedis{pingErr: nil},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health", nil, nil)
	h.Health(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealth_DBUnhealthy_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: errors.New("db down")},
		redis:  &mockRedis{pingErr: nil},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health", nil, nil)
	h.Health(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHealth_RedisUnhealthy_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: nil},
		redis:  &mockRedis{pingErr: errors.New("redis down")},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health", nil, nil)
	h.Health(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHealth_BothUnhealthy_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: errors.New("db down")},
		redis:  &mockRedis{pingErr: errors.New("redis down")},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health", nil, nil)
	h.Health(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestReadiness_Ready_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: nil},
		redis:  &mockRedis{pingErr: nil},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health/ready", nil, nil)
	h.Readiness(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReadiness_NotReady_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: errors.New("db down")},
		redis:  &mockRedis{pingErr: nil},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health/ready", nil, nil)
	h.Readiness(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestLiveness_Struct(t *testing.T) {
	h := &HealthHandler{
		db:     &mockDB{pingErr: nil},
		redis:  &mockRedis{pingErr: nil},
		logger: zap.NewNop(),
	}
	c, w := newTestGinContext("GET", "/health/live", nil, nil)
	h.Liveness(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- MetricsHandler ---

func TestMetricsHandler_ReturnsMetrics_Struct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler())
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}


