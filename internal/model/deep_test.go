package model

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPaginationParams_Normalize_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		wantPage int
		wantSize int
	}{
		{"both zero", 0, 0, 1, 20},
		{"negative page negative size", -5, -10, 1, 20},
		{"page one size one", 1, 1, 1, 1},
		{"page one size exactly 100", 1, 100, 1, 100},
		{"page one size 101", 1, 101, 1, 100},
		{"page one size 999", 1, 999, 1, 100},
		{"valid page 50 size 50", 50, 50, 50, 50},
		{"page 1000 size 10", 1000, 10, 1000, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			p.Normalize()
			if p.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.PageSize != tt.wantSize {
				t.Errorf("PageSize = %d, want %d", p.PageSize, tt.wantSize)
			}
		})
	}
}

func TestPaginationParams_Offset_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		want     int
	}{
		{"page 1 size 1", 1, 1, 0},
		{"page 2 size 1", 2, 1, 1},
		{"page 1 size 100", 1, 100, 0},
		{"page 10 size 100", 10, 100, 900},
		{"page 100 size 1", 100, 1, 99},
		{"page 5 size 25", 5, 25, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			if got := p.Offset(); got != tt.want {
				t.Errorf("Offset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPaginationParams_Offset_AfterNormalize(t *testing.T) {
	p := PaginationParams{Page: -1, PageSize: -1}
	p.Normalize()
	if got := p.Offset(); got != 0 {
		t.Errorf("Offset() after normalize = %d, want 0", got)
	}
}

func TestPaginationParams_JSON(t *testing.T) {
	p := PaginationParams{Page: 3, PageSize: 25}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var p2 PaginationParams
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if p2.Page != 3 || p2.PageSize != 25 {
		t.Errorf("got page=%d size=%d, want 3, 25", p2.Page, p2.PageSize)
	}
}

func TestPaginatedResponse_JSON(t *testing.T) {
	resp := NewPaginatedResponse([]string{"a", "b"}, 2, 10, 25)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var resp2 PaginatedResponse
	if err := json.Unmarshal(data, &resp2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp2.Page != 2 || resp2.PageSize != 10 || resp2.Total != 25 || resp2.TotalPages != 3 {
		t.Errorf("got page=%d size=%d total=%d pages=%d, want 2,10,25,3", resp2.Page, resp2.PageSize, resp2.Total, resp2.TotalPages)
	}
}

func TestNewPaginatedResponse_ZeroTotal(t *testing.T) {
	resp := NewPaginatedResponse([]string{}, 1, 10, 0)
	if resp.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0 for zero total", resp.TotalPages)
	}
}

func TestNewPaginatedResponse_ExactDivision(t *testing.T) {
	tests := []struct {
		total     int64
		pageSize  int
		wantPages int
	}{
		{10, 10, 1},
		{20, 10, 2},
		{100, 10, 10},
		{0, 10, 0},
		{1, 1, 1},
		{30, 5, 6},
	}

	for _, tt := range tests {
		resp := NewPaginatedResponse([]string{}, 1, tt.pageSize, tt.total)
		if resp.TotalPages != tt.wantPages {
			t.Errorf("TotalPages for total=%d pageSize=%d = %d, want %d", tt.total, tt.pageSize, resp.TotalPages, tt.wantPages)
		}
	}
}

func TestNewPaginatedResponse_Remainder(t *testing.T) {
	tests := []struct {
		total     int64
		pageSize  int
		wantPages int
	}{
		{11, 10, 2},
		{21, 10, 3},
		{1, 10, 1},
		{15, 7, 3},
	}

	for _, tt := range tests {
		resp := NewPaginatedResponse([]string{}, 1, tt.pageSize, tt.total)
		if resp.TotalPages != tt.wantPages {
			t.Errorf("TotalPages for total=%d pageSize=%d = %d, want %d", tt.total, tt.pageSize, resp.TotalPages, tt.wantPages)
		}
	}
}

func TestBackgroundTask_AllStatusConstants(t *testing.T) {
	statuses := map[TaskStatus]string{
		TaskStatusPending:   "pending",
		TaskStatusRunning:   "running",
		TaskStatusCompleted: "completed",
		TaskStatusFailed:    "failed",
		TaskStatusDead:      "dead",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("TaskStatus = %q, want %q", status, expected)
		}
	}
}

func TestBackgroundTask_Fields(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	nextRun := now.Add(time.Hour)
	lockedAt := now.Add(time.Minute)

	bt := BackgroundTask{
		ID:          uuid.New(),
		Type:        "reconciliation",
		Payload:     json.RawMessage(`{"merchant_id":"abc"}`),
		Status:      TaskStatusRunning,
		Priority:    5,
		Attempts:    2,
		MaxAttempts: 5,
		LastError:   "timeout",
		NextRunAt:   &nextRun,
		LockedBy:    "worker-1",
		LockedAt:    &lockedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if bt.Priority != 5 {
		t.Errorf("Priority = %d, want 5", bt.Priority)
	}
	if bt.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", bt.Attempts)
	}
	if bt.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", bt.MaxAttempts)
	}
	if bt.LastError != "timeout" {
		t.Errorf("LastError = %q, want %q", bt.LastError, "timeout")
	}
	if bt.LockedBy != "worker-1" {
		t.Errorf("LockedBy = %q, want %q", bt.LockedBy, "worker-1")
	}
}

func TestBackgroundTask_JSON_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	nextRun := now.Add(time.Hour)

	bt := BackgroundTask{
		ID:          uuid.New(),
		Type:        "export",
		Payload:     json.RawMessage(`{"format":"csv"}`),
		Status:      TaskStatusPending,
		Priority:    1,
		Attempts:    0,
		MaxAttempts: 3,
		NextRunAt:   &nextRun,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(bt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var bt2 BackgroundTask
	if err := json.Unmarshal(data, &bt2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bt2.Type != "export" {
		t.Errorf("Type = %q, want %q", bt2.Type, "export")
	}
	if bt2.Status != TaskStatusPending {
		t.Errorf("Status = %q, want %q", bt2.Status, TaskStatusPending)
	}
	if bt2.Priority != 1 {
		t.Errorf("Priority = %d, want 1", bt2.Priority)
	}
	if bt2.NextRunAt == nil {
		t.Error("NextRunAt should not be nil")
	}
}

func TestBackgroundTask_NilOptionalFields(t *testing.T) {
	bt := BackgroundTask{
		ID:     uuid.New(),
		Type:   "test",
		Status: TaskStatusPending,
	}

	data, err := json.Marshal(bt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var bt2 BackgroundTask
	if err := json.Unmarshal(data, &bt2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bt2.NextRunAt != nil {
		t.Error("NextRunAt should be nil")
	}
	if bt2.LockedAt != nil {
		t.Error("LockedAt should be nil")
	}
}

func TestExchangeRate_Fields(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	er := ExchangeRate{
		ID:            42,
		BaseCurrency:  "USD",
		QuoteCurrency: "EUR",
		Rate:          "0.9234",
		Source:        "frankfurter",
		FetchedAt:     now,
		ExpiresAt:     now.Add(time.Hour),
	}

	if er.ID != 42 {
		t.Errorf("ID = %d, want 42", er.ID)
	}
	if er.BaseCurrency != "USD" {
		t.Errorf("BaseCurrency = %q, want %q", er.BaseCurrency, "USD")
	}
	if er.QuoteCurrency != "EUR" {
		t.Errorf("QuoteCurrency = %q, want %q", er.QuoteCurrency, "EUR")
	}
	if er.Rate != "0.9234" {
		t.Errorf("Rate = %q, want %q", er.Rate, "0.9234")
	}
	if er.Source != "frankfurter" {
		t.Errorf("Source = %q, want %q", er.Source, "frankfurter")
	}
}

func TestExchangeRate_JSON_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	er := ExchangeRate{
		ID:            1,
		BaseCurrency:  "GBP",
		QuoteCurrency: "JPY",
		Rate:          "185.12",
		Source:        "ecb",
		FetchedAt:     now,
		ExpiresAt:     now.Add(2 * time.Hour),
	}

	data, err := json.Marshal(er)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var er2 ExchangeRate
	if err := json.Unmarshal(data, &er2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if er2.BaseCurrency != "GBP" {
		t.Errorf("BaseCurrency = %q, want %q", er2.BaseCurrency, "GBP")
	}
	if er2.Rate != "185.12" {
		t.Errorf("Rate = %q, want %q", er2.Rate, "185.12")
	}
}

func TestIdempotencyKey_Fields(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	ik := IdempotencyKey{
		ID:         99,
		KeyHash:    "sha256hash",
		Response:   json.RawMessage(`{"status":"created"}`),
		StatusCode: 201,
		MerchantID: uuid.New(),
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}

	if ik.ID != 99 {
		t.Errorf("ID = %d, want 99", ik.ID)
	}
	if ik.StatusCode != 201 {
		t.Errorf("StatusCode = %d, want 201", ik.StatusCode)
	}
}

func TestIdempotencyKey_JSON_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	respJSON, _ := json.Marshal(map[string]string{"id": "txn_123"})
	ik := IdempotencyKey{
		ID:         1,
		KeyHash:    "hash123",
		Response:   respJSON,
		StatusCode: 200,
		MerchantID: uuid.New(),
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}

	data, err := json.Marshal(ik)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var ik2 IdempotencyKey
	if err := json.Unmarshal(data, &ik2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if ik2.KeyHash != "hash123" {
		t.Errorf("KeyHash = %q, want %q", ik2.KeyHash, "hash123")
	}
	if ik2.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", ik2.StatusCode)
	}

	var resp map[string]string
	if err := json.Unmarshal(ik2.Response, &resp); err != nil {
		t.Fatalf("Response unmarshal failed: %v", err)
	}
	if resp["id"] != "txn_123" {
		t.Errorf("Response.id = %q, want %q", resp["id"], "txn_123")
	}
}

func TestAppError_SentinelErrors_AllFields(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantCode   string
		wantMsg    string
		wantStatus int
	}{
		{"ErrNotFound", ErrNotFound, "NOT_FOUND", "resource not found", http.StatusNotFound},
		{"ErrUnauthorized", ErrUnauthorized, "UNAUTHORIZED", "unauthorized", http.StatusUnauthorized},
		{"ErrForbidden", ErrForbidden, "FORBIDDEN", "forbidden", http.StatusForbidden},
		{"ErrConflict", ErrConflict, "CONFLICT", "resource conflict", http.StatusConflict},
		{"ErrValidation", ErrValidation, "VALIDATION_ERROR", "validation failed", http.StatusUnprocessableEntity},
		{"ErrInternal", ErrInternal, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError},
		{"ErrRateLimited", ErrRateLimited, "RATE_LIMITED", "rate limit exceeded", http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.wantMsg)
			}
			if tt.err.HTTPStatus != tt.wantStatus {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.wantStatus)
			}
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestAppError_Error_ImplementsErrorInterface(t *testing.T) {
	var err error = &AppError{}
	_ = err
}

func TestNewNotFoundError_Messages(t *testing.T) {
	tests := []struct {
		resource string
		want     string
	}{
		{"user", "user not found"},
		{"merchant", "merchant not found"},
		{"transaction", "transaction not found"},
		{"", " not found"},
		{"API Key", "API Key not found"},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			err := NewNotFoundError(tt.resource)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
			if err.HTTPStatus != http.StatusNotFound {
				t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusNotFound)
			}
		})
	}
}

func TestNewValidationError_Messages(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"email required", "email is required"},
		{"invalid amount", "amount must be positive"},
		{"empty string", ""},
		{"unicode", "金额必须为正数"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.message)
			if err.Error() != tt.message {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.message)
			}
			if err.Code != "VALIDATION_ERROR" {
				t.Errorf("Code = %q, want %q", err.Code, "VALIDATION_ERROR")
			}
			if err.HTTPStatus != http.StatusUnprocessableEntity {
				t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestNewConflictError_Messages(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"email exists", "email already exists"},
		{"duplicate key", "duplicate key value"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConflictError(tt.message)
			if err.Error() != tt.message {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.message)
			}
			if err.Code != "CONFLICT" {
				t.Errorf("Code = %q, want %q", err.Code, "CONFLICT")
			}
			if err.HTTPStatus != http.StatusConflict {
				t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusConflict)
			}
		})
	}
}

func TestAppError_JSON_Serialization(t *testing.T) {
	err := NewValidationError("field is required")
	data, err2 := json.Marshal(err)
	if err2 != nil {
		t.Fatalf("marshal failed: %v", err2)
	}

	var decoded map[string]interface{}
	if err3 := json.Unmarshal(data, &decoded); err3 != nil {
		t.Fatalf("unmarshal failed: %v", err3)
	}

	if decoded["code"] != "VALIDATION_ERROR" {
		t.Errorf("code = %v, want VALIDATION_ERROR", decoded["code"])
	}
	if decoded["message"] != "field is required" {
		t.Errorf("message = %v, want 'field is required'", decoded["message"])
	}
}

func TestMerchantStatus_AllValues(t *testing.T) {
	statuses := map[MerchantStatus]string{
		MerchantStatusActive:              "active",
		MerchantStatusSuspended:           "suspended",
		MerchantStatusPendingVerification: "pending_verification",
		MerchantStatusPending:             "pending",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("MerchantStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 4 {
		t.Errorf("expected 4 merchant statuses, got %d", len(statuses))
	}
}

func TestKycStatus_AllValues(t *testing.T) {
	statuses := map[KycStatus]string{
		KycStatusPending:    "pending",
		KycStatusVerified:   "verified",
		KycStatusRejected:   "rejected",
		KycStatusInProgress: "in_progress",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("KycStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 4 {
		t.Errorf("expected 4 KYC statuses, got %d", len(statuses))
	}
}

func TestTransactionStatus_AllValues(t *testing.T) {
	statuses := map[TransactionStatus]string{
		TransactionStatusPending:    "pending",
		TransactionStatusProcessing: "processing",
		TransactionStatusSucceeded:  "succeeded",
		TransactionStatusFailed:     "failed",
		TransactionStatusCancelled:  "cancelled",
		TransactionStatusReversed:   "reversed",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("TransactionStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 6 {
		t.Errorf("expected 6 transaction statuses, got %d", len(statuses))
	}
}

func TestTransactionType_AllValues(t *testing.T) {
	types := map[TransactionType]string{
		TransactionTypeCharge: "charge",
		TransactionTypeRefund: "refund",
		TransactionTypePayout: "payout",
	}

	for tt, want := range types {
		if string(tt) != want {
			t.Errorf("TransactionType = %q, want %q", tt, want)
		}
	}
	if len(types) != 3 {
		t.Errorf("expected 3 transaction types, got %d", len(types))
	}
}

func TestSubscriptionStatus_AllValues(t *testing.T) {
	statuses := map[SubscriptionStatus]string{
		SubscriptionStatusActive:    "active",
		SubscriptionStatusPastDue:   "past_due",
		SubscriptionStatusCancelled: "cancelled",
		SubscriptionStatusUnpaid:    "unpaid",
		SubscriptionStatusTrialing:  "trialing",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("SubscriptionStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 subscription statuses, got %d", len(statuses))
	}
}

func TestSubscriptionInterval_AllValues(t *testing.T) {
	intervals := map[SubscriptionInterval]string{
		SubscriptionIntervalDay:   "day",
		SubscriptionIntervalWeek:  "week",
		SubscriptionIntervalMonth: "month",
		SubscriptionIntervalYear:  "year",
	}

	for iv, want := range intervals {
		if string(iv) != want {
			t.Errorf("SubscriptionInterval = %q, want %q", iv, want)
		}
	}
	if len(intervals) != 4 {
		t.Errorf("expected 4 subscription intervals, got %d", len(intervals))
	}
}

func TestInvoiceStatus_AllValues(t *testing.T) {
	statuses := map[InvoiceStatus]string{
		InvoiceStatusDraft:        "draft",
		InvoiceStatusOpen:         "open",
		InvoiceStatusPaid:         "paid",
		InvoiceStatusVoid:         "void",
		InvoiceStatusUncollectible: "uncollectible",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("InvoiceStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 invoice statuses, got %d", len(statuses))
	}
}

func TestPayoutStatus_AllValues(t *testing.T) {
	statuses := map[PayoutStatus]string{
		PayoutStatusPending:   "pending",
		PayoutStatusInTransit: "in_transit",
		PayoutStatusPaid:      "paid",
		PayoutStatusFailed:    "failed",
		PayoutStatusCancelled: "cancelled",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("PayoutStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 payout statuses, got %d", len(statuses))
	}
}

func TestPayoutMethod_AllValues(t *testing.T) {
	methods := map[PayoutMethod]string{
		PayoutMethodStandard: "standard",
		PayoutMethodInstant:  "instant",
	}

	for m, want := range methods {
		if string(m) != want {
			t.Errorf("PayoutMethod = %q, want %q", m, want)
		}
	}
	if len(methods) != 2 {
		t.Errorf("expected 2 payout methods, got %d", len(methods))
	}
}

func TestDisputeStatus_AllValues(t *testing.T) {
	statuses := map[DisputeStatus]string{
		DisputeStatusWarningNeedsResponse: "warning_needs_response",
		DisputeStatusUnderReview:          "under_review",
		DisputeStatusLost:                 "lost",
		DisputeStatusWon:                  "won",
		DisputeStatusClosed:               "closed",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("DisputeStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 dispute statuses, got %d", len(statuses))
	}
}

func TestPaymentMethodType_AllValues(t *testing.T) {
	types := map[PaymentMethodType]string{
		PaymentMethodTypeCard:       "card",
		PaymentMethodTypeBankAccount: "bank_account",
		PaymentMethodTypeWallet:     "wallet",
	}

	for pt, want := range types {
		if string(pt) != want {
			t.Errorf("PaymentMethodType = %q, want %q", pt, want)
		}
	}
	if len(types) != 3 {
		t.Errorf("expected 3 payment method types, got %d", len(types))
	}
}

func TestHealthStatus_AllValues(t *testing.T) {
	statuses := map[HealthStatus]string{
		HealthStatusHealthy:   "healthy",
		HealthStatusDegraded:  "degraded",
		HealthStatusUnhealthy: "unhealthy",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("HealthStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 3 {
		t.Errorf("expected 3 health statuses, got %d", len(statuses))
	}
}

func TestUserRole_AllValues(t *testing.T) {
	roles := map[UserRole]string{
		RoleRootAdmin:    "root_admin",
		RoleAccountAdmin: "account_admin",
		RoleUser:         "user",
	}

	for r, want := range roles {
		if string(r) != want {
			t.Errorf("UserRole = %q, want %q", r, want)
		}
	}
	if len(roles) != 3 {
		t.Errorf("expected 3 user roles, got %d", len(roles))
	}
}

func TestActorType_AllValues(t *testing.T) {
	types := map[ActorType]string{
		ActorTypeRootAdmin:    "root_admin",
		ActorTypeAccountAdmin: "account_admin",
		ActorTypeUser:         "user",
		ActorTypeSystem:       "system",
		ActorTypeAPIKey:       "api_key",
	}

	for at, want := range types {
		if string(at) != want {
			t.Errorf("ActorType = %q, want %q", at, want)
		}
	}
	if len(types) != 5 {
		t.Errorf("expected 5 actor types, got %d", len(types))
	}
}

func TestTaskStatus_AllValues(t *testing.T) {
	statuses := map[TaskStatus]string{
		TaskStatusPending:   "pending",
		TaskStatusRunning:   "running",
		TaskStatusCompleted: "completed",
		TaskStatusFailed:    "failed",
		TaskStatusDead:      "dead",
	}

	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("TaskStatus = %q, want %q", s, want)
		}
	}
	if len(statuses) != 5 {
		t.Errorf("expected 5 task statuses, got %d", len(statuses))
	}
}

func TestMerchant_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	deleted := now.Add(time.Hour)

	m := Merchant{
		ID:              uuid.New(),
		LegalName:       "Test Corp",
		TradeName:       "Test",
		Name:            "Test Shop",
		Email:           "test@example.com",
		Phone:           "+1-555-0100",
		Country:         "US",
		Currency:        "USD",
		Slug:            "test-shop",
		Status:          MerchantStatusActive,
		KycStatus:       KycStatusVerified,
		Timezone: "UTC",
		Branding:        json.RawMessage(`{"logo":"url"}`),
		Settings:        json.RawMessage(`{"auto_payout":true}`),
		CreatedAt:       now,
		UpdatedAt:       now,
		DeletedAt:       &deleted,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m2 Merchant
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if m2.ID != m.ID {
		t.Error("ID mismatch")
	}
	if m2.LegalName != "Test Corp" {
		t.Errorf("LegalName = %q", m2.LegalName)
	}
	if m2.TradeName != "Test" {
		t.Errorf("TradeName = %q", m2.TradeName)
	}
	if m2.Status != MerchantStatusActive {
		t.Errorf("Status = %q", m2.Status)
	}
	if m2.KycStatus != KycStatusVerified {
		t.Errorf("KycStatus = %q", m2.KycStatus)
	}
	if m2.DeletedAt == nil {
		t.Error("DeletedAt should not be nil")
	}
}

func TestTransaction_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	processed := now.Add(time.Minute)
	netAmount := int64(9500)

	tx := Transaction{
		ID:                    uuid.New(),
		MerchantID:            uuid.New(),
		CustomerID:            uuid.New(),
		Provider:              "stripe",
		ProviderTransactionID: "txn_abc",
		Type:                  TransactionTypeCharge,
		Amount:                10000,
		Currency:              "USD",
		Status:                TransactionStatusSucceeded,
		PaymentMethodID:       uuid.New(),
		IdempotencyKey:        "idem_123",
		Description:           "Test",
		Metadata:              json.RawMessage(`{"key":"val"}`),
		ErrorCode:             "card_declined",
		ErrorMessage:          "Card was declined",
		FeeAmount:             300,
		NetAmount:             &netAmount,
		ProcessedAt:           &processed,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var tx2 Transaction
	if err := json.Unmarshal(data, &tx2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if tx2.Amount != 10000 {
		t.Errorf("Amount = %d, want 10000", tx2.Amount)
	}
	if tx2.FeeAmount != 300 {
		t.Errorf("FeeAmount = %d, want 300", tx2.FeeAmount)
	}
	if tx2.NetAmount == nil || *tx2.NetAmount != 9500 {
		t.Errorf("NetAmount = %v, want 9500", tx2.NetAmount)
	}
	if tx2.ErrorCode != "card_declined" {
		t.Errorf("ErrorCode = %q", tx2.ErrorCode)
	}
	if tx2.Type != TransactionTypeCharge {
		t.Errorf("Type = %q", tx2.Type)
	}
	if tx2.Status != TransactionStatusSucceeded {
		t.Errorf("Status = %q", tx2.Status)
	}
}

func TestCustomer_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)

	c := Customer{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		ExternalID: "ext-456",
		Name:       "Jane Doe",
		Email:      "jane@test.com",
		Phone:      "+4412345678",
		Metadata:   json.RawMessage(`{"source":"web"}`),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var c2 Customer
	if err := json.Unmarshal(data, &c2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if c2.ExternalID != "ext-456" {
		t.Errorf("ExternalID = %q", c2.ExternalID)
	}
	if c2.Name != "Jane Doe" {
		t.Errorf("Name = %q", c2.Name)
	}
}

func TestSubscription_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	cancelAt := now.Add(30 * 24 * time.Hour)
	trialStart := now
	trialEnd := now.Add(14 * 24 * time.Hour)

	s := Subscription{
		ID:                     uuid.New(),
		MerchantID:             uuid.New(),
		CustomerID:             uuid.New(),
		Provider:               "stripe",
		ProviderSubscriptionID: "sub_123",
		PlanID:                 "plan_abc",
		Status:                 SubscriptionStatusActive,
		Amount:                 2999,
		Currency:               "USD",
		Interval:               SubscriptionIntervalMonth,
		IntervalCount:          1,
		CurrentPeriodStart:     now,
		CurrentPeriodEnd:       now.AddDate(0, 1, 0),
		CancelAt:               &cancelAt,
		TrialStart:             &trialStart,
		TrialEnd:               &trialEnd,
		Metadata:               json.RawMessage(`{"plan":"pro"}`),
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var s2 Subscription
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if s2.Amount != 2999 {
		t.Errorf("Amount = %d, want 2999", s2.Amount)
	}
	if s2.Interval != SubscriptionIntervalMonth {
		t.Errorf("Interval = %q, want %q", s2.Interval, SubscriptionIntervalMonth)
	}
	if s2.Status != SubscriptionStatusActive {
		t.Errorf("Status = %q, want %q", s2.Status, SubscriptionStatusActive)
	}
	if s2.CancelAt == nil {
		t.Error("CancelAt should not be nil")
	}
	if s2.TrialStart == nil {
		t.Error("TrialStart should not be nil")
	}
}

func TestInvoice_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	due := now.Add(30 * 24 * time.Hour)
	paid := now.Add(5 * 24 * 24 * time.Hour)
	subID := uuid.New()

	inv := Invoice{
		ID:                uuid.New(),
		MerchantID:        uuid.New(),
		CustomerID:        uuid.New(),
		SubscriptionID:    &subID,
		Provider:          "stripe",
		ProviderInvoiceID: "inv_456",
		Amount:            10000,
		Currency:          "EUR",
		Status:            InvoiceStatusPaid,
		DueDate:           &due,
		PaidAt:            &paid,
		PeriodStart:       now,
		PeriodEnd:         now.AddDate(0, 1, 0),
		Metadata:          json.RawMessage(`{"items":3}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var inv2 Invoice
	if err := json.Unmarshal(data, &inv2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if inv2.Amount != 10000 {
		t.Errorf("Amount = %d, want 10000", inv2.Amount)
	}
	if inv2.SubscriptionID == nil {
		t.Error("SubscriptionID should not be nil")
	}
	if inv2.DueDate == nil {
		t.Error("DueDate should not be nil")
	}
	if inv2.PaidAt == nil {
		t.Error("PaidAt should not be nil")
	}
}

func TestPayout_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	arrival := now.Add(3 * 24 * time.Hour)

	p := Payout{
		ID:               uuid.New(),
		MerchantID:       uuid.New(),
		Provider:         "stripe",
		ProviderPayoutID: "po_789",
		Amount:           50000,
		Currency:         "USD",
		Status:           PayoutStatusInTransit,
		Method:           PayoutMethodInstant,
		ArrivalDate:      &arrival,
		FeeAmount:        250,
		Metadata:         json.RawMessage(`{"batch":"1"}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var p2 Payout
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if p2.Amount != 50000 {
		t.Errorf("Amount = %d, want 50000", p2.Amount)
	}
	if p2.Method != PayoutMethodInstant {
		t.Errorf("Method = %q, want %q", p2.Method, PayoutMethodInstant)
	}
	if p2.FeeAmount != 250 {
		t.Errorf("FeeAmount = %d, want 250", p2.FeeAmount)
	}
	if p2.ArrivalDate == nil {
		t.Error("ArrivalDate should not be nil")
	}
}

func TestDispute_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	deadline := now.Add(14 * 24 * time.Hour)
	submitted := now.Add(7 * 24 * time.Hour)

	d := Dispute{
		ID:                    uuid.New(),
		TransactionID:         uuid.New(),
		MerchantID:            uuid.New(),
		Provider:              "stripe",
		ProviderDisputeID:     "dp_abc",
		Reason:                "fraudulent",
		Status:                DisputeStatusWon,
		Amount:                5000,
		EvidenceDeadline:      &deadline,
		EvidenceSubmittedAt:   &submitted,
		Resolution:            "merchant_won",
		Metadata:              json.RawMessage(`{"evidence":"receipt"}`),
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var d2 Dispute
	if err := json.Unmarshal(data, &d2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if d2.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", d2.Amount)
	}
	if d2.Status != DisputeStatusWon {
		t.Errorf("Status = %q, want %q", d2.Status, DisputeStatusWon)
	}
	if d2.EvidenceDeadline == nil {
		t.Error("EvidenceDeadline should not be nil")
	}
	if d2.Resolution != "merchant_won" {
		t.Errorf("Resolution = %q", d2.Resolution)
	}
}

func TestPaymentMethod_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)

	pm := PaymentMethod{
		ID:           uuid.New(),
		MerchantID:   uuid.New(),
		CustomerID:   uuid.New(),
		Type:         PaymentMethodTypeCard,
		Provider:     "stripe",
		ProviderToken: "tok_123",
		Fingerprint:  "fp_abc",
		Brand:        "visa",
		Last4:        "4242",
		ExpMonth:     12,
		ExpYear:      2028,
		IsDefault:    true,
		Metadata:     json.RawMessage(`{"network":"visa"}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(pm)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var pm2 PaymentMethod
	if err := json.Unmarshal(data, &pm2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if pm2.Type != PaymentMethodTypeCard {
		t.Errorf("Type = %q, want %q", pm2.Type, PaymentMethodTypeCard)
	}
	if pm2.Last4 != "4242" {
		t.Errorf("Last4 = %q, want %q", pm2.Last4, "4242")
	}
	if pm2.ExpMonth != 12 {
		t.Errorf("ExpMonth = %d, want 12", pm2.ExpMonth)
	}
	if pm2.ExpYear != 2028 {
		t.Errorf("ExpYear = %d, want 2028", pm2.ExpYear)
	}
	if !pm2.IsDefault {
		t.Error("IsDefault should be true")
	}
}

func TestUser_JSON_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	mfaSecret := "JBSWY3DPEHPK3PXP"

	u := User{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: "secret",
		Name:         "Admin",
		Role:         RoleRootAdmin,
		MerchantID:   uuid.New(),
		IsActive:     true,
		MfaEnabled:   true,
		MfaSecret:    &mfaSecret,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var u2 User
	if err := json.Unmarshal(data, &u2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if u2.Email != "admin@example.com" {
		t.Errorf("Email = %q", u2.Email)
	}
	if u2.Role != RoleRootAdmin {
		t.Errorf("Role = %q", u2.Role)
	}
	if u2.Name != "Admin" {
		t.Errorf("Name = %q", u2.Name)
	}
}

func TestUser_PasswordHash_Omitted(t *testing.T) {
	u := User{
		ID:           uuid.New(),
		PasswordHash: "super_secret_hash",
		Name:         "Test",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var u2 User
	if err := json.Unmarshal(data, &u2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if u2.PasswordHash != "" {
		t.Errorf("PasswordHash should be omitted from JSON, got %q", u2.PasswordHash)
	}
}

func TestApiKey_JSON_Roundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	expires := now.Add(90 * 24 * time.Hour)
	lastUsed := now.Add(time.Hour)

	ak := ApiKey{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		UserID:     uuid.New(),
		Name:       "Production Key",
		KeyPrefix:  "hx_12345",
		KeyHash:    "sha256hash",
		Scopes:     []string{"payments:read", "payments:write"},
		RateLimit:  1000,
		IsActive:   true,
		ExpiresAt:  &expires,
		CreatedAt:  now,
		LastUsedAt: &lastUsed,
	}

	data, err := json.Marshal(ak)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var ak2 ApiKey
	if err := json.Unmarshal(data, &ak2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if ak2.Name != "Production Key" {
		t.Errorf("Name = %q, want %q", ak2.Name, "Production Key")
	}
	if ak2.RateLimit != 1000 {
		t.Errorf("RateLimit = %d, want 1000", ak2.RateLimit)
	}
	if len(ak2.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(ak2.Scopes))
	}
	if ak2.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
	if ak2.LastUsedAt == nil {
		t.Error("LastUsedAt should not be nil")
	}
}

func TestApiKey_KeyHash_Omitted(t *testing.T) {
	ak := ApiKey{
		ID:      uuid.New(),
		KeyHash: "secret_hash",
		Name:    "Test",
	}

	data, err := json.Marshal(ak)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var ak2 ApiKey
	if err := json.Unmarshal(data, &ak2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if ak2.KeyHash != "" {
		t.Errorf("KeyHash should be omitted, got %q", ak2.KeyHash)
	}
}

func testMarshalUnmarshal(t *testing.T, v interface{}, target interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}

func TestWebhookConfig_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)

	wc := WebhookConfig{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		URL:        "https://example.com/hook",
		Secret:     "whsec_123",
		Events:     []string{"payment.succeeded", "invoice.paid", "*"},
		IsActive:   true,
		Metadata:   json.RawMessage(`{"retry":3}`),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	var wc2 WebhookConfig
	testMarshalUnmarshal(t, wc, &wc2)

	if wc2.URL != "https://example.com/hook" {
		t.Errorf("URL = %q", wc2.URL)
	}
	if len(wc2.Events) != 3 {
		t.Errorf("Events length = %d, want 3", len(wc2.Events))
	}
	if wc2.Events[2] != "*" {
		t.Errorf("Events[2] = %q, want *", wc2.Events[2])
	}
}

func TestAuditLog_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)

	al := AuditLog{
		ID:           uuid.New(),
		MerchantID:   uuid.New(),
		ActorID:      uuid.New(),
		ActorType:    ActorTypeAccountAdmin,
		Action:       "transaction.refund",
		ResourceType: "transaction",
		ResourceID:   "txn_abc",
		Changes:      json.RawMessage(`{"status":{"old":"succeeded","new":"refunded"}}`),
		IPAddress:    "192.168.1.100",
		UserAgent:    "Mozilla/5.0",
		CreatedAt:    now,
	}

	var al2 AuditLog
	testMarshalUnmarshal(t, al, &al2)

	if al2.ActorType != ActorTypeAccountAdmin {
		t.Errorf("ActorType = %q, want %q", al2.ActorType, ActorTypeAccountAdmin)
	}
	if al2.Action != "transaction.refund" {
		t.Errorf("Action = %q", al2.Action)
	}
	if al2.IPAddress != "192.168.1.100" {
		t.Errorf("IPAddress = %q", al2.IPAddress)
	}
}

func TestProviderConfig_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	healthCheck := now.Add(-5 * time.Minute)
	configJSON := json.RawMessage(`{"api_key":"sk_test","webhook_url":"https://example.com"}`)
	metadataJSON := json.RawMessage(`{"version":2}`)

	pc := ProviderConfig{
		ID:               uuid.New(),
		MerchantID:       uuid.New(),
		Provider:         "stripe",
		IsActive:         true,
		Config:           configJSON,
		FallbackOrder:    1,
		HealthStatus:     HealthStatusHealthy,
		LastHealthCheck:  &healthCheck,
		Metadata:         metadataJSON,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	var pc2 ProviderConfig
	testMarshalUnmarshal(t, pc, &pc2)

	if pc2.Provider != "stripe" {
		t.Errorf("Provider = %q, want %q", pc2.Provider, "stripe")
	}
	if pc2.FallbackOrder != 1 {
		t.Errorf("FallbackOrder = %d, want 1", pc2.FallbackOrder)
	}
	if pc2.HealthStatus != HealthStatusHealthy {
		t.Errorf("HealthStatus = %q, want %q", pc2.HealthStatus, HealthStatusHealthy)
	}
	if pc2.LastHealthCheck == nil {
		t.Error("LastHealthCheck should not be nil")
	}
}

func TestExchangeRate_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)

	er := ExchangeRate{
		ID:            42,
		BaseCurrency:  "USD",
		QuoteCurrency: "CHF",
		Rate:          "0.8812",
		Source:        "frankfurter",
		FetchedAt:     now,
		ExpiresAt:     now.Add(time.Hour),
	}

	var er2 ExchangeRate
	testMarshalUnmarshal(t, er, &er2)

	if er2.ID != 42 {
		t.Errorf("ID = %d, want 42", er2.ID)
	}
	if er2.Rate != "0.8812" {
		t.Errorf("Rate = %q, want %q", er2.Rate, "0.8812")
	}
	if er2.Source != "frankfurter" {
		t.Errorf("Source = %q, want %q", er2.Source, "frankfurter")
	}
}

func TestIdempotencyKey_JSON_Roundtrip_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Microsecond)
	respJSON := json.RawMessage(`{"id":"txn_xyz","status":"succeeded"}`)

	ik := IdempotencyKey{
		ID:         7,
		KeyHash:    "abcdef1234567890",
		Response:   respJSON,
		StatusCode: 200,
		MerchantID: uuid.New(),
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}

	var ik2 IdempotencyKey
	testMarshalUnmarshal(t, ik, &ik2)

	if ik2.ID != 7 {
		t.Errorf("ID = %d, want 7", ik2.ID)
	}
	if ik2.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", ik2.StatusCode)
	}
}

func TestAppError_PointersAreNotShared(t *testing.T) {
	err1 := NewNotFoundError("user")
	err2 := NewNotFoundError("merchant")

	if err1 == err2 {
		t.Error("NewNotFoundError should return different pointers")
	}
	if err1.Error() == err2.Error() {
		t.Errorf("errors should have different messages: %q vs %q", err1.Error(), err2.Error())
	}
}
