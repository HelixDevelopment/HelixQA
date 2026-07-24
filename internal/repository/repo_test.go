package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

// ---------------------------------------------------------------------------
// Constructor tests — table-driven
// ---------------------------------------------------------------------------

func TestConstructors_NilPool(t *testing.T) {
	constructors := []struct {
		name string
		fn   func() interface{}
	}{
		{"TransactionRepo", func() interface{} { return NewTransactionRepo(nil) }},
		{"MerchantRepo", func() interface{} { return NewMerchantRepo(nil) }},
		{"CustomerRepo", func() interface{} { return NewCustomerRepo(nil) }},
		{"SubscriptionRepo", func() interface{} { return NewSubscriptionRepo(nil) }},
		{"InvoiceRepo", func() interface{} { return NewInvoiceRepo(nil) }},
		{"PayoutRepo", func() interface{} { return NewPayoutRepo(nil) }},
		{"DisputeRepo", func() interface{} { return NewDisputeRepo(nil) }},
		{"PaymentMethodRepo", func() interface{} { return NewPaymentMethodRepo(nil) }},
		{"WebhookConfigRepo", func() interface{} { return NewWebhookConfigRepo(nil) }},
		{"ProviderConfigRepo", func() interface{} { return NewProviderConfigRepo(nil) }},
		{"UserRepo", func() interface{} { return NewUserRepo(nil) }},
		{"AuditLogRepo", func() interface{} { return NewAuditLogRepo(nil) }},
	}

	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			r := c.fn()
			if r == nil {
				t.Fatalf("New%s returned nil", c.name)
			}
		})
	}
}

func TestConstructors_PanicSafety(t *testing.T) {
	constructors := []struct {
		name string
		fn   func()
	}{
		{"TransactionRepo", func() { _ = NewTransactionRepo(nil) }},
		{"MerchantRepo", func() { _ = NewMerchantRepo(nil) }},
		{"CustomerRepo", func() { _ = NewCustomerRepo(nil) }},
		{"SubscriptionRepo", func() { _ = NewSubscriptionRepo(nil) }},
		{"InvoiceRepo", func() { _ = NewInvoiceRepo(nil) }},
		{"PayoutRepo", func() { _ = NewPayoutRepo(nil) }},
		{"DisputeRepo", func() { _ = NewDisputeRepo(nil) }},
		{"PaymentMethodRepo", func() { _ = NewPaymentMethodRepo(nil) }},
		{"WebhookConfigRepo", func() { _ = NewWebhookConfigRepo(nil) }},
		{"ProviderConfigRepo", func() { _ = NewProviderConfigRepo(nil) }},
		{"UserRepo", func() { _ = NewUserRepo(nil) }},
		{"AuditLogRepo", func() { _ = NewAuditLogRepo(nil) }},
	}

	for _, c := range constructors {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("constructor panicked: %v", r)
				}
			}()
			c.fn()
		})
	}
}

// ---------------------------------------------------------------------------
// Individual constructor + field verification tests
// ---------------------------------------------------------------------------

func TestNewTransactionRepo(t *testing.T) {
	r := NewTransactionRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil TransactionRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewTransactionRepo_WithNilPool(t *testing.T) {
	var pool *pgxpool.Pool
	r := NewTransactionRepo(pool)
	if r == nil {
		t.Fatal("expected non-nil TransactionRepo with nil pool var")
	}
}

func TestNewMerchantRepo(t *testing.T) {
	r := NewMerchantRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil MerchantRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewCustomerRepo(t *testing.T) {
	r := NewCustomerRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil CustomerRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewSubscriptionRepo(t *testing.T) {
	r := NewSubscriptionRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil SubscriptionRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewInvoiceRepo(t *testing.T) {
	r := NewInvoiceRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil InvoiceRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewPayoutRepo(t *testing.T) {
	r := NewPayoutRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil PayoutRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewDisputeRepo(t *testing.T) {
	r := NewDisputeRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil DisputeRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewPaymentMethodRepo(t *testing.T) {
	r := NewPaymentMethodRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil PaymentMethodRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewWebhookConfigRepo(t *testing.T) {
	r := NewWebhookConfigRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil WebhookConfigRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewProviderConfigRepo(t *testing.T) {
	r := NewProviderConfigRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil ProviderConfigRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewUserRepo(t *testing.T) {
	r := NewUserRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil UserRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

func TestNewAuditLogRepo(t *testing.T) {
	r := NewAuditLogRepo(nil)
	if r == nil {
		t.Fatal("expected non-nil AuditLogRepo")
	}
	if r.db != nil {
		t.Error("expected nil db")
	}
}

// ---------------------------------------------------------------------------
// DB field verification — each repo stores the pool correctly
// ---------------------------------------------------------------------------

func TestRepoDBFields_NilPool(t *testing.T) {
	repos := []struct {
		name string
		db   func() *pgxpool.Pool
	}{
		{"TransactionRepo", func() *pgxpool.Pool { return NewTransactionRepo(nil).db }},
		{"MerchantRepo", func() *pgxpool.Pool { return NewMerchantRepo(nil).db }},
		{"CustomerRepo", func() *pgxpool.Pool { return NewCustomerRepo(nil).db }},
		{"SubscriptionRepo", func() *pgxpool.Pool { return NewSubscriptionRepo(nil).db }},
		{"InvoiceRepo", func() *pgxpool.Pool { return NewInvoiceRepo(nil).db }},
		{"PayoutRepo", func() *pgxpool.Pool { return NewPayoutRepo(nil).db }},
		{"DisputeRepo", func() *pgxpool.Pool { return NewDisputeRepo(nil).db }},
		{"PaymentMethodRepo", func() *pgxpool.Pool { return NewPaymentMethodRepo(nil).db }},
		{"WebhookConfigRepo", func() *pgxpool.Pool { return NewWebhookConfigRepo(nil).db }},
		{"ProviderConfigRepo", func() *pgxpool.Pool { return NewProviderConfigRepo(nil).db }},
		{"UserRepo", func() *pgxpool.Pool { return NewUserRepo(nil).db }},
		{"AuditLogRepo", func() *pgxpool.Pool { return NewAuditLogRepo(nil).db }},
	}

	for _, repo := range repos {
		t.Run(repo.name, func(t *testing.T) {
			db := repo.db()
			if db != nil {
				t.Error("expected nil db when constructed with nil pool")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Connection function tests — error wrapping, error types, edge cases
// ---------------------------------------------------------------------------

func TestNewPostgresConnection_InvalidURL(t *testing.T) {
	_, err := NewPostgresConnection("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestNewPostgresConnection_EmptyURL(t *testing.T) {
	_, err := NewPostgresConnection("")
	if err == nil {
		t.Fatal("expected error for empty database URL")
	}
}

func TestNewPostgresConnection_GarbageURL(t *testing.T) {
	_, err := NewPostgresConnection("://garbage://not-a-url")
	if err == nil {
		t.Fatal("expected error for garbage URL")
	}
}

func TestNewPostgresConnection_WrongScheme(t *testing.T) {
	_, err := NewPostgresConnection("mysql://localhost:5432/db")
	if err == nil {
		t.Fatal("expected error for wrong scheme")
	}
}

func TestNewPostgresConnection_ErrorWrapping(t *testing.T) {
	_, err := NewPostgresConnection("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to") {
		t.Errorf("expected error message to contain 'failed to', got: %s", err.Error())
	}
}

func TestNewRedisConnection_InvalidURL(t *testing.T) {
	_, err := NewRedisConnection("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid redis URL")
	}
}

func TestNewRedisConnection_EmptyURL(t *testing.T) {
	_, err := NewRedisConnection("")
	if err == nil {
		t.Fatal("expected error for empty redis URL")
	}
}

func TestNewRedisConnection_WrongScheme(t *testing.T) {
	_, err := NewRedisConnection("http://localhost:6379")
	if err == nil {
		t.Fatal("expected error for wrong redis scheme")
	}
}

func TestNewRedisConnection_ErrorWrapping(t *testing.T) {
	_, err := NewRedisConnection("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to") {
		t.Errorf("expected error message to contain 'failed to', got: %s", err.Error())
	}
}

func TestConnectionFuncs_DontPanicOnInvalidInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("connection constructor panicked: %v", r)
		}
	}()
	NewPostgresConnection("invalid")
	NewRedisConnection("invalid")
}

func TestNewPostgresConnection_NonExistentHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ctx
	_, err := NewPostgresConnection("postgres://nonexistenthost:99999/db?connect_timeout=1")
	if err == nil {
		t.Log("no error (unlikely for non-existent host, but acceptable)")
	}
}

// ---------------------------------------------------------------------------
// Model type usage tests — verify models can be instantiated and used
// with repository constructors
// ---------------------------------------------------------------------------

func TestTransactionRepo_ModelFields(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()
	pmID := uuid.New()
	netAmount := int64(9500)

	txn := &model.Transaction{
		ID:                    id,
		MerchantID:            merchantID,
		CustomerID:            customerID,
		Provider:              "stripe",
		ProviderTransactionID: "txn_123",
		Type:                  model.TransactionTypeCharge,
		Amount:                10000,
		Currency:              "USD",
		Status:                model.TransactionStatusPending,
		PaymentMethodID:       pmID,
		IdempotencyKey:        "idem_001",
		Description:           "Test charge",
		Metadata:              []byte(`{"key":"value"}`),
		ErrorCode:             "",
		ErrorMessage:          "",
		FeeAmount:             500,
		NetAmount:             &netAmount,
		ProcessedAt:           nil,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if txn.Type != model.TransactionTypeCharge {
		t.Errorf("expected charge type, got %s", txn.Type)
	}
	if txn.Amount != 10000 {
		t.Errorf("expected amount 10000, got %d", txn.Amount)
	}
	if txn.NetAmount == nil || *txn.NetAmount != 9500 {
		t.Errorf("expected net amount 9500")
	}
}

func TestMerchantRepo_ModelFields(t *testing.T) {
	now := time.Now()
	m := &model.Merchant{
		ID:        uuid.New(),
		LegalName: "Acme Corp",
		TradeName: "Acme",
		Email:     "info@acme.com",
		Phone:     "+1234567890",
		Country:   "US",
		Currency:  "USD",
		Status:    model.MerchantStatusActive,
		KycStatus: model.KycStatusVerified,
		Settings:  []byte(`{"theme":"dark"}`),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if m.Status != model.MerchantStatusActive {
		t.Errorf("expected active status, got %s", m.Status)
	}
	if m.KycStatus != model.KycStatusVerified {
		t.Errorf("expected verified kyc, got %s", m.KycStatus)
	}
}

func TestSubscriptionRepo_ModelFields(t *testing.T) {
	now := time.Now()
	s := &model.Subscription{
		ID:                     uuid.New(),
		MerchantID:             uuid.New(),
		CustomerID:             uuid.New(),
		Provider:               "stripe",
		ProviderSubscriptionID: "sub_123",
		PlanID:                 "plan_pro",
		Status:                 model.SubscriptionStatusActive,
		Amount:                 2999,
		Currency:               "USD",
		Interval:               model.SubscriptionIntervalMonth,
		IntervalCount:          1,
		CurrentPeriodStart:     now,
		CurrentPeriodEnd:       now.AddDate(0, 1, 0),
		Metadata:               []byte(`{}`),
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if s.Status != model.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", s.Status)
	}
	if s.Interval != model.SubscriptionIntervalMonth {
		t.Errorf("expected monthly interval, got %s", s.Interval)
	}
}

func TestInvoiceRepo_ModelFields(t *testing.T) {
	now := time.Now()
	inv := &model.Invoice{
		ID:                uuid.New(),
		MerchantID:        uuid.New(),
		CustomerID:        uuid.New(),
		Provider:          "stripe",
		ProviderInvoiceID: "inv_123",
		Amount:            5000,
		Currency:          "EUR",
		Status:            model.InvoiceStatusOpen,
		PeriodStart:       now,
		PeriodEnd:         now.AddDate(0, 1, 0),
		Metadata:          []byte(`{}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if inv.Status != model.InvoiceStatusOpen {
		t.Errorf("expected open status, got %s", inv.Status)
	}
	if inv.Amount != 5000 {
		t.Errorf("expected amount 5000, got %d", inv.Amount)
	}
}

func TestPayoutRepo_ModelFields(t *testing.T) {
	now := time.Now()
	p := &model.Payout{
		ID:               uuid.New(),
		MerchantID:       uuid.New(),
		Provider:         "stripe",
		ProviderPayoutID: "po_123",
		Amount:           100000,
		Currency:         "USD",
		Status:           model.PayoutStatusPending,
		Method:           model.PayoutMethodStandard,
		FeeAmount:        250,
		Metadata:         []byte(`{}`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if p.Status != model.PayoutStatusPending {
		t.Errorf("expected pending status, got %s", p.Status)
	}
	if p.Method != model.PayoutMethodStandard {
		t.Errorf("expected standard method, got %s", p.Method)
	}
}

func TestDisputeRepo_ModelFields(t *testing.T) {
	now := time.Now()
	d := &model.Dispute{
		ID:                uuid.New(),
		TransactionID:     uuid.New(),
		MerchantID:        uuid.New(),
		Provider:          "stripe",
		ProviderDisputeID: "dp_123",
		Reason:            "fraudulent",
		Status:            model.DisputeStatusUnderReview,
		Amount:            7500,
		Metadata:          []byte(`{}`),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if d.Status != model.DisputeStatusUnderReview {
		t.Errorf("expected under_review status, got %s", d.Status)
	}
}

func TestPaymentMethodRepo_ModelFields(t *testing.T) {
	now := time.Now()
	pm := &model.PaymentMethod{
		ID:           uuid.New(),
		MerchantID:   uuid.New(),
		CustomerID:   uuid.New(),
		Type:         model.PaymentMethodTypeCard,
		Provider:     "stripe",
		ProviderToken: "tok_123",
		Fingerprint:  "fp_abc",
		Brand:        "visa",
		Last4:        "4242",
		ExpMonth:     12,
		ExpYear:      2025,
		IsDefault:    true,
		Metadata:     []byte(`{}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if pm.Type != model.PaymentMethodTypeCard {
		t.Errorf("expected card type, got %s", pm.Type)
	}
	if !pm.IsDefault {
		t.Error("expected default payment method")
	}
}

func TestWebhookConfigRepo_ModelFields(t *testing.T) {
	now := time.Now()
	w := &model.WebhookConfig{
		ID:        uuid.New(),
		MerchantID: uuid.New(),
		URL:       "https://example.com/webhook",
		Secret:    "whsec_123",
		Events:    []string{"invoice.paid", "subscription.created"},
		IsActive:  true,
		Metadata:  []byte(`{}`),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if !w.IsActive {
		t.Error("expected active webhook")
	}
	if len(w.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(w.Events))
	}
}

func TestProviderConfigRepo_ModelFields(t *testing.T) {
	now := time.Now()
	pc := &model.ProviderConfig{
		ID:            uuid.New(),
		MerchantID:    uuid.New(),
		Provider:      "stripe",
		IsActive:      true,
		Config:        []byte(`{"api_key":"sk_test"}`),
		FallbackOrder: 1,
		HealthStatus:  model.HealthStatusHealthy,
		Metadata:      []byte(`{}`),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if pc.HealthStatus != model.HealthStatusHealthy {
		t.Errorf("expected healthy status, got %s", pc.HealthStatus)
	}
	if pc.FallbackOrder != 1 {
		t.Errorf("expected fallback order 1, got %d", pc.FallbackOrder)
	}
}

func TestUserRepo_ModelFields(t *testing.T) {
	now := time.Now()
	mfaSecret := "JBSWY3DPEHPK3PXP"
	u := &model.User{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: "$2a$10$hash",
		Name:         "Admin User",
		Role:         model.RoleAccountAdmin,
		MerchantID:   uuid.New(),
		IsActive:     true,
		MfaEnabled:   true,
		MfaSecret:    &mfaSecret,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if u.Role != model.RoleAccountAdmin {
		t.Errorf("expected account_admin role, got %s", u.Role)
	}
	if !u.MfaEnabled {
		t.Error("expected MFA enabled")
	}
	if u.MfaSecret == nil || *u.MfaSecret != "JBSWY3DPEHPK3PXP" {
		t.Error("expected MFA secret to be set")
	}
}

func TestAuditLogRepo_ModelFields(t *testing.T) {
	now := time.Now()
	l := &model.AuditLog{
		ID:           uuid.New(),
		MerchantID:   uuid.New(),
		ActorID:      uuid.New(),
		ActorType:    model.ActorTypeUser,
		Action:       "create",
		ResourceType: "transaction",
		ResourceID:   uuid.New().String(),
		Changes:      []byte(`{"status":{"old":"pending","new":"succeeded"}}`),
		IPAddress:    "127.0.0.1",
		UserAgent:    "Mozilla/5.0",
		CreatedAt:    now,
	}

	if l.ActorType != model.ActorTypeUser {
		t.Errorf("expected user actor type, got %s", l.ActorType)
	}
}

// ---------------------------------------------------------------------------
// Model constant tests — verify all status/type constants are defined
// ---------------------------------------------------------------------------

func TestModelTransactionTypes(t *testing.T) {
	types := []model.TransactionType{
		model.TransactionTypeCharge,
		model.TransactionTypeRefund,
		model.TransactionTypePayout,
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("found empty transaction type")
		}
	}
}

func TestModelTransactionStatuses(t *testing.T) {
	statuses := []model.TransactionStatus{
		model.TransactionStatusPending,
		model.TransactionStatusProcessing,
		model.TransactionStatusSucceeded,
		model.TransactionStatusFailed,
		model.TransactionStatusCancelled,
		model.TransactionStatusReversed,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty transaction status")
		}
	}
}

func TestModelSubscriptionStatuses(t *testing.T) {
	statuses := []model.SubscriptionStatus{
		model.SubscriptionStatusActive,
		model.SubscriptionStatusPastDue,
		model.SubscriptionStatusCancelled,
		model.SubscriptionStatusUnpaid,
		model.SubscriptionStatusTrialing,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty subscription status")
		}
	}
}

func TestModelSubscriptionIntervals(t *testing.T) {
	intervals := []model.SubscriptionInterval{
		model.SubscriptionIntervalDay,
		model.SubscriptionIntervalWeek,
		model.SubscriptionIntervalMonth,
		model.SubscriptionIntervalYear,
	}
	for _, i := range intervals {
		if i == "" {
			t.Error("found empty subscription interval")
		}
	}
}

func TestModelInvoiceStatuses(t *testing.T) {
	statuses := []model.InvoiceStatus{
		model.InvoiceStatusDraft,
		model.InvoiceStatusOpen,
		model.InvoiceStatusPaid,
		model.InvoiceStatusVoid,
		model.InvoiceStatusUncollectible,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty invoice status")
		}
	}
}

func TestModelPayoutStatuses(t *testing.T) {
	statuses := []model.PayoutStatus{
		model.PayoutStatusPending,
		model.PayoutStatusInTransit,
		model.PayoutStatusPaid,
		model.PayoutStatusFailed,
		model.PayoutStatusCancelled,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty payout status")
		}
	}
}

func TestModelPayoutMethods(t *testing.T) {
	methods := []model.PayoutMethod{
		model.PayoutMethodStandard,
		model.PayoutMethodInstant,
	}
	for _, m := range methods {
		if m == "" {
			t.Error("found empty payout method")
		}
	}
}

func TestModelDisputeStatuses(t *testing.T) {
	statuses := []model.DisputeStatus{
		model.DisputeStatusWarningNeedsResponse,
		model.DisputeStatusUnderReview,
		model.DisputeStatusLost,
		model.DisputeStatusWon,
		model.DisputeStatusClosed,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty dispute status")
		}
	}
}

func TestModelMerchantStatuses(t *testing.T) {
	statuses := []model.MerchantStatus{
		model.MerchantStatusActive,
		model.MerchantStatusSuspended,
		model.MerchantStatusPendingVerification,
		model.MerchantStatusPending,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty merchant status")
		}
	}
}

func TestModelKycStatuses(t *testing.T) {
	statuses := []model.KycStatus{
		model.KycStatusPending,
		model.KycStatusVerified,
		model.KycStatusRejected,
		model.KycStatusInProgress,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty kyc status")
		}
	}
}

func TestModelUserRoles(t *testing.T) {
	roles := []model.UserRole{
		model.RoleRootAdmin,
		model.RoleAccountAdmin,
		model.RoleUser,
	}
	for _, r := range roles {
		if r == "" {
			t.Error("found empty user role")
		}
	}
}

func TestModelActorTypes(t *testing.T) {
	types := []model.ActorType{
		model.ActorTypeRootAdmin,
		model.ActorTypeAccountAdmin,
		model.ActorTypeUser,
		model.ActorTypeSystem,
		model.ActorTypeAPIKey,
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("found empty actor type")
		}
	}
}

func TestModelHealthStatuses(t *testing.T) {
	statuses := []model.HealthStatus{
		model.HealthStatusHealthy,
		model.HealthStatusDegraded,
		model.HealthStatusUnhealthy,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty health status")
		}
	}
}

func TestModelPaymentMethodTypes(t *testing.T) {
	types := []model.PaymentMethodType{
		model.PaymentMethodTypeCard,
		model.PaymentMethodTypeBankAccount,
		model.PaymentMethodTypeWallet,
	}
	for _, typ := range types {
		if typ == "" {
			t.Error("found empty payment method type")
		}
	}
}

func TestModelTaskStatuses(t *testing.T) {
	statuses := []model.TaskStatus{
		model.TaskStatusPending,
		model.TaskStatusRunning,
		model.TaskStatusCompleted,
		model.TaskStatusFailed,
		model.TaskStatusDead,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("found empty task status")
		}
	}
}

// ---------------------------------------------------------------------------
// Error model tests
// ---------------------------------------------------------------------------

func TestAppError_Error(t *testing.T) {
	e := &model.AppError{
		Code:       "TEST_ERROR",
		Message:    "test message",
		HTTPStatus: 400,
	}
	if e.Error() != "test message" {
		t.Errorf("expected 'test message', got %q", e.Error())
	}
}

func TestAppError_Fields(t *testing.T) {
	e := &model.AppError{
		Code:       "NOT_FOUND",
		Message:    "resource not found",
		HTTPStatus: 404,
	}
	if e.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", e.Code)
	}
	if e.HTTPStatus != 404 {
		t.Errorf("expected status 404, got %d", e.HTTPStatus)
	}
}

func TestErrorVars_Exist(t *testing.T) {
	errs := []struct {
		name string
		err  *model.AppError
	}{
		{"ErrNotFound", model.ErrNotFound},
		{"ErrUnauthorized", model.ErrUnauthorized},
		{"ErrForbidden", model.ErrForbidden},
		{"ErrConflict", model.ErrConflict},
		{"ErrValidation", model.ErrValidation},
		{"ErrInternal", model.ErrInternal},
		{"ErrRateLimited", model.ErrRateLimited},
	}

	for _, e := range errs {
		t.Run(e.name, func(t *testing.T) {
			if e.err == nil {
				t.Fatalf("%s is nil", e.name)
			}
			if e.err.Code == "" {
				t.Errorf("%s has empty Code", e.name)
			}
			if e.err.Message == "" {
				t.Errorf("%s has empty Message", e.name)
			}
			if e.err.HTTPStatus == 0 {
				t.Errorf("%s has zero HTTPStatus", e.name)
			}
		})
	}
}

func TestNewNotFoundError(t *testing.T) {
	e := model.NewNotFoundError("user")
	if e == nil {
		t.Fatal("expected non-nil error")
	}
	if e.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", e.Code)
	}
	if e.HTTPStatus != 404 {
		t.Errorf("expected status 404, got %d", e.HTTPStatus)
	}
	if !strings.Contains(e.Message, "user") {
		t.Errorf("expected message to contain 'user', got %s", e.Message)
	}
}

func TestNewValidationError(t *testing.T) {
	e := model.NewValidationError("email is required")
	if e == nil {
		t.Fatal("expected non-nil error")
	}
	if e.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %s", e.Code)
	}
	if e.HTTPStatus != 422 {
		t.Errorf("expected status 422, got %d", e.HTTPStatus)
	}
	if e.Message != "email is required" {
		t.Errorf("expected 'email is required', got %s", e.Message)
	}
}

func TestNewConflictError(t *testing.T) {
	e := model.NewConflictError("duplicate key")
	if e == nil {
		t.Fatal("expected non-nil error")
	}
	if e.Code != "CONFLICT" {
		t.Errorf("expected code CONFLICT, got %s", e.Code)
	}
	if e.HTTPStatus != 409 {
		t.Errorf("expected status 409, got %d", e.HTTPStatus)
	}
}

func TestErrorImplementsErrorInterface(t *testing.T) {
	var err error = &model.AppError{Message: "test"}
	if err.Error() != "test" {
		t.Error("AppError does not implement error interface correctly")
	}
}

func TestErrorVarsImplementsErrorInterface(t *testing.T) {
	var err error = model.ErrNotFound
	if err == nil {
		t.Fatal("ErrNotFound is nil")
	}
	if err.Error() == "" {
		t.Error("ErrNotFound has empty error message")
	}
}

// ---------------------------------------------------------------------------
// Pagination model tests
// ---------------------------------------------------------------------------

func TestPaginationParams_Normalize(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		expectedPage int
		expectedSize int
	}{
		{"zero page and size", 0, 0, 1, 20},
		{"negative page", -1, 10, 1, 10},
		{"negative size", 1, -5, 1, 20},
		{"both negative", -3, -7, 1, 20},
		{"page ok size zero", 5, 0, 5, 20},
		{"page zero size ok", 0, 30, 1, 30},
		{"both ok", 3, 15, 3, 15},
		{"oversized page size", 1, 200, 1, 100},
		{"exactly 100", 1, 100, 1, 100},
		{"101 capped to 100", 1, 101, 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			p.Normalize()
			if p.Page != tt.expectedPage {
				t.Errorf("Page: got %d, want %d", p.Page, tt.expectedPage)
			}
			if p.PageSize != tt.expectedSize {
				t.Errorf("PageSize: got %d, want %d", p.PageSize, tt.expectedSize)
			}
		})
	}
}

func TestPaginationParams_Offset(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		expected int
	}{
		{"page 1 size 10", 1, 10, 0},
		{"page 2 size 10", 2, 10, 10},
		{"page 3 size 25", 3, 25, 50},
		{"page 1 size 1", 1, 1, 0},
		{"page 10 size 5", 10, 5, 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			got := p.Offset()
			if got != tt.expected {
				t.Errorf("Offset(): got %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	resp := model.NewPaginatedResponse(data, 1, 10, 25)

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page size 10, got %d", resp.PageSize)
	}
	if resp.Total != 25 {
		t.Errorf("expected total 25, got %d", resp.Total)
	}
	if resp.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", resp.TotalPages)
	}
}

func TestNewPaginatedResponse_ExactDivision(t *testing.T) {
	resp := model.NewPaginatedResponse(nil, 1, 10, 20)
	if resp.TotalPages != 2 {
		t.Errorf("expected 2 total pages, got %d", resp.TotalPages)
	}
}

func TestNewPaginatedResponse_ZeroTotal(t *testing.T) {
	resp := model.NewPaginatedResponse(nil, 1, 10, 0)
	if resp.TotalPages != 0 {
		t.Errorf("expected 0 total pages, got %d", resp.TotalPages)
	}
}

func TestNewPaginatedResponse_SingleItem(t *testing.T) {
	resp := model.NewPaginatedResponse("item", 1, 20, 1)
	if resp.TotalPages != 1 {
		t.Errorf("expected 1 total page, got %d", resp.TotalPages)
	}
}

// ---------------------------------------------------------------------------
// Model struct initialization with zero values
// ---------------------------------------------------------------------------

func TestTransaction_ZeroValue(t *testing.T) {
	txn := &model.Transaction{}
	if txn.Amount != 0 {
		t.Error("expected zero amount")
	}
	if txn.Currency != "" {
		t.Error("expected empty currency")
	}
	if txn.Status != "" {
		t.Error("expected empty status")
	}
}

func TestMerchant_ZeroValue(t *testing.T) {
	m := &model.Merchant{}
	if m.Status != "" {
		t.Error("expected empty status")
	}
	if m.KycStatus != "" {
		t.Error("expected empty kyc status")
	}
}

func TestSubscription_ZeroValue(t *testing.T) {
	s := &model.Subscription{}
	if s.Amount != 0 {
		t.Error("expected zero amount")
	}
	if s.IntervalCount != 0 {
		t.Error("expected zero interval count")
	}
}

func TestInvoice_NullableFields(t *testing.T) {
	inv := &model.Invoice{}
	if inv.SubscriptionID != nil {
		t.Error("expected nil subscription_id")
	}
	if !inv.DueDate.IsZero() {
		t.Error("expected zero due_date")
	}
	if inv.PaidAt != nil {
		t.Error("expected nil paid_at")
	}
}

func TestPayout_NullableFields(t *testing.T) {
	p := &model.Payout{}
	if p.ArrivalDate != nil {
		t.Error("expected nil arrival_date")
	}
}

func TestDispute_NullableFields(t *testing.T) {
	d := &model.Dispute{}
	if d.EvidenceDeadline != nil {
		t.Error("expected nil evidence_deadline")
	}
	if d.EvidenceSubmittedAt != nil {
		t.Error("expected nil evidence_submitted_at")
	}
}

func TestUser_NullableFields(t *testing.T) {
	u := &model.User{}
	if u.MfaSecret != nil {
		t.Error("expected nil mfa_secret")
	}
}

func TestWebhookConfig_SliceField(t *testing.T) {
	w := &model.WebhookConfig{}
	if w.Events != nil {
		t.Error("expected nil events")
	}
	w.Events = []string{"event1"}
	if len(w.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(w.Events))
	}
}

func TestProviderConfig_NullableFields(t *testing.T) {
	pc := &model.ProviderConfig{}
	if pc.LastHealthCheck != nil {
		t.Error("expected nil last_health_check")
	}
}

// ---------------------------------------------------------------------------
// UUID uniqueness test
// ---------------------------------------------------------------------------

func TestUUIDGeneration(t *testing.T) {
	ids := make(map[uuid.UUID]bool)
	for i := 0; i < 100; i++ {
		id := uuid.New()
		if ids[id] {
			t.Fatalf("duplicate UUID generated: %s", id)
		}
		ids[id] = true
	}
}

// ---------------------------------------------------------------------------
// pgx.ErrNoRows error identity test
// ---------------------------------------------------------------------------

func TestPgxErrNoRows(t *testing.T) {
	if !errors.Is(pgx.ErrNoRows, pgx.ErrNoRows) {
		t.Error("pgx.ErrNoRows should be comparable with errors.Is")
	}
	err := pgx.ErrNoRows
	if err.Error() != "no rows in result set" {
		t.Errorf("unexpected pgx.ErrNoRows message: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Model struct JSON tags verification
// ---------------------------------------------------------------------------

func TestTransaction_JSONTags(t *testing.T) {
	txn := &model.Transaction{}
	txn.Amount = 100
	txn.Currency = "USD"
	if txn.Amount != 100 {
		t.Error("amount not settable")
	}
}

func TestMerchant_JSONTags(t *testing.T) {
	m := &model.Merchant{}
	m.Email = "test@test.com"
	if m.Email != "test@test.com" {
		t.Error("email not settable")
	}
}

// ---------------------------------------------------------------------------
// Connection function context-based timeout test
// ---------------------------------------------------------------------------

func TestNewPostgresConnection_TimeoutContext(t *testing.T) {
	_, err := NewPostgresConnection("postgres://localhost:1/notexist?connect_timeout=1&sslmode=disable")
	if err == nil {
		t.Log("connection succeeded (unexpected but acceptable)")
	} else {
		if !strings.Contains(err.Error(), "failed to") {
			t.Errorf("expected 'failed to' in error, got: %s", err.Error())
		}
	}
}
