package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCustomer_JSON(t *testing.T) {
	c := Customer{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		Email:      "test@example.com",
		Name:       "John Doe",
		Phone:      "+1234567890",
		ExternalID: "ext-123",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var c2 Customer
	if err := json.Unmarshal(data, &c2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if c2.Email != c.Email {
		t.Errorf("email = %q, want %q", c2.Email, c.Email)
	}
	if c2.Name != c.Name {
		t.Errorf("name = %q, want %q", c2.Name, c.Name)
	}
}

func TestSubscription_JSON(t *testing.T) {
	s := Subscription{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		CustomerID: uuid.New(),
		Amount:     1000,
		Currency:   "USD",
		Interval:   SubscriptionIntervalMonth,
		Status:     SubscriptionStatusActive,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var s2 Subscription
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if s2.Amount != 1000 {
		t.Errorf("amount = %d, want 1000", s2.Amount)
	}
	if s2.Interval != SubscriptionIntervalMonth {
		t.Errorf("interval = %q, want %q", s2.Interval, SubscriptionIntervalMonth)
	}
}

func TestSubscription_StatusConstants(t *testing.T) {
	statuses := []SubscriptionStatus{
		SubscriptionStatusActive,
		SubscriptionStatusCancelled,
		SubscriptionStatusPastDue,
		SubscriptionStatusTrialing,
		SubscriptionStatusUnpaid,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestInvoice_JSON(t *testing.T) {
	inv := Invoice{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		CustomerID: uuid.New(),
		Amount:     5000,
		Currency:   "EUR",
		Status:     InvoiceStatusOpen,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var inv2 Invoice
	if err := json.Unmarshal(data, &inv2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if inv2.Amount != 5000 {
		t.Errorf("amount = %d, want 5000", inv2.Amount)
	}
}

func TestInvoice_StatusConstants(t *testing.T) {
	statuses := []InvoiceStatus{
		InvoiceStatusDraft,
		InvoiceStatusOpen,
		InvoiceStatusPaid,
		InvoiceStatusVoid,
		InvoiceStatusUncollectible,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestPayout_JSON(t *testing.T) {
	p := Payout{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		Amount:     10000,
		Currency:   "USD",
		Status:     PayoutStatusPending,
		Method:     PayoutMethodStandard,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var p2 Payout
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if p2.Amount != 10000 {
		t.Errorf("amount = %d, want 10000", p2.Amount)
	}
}

func TestPayout_MethodConstants(t *testing.T) {
	methods := []PayoutMethod{
		PayoutMethodStandard,
		PayoutMethodInstant,
	}

	for _, m := range methods {
		if m == "" {
			t.Error("method should not be empty")
		}
	}
}

func TestDispute_JSON(t *testing.T) {
	d := Dispute{
		ID:            uuid.New(),
		MerchantID:    uuid.New(),
		TransactionID: uuid.New(),
		Amount:        500,
		Reason:        "fraudulent",
		Status:        DisputeStatusUnderReview,
		CreatedAt:     time.Now(),
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var d2 Dispute
	if err := json.Unmarshal(data, &d2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if d2.Amount != 500 {
		t.Errorf("amount = %d, want 500", d2.Amount)
	}
}

func TestDispute_StatusConstants(t *testing.T) {
	statuses := []DisputeStatus{
		DisputeStatusWarningNeedsResponse,
		DisputeStatusUnderReview,
		DisputeStatusWon,
		DisputeStatusLost,
		DisputeStatusClosed,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestWebhookConfig_JSON(t *testing.T) {
	wc := WebhookConfig{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		URL:        "https://example.com/webhook",
		Events:     []string{"payment.succeeded", "payment.failed"},
		IsActive:   true,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var wc2 WebhookConfig
	if err := json.Unmarshal(data, &wc2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if wc2.URL != "https://example.com/webhook" {
		t.Errorf("url = %q, want %q", wc2.URL, "https://example.com/webhook")
	}
	if len(wc2.Events) != 2 {
		t.Errorf("events length = %d, want 2", len(wc2.Events))
	}
}

func TestPaymentMethod_JSON(t *testing.T) {
	pm := PaymentMethod{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		CustomerID: uuid.New(),
		Type:       PaymentMethodTypeCard,
		Last4:      "4242",
		Brand:      "visa",
		IsDefault:  true,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(pm)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var pm2 PaymentMethod
	if err := json.Unmarshal(data, &pm2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if pm2.Last4 != "4242" {
		t.Errorf("last4 = %q, want %q", pm2.Last4, "4242")
	}
	if pm2.Brand != "visa" {
		t.Errorf("brand = %q, want %q", pm2.Brand, "visa")
	}
}

func TestPaymentMethod_TypeConstants(t *testing.T) {
	types := []PaymentMethodType{
		PaymentMethodTypeCard,
		PaymentMethodTypeBankAccount,
		PaymentMethodTypeWallet,
	}

	for _, mt := range types {
		if mt == "" {
			t.Error("type should not be empty")
		}
	}
}

func TestProviderConfig_JSON(t *testing.T) {
	configJSON, _ := json.Marshal(map[string]interface{}{"api_key": "sk_test"})
	pc := ProviderConfig{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		Provider:   "stripe",
		IsActive:   true,
		Config:     configJSON,
		CreatedAt:  time.Now(),
	}

	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var pc2 ProviderConfig
	if err := json.Unmarshal(data, &pc2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if pc2.Provider != "stripe" {
		t.Errorf("provider = %q, want %q", pc2.Provider, "stripe")
	}
}

func TestProviderConfig_HealthStatusConstants(t *testing.T) {
	statuses := []HealthStatus{
		HealthStatusHealthy,
		HealthStatusDegraded,
		HealthStatusUnhealthy,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestPaginationParams_Normalize(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		wantPage int
		wantSize int
	}{
		{"zero defaults", 0, 0, 1, 20},
		{"negative page", -1, 10, 1, 10},
		{"too large page size", 1, 200, 1, 100},
		{"valid", 3, 50, 3, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			p.Normalize()
			if p.Page != tt.wantPage {
				t.Errorf("page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.PageSize != tt.wantSize {
				t.Errorf("pageSize = %d, want %d", p.PageSize, tt.wantSize)
			}
		})
	}
}

func TestPaginationParams_Offset(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		want     int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 10, 20},
	}

	for _, tt := range tests {
		p := PaginationParams{Page: tt.page, PageSize: tt.pageSize}
		if got := p.Offset(); got != tt.want {
			t.Errorf("Offset(%d, %d) = %d, want %d", tt.page, tt.pageSize, got, tt.want)
		}
	}
}

func TestPaginatedResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	resp := NewPaginatedResponse(data, 1, 10, 25)

	if resp.Page != 1 {
		t.Errorf("page = %d, want 1", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("pageSize = %d, want 10", resp.PageSize)
	}
	if resp.Total != 25 {
		t.Errorf("total = %d, want 25", resp.Total)
	}
	if resp.TotalPages != 3 {
		t.Errorf("totalPages = %d, want 3", resp.TotalPages)
	}
}

func TestPaginatedResponse_ExactDivision(t *testing.T) {
	resp := NewPaginatedResponse([]string{}, 1, 10, 20)
	if resp.TotalPages != 2 {
		t.Errorf("totalPages = %d, want 2", resp.TotalPages)
	}
}

func TestIdempotencyKey_JSON(t *testing.T) {
	responseJSON, _ := json.Marshal(map[string]string{"status": "ok"})
	ik := IdempotencyKey{
		ID:         1,
		KeyHash:    "abc123",
		Response:   responseJSON,
		StatusCode: 200,
		MerchantID: uuid.New(),
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	data, err := json.Marshal(ik)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var ik2 IdempotencyKey
	if err := json.Unmarshal(data, &ik2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if ik2.KeyHash != "abc123" {
		t.Errorf("keyHash = %q, want %q", ik2.KeyHash, "abc123")
	}
}

func TestBackgroundTask_JSON(t *testing.T) {
	bt := BackgroundTask{
		ID:          uuid.New(),
		Type:        "reconciliation",
		Status:      TaskStatusPending,
		Priority:    1,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(bt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var bt2 BackgroundTask
	if err := json.Unmarshal(data, &bt2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bt2.Type != "reconciliation" {
		t.Errorf("type = %q, want %q", bt2.Type, "reconciliation")
	}
}

func TestBackgroundTask_StatusConstants(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusDead,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestAuditLog_JSON(t *testing.T) {
	al := AuditLog{
		ID:           uuid.New(),
		MerchantID:   uuid.New(),
		ActorID:      uuid.New(),
		ActorType:    ActorTypeRootAdmin,
		Action:       "merchant.update",
		ResourceType: "merchant",
		ResourceID:   "123",
		CreatedAt:    time.Now(),
	}

	data, err := json.Marshal(al)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var al2 AuditLog
	if err := json.Unmarshal(data, &al2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if al2.Action != "merchant.update" {
		t.Errorf("action = %q, want %q", al2.Action, "merchant.update")
	}
}

func TestAuditLog_ActorTypeConstants(t *testing.T) {
	types := []ActorType{
		ActorTypeRootAdmin,
		ActorTypeAccountAdmin,
		ActorTypeUser,
		ActorTypeSystem,
		ActorTypeAPIKey,
	}

	for _, at := range types {
		if at == "" {
			t.Error("actor type should not be empty")
		}
	}
}

func TestExchangeRate_JSON(t *testing.T) {
	er := ExchangeRate{
		ID:            1,
		BaseCurrency:  "USD",
		QuoteCurrency: "EUR",
		Rate:          "1.1234",
		Source:        "frankfurter",
		FetchedAt:     time.Now(),
	}

	data, err := json.Marshal(er)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var er2 ExchangeRate
	if err := json.Unmarshal(data, &er2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if er2.Rate != "1.1234" {
		t.Errorf("rate = %q, want %q", er2.Rate, "1.1234")
	}
}
