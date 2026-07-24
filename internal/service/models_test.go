package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
)

func TestSubscriptionModel_StatusConstants(t *testing.T) {
	statuses := []model.SubscriptionStatus{
		model.SubscriptionStatusActive,
		model.SubscriptionStatusPastDue,
		model.SubscriptionStatusCancelled,
		model.SubscriptionStatusUnpaid,
		model.SubscriptionStatusTrialing,
	}

	seen := make(map[model.SubscriptionStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status constant: %s", s)
		}
		seen[s] = true
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestSubscriptionModel_IntervalConstants(t *testing.T) {
	intervals := []model.SubscriptionInterval{
		model.SubscriptionIntervalDay,
		model.SubscriptionIntervalWeek,
		model.SubscriptionIntervalMonth,
		model.SubscriptionIntervalYear,
	}

	seen := make(map[model.SubscriptionInterval]bool)
	for _, iv := range intervals {
		if seen[iv] {
			t.Errorf("duplicate interval constant: %s", iv)
		}
		seen[iv] = true
		if iv == "" {
			t.Error("interval should not be empty")
		}
	}
}

func TestSubscriptionModel_Fields(t *testing.T) {
	now := time.Now()
	cancelAt := now.Add(24 * time.Hour)
	cancelledAt := now.Add(-24 * time.Hour)
	trialStart := now.Add(-7 * 24 * time.Hour)
	trialEnd := now.Add(7 * 24 * time.Hour)
	metadata := json.RawMessage(`{"key":"value"}`)

	sub := &model.Subscription{
		ID:                 uuid.New(),
		MerchantID:         uuid.New(),
		CustomerID:         uuid.New(),
		Provider:           "stripe",
		ProviderSubscriptionID: "sub_123",
		PlanID:             "plan_abc",
		Status:             model.SubscriptionStatusActive,
		Amount:             5000,
		Currency:           "USD",
		Interval:           model.SubscriptionIntervalMonth,
		IntervalCount:      1,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		CancelAt:           &cancelAt,
		CancelledAt:        &cancelledAt,
		TrialStart:         &trialStart,
		TrialEnd:           &trialEnd,
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if sub.Status != model.SubscriptionStatusActive {
		t.Errorf("Status = %s, want active", sub.Status)
	}
	if sub.Interval != model.SubscriptionIntervalMonth {
		t.Errorf("Interval = %s, want month", sub.Interval)
	}
	if sub.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", sub.Amount)
	}
	if sub.CancelAt == nil {
		t.Error("CancelAt should not be nil")
	}
	if sub.CancelledAt == nil {
		t.Error("CancelledAt should not be nil")
	}
	if sub.TrialStart == nil {
		t.Error("TrialStart should not be nil")
	}
	if sub.TrialEnd == nil {
		t.Error("TrialEnd should not be nil")
	}
	if len(sub.Metadata) == 0 {
		t.Error("Metadata should not be empty")
	}
}

func TestSubscriptionModel_JSON(t *testing.T) {
	sub := &model.Subscription{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		CustomerID: uuid.New(),
		Status:     model.SubscriptionStatusActive,
		Amount:     5000,
		Currency:   "USD",
		Interval:   model.SubscriptionIntervalMonth,
	}

	data, err := json.Marshal(sub)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded model.Subscription
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.ID != sub.ID {
		t.Error("ID mismatch")
	}
	if decoded.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", decoded.Amount)
	}
}

func TestSubscriptionModel_NilOptionalFields(t *testing.T) {
	sub := &model.Subscription{}
	if sub.CancelAt != nil {
		t.Error("CancelAt should be nil by default")
	}
	if sub.CancelledAt != nil {
		t.Error("CancelledAt should be nil by default")
	}
	if sub.TrialStart != nil {
		t.Error("TrialStart should be nil by default")
	}
	if sub.TrialEnd != nil {
		t.Error("TrialEnd should be nil by default")
	}
}

func TestSubscriptionModel_AllStatuses(t *testing.T) {
	statuses := map[model.SubscriptionStatus]string{
		model.SubscriptionStatusActive:    "active",
		model.SubscriptionStatusPastDue:   "past_due",
		model.SubscriptionStatusCancelled: "cancelled",
		model.SubscriptionStatusUnpaid:    "unpaid",
		model.SubscriptionStatusTrialing:  "trialing",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestSubscriptionModel_AllIntervals(t *testing.T) {
	intervals := map[model.SubscriptionInterval]string{
		model.SubscriptionIntervalDay:   "day",
		model.SubscriptionIntervalWeek:  "week",
		model.SubscriptionIntervalMonth: "month",
		model.SubscriptionIntervalYear:  "year",
	}

	for interval, expected := range intervals {
		if string(interval) != expected {
			t.Errorf("interval %v = %q, want %q", interval, string(interval), expected)
		}
	}
}

func TestInvoiceModel_StatusConstants(t *testing.T) {
	statuses := []model.InvoiceStatus{
		model.InvoiceStatusDraft,
		model.InvoiceStatusOpen,
		model.InvoiceStatusPaid,
		model.InvoiceStatusVoid,
		model.InvoiceStatusUncollectible,
	}

	seen := make(map[model.InvoiceStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestInvoiceModel_Fields(t *testing.T) {
	dueDate := time.Now().Add(30 * 24 * time.Hour)
	paidAt := time.Now()
	subscriptionID := uuid.New()
	metadata := json.RawMessage(`{}`)

	inv := &model.Invoice{
		ID:             uuid.New(),
		MerchantID:     uuid.New(),
		CustomerID:     uuid.New(),
		SubscriptionID: &subscriptionID,
		Provider:       "stripe",
		Amount:         10000,
		Currency:       "USD",
		Status:         model.InvoiceStatusDraft,
		DueDate:        &dueDate,
		PaidAt:         &paidAt,
		PeriodStart:    time.Now().AddDate(0, -1, 0),
		PeriodEnd:      time.Now(),
		Metadata:       metadata,
	}

	if inv.Status != model.InvoiceStatusDraft {
		t.Errorf("Status = %s, want draft", inv.Status)
	}
	if inv.Amount != 10000 {
		t.Errorf("Amount = %d, want 10000", inv.Amount)
	}
	if inv.SubscriptionID == nil {
		t.Error("SubscriptionID should not be nil")
	}
	if inv.DueDate == nil {
		t.Error("DueDate should not be nil")
	}
	if inv.PaidAt == nil {
		t.Error("PaidAt should not be nil")
	}
}

func TestInvoiceModel_JSON(t *testing.T) {
	inv := &model.Invoice{
		ID:       uuid.New(),
		Amount:   5000,
		Currency: "EUR",
		Status:   model.InvoiceStatusOpen,
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded model.Invoice
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", decoded.Amount)
	}
	if decoded.Currency != "EUR" {
		t.Errorf("Currency = %s, want EUR", decoded.Currency)
	}
}

func TestInvoiceModel_AllStatuses(t *testing.T) {
	statuses := map[model.InvoiceStatus]string{
		model.InvoiceStatusDraft:         "draft",
		model.InvoiceStatusOpen:          "open",
		model.InvoiceStatusPaid:          "paid",
		model.InvoiceStatusVoid:          "void",
		model.InvoiceStatusUncollectible: "uncollectible",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestInvoiceModel_NilOptionalFields(t *testing.T) {
	inv := &model.Invoice{}
	if inv.SubscriptionID != nil {
		t.Error("SubscriptionID should be nil by default")
	}
	if inv.DueDate != nil {
		t.Error("DueDate should be nil by default")
	}
	if inv.PaidAt != nil {
		t.Error("PaidAt should be nil by default")
	}
}

func TestDisputeModel_StatusConstants(t *testing.T) {
	statuses := []model.DisputeStatus{
		model.DisputeStatusWarningNeedsResponse,
		model.DisputeStatusUnderReview,
		model.DisputeStatusLost,
		model.DisputeStatusWon,
		model.DisputeStatusClosed,
	}

	seen := make(map[model.DisputeStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestDisputeModel_Fields(t *testing.T) {
	deadline := time.Now().Add(14 * 24 * time.Hour)
	submitted := time.Now()
	metadata := json.RawMessage(`{"reason":"fraud"}`)

	d := &model.Dispute{
		ID:                    uuid.New(),
		TransactionID:         uuid.New(),
		MerchantID:            uuid.New(),
		Provider:              "stripe",
		ProviderDisputeID:     "dp_123",
		Reason:                "fraudulent",
		Status:                model.DisputeStatusWarningNeedsResponse,
		Amount:                5000,
		EvidenceDeadline:      &deadline,
		EvidenceSubmittedAt:   &submitted,
		Resolution:            "pending",
		Metadata:              metadata,
	}

	if d.Status != model.DisputeStatusWarningNeedsResponse {
		t.Errorf("Status = %s, want warning_needs_response", d.Status)
	}
	if d.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", d.Amount)
	}
	if d.EvidenceDeadline == nil {
		t.Error("EvidenceDeadline should not be nil")
	}
	if d.EvidenceSubmittedAt == nil {
		t.Error("EvidenceSubmittedAt should not be nil")
	}
}

func TestDisputeModel_JSON(t *testing.T) {
	d := &model.Dispute{
		ID:       uuid.New(),
		Amount:   7500,
		Reason:   "duplicate",
		Status:   model.DisputeStatusUnderReview,
		Provider: "stripe",
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded model.Dispute
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Amount != 7500 {
		t.Errorf("Amount = %d, want 7500", decoded.Amount)
	}
}

func TestDisputeModel_AllStatuses(t *testing.T) {
	statuses := map[model.DisputeStatus]string{
		model.DisputeStatusWarningNeedsResponse: "warning_needs_response",
		model.DisputeStatusUnderReview:          "under_review",
		model.DisputeStatusLost:                 "lost",
		model.DisputeStatusWon:                  "won",
		model.DisputeStatusClosed:               "closed",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestPayoutModel_StatusConstants(t *testing.T) {
	statuses := []model.PayoutStatus{
		model.PayoutStatusPending,
		model.PayoutStatusInTransit,
		model.PayoutStatusPaid,
		model.PayoutStatusFailed,
		model.PayoutStatusCancelled,
	}

	seen := make(map[model.PayoutStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestPayoutModel_MethodConstants(t *testing.T) {
	methods := []model.PayoutMethod{
		model.PayoutMethodStandard,
		model.PayoutMethodInstant,
	}

	seen := make(map[model.PayoutMethod]bool)
	for _, m := range methods {
		if seen[m] {
			t.Errorf("duplicate method: %s", m)
		}
		seen[m] = true
	}
}

func TestPayoutModel_Fields(t *testing.T) {
	arrivalDate := time.Now().Add(3 * 24 * time.Hour)
	metadata := json.RawMessage(`{"batch":"1"}`)

	p := &model.Payout{
		ID:               uuid.New(),
		MerchantID:       uuid.New(),
		Provider:         "stripe",
		ProviderPayoutID: "po_123",
		Amount:           50000,
		Currency:         "USD",
		Status:           model.PayoutStatusPending,
		Method:           model.PayoutMethodStandard,
		ArrivalDate:      &arrivalDate,
		FeeAmount:         500,
		Metadata:         metadata,
	}

	if p.Status != model.PayoutStatusPending {
		t.Errorf("Status = %s, want pending", p.Status)
	}
	if p.Method != model.PayoutMethodStandard {
		t.Errorf("Method = %s, want standard", p.Method)
	}
	if p.Amount != 50000 {
		t.Errorf("Amount = %d, want 50000", p.Amount)
	}
	if p.FeeAmount != 500 {
		t.Errorf("FeeAmount = %d, want 500", p.FeeAmount)
	}
	if p.ArrivalDate == nil {
		t.Error("ArrivalDate should not be nil")
	}
}

func TestPayoutModel_JSON(t *testing.T) {
	p := &model.Payout{
		ID:       uuid.New(),
		Amount:   25000,
		Currency: "EUR",
		Status:   model.PayoutStatusPaid,
		Method:   model.PayoutMethodInstant,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded model.Payout
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Amount != 25000 {
		t.Errorf("Amount = %d, want 25000", decoded.Amount)
	}
	if decoded.Method != model.PayoutMethodInstant {
		t.Errorf("Method = %s, want instant", decoded.Method)
	}
}

func TestPayoutModel_AllStatuses(t *testing.T) {
	statuses := map[model.PayoutStatus]string{
		model.PayoutStatusPending:   "pending",
		model.PayoutStatusInTransit: "in_transit",
		model.PayoutStatusPaid:      "paid",
		model.PayoutStatusFailed:    "failed",
		model.PayoutStatusCancelled: "cancelled",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestPayoutModel_AllMethods(t *testing.T) {
	methods := map[model.PayoutMethod]string{
		model.PayoutMethodStandard: "standard",
		model.PayoutMethodInstant:  "instant",
	}

	for method, expected := range methods {
		if string(method) != expected {
			t.Errorf("method %v = %q, want %q", method, string(method), expected)
		}
	}
}

func TestTransactionModel_TypeConstants(t *testing.T) {
	types := []model.TransactionType{
		model.TransactionTypeCharge,
		model.TransactionTypeRefund,
		model.TransactionTypePayout,
	}

	seen := make(map[model.TransactionType]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate type: %s", typ)
		}
		seen[typ] = true
	}
}

func TestTransactionModel_StatusConstants(t *testing.T) {
	statuses := []model.TransactionStatus{
		model.TransactionStatusPending,
		model.TransactionStatusProcessing,
		model.TransactionStatusSucceeded,
		model.TransactionStatusFailed,
		model.TransactionStatusCancelled,
		model.TransactionStatusReversed,
	}

	seen := make(map[model.TransactionStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestTransactionModel_Fields(t *testing.T) {
	processedAt := time.Now()
	netAmount := int64(9500)
	metadata := json.RawMessage(`{"fee":500}`)

	tx := &model.Transaction{
		ID:                    uuid.New(),
		MerchantID:            uuid.New(),
		CustomerID:            uuid.New(),
		Provider:              "stripe",
		ProviderTransactionID: "txn_123",
		Type:                  model.TransactionTypeCharge,
		Amount:                10000,
		Currency:              "USD",
		Status:                model.TransactionStatusPending,
		PaymentMethodID:       uuid.New(),
		IdempotencyKey:        "idem_abc",
		Description:           "Test payment",
		Metadata:              metadata,
		ErrorCode:             "",
		ErrorMessage:          "",
		FeeAmount:             500,
		NetAmount:             &netAmount,
		ProcessedAt:           &processedAt,
	}

	if tx.Type != model.TransactionTypeCharge {
		t.Errorf("Type = %s, want charge", tx.Type)
	}
	if tx.Status != model.TransactionStatusPending {
		t.Errorf("Status = %s, want pending", tx.Status)
	}
	if tx.Amount != 10000 {
		t.Errorf("Amount = %d, want 10000", tx.Amount)
	}
	if tx.NetAmount == nil || *tx.NetAmount != 9500 {
		t.Errorf("NetAmount = %v, want 9500", tx.NetAmount)
	}
	if tx.ProcessedAt == nil {
		t.Error("ProcessedAt should not be nil")
	}
	if tx.FeeAmount != 500 {
		t.Errorf("FeeAmount = %d, want 500", tx.FeeAmount)
	}
}

func TestTransactionModel_JSON(t *testing.T) {
	tx := &model.Transaction{
		ID:       uuid.New(),
		Amount:   7500,
		Currency: "GBP",
		Type:     model.TransactionTypeRefund,
		Status:   model.TransactionStatusReversed,
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded model.Transaction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Amount != 7500 {
		t.Errorf("Amount = %d, want 7500", decoded.Amount)
	}
	if decoded.Type != model.TransactionTypeRefund {
		t.Errorf("Type = %s, want refund", decoded.Type)
	}
}

func TestTransactionModel_NilOptionalFields(t *testing.T) {
	tx := &model.Transaction{}
	if tx.NetAmount != nil {
		t.Error("NetAmount should be nil by default")
	}
	if tx.ProcessedAt != nil {
		t.Error("ProcessedAt should be nil by default")
	}
}

func TestTransactionModel_AllTypes(t *testing.T) {
	types := map[model.TransactionType]string{
		model.TransactionTypeCharge: "charge",
		model.TransactionTypeRefund: "refund",
		model.TransactionTypePayout: "payout",
	}

	for typ, expected := range types {
		if string(typ) != expected {
			t.Errorf("type %v = %q, want %q", typ, string(typ), expected)
		}
	}
}

func TestTransactionModel_AllStatuses(t *testing.T) {
	statuses := map[model.TransactionStatus]string{
		model.TransactionStatusPending:    "pending",
		model.TransactionStatusProcessing: "processing",
		model.TransactionStatusSucceeded:  "succeeded",
		model.TransactionStatusFailed:     "failed",
		model.TransactionStatusCancelled:  "cancelled",
		model.TransactionStatusReversed:   "reversed",
	}

	for status, expected := range statuses {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestModelErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *model.AppError
		code     string
		message  string
		httpCode int
	}{
		{"ErrNotFound", model.ErrNotFound, "NOT_FOUND", "resource not found", 404},
		{"ErrUnauthorized", model.ErrUnauthorized, "UNAUTHORIZED", "unauthorized", 401},
		{"ErrForbidden", model.ErrForbidden, "FORBIDDEN", "forbidden", 403},
		{"ErrConflict", model.ErrConflict, "CONFLICT", "resource conflict", 409},
		{"ErrValidation", model.ErrValidation, "VALIDATION_ERROR", "validation failed", 422},
		{"ErrInternal", model.ErrInternal, "INTERNAL_ERROR", "internal server error", 500},
		{"ErrRateLimited", model.ErrRateLimited, "RATE_LIMITED", "rate limit exceeded", 429},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.code)
			}
			if tt.err.Message != tt.message {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.message)
			}
			if tt.err.HTTPStatus != tt.httpCode {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.httpCode)
			}
			if tt.err.Error() != tt.message {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.message)
			}
		})
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := model.NewNotFoundError("user")
	if err.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want NOT_FOUND", err.Code)
	}
	if err.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", err.HTTPStatus)
	}
	if err.Error() != "user not found" {
		t.Errorf("Error() = %q, want 'user not found'", err.Error())
	}
}

func TestNewValidationError(t *testing.T) {
	err := model.NewValidationError("invalid email")
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("Code = %q, want VALIDATION_ERROR", err.Code)
	}
	if err.HTTPStatus != 422 {
		t.Errorf("HTTPStatus = %d, want 422", err.HTTPStatus)
	}
	if err.Error() != "invalid email" {
		t.Errorf("Error() = %q, want 'invalid email'", err.Error())
	}
}

func TestNewConflictError(t *testing.T) {
	err := model.NewConflictError("email already exists")
	if err.Code != "CONFLICT" {
		t.Errorf("Code = %q, want CONFLICT", err.Code)
	}
	if err.HTTPStatus != 409 {
		t.Errorf("HTTPStatus = %d, want 409", err.HTTPStatus)
	}
	if err.Error() != "email already exists" {
		t.Errorf("Error() = %q, want 'email already exists'", err.Error())
	}
}

func TestNewNotFoundError_EmptyResource(t *testing.T) {
	err := model.NewNotFoundError("")
	if err.Error() != " not found" {
		t.Errorf("Error() = %q, want ' not found'", err.Error())
	}
}

func TestAppError_IsError(t *testing.T) {
	err := model.NewValidationError("test")
	var e error = err
	if e == nil {
		t.Fatal("AppError should implement error interface")
	}
}

func TestModelPaginatedResponse_Fields(t *testing.T) {
	p := &model.PaginatedResponse{
		Data:       []string{"a", "b"},
		Page:       2,
		PageSize:   20,
		Total:      100,
		TotalPages: 5,
	}

	if p.Page != 2 {
		t.Errorf("Page = %d, want 2", p.Page)
	}
	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", p.PageSize)
	}
	if p.Total != 100 {
		t.Errorf("Total = %d, want 100", p.Total)
	}
	if p.TotalPages != 5 {
		t.Errorf("TotalPages = %d, want 5", p.TotalPages)
	}
}

func TestPaginationParams_Normalize(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{"zero page", 0, 10, 1, 10},
		{"negative page", -1, 10, 1, 10},
		{"zero page size", 1, 0, 1, 20},
		{"negative page size", 1, -5, 1, 20},
		{"too large page size", 1, 200, 1, 100},
		{"normal values", 3, 50, 3, 50},
		{"page size exactly 100", 1, 100, 1, 100},
		{"page size 1", 1, 1, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			p.Normalize()
			if p.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tt.wantPage)
			}
			if p.PageSize != tt.wantPageSize {
				t.Errorf("PageSize = %d, want %d", p.PageSize, tt.wantPageSize)
			}
		})
	}
}

func TestPaginationParams_Offset(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		want     int
	}{
		{"page 1", 1, 20, 0},
		{"page 2", 2, 20, 20},
		{"page 3 page size 10", 3, 10, 20},
		{"page 1 page size 50", 1, 50, 0},
		{"page 5 page size 10", 5, 10, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			got := p.Offset()
			if got != tt.want {
				t.Errorf("Offset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		total        int64
		wantPages    int
	}{
		{"exact division", 1, 10, 100, 10},
		{"with remainder", 1, 10, 105, 11},
		{"total less than page size", 1, 20, 5, 1},
		{"zero total", 1, 20, 0, 0},
		{"single item", 1, 20, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := model.NewPaginatedResponse(nil, tt.page, tt.pageSize, tt.total)
			if resp.Page != tt.page {
				t.Errorf("Page = %d, want %d", resp.Page, tt.page)
			}
			if resp.PageSize != tt.pageSize {
				t.Errorf("PageSize = %d, want %d", resp.PageSize, tt.pageSize)
			}
			if resp.Total != tt.total {
				t.Errorf("Total = %d, want %d", resp.Total, tt.total)
			}
			if resp.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", resp.TotalPages, tt.wantPages)
			}
		})
	}
}
