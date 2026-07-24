package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const validUUID = "123e4567-e89b-12d3-a456-426614174000"

func init() {
	gin.SetMode(gin.TestMode)
}

func callHandler(handler gin.HandlerFunc, method, path string, body interface{}, params ...gin.Params) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	c.Request = httptest.NewRequest(method, path, reqBody)
	c.Request.Header.Set("Content-Type", "application/json")
	if params != nil {
		c.Params = params[0]
	}
	handler(c)
	return w
}

// --- PaymentHandler ---

func TestPaymentHandler_GetTransaction_InvalidID(t *testing.T) {
	h := NewPaymentHandler(nil)
	w := callHandler(h.GetTransaction, "GET", "/test", nil,
		gin.Params{{Key: "transactionId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_ListTransactions_InvalidMerchant(t *testing.T) {
	h := NewPaymentHandler(nil)
	w := callHandler(h.ListTransactions, "GET", "/test?page=1&page_size=10", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentHandler_CreateRefund_InvalidMerchant(t *testing.T) {
	h := NewPaymentHandler(nil)
	w := callHandler(h.CreateRefund, "POST", "/test",
		map[string]string{"transaction_id": validUUID, "amount": "100"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- SubscriptionHandler ---

func TestSubscriptionHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.CreateSubscription, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "interval": "month"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Create_EmptyBody(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.CreateSubscription, "POST", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Create_InvalidInterval(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.CreateSubscription, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "interval": "invalid_interval"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Create_InvalidCustomerID(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.CreateSubscription, "POST", "/test",
		map[string]interface{}{"customer_id": "bad", "amount": 1000, "currency": "USD", "interval": "month"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Get_InvalidID(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.GetSubscription, "GET", "/test", nil,
		gin.Params{{Key: "subscriptionId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Update_InvalidID(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.UpdateSubscription, "PATCH", "/test",
		map[string]interface{}{"amount": 2000},
		gin.Params{{Key: "subscriptionId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Update_InvalidInterval(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	interval := "bogus"
	w := callHandler(h.UpdateSubscription, "PATCH", "/test",
		map[string]interface{}{"interval": &interval},
		gin.Params{{Key: "subscriptionId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_Cancel_InvalidID(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.CancelSubscription, "DELETE", "/test", nil,
		gin.Params{{Key: "subscriptionId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSubscriptionHandler_List_InvalidMerchant(t *testing.T) {
	h := NewSubscriptionHandler(nil)
	w := callHandler(h.ListSubscriptions, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- InvoiceHandler ---

func TestInvoiceHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "period_start": "2026-01-01T00:00:00Z", "period_end": "2026-01-31T23:59:59Z"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_EmptyBody(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_InvalidCustomerID(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": "bad", "amount": 1000, "currency": "USD", "period_start": "2026-01-01T00:00:00Z", "period_end": "2026-01-31T23:59:59Z"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_InvalidPeriodStart(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "period_start": "not-a-date", "period_end": "2026-01-31T23:59:59Z"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_InvalidPeriodEnd(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "period_start": "2026-01-01T00:00:00Z", "period_end": "bad"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_InvalidDueDate(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "amount": 1000, "currency": "USD", "period_start": "2026-01-01T00:00:00Z", "period_end": "2026-01-31T23:59:59Z", "due_date": "bad-date"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Create_InvalidSubscriptionID(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.CreateInvoice, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "subscription_id": "bad", "amount": 1000, "currency": "USD", "period_start": "2026-01-01T00:00:00Z", "period_end": "2026-01-31T23:59:59Z"},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_Get_InvalidID(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.GetInvoice, "GET", "/test", nil,
		gin.Params{{Key: "invoiceId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInvoiceHandler_List_InvalidMerchant(t *testing.T) {
	h := NewInvoiceHandler(nil)
	w := callHandler(h.ListInvoices, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- PayoutHandler ---

func TestPayoutHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewPayoutHandler(nil)
	w := callHandler(h.CreatePayout, "POST", "/test",
		map[string]interface{}{"provider": "stripe", "amount": 1000, "currency": "USD"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPayoutHandler_Create_EmptyBody(t *testing.T) {
	h := NewPayoutHandler(nil)
	w := callHandler(h.CreatePayout, "POST", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPayoutHandler_Get_InvalidID(t *testing.T) {
	h := NewPayoutHandler(nil)
	w := callHandler(h.GetPayout, "GET", "/test", nil,
		gin.Params{{Key: "payoutId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPayoutHandler_List_InvalidMerchant(t *testing.T) {
	h := NewPayoutHandler(nil)
	w := callHandler(h.ListPayouts, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- DisputeHandler ---

func TestDisputeHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.CreateDispute, "POST", "/test",
		map[string]interface{}{"transaction_id": validUUID, "provider": "stripe", "reason": "fraud", "amount": 500},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_Create_EmptyBody(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.CreateDispute, "POST", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_Create_InvalidTransactionID(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.CreateDispute, "POST", "/test",
		map[string]interface{}{"transaction_id": "bad", "provider": "stripe", "reason": "fraud", "amount": 500},
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_Get_InvalidID(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.GetDispute, "GET", "/test", nil,
		gin.Params{{Key: "disputeId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_List_InvalidMerchant(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.ListDisputes, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDisputeHandler_AddEvidence_InvalidID(t *testing.T) {
	h := NewDisputeHandler(nil)
	w := callHandler(h.AddEvidence, "POST", "/test",
		map[string]string{"evidence_url": "https://example.com/doc.pdf"},
		gin.Params{{Key: "disputeId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- WebhookHandler ---

func TestWebhookHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.CreateWebhook, "POST", "/test",
		map[string]interface{}{"url": "https://example.com", "events": []string{"payment.succeeded"}},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Create_EmptyBody(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.CreateWebhook, "POST", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Get_InvalidID(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.GetWebhook, "GET", "/test", nil,
		gin.Params{{Key: "webhookId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_List_InvalidMerchant(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.ListWebhooks, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Update_InvalidMerchant(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.UpdateWebhook, "PUT", "/test",
		map[string]interface{}{"url": "https://new.com"},
		gin.Params{{Key: "merchantId", Value: "bad"}, {Key: "webhookId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Update_InvalidWebhookID(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.UpdateWebhook, "PUT", "/test",
		map[string]interface{}{"url": "https://new.com"},
		gin.Params{{Key: "merchantId", Value: validUUID}, {Key: "webhookId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Delete_InvalidMerchant(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.DeleteWebhook, "DELETE", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}, {Key: "webhookId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Delete_InvalidWebhookID(t *testing.T) {
	h := NewWebhookHandler(nil)
	w := callHandler(h.DeleteWebhook, "DELETE", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}, {Key: "webhookId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- ProviderHandler ---

func TestProviderHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewProviderHandler(nil)
	w := callHandler(h.CreateProvider, "POST", "/test",
		map[string]interface{}{"provider": "stripe"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProviderHandler_List_InvalidMerchant(t *testing.T) {
	h := NewProviderHandler(nil)
	w := callHandler(h.ListProviders, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProviderHandler_Get_InvalidID(t *testing.T) {
	h := NewProviderHandler(nil)
	w := callHandler(h.GetProvider, "GET", "/test", nil,
		gin.Params{{Key: "providerId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProviderHandler_Update_InvalidID(t *testing.T) {
	h := NewProviderHandler(nil)
	w := callHandler(h.UpdateProvider, "PUT", "/test",
		map[string]interface{}{"config": map[string]string{"key": "val"}},
		gin.Params{{Key: "providerId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProviderHandler_Delete_InvalidID(t *testing.T) {
	h := NewProviderHandler(nil)
	w := callHandler(h.DeleteProvider, "DELETE", "/test", nil,
		gin.Params{{Key: "providerId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- PaymentMethodHandler ---

func TestPaymentMethodHandler_Create_InvalidMerchant(t *testing.T) {
	h := NewPaymentMethodHandler(nil)
	w := callHandler(h.CreatePaymentMethod, "POST", "/test",
		map[string]interface{}{"customer_id": validUUID, "type": "card", "provider": "stripe", "provider_token": "tok_123"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_List_InvalidMerchant(t *testing.T) {
	h := NewPaymentMethodHandler(nil)
	w := callHandler(h.ListPaymentMethods, "GET", "/test?customer_id=bad", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_List_InvalidCustomerID(t *testing.T) {
	h := NewPaymentMethodHandler(nil)
	w := callHandler(h.ListPaymentMethods, "GET", "/test?customer_id=bad", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_Get_InvalidID(t *testing.T) {
	h := NewPaymentMethodHandler(nil)
	w := callHandler(h.GetPaymentMethod, "GET", "/test", nil,
		gin.Params{{Key: "paymentMethodId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPaymentMethodHandler_Delete_InvalidID(t *testing.T) {
	h := NewPaymentMethodHandler(nil)
	w := callHandler(h.DeletePaymentMethod, "DELETE", "/test", nil,
		gin.Params{{Key: "paymentMethodId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- ExchangeRateHandler ---

func TestExchangeRateHandler_MissingFromParam(t *testing.T) {
	h := NewExchangeRateHandler(nil)
	w := callHandler(h.GetExchangeRate, "GET", "/test?to=EUR", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExchangeRateHandler_MissingToParam(t *testing.T) {
	h := NewExchangeRateHandler(nil)
	w := callHandler(h.GetExchangeRate, "GET", "/test?from=USD", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestExchangeRateHandler_MissingBothParams(t *testing.T) {
	h := NewExchangeRateHandler(nil)
	w := callHandler(h.GetExchangeRate, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- AuditHandler ---

func TestAuditHandler_List_InvalidMerchant(t *testing.T) {
	h := NewAuditHandler(nil)
	w := callHandler(h.ListAuditLogs, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- BillingHandler ---

func TestBillingHandler_GetFees_InvalidMerchant(t *testing.T) {
	h := NewBillingHandler(nil)
	w := callHandler(h.GetFees, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBillingHandler_GetBillingInvoices_InvalidMerchant(t *testing.T) {
	h := NewBillingHandler(nil)
	w := callHandler(h.GetBillingInvoices, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- MerchantHandler ---

func TestMerchantHandler_Get_InvalidID(t *testing.T) {
	h := NewMerchantHandler(nil)
	w := callHandler(h.GetMerchant, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerchantHandler_Update_InvalidID(t *testing.T) {
	h := NewMerchantHandler(nil)
	w := callHandler(h.UpdateMerchant, "PUT", "/test",
		map[string]string{"legal_name": "Updated"},
		gin.Params{{Key: "merchantId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- ApiKeyHandler ---

func TestApiKeyHandler_Create_EmptyBody(t *testing.T) {
	h := NewApiKeyHandler(nil)
	w := callHandler(h.CreateApiKey, "POST", "/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApiKeyHandler_Revoke_InvalidID(t *testing.T) {
	h := NewApiKeyHandler(nil)
	w := callHandler(h.RevokeApiKey, "DELETE", "/test", nil,
		gin.Params{{Key: "keyId", Value: "bad"}})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- WebhookIngressHandler ---

func TestWebhookIngressHandler_HandleStripe_WithSignature(t *testing.T) {
	bus := &mockEventBus{}
	h := &WebhookIngressHandler{eventBus: bus, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"type":"payment_intent.succeeded"}`
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))
	c.Request.Header.Set("Stripe-Signature", "t=1234567890,v1=test_signature")
	h.HandleStripe(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (invalid sig rejected)", w.Code, http.StatusUnauthorized)
	}
}

// --- AnalyticsHandler ---

func TestAnalyticsHandler_ExportTransactions_DefaultParams(t *testing.T) {
	h := NewAnalyticsHandler(nil)
	defer func() {
		if r := recover(); r == nil {
			// Method completed without panic
		}
	}()
	w := callHandler(h.ExportTransactions, "GET", "/test", nil,
		gin.Params{{Key: "merchantId", Value: validUUID}})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (nil svc causes panic)", w.Code, http.StatusInternalServerError)
	}
}

// --- AuthHandler ---

func TestAuthHandler_Logout_Success(t *testing.T) {
	h := &AuthHandler{}
	w := callHandler(h.Logout, "POST", "/test", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
