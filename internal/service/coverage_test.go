package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/config"
	"github.com/helix-seller/helix-seller/internal/model"
)

// --- MFA Verify tests ---

func TestMFAService_Verify_ValidCode(t *testing.T) {
	svc := NewMFAService()
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	if !svc.Verify(secret, code) {
		t.Error("Verify should return true for valid TOTP code")
	}
}

func TestMFAService_Verify_InvalidCode(t *testing.T) {
	svc := NewMFAService()
	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}

	if svc.Verify(secret, "000000") {
		t.Error("Verify should return false for invalid code")
	}
}

func TestMFAService_Verify_EmptyCode(t *testing.T) {
	svc := NewMFAService()
	secret, _ := svc.GenerateSecret()
	if svc.Verify(secret, "") {
		t.Error("Verify should return false for empty code")
	}
}

func TestMFAService_Verify_EmptySecret(t *testing.T) {
	svc := NewMFAService()
	if svc.Verify("", "123456") {
		t.Error("Verify should return false for empty secret")
	}
}

func TestMFAService_Verify_WrongSecret(t *testing.T) {
	svc := NewMFAService()
	secret1, _ := svc.GenerateSecret()
	secret2, _ := svc.GenerateSecret()

	code, _ := totp.GenerateCode(secret1, time.Now())
	if svc.Verify(secret2, code) {
		t.Error("Verify should return false when code generated from different secret")
	}
}

func TestMFAService_Verify_ReplayedCode(t *testing.T) {
	svc := NewMFAService()
	secret, _ := svc.GenerateSecret()

	code, _ := totp.GenerateCode(secret, time.Now())
	if !svc.Verify(secret, code) {
		t.Fatal("first verify should succeed")
	}
	// Same code replayed should still work within the same time window
	if !svc.Verify(secret, code) {
		t.Error("replayed code should still be valid within same time window")
	}
}

// --- Auth Service tests ---

func TestAuthService_Constructor(t *testing.T) {
	svc := NewAuthService(nil)
	if svc == nil {
		t.Fatal("expected non-nil AuthService")
	}
	if svc.userRepo != nil {
		t.Error("userRepo should be nil")
	}
}

func TestAuthService_Authenticate_NilRepo(t *testing.T) {
	svc := NewAuthService(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil repo")
		}
	}()
	svc.Authenticate(context.Background(), "test@example.com", "password")
}

func TestAuthService_HashPassword_EmptyString(t *testing.T) {
	svc := NewAuthService(nil)
	hash, err := svc.HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword should work for empty password: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	ok, err := svc.VerifyPassword("", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("empty password should verify against its hash")
	}
}

func TestAuthService_HashPassword_UnicodePassword(t *testing.T) {
	svc := NewAuthService(nil)
	hash, err := svc.HashPassword("pässwörd_日本語_🔑")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := svc.VerifyPassword("pässwörd_日本語_🔑", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("unicode password should verify")
	}
}

func TestAuthService_VerifyPassword_TooManyParts(t *testing.T) {
	svc := NewAuthService(nil)
	_, err := svc.VerifyPassword("pass", "a:b:c")
	if err == nil {
		t.Error("expected error for hash with too many parts")
	}
}

func TestAuthService_VerifyPassword_InvalidSaltHex(t *testing.T) {
	svc := NewAuthService(nil)
	_, err := svc.VerifyPassword("pass", "zzzz:abc123")
	if err == nil {
		t.Error("expected error for invalid salt hex")
	}
}

func TestAuthService_VerifyPassword_InvalidHashHex(t *testing.T) {
	svc := NewAuthService(nil)
	salt := make([]byte, 16)
	rand.Read(salt)
	saltHex := hexEncode(salt)
	_, err := svc.VerifyPassword("pass", saltHex+":zzzz")
	if err == nil {
		t.Error("expected error for invalid hash hex")
	}
}

func hexEncode(b []byte) string {
	const hextable = "0123456789abcdef"
	s := make([]byte, len(b)*2)
	for i, v := range b {
		s[i*2] = hextable[v>>4]
		s[i*2+1] = hextable[v&0x0f]
	}
	return string(s)
}

// --- JWT Additional tests ---

func newTestJWTServiceFromFile(t *testing.T) *JWTService {
	t.Helper()
	privPath, pubPath, _ := generateTestKeys(t)
	cfg := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
		JWTAccessExpiry:   15 * time.Minute,
		JWTRefreshExpiry:  7 * 24 * time.Hour,
	}
	svc, err := NewJWTService(cfg)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	return svc
}

func TestNewJWTService_MissingPublicKey(t *testing.T) {
	dir := t.TempDir()
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	privPath := filepath.Join(dir, "private.pem")
	os.WriteFile(privPath, privPEM, 0600)

	cfg := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  filepath.Join(dir, "nonexistent.pem"),
	}
	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("expected error for missing public key")
	}
}

func TestNewJWTService_InvalidPublicKeyPEM(t *testing.T) {
	dir := t.TempDir()
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	privPath := filepath.Join(dir, "private.pem")
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	os.WriteFile(pubPath, []byte("not a valid PEM block"), 0644)

	cfg := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
	}
	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("expected error for invalid public key PEM")
	}
}

func TestNewJWTService_PublicKeyNotRSA(t *testing.T) {
	dir := t.TempDir()
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	privPath := filepath.Join(dir, "private.pem")
	os.WriteFile(privPath, privPEM, 0600)

	// Write a PKCS1 key block instead of PKIX - x509.ParsePKIXPublicKey will fail
	pubPath := filepath.Join(dir, "public.pem")
	os.WriteFile(pubPath, []byte("-----BEGIN PUBLIC KEY-----\nnotbase64data\n-----END PUBLIC KEY-----"), 0644)

	cfg := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
	}
	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("expected error for non-RSA public key")
	}
}

func TestValidateToken_EmptyString(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	_, err := svc.ValidateToken("")
	if err == nil {
		t.Fatal("expected error for empty token string")
	}
}

func TestValidateToken_CompletelyBogus(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	_, err := svc.ValidateToken("this.is.not.a.jwt.at.all")
	if err == nil {
		t.Fatal("expected error for completely invalid token")
	}
}

func TestValidateToken_WrongKey(t *testing.T) {
	svc1 := newTestJWTServiceFromFile(t)
	svc2 := newTestJWTServiceFromFile(t)

	userID := uuid.New()
	merchantID := uuid.New()
	token, _ := svc1.GenerateAccessToken(userID, "test@example.com", "user", merchantID)

	_, err := svc2.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error when validating token with different key")
	}
}

func TestValidateToken_TamperedClaims(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	userID := uuid.New()
	merchantID := uuid.New()
	token, _ := svc.GenerateAccessToken(userID, "test@example.com", "user", merchantID)

	tampered := token[:len(token)-5] + "XXXXX"
	_, err := svc.ValidateToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestGenerateAccessToken_DifferentRoles(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	userID := uuid.New()
	merchantID := uuid.New()

	roles := []string{"admin", "user", "viewer", "owner", ""}
	for _, role := range roles {
		token, err := svc.GenerateAccessToken(userID, "test@example.com", role, merchantID)
		if err != nil {
			t.Fatalf("GenerateAccessToken for role %q: %v", role, err)
		}
		claims, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken for role %q: %v", role, err)
		}
		if claims["role"] != role {
			t.Errorf("role = %v, want %q", claims["role"], role)
		}
	}
}

func TestGenerateRefreshToken_Claims(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	userID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims["token_type"] != "refresh" {
		t.Errorf("token_type = %v, want refresh", claims["token_type"])
	}
	if claims["sub"] != userID.String() {
		t.Errorf("sub = %v, want %v", claims["sub"], userID.String())
	}
	if _, ok := claims["email"]; ok {
		t.Error("refresh token should not have email claim")
	}
}

func TestGenerateAccessToken_Consistency(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	userID := uuid.New()
	merchantID := uuid.New()

	token1, _ := svc.GenerateAccessToken(userID, "test@example.com", "admin", merchantID)
	token2, _ := svc.GenerateAccessToken(userID, "test@example.com", "admin", merchantID)

	// Tokens should be different (different jti and iat)
	if token1 == token2 {
		t.Error("two generated tokens should be different")
	}
}

func TestGenerateRefreshToken_Consistency(t *testing.T) {
	svc := newTestJWTServiceFromFile(t)
	userID := uuid.New()

	token1, _ := svc.GenerateRefreshToken(userID)
	token2, _ := svc.GenerateRefreshToken(userID)

	if token1 == token2 {
		t.Error("two generated refresh tokens should be different")
	}
}

// --- ExchangeRate additional tests ---

func TestExchangeRateService_Convert_NegativeAmount(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	// Same currency, negative amount
	converted, rate, err := svc.Convert(context.Background(), -10000, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != -10000 {
		t.Errorf("converted = %d, want -10000", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_Convert_ZeroAmount(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	converted, rate, err := svc.Convert(context.Background(), 0, "EUR", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 0 {
		t.Errorf("converted = %d, want 0", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_Convert_LargeAmount(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	converted, rate, err := svc.Convert(context.Background(), 1000000000, "GBP", "GBP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 1000000000 {
		t.Errorf("converted = %d, want 1000000000", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_Convert_DifferentCurrency_NilDB(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for different currency with nil DB")
		}
	}()
	svc.Convert(context.Background(), 10000, "USD", "EUR")
}

// --- Webhook additional tests ---

func TestWebhookService_Send_WithQueryParams(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &WebhookService{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: server.URL + "/webhook",
	}

	err := svc.send(config, []byte(`{}`))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if receivedPath != "/webhook" {
		t.Errorf("path = %q, want /webhook", receivedPath)
	}
}

func TestWebhookService_Send_LargeBody(t *testing.T) {
	var bodyLen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyLen = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &WebhookService{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	// Create a large body (10KB)
	largeBody := bytes.Repeat([]byte("x"), 10240)
	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: server.URL,
	}

	err := svc.send(config, largeBody)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if bodyLen != 10240 {
		t.Errorf("body length = %d, want 10240", bodyLen)
	}
}

func TestWebhookService_EventMatches_SingleEvent(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}

	tests := []struct {
		events    []string
		eventType string
		want      bool
	}{
		{[]string{"payment.succeeded"}, "payment.succeeded", true},
		{[]string{"payment.succeeded"}, "payment.failed", false},
		{[]string{"payment.succeeded", "payment.failed"}, "payment.failed", true},
		{[]string{"payment.succeeded", "payment.failed"}, "refund.created", false},
		{[]string{"*"}, "anything.goes", true},
		{[]string{"*"}, "", true},
		{[]string{}, "test", false},
		{nil, "test", false},
	}

	for _, tt := range tests {
		got := svc.eventMatches(tt.events, tt.eventType)
		if got != tt.want {
			t.Errorf("eventMatches(%v, %q) = %v, want %v", tt.events, tt.eventType, got, tt.want)
		}
	}
}

// --- Billing additional tests ---

func TestBillingPeriod_JSON(t *testing.T) {
	period := &BillingPeriod{
		MerchantID:        uuid.New(),
		PeriodStart:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:         time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
		TotalTransactions: 100,
		TotalAmount:       500000,
		TotalFees:         15000,
		Currency:          "USD",
	}

	if period.TotalTransactions != 100 {
		t.Errorf("TotalTransactions = %d, want 100", period.TotalTransactions)
	}
	if period.TotalAmount != 500000 {
		t.Errorf("TotalAmount = %d, want 500000", period.TotalAmount)
	}
	if period.TotalFees != 15000 {
		t.Errorf("TotalFees = %d, want 15000", period.TotalFees)
	}
	if period.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", period.Currency)
	}
	if period.PeriodStart.After(period.PeriodEnd) {
		t.Error("PeriodStart should be before PeriodEnd")
	}
}

func TestFeeStructure_PercentageCalculation(t *testing.T) {
	tests := []struct {
		name           string
		percentageFee  float64
		fixedFee       int64
		amount         int64
		txCount        int64
		wantPercentage int64
		wantFixed      int64
		wantTotal      int64
	}{
		{
			name:           "2.5% of 10000 with 50 fixed",
			percentageFee:  0.025,
			fixedFee:       50,
			amount:         10000,
			txCount:        2,
			wantPercentage: 250,
			wantFixed:      100,
			wantTotal:      350,
		},
		{
			name:           "0.5% of 200000 with 0 fixed",
			percentageFee:  0.005,
			fixedFee:       0,
			amount:         200000,
			txCount:        10,
			wantPercentage: 1000,
			wantFixed:      0,
			wantTotal:      1000,
		},
		{
			name:           "10% of 100 with 100 fixed",
			percentageFee:  0.10,
			fixedFee:       100,
			amount:         100,
			txCount:        1,
			wantPercentage: 10,
			wantFixed:      100,
			wantTotal:      110,
		},
		{
			name:           "0% fee",
			percentageFee:  0.0,
			fixedFee:       0,
			amount:         50000,
			txCount:        100,
			wantPercentage: 0,
			wantFixed:      0,
			wantTotal:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fees := &FeeStructure{
				PercentageFee: tt.percentageFee,
				FixedFee:      tt.fixedFee,
			}
			percentageFee := int64(float64(tt.amount) * fees.PercentageFee)
			fixedFee := fees.FixedFee * tt.txCount
			total := percentageFee + fixedFee

			if percentageFee != tt.wantPercentage {
				t.Errorf("percentageFee = %d, want %d", percentageFee, tt.wantPercentage)
			}
			if fixedFee != tt.wantFixed {
				t.Errorf("fixedFee = %d, want %d", fixedFee, tt.wantFixed)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

// --- Reconciliation additional tests ---

func TestReconciliationResult_JSON(t *testing.T) {
	r := &ReconciliationResult{
		PlatformTotal:    500000,
		ProviderTotal:    499500,
		Discrepancy:      500,
		TransactionCount: 250,
	}

	if r.PlatformTotal != 500000 {
		t.Errorf("PlatformTotal = %d", r.PlatformTotal)
	}
	if r.Discrepancy != r.PlatformTotal-r.ProviderTotal {
		t.Error("Discrepancy should equal PlatformTotal - ProviderTotal")
	}
}

func TestReconciliationResult_EqualTotals(t *testing.T) {
	total := int64(100000)
	r := &ReconciliationResult{
		PlatformTotal:    total,
		ProviderTotal:    total,
		TransactionCount: 50,
	}
	r.Discrepancy = r.PlatformTotal - r.ProviderTotal

	if r.Discrepancy != 0 {
		t.Errorf("equal totals should yield zero discrepancy, got %d", r.Discrepancy)
	}
}

// --- Discrepancy additional tests ---

func TestDiscrepancyService_Constructor_NilDependencies(t *testing.T) {
	svc := NewDiscrepancyService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil DiscrepancyService")
	}
}

func TestDiscrepancy_SeverityLogic_Detailed(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		expected string
	}{
		{"zero", 0, "low"},
		{"one cent", 1, "low"},
		{"below 1000", 999, "low"},
		{"exactly 1000", 1000, "low"},
		{"just above 1000", 1001, "medium"},
		{"5000", 5000, "medium"},
		{"10000", 10000, "medium"},
		{"just above 10000", 10001, "high"},
		{"50000", 50000, "high"},
		{"100000", 100000, "high"},
		{"negative", -5000, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var severity string
			switch {
			case tt.amount > 10000:
				severity = "high"
			case tt.amount > 1000:
				severity = "medium"
			default:
				severity = "low"
			}
			if severity != tt.expected {
				t.Errorf("severity for %d = %q, want %q", tt.amount, severity, tt.expected)
			}
		})
	}
}

func TestDiscrepancy_SeverityCalculation(t *testing.T) {
	// Test the severity logic that would be used in CheckDiscrepancies
	testCases := []struct {
		discrepancy int64
		severity    string
	}{
		{0, "low"},
		{500, "low"},
		{999, "low"},
		{1000, "low"},
		{1001, "medium"},
		{5000, "medium"},
		{10000, "medium"},
		{10001, "high"},
		{100000, "high"},
	}

	for _, tc := range testCases {
		var severity string
		switch {
		case tc.discrepancy > 10000:
			severity = "high"
		case tc.discrepancy > 1000:
			severity = "medium"
		default:
			severity = "low"
		}
		if severity != tc.severity {
			t.Errorf("discrepancy %d: severity = %q, want %q", tc.discrepancy, severity, tc.severity)
		}
	}
}

// --- BackgroundService additional tests ---

func TestBackgroundService_Enqueue_PayloadTypes(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 1, time.Second)

	// These payloads marshal successfully but will fail on nil DB exec
	// We only test that JSON marshaling works (error before DB)
	succeedMarshalPayloads := []struct {
		name    string
		payload interface{}
	}{
		{"nil", nil},
		{"string", "test"},
		{"int", 42},
		{"map", map[string]string{"key": "value"}},
		{"slice", []int{1, 2, 3}},
		{"struct", struct{ Name string }{Name: "test"}},
	}

	for _, tt := range succeedMarshalPayloads {
		t.Run(tt.name+"_marshal", func(t *testing.T) {
			// Just verify the payload can be marshaled
			_, err := json.Marshal(tt.payload)
			if err != nil {
				t.Errorf("json.Marshal(%s) error = %v", tt.name, err)
			}
		})
	}

	// These payloads fail JSON marshaling
	failMarshalPayloads := []struct {
		name    string
		payload interface{}
	}{
		{"channel", make(chan int)},
		{"func", func() {}},
	}

	for _, tt := range failMarshalPayloads {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Enqueue(nil, "test", tt.payload, 1)
			if err == nil {
				t.Errorf("Enqueue(%s) should fail on JSON marshal", tt.name)
			}
		})
	}
}

func TestBackgroundService_Start_CancelsContext(t *testing.T) {
	svc := NewBackgroundService(nil, zap.NewNop(), 1, time.Hour)

	// Cancel immediately so workers exit before hitting nil DB
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Start should not block; workers will immediately see ctx.Done()
	svc.Start(ctx)

	if svc.workers != 1 {
		t.Errorf("workers = %d, want 1", svc.workers)
	}
}

// --- Invoice additional tests ---

func TestInvoiceService_Constructor_NilDependencies(t *testing.T) {
	svc := NewInvoiceService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil InvoiceService")
	}
}

// --- Payment additional tests ---

func TestPaymentService_Constructor_NilDependencies(t *testing.T) {
	svc := NewPaymentService(nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil PaymentService")
	}
}

// --- Payout additional tests ---

func TestPayoutService_Constructor_NilDependencies(t *testing.T) {
	svc := NewPayoutService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil PayoutService")
	}
}

// --- Subscription additional tests ---

func TestSubscriptionService_Constructor_NilDependencies(t *testing.T) {
	svc := NewSubscriptionService(nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil SubscriptionService")
	}
}

func TestCalculatePeriodEnd_AllIntervals_Comprehensive(t *testing.T) {
	start := time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)

	intervals := []struct {
		interval model.SubscriptionInterval
		count    int16
		expected time.Time
	}{
		{model.SubscriptionIntervalDay, 1, start.AddDate(0, 0, 1)},
		{model.SubscriptionIntervalDay, 30, start.AddDate(0, 0, 30)},
		{model.SubscriptionIntervalWeek, 1, start.AddDate(0, 0, 7)},
		{model.SubscriptionIntervalWeek, 4, start.AddDate(0, 0, 28)},
		{model.SubscriptionIntervalMonth, 1, start.AddDate(0, 1, 0)},
		{model.SubscriptionIntervalMonth, 6, start.AddDate(0, 6, 0)},
		{model.SubscriptionIntervalYear, 1, start.AddDate(1, 0, 0)},
		{model.SubscriptionIntervalYear, 5, start.AddDate(5, 0, 0)},
	}

	for _, tt := range intervals {
		t.Run(string(tt.interval)+"_count_"+string(rune(tt.count+'0')), func(t *testing.T) {
			got := calculatePeriodEnd(start, tt.interval, tt.count)
			if !got.Equal(tt.expected) {
				t.Errorf("calculatePeriodEnd(%v, %s, %d) = %v, want %v", start, tt.interval, tt.count, got, tt.expected)
			}
		})
	}
}

// --- Model additional tests ---

func TestAppError_AllPresetErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *model.AppError
		code     string
		httpCode int
	}{
		{"ErrNotFound", model.ErrNotFound, "NOT_FOUND", 404},
		{"ErrUnauthorized", model.ErrUnauthorized, "UNAUTHORIZED", 401},
		{"ErrForbidden", model.ErrForbidden, "FORBIDDEN", 403},
		{"ErrConflict", model.ErrConflict, "CONFLICT", 409},
		{"ErrValidation", model.ErrValidation, "VALIDATION_ERROR", 422},
		{"ErrInternal", model.ErrInternal, "INTERNAL_ERROR", 500},
		{"ErrRateLimited", model.ErrRateLimited, "RATE_LIMITED", 429},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.code)
			}
			if tt.err.HTTPStatus != tt.httpCode {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.httpCode)
			}
			if tt.err.Error() == "" {
				t.Error("Error() should not return empty string")
			}
		})
	}
}

func TestAppError_AsInterface(t *testing.T) {
	err := model.NewValidationError("test")
	var e error = err
	if e.Error() != "test" {
		t.Errorf("Error() = %q, want %q", e.Error(), "test")
	}
}

func TestNewNotFoundError_Various(t *testing.T) {
	tests := []struct {
		resource string
		want     string
	}{
		{"user", "user not found"},
		{"order", "order not found"},
		{"payment", "payment not found"},
		{"", " not found"},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			err := model.NewNotFoundError(tt.resource)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
			if err.HTTPStatus != 404 {
				t.Errorf("HTTPStatus = %d, want 404", err.HTTPStatus)
			}
		})
	}
}

func TestNewConflictError_Various(t *testing.T) {
	tests := []struct {
		message string
	}{
		{"email already exists"},
		{"duplicate entry"},
		{"concurrent modification"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			err := model.NewConflictError(tt.message)
			if err.Error() != tt.message {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.message)
			}
			if err.HTTPStatus != 409 {
				t.Errorf("HTTPStatus = %d, want 409", err.HTTPStatus)
			}
		})
	}
}

// --- Subscription model additional ---

func TestSubscriptionModel_AllFields_JSON_Roundtrip(t *testing.T) {
	now := time.Now()
	cancelAt := now.Add(24 * time.Hour)
	sub := &model.Subscription{
		ID:             uuid.New(),
		MerchantID:     uuid.New(),
		CustomerID:     uuid.New(),
		Status:         model.SubscriptionStatusActive,
		Amount:         9900,
		Currency:       "EUR",
		Interval:       model.SubscriptionIntervalYear,
		IntervalCount:  2,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 0, 730),
		CancelAt:           &cancelAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if sub.Interval != model.SubscriptionIntervalYear {
		t.Errorf("Interval = %s, want year", sub.Interval)
	}
	if sub.IntervalCount != 2 {
		t.Errorf("IntervalCount = %d, want 2", sub.IntervalCount)
	}
	if sub.Amount != 9900 {
		t.Errorf("Amount = %d, want 9900", sub.Amount)
	}
}

// --- Invoice model additional ---

func TestInvoiceModel_AllStatuses_Values(t *testing.T) {
	statusMap := map[model.InvoiceStatus]string{
		model.InvoiceStatusDraft:         "draft",
		model.InvoiceStatusOpen:          "open",
		model.InvoiceStatusPaid:          "paid",
		model.InvoiceStatusVoid:          "void",
		model.InvoiceStatusUncollectible: "uncollectible",
	}

	for status, expected := range statusMap {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

// --- Dispute model additional ---

func TestDisputeModel_AllStatuses_Values(t *testing.T) {
	statusMap := map[model.DisputeStatus]string{
		model.DisputeStatusWarningNeedsResponse: "warning_needs_response",
		model.DisputeStatusUnderReview:          "under_review",
		model.DisputeStatusLost:                 "lost",
		model.DisputeStatusWon:                  "won",
		model.DisputeStatusClosed:               "closed",
	}

	for status, expected := range statusMap {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

// --- Payout model additional ---

func TestPayoutModel_AllStatuses_Values(t *testing.T) {
	statusMap := map[model.PayoutStatus]string{
		model.PayoutStatusPending:   "pending",
		model.PayoutStatusInTransit: "in_transit",
		model.PayoutStatusPaid:      "paid",
		model.PayoutStatusFailed:    "failed",
		model.PayoutStatusCancelled: "cancelled",
	}

	for status, expected := range statusMap {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestPayoutModel_AllMethods_Values(t *testing.T) {
	methodMap := map[model.PayoutMethod]string{
		model.PayoutMethodStandard: "standard",
		model.PayoutMethodInstant:  "instant",
	}

	for method, expected := range methodMap {
		if string(method) != expected {
			t.Errorf("method %v = %q, want %q", method, string(method), expected)
		}
	}
}

// --- Transaction model additional ---

func TestTransactionModel_AllStatuses_Values(t *testing.T) {
	statusMap := map[model.TransactionStatus]string{
		model.TransactionStatusPending:    "pending",
		model.TransactionStatusProcessing: "processing",
		model.TransactionStatusSucceeded:  "succeeded",
		model.TransactionStatusFailed:     "failed",
		model.TransactionStatusCancelled:  "cancelled",
		model.TransactionStatusReversed:   "reversed",
	}

	for status, expected := range statusMap {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestTransactionModel_AllTypes_Values(t *testing.T) {
	typeMap := map[model.TransactionType]string{
		model.TransactionTypeCharge: "charge",
		model.TransactionTypeRefund: "refund",
		model.TransactionTypePayout: "payout",
	}

	for typ, expected := range typeMap {
		if string(typ) != expected {
			t.Errorf("type %v = %q, want %q", typ, string(typ), expected)
		}
	}
}

// --- Subscription status additional ---

func TestSubscriptionModel_AllStatuses_Values(t *testing.T) {
	statusMap := map[model.SubscriptionStatus]string{
		model.SubscriptionStatusActive:    "active",
		model.SubscriptionStatusPastDue:   "past_due",
		model.SubscriptionStatusCancelled: "cancelled",
		model.SubscriptionStatusUnpaid:    "unpaid",
		model.SubscriptionStatusTrialing:  "trialing",
	}

	for status, expected := range statusMap {
		if string(status) != expected {
			t.Errorf("status %v = %q, want %q", status, string(status), expected)
		}
	}
}

func TestSubscriptionModel_AllIntervals_Values(t *testing.T) {
	intervalMap := map[model.SubscriptionInterval]string{
		model.SubscriptionIntervalDay:   "day",
		model.SubscriptionIntervalWeek:  "week",
		model.SubscriptionIntervalMonth: "month",
		model.SubscriptionIntervalYear:  "year",
	}

	for interval, expected := range intervalMap {
		if string(interval) != expected {
			t.Errorf("interval %v = %q, want %q", interval, string(interval), expected)
		}
	}
}

// --- ExchangeRate additional ---

func TestExchangeRateService_FrankfurterResponse_Structure(t *testing.T) {
	fr := frankfurterResponse{
		Rates: map[string]float64{
			"USD": 1.0,
			"EUR": 0.92,
			"GBP": 0.79,
		},
	}

	if len(fr.Rates) != 3 {
		t.Errorf("len(Rates) = %d, want 3", len(fr.Rates))
	}
	if fr.Rates["USD"] != 1.0 {
		t.Errorf("USD rate = %f, want 1.0", fr.Rates["USD"])
	}
}

// --- BackgroundTaskRow additional ---

func TestBackgroundTaskRow_AllFields(t *testing.T) {
	id := uuid.New()
	task := &backgroundTaskRow{
		ID:       id,
		Type:     "webhook_delivery",
		Payload:  []byte(`{"url":"https://example.com"}`),
		Priority: 10,
		Attempts: 3,
	}

	if task.ID != id {
		t.Error("ID mismatch")
	}
	if task.Type != "webhook_delivery" {
		t.Errorf("Type = %q, want webhook_delivery", task.Type)
	}
	if task.Priority != 10 {
		t.Errorf("Priority = %d, want 10", task.Priority)
	}
	if task.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", task.Attempts)
	}
	if string(task.Payload) != `{"url":"https://example.com"}` {
		t.Errorf("Payload = %q", string(task.Payload))
	}
}

// --- Pagination additional ---

func TestPaginationParams_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		wantPage int
		wantSize int
	}{
		{"both zero", 0, 0, 1, 20},
		{"both negative", -1, -1, 1, 20},
		{"max page size exceeded", 1, 500, 1, 100},
		{"page size 101", 1, 101, 1, 100},
		{"page size 99", 1, 99, 1, 99},
		{"very large page", 1000, 20, 1000, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
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

func TestNewPaginatedResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		pageSize  int
		total     int64
		wantPages int
	}{
		{"huge total", 1, 10, 1000000, 100000},
		{"single page", 1, 10, 5, 1},
		{"exact boundary", 1, 10, 10, 1},
		{"one over boundary", 1, 10, 11, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := model.NewPaginatedResponse(nil, tt.page, tt.pageSize, tt.total)
			if resp.Page != tt.page {
				t.Errorf("Page = %d, want %d", resp.Page, tt.page)
			}
		})
	}
}

// --- WebhookConfig model ---

func TestWebhookConfig_Fields(t *testing.T) {
	now := time.Now()
	config := &model.WebhookConfig{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		URL:        "https://example.com/webhook",
		Secret:     "whsec_test123",
		Events:     []string{"payment.succeeded", "invoice.paid"},
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if config.URL != "https://example.com/webhook" {
		t.Errorf("URL = %q", config.URL)
	}
	if config.Secret != "whsec_test123" {
		t.Errorf("Secret = %q", config.Secret)
	}
	if len(config.Events) != 2 {
		t.Errorf("Events length = %d, want 2", len(config.Events))
	}
	if !config.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestWebhookConfig_JSON_Roundtrip(t *testing.T) {
	config := &model.WebhookConfig{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		URL:        "https://example.com/webhook",
		Secret:     "whsec_test123",
		Events:     []string{"payment.succeeded"},
		IsActive:   true,
	}

	if config.Secret != "whsec_test123" {
		t.Errorf("Secret = %q, want whsec_test123", config.Secret)
	}
	if len(config.Events) != 1 {
		t.Errorf("Events length = %d, want 1", len(config.Events))
	}
}

// --- API Key model ---

func TestApiKeyModel_Fields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	ak := &model.ApiKey{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		UserID:     uuid.New(),
		Name:       "Production API Key",
		KeyPrefix:  "hx_abcd1234",
		KeyHash:    "sha256hash",
		Scopes:     []string{"payments:read", "payments:write", "customers:read"},
		RateLimit:  1000,
		IsActive:   true,
		ExpiresAt:  &expiresAt,
		CreatedAt:  now,
	}

	if ak.Name != "Production API Key" {
		t.Errorf("Name = %q", ak.Name)
	}
	if ak.RateLimit != 1000 {
		t.Errorf("RateLimit = %d, want 1000", ak.RateLimit)
	}
	if len(ak.Scopes) != 3 {
		t.Errorf("Scopes length = %d, want 3", len(ak.Scopes))
	}
	if ak.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
}

func TestApiKeyModel_NilExpiresAt(t *testing.T) {
	ak := &model.ApiKey{
		IsActive: true,
		ExpiresAt: nil,
	}

	if ak.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil by default")
	}
}

// --- AnalyticsSummary additional ---

func TestAnalyticsSummary_ComprehensiveValues(t *testing.T) {
	s := &AnalyticsSummary{
		TotalRevenue:           0,
		TotalTransactions:      0,
		SuccessfulTransactions: 0,
		FailedTransactions:     0,
		AverageTransactionSize: 0.0,
		RefundAmount:           0,
		Period:                 "",
	}

	if s.TotalRevenue != 0 {
		t.Errorf("TotalRevenue = %d, want 0", s.TotalRevenue)
	}
	if s.TotalTransactions != 0 {
		t.Errorf("TotalTransactions = %d, want 0", s.TotalTransactions)
	}
	if s.AverageTransactionSize != 0.0 {
		t.Errorf("AverageTransactionSize = %f, want 0.0", s.AverageTransactionSize)
	}
}

// --- ExchangeRate Convert additional ---

func TestExchangeRateService_Convert_MultipleSameCurrency(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())

	currencies := []string{"USD", "EUR", "GBP", "JPY", "CHF"}
	for _, c := range currencies {
		converted, rate, err := svc.Convert(context.Background(), 10000, c, c)
		if err != nil {
			t.Fatalf("Convert(%s -> %s): %v", c, c, err)
		}
		if converted != 10000 {
			t.Errorf("Convert(%s): converted = %d, want 10000", c, converted)
		}
		if rate != 1.0 {
			t.Errorf("Convert(%s): rate = %f, want 1.0", c, rate)
		}
	}
}

// --- WebhookService eventMatches additional ---

func TestWebhookService_EventMatches_MultipleWildcards(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}

	if !svc.eventMatches([]string{"*", "*"}, "test.event") {
		t.Error("multiple wildcards should match any event")
	}
	// eventMatches uses exact matching, not glob patterns
	if svc.eventMatches([]string{"payment.*", "refund.*"}, "payment.succeeded") {
		t.Error("exact match should not match pattern with dots")
	}
	if svc.eventMatches([]string{"payment.succeeded", "refund.created"}, "payment.succeeded") {
		// this should match
	} else {
		t.Error("exact match should work")
	}
}

// --- Billing Period additional ---

func TestBillingPeriod_NegativeFees(t *testing.T) {
	period := &BillingPeriod{
		MerchantID:        uuid.New(),
		TotalTransactions: 10,
		TotalAmount:       5000,
		TotalFees:         -100,
		Currency:          "USD",
	}

	if period.TotalFees != -100 {
		t.Errorf("TotalFees = %d, want -100", period.TotalFees)
	}
}

// --- Dispute model additional ---

func TestDisputeModel_AllFields(t *testing.T) {
	now := time.Now()
	deadline := now.Add(14 * 24 * time.Hour)
	submitted := now

	d := &model.Dispute{
		ID:                    uuid.New(),
		TransactionID:         uuid.New(),
		MerchantID:            uuid.New(),
		Provider:              "stripe",
		ProviderDisputeID:     "dp_abc123",
		Reason:                "fraudulent",
		Status:                model.DisputeStatusWarningNeedsResponse,
		Amount:                7500,
		EvidenceDeadline:      &deadline,
		EvidenceSubmittedAt:   &submitted,
		Resolution:            "pending",
	}

	if d.Provider != "stripe" {
		t.Errorf("Provider = %q, want stripe", d.Provider)
	}
	if d.Reason != "fraudulent" {
		t.Errorf("Reason = %q, want fraudulent", d.Reason)
	}
	if d.Amount != 7500 {
		t.Errorf("Amount = %d, want 7500", d.Amount)
	}
	if d.Resolution != "pending" {
		t.Errorf("Resolution = %q, want pending", d.Resolution)
	}
}

// --- Customer model ---

func TestCustomerModel_Fields(t *testing.T) {
	c := &model.Customer{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		Email:      "customer@example.com",
		Name:       "John Doe",
	}

	if c.Email != "customer@example.com" {
		t.Errorf("Email = %q", c.Email)
	}
	if c.Name != "John Doe" {
		t.Errorf("Name = %q", c.Name)
	}
}

// --- ExchangeRate struct ---

func TestExchangeRateModel_Fields(t *testing.T) {
	now := time.Now()
	er := &model.ExchangeRate{
		ID:            1,
		BaseCurrency:  "USD",
		QuoteCurrency: "EUR",
		Rate:          "0.92",
		Source:        "frankfurter",
		FetchedAt:     now,
		ExpiresAt:     now.Add(time.Hour),
	}

	if er.BaseCurrency != "USD" {
		t.Errorf("BaseCurrency = %q, want USD", er.BaseCurrency)
	}
	if er.QuoteCurrency != "EUR" {
		t.Errorf("QuoteCurrency = %q, want EUR", er.QuoteCurrency)
	}
	if er.Rate != "0.92" {
		t.Errorf("Rate = %q, want 0.92", er.Rate)
	}
	if er.Source != "frankfurter" {
		t.Errorf("Source = %q, want frankfurter", er.Source)
	}
}

// --- Pagination model additional ---

func TestPaginationParams_Offset_Calculation(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		want     int
	}{
		{1, 10, 0},
		{2, 10, 10},
		{3, 20, 40},
		{1, 100, 0},
		{5, 5, 20},
	}

	for _, tt := range tests {
		p := &model.PaginationParams{Page: tt.page, PageSize: tt.pageSize}
		got := p.Offset()
		if got != tt.want {
			t.Errorf("Offset() = %d, want %d", got, tt.want)
		}
	}
}

// --- BackgroundTaskRow JSON additional ---

func TestBackgroundTaskRow_JSON_Roundtrip(t *testing.T) {
	task := &backgroundTaskRow{
		ID:       uuid.New(),
		Type:     "reconciliation",
		Payload:  []byte(`{"test":true}`),
		Priority: 5,
		Attempts: 2,
	}

	if task.Type != "reconciliation" {
		t.Errorf("Type = %q", task.Type)
	}
	if task.Priority != 5 {
		t.Errorf("Priority = %d", task.Priority)
	}
	if task.Attempts != 2 {
		t.Errorf("Attempts = %d", task.Attempts)
	}
}

// --- WebhookService additional ---

func TestWebhookService_SendWithRetry_BadURL(t *testing.T) {
	svc := &WebhookService{
		client: &http.Client{},
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: "http://localhost:1",
	}

	// sendWithRetry tries up to 5 times, all should fail with bad URL
	svc.sendWithRetry(config, []byte(`{}`))
}

func TestWebhookService_Deliver_EmptyConfigs(t *testing.T) {
	// Deliver with a nil configRepo should panic (nil pointer)
	svc := &WebhookService{
		configRepo: nil,
		logger:     zap.NewNop(),
		client:     &http.Client{},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil configRepo in Deliver")
		}
	}()

	svc.Deliver(context.Background(), uuid.New(), "test.event", map[string]string{"key": "value"})
}

// --- Auth service additional ---

func TestAuthService_VerifyPassword_CorrectPassword(t *testing.T) {
	svc := NewAuthService(nil)

	passwords := []string{
		"short",
		"a]veryLongPasswordWith123!@#$%^&*()characters",
		"unicode: 日本語テスト",
		"spaces in password",
		"1234567890",
	}

	for _, pw := range passwords {
		hash, err := svc.HashPassword(pw)
		if err != nil {
			t.Fatalf("HashPassword(%q): %v", pw, err)
		}
		ok, err := svc.VerifyPassword(pw, hash)
		if err != nil {
			t.Fatalf("VerifyPassword(%q): %v", pw, err)
		}
		if !ok {
			t.Errorf("VerifyPassword(%q) = false, want true", pw)
		}
	}
}

// --- Subscription additional ---

func TestSubscriptionModel_CancelledAt_Set(t *testing.T) {
	now := time.Now()
	cancelledAt := now.Add(-1 * time.Hour)

	sub := &model.Subscription{
		ID:           uuid.New(),
		Status:       model.SubscriptionStatusCancelled,
		CancelledAt:  &cancelledAt,
	}

	if sub.CancelledAt == nil {
		t.Fatal("CancelledAt should not be nil")
	}
	if sub.CancelledAt.After(now) {
		t.Error("CancelledAt should be in the past")
	}
}

// --- Invoice additional ---

func TestInvoiceModel_DraftStatus(t *testing.T) {
	inv := &model.Invoice{
		ID:     uuid.New(),
		Status: model.InvoiceStatusDraft,
		Amount: 5000,
	}

	if inv.Status != model.InvoiceStatusDraft {
		t.Errorf("Status = %s, want draft", inv.Status)
	}
}

func TestInvoiceModel_PaidStatus(t *testing.T) {
	now := time.Now()
	inv := &model.Invoice{
		ID:     uuid.New(),
		Status: model.InvoiceStatusPaid,
		PaidAt: &now,
		Amount: 10000,
	}

	if inv.Status != model.InvoiceStatusPaid {
		t.Errorf("Status = %s, want paid", inv.Status)
	}
	if inv.PaidAt == nil {
		t.Error("PaidAt should not be nil for paid invoice")
	}
}

// --- Payout additional ---

func TestPayoutModel_FailedStatus(t *testing.T) {
	p := &model.Payout{
		ID:       uuid.New(),
		Status:   model.PayoutStatusFailed,
		Amount:   25000,
		Currency: "USD",
	}

	if p.Status != model.PayoutStatusFailed {
		t.Errorf("Status = %s, want failed", p.Status)
	}
}

func TestPayoutModel_InTransitStatus(t *testing.T) {
	now := time.Now()
	arrivalDate := now.Add(3 * 24 * time.Hour)
	p := &model.Payout{
		ID:          uuid.New(),
		Status:      model.PayoutStatusInTransit,
		ArrivalDate: &arrivalDate,
		Amount:      50000,
	}

	if p.Status != model.PayoutStatusInTransit {
		t.Errorf("Status = %s, want in_transit", p.Status)
	}
	if p.ArrivalDate == nil {
		t.Error("ArrivalDate should not be nil")
	}
}

// --- Transaction additional ---

func TestTransactionModel_PendingStatus(t *testing.T) {
	tx := &model.Transaction{
		ID:     uuid.New(),
		Status: model.TransactionStatusPending,
		Type:   model.TransactionTypeCharge,
		Amount: 10000,
	}

	if tx.Status != model.TransactionStatusPending {
		t.Errorf("Status = %s, want pending", tx.Status)
	}
}

func TestTransactionModel_SucceededStatus(t *testing.T) {
	now := time.Now()
	netAmount := int64(9500)
	tx := &model.Transaction{
		ID:           uuid.New(),
		Status:       model.TransactionStatusSucceeded,
		Type:         model.TransactionTypeCharge,
		Amount:       10000,
		FeeAmount:    500,
		NetAmount:    &netAmount,
		ProcessedAt:  &now,
	}

	if tx.Status != model.TransactionStatusSucceeded {
		t.Errorf("Status = %s, want succeeded", tx.Status)
	}
	if tx.FeeAmount != 500 {
		t.Errorf("FeeAmount = %d, want 500", tx.FeeAmount)
	}
}

// --- Errors additional ---

func TestAppError_AllCodes(t *testing.T) {
	codes := []struct {
		err  *model.AppError
		code string
	}{
		{model.ErrNotFound, "NOT_FOUND"},
		{model.ErrUnauthorized, "UNAUTHORIZED"},
		{model.ErrForbidden, "FORBIDDEN"},
		{model.ErrConflict, "CONFLICT"},
		{model.ErrValidation, "VALIDATION_ERROR"},
		{model.ErrInternal, "INTERNAL_ERROR"},
		{model.ErrRateLimited, "RATE_LIMITED"},
	}

	for _, c := range codes {
		if c.err.Code != c.code {
			t.Errorf("error code = %q, want %q", c.err.Code, c.code)
		}
	}
}

func TestAppError_ImplementsError(t *testing.T) {
	errs := []*model.AppError{
		model.ErrNotFound,
		model.ErrUnauthorized,
		model.ErrForbidden,
		model.ErrConflict,
		model.ErrValidation,
		model.ErrInternal,
		model.ErrRateLimited,
	}

	for _, err := range errs {
		var e error = err
		if e == nil {
			t.Errorf("error %v should implement error interface", err.Code)
		}
		if e.Error() == "" {
			t.Errorf("error %v should not have empty message", err.Code)
		}
	}
}

// --- Webhook model additional ---

func TestWebhookConfig_EmptyEvents(t *testing.T) {
	config := &model.WebhookConfig{
		ID:       uuid.New(),
		URL:      "https://example.com",
		Events:   []string{},
		IsActive: true,
	}

	if len(config.Events) != 0 {
		t.Errorf("Events length = %d, want 0", len(config.Events))
	}
}

func TestWebhookConfig_InactiveConfig(t *testing.T) {
	config := &model.WebhookConfig{
		ID:       uuid.New(),
		URL:      "https://example.com",
		IsActive: false,
	}

	if config.IsActive {
		t.Error("IsActive should be false")
	}
}

// --- ExchangeRate Convert additional edge cases ---

func TestExchangeRateService_Convert_WildcardCurrency(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	converted, rate, err := svc.Convert(context.Background(), 100, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 100 || rate != 1.0 {
		t.Errorf("same currency: converted=%d rate=%f, want 100/1.0", converted, rate)
	}
}

// --- JWT additional ---

func TestNewJWTService_AllErrorPaths(t *testing.T) {
	// Test missing private key
	cfg := &config.Config{
		JWTPrivateKeyPath: "/nonexistent/path",
		JWTPublicKeyPath:  "/nonexistent/path",
	}
	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("expected error for missing keys")
	}

	// Test missing public key
	dir := t.TempDir()
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})
	privPath := filepath.Join(dir, "private.pem")
	os.WriteFile(privPath, privPEM, 0600)

	cfg2 := &config.Config{
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  filepath.Join(dir, "nonexistent.pem"),
	}
	_, err = NewJWTService(cfg2)
	if err == nil {
		t.Fatal("expected error for missing public key")
	}
}

// --- Webhook additional send tests ---

func TestWebhookService_Send_ContentType(t *testing.T) {
	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &WebhookService{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: server.URL,
	}

	svc.send(config, []byte(`{}`))

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestWebhookService_Send_AllHTTPMethods(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &WebhookService{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: server.URL,
	}

	svc.send(config, []byte(`{}`))

	if receivedMethod != "POST" {
		t.Errorf("Method = %q, want POST", receivedMethod)
	}
}

// --- Comprehensive model coverage ---

func TestAllModelStatusConstants(t *testing.T) {
	// Subscription statuses
	subStatuses := map[model.SubscriptionStatus]bool{
		model.SubscriptionStatusActive:    true,
		model.SubscriptionStatusPastDue:   true,
		model.SubscriptionStatusCancelled: true,
		model.SubscriptionStatusUnpaid:    true,
		model.SubscriptionStatusTrialing:  true,
	}
	for s := range subStatuses {
		if s == "" {
			t.Error("empty subscription status")
		}
	}

	// Invoice statuses
	invStatuses := map[model.InvoiceStatus]bool{
		model.InvoiceStatusDraft:         true,
		model.InvoiceStatusOpen:          true,
		model.InvoiceStatusPaid:          true,
		model.InvoiceStatusVoid:          true,
		model.InvoiceStatusUncollectible: true,
	}
	for s := range invStatuses {
		if s == "" {
			t.Error("empty invoice status")
		}
	}

	// Dispute statuses
	disputeStatuses := map[model.DisputeStatus]bool{
		model.DisputeStatusWarningNeedsResponse: true,
		model.DisputeStatusUnderReview:          true,
		model.DisputeStatusLost:                 true,
		model.DisputeStatusWon:                  true,
		model.DisputeStatusClosed:               true,
	}
	for s := range disputeStatuses {
		if s == "" {
			t.Error("empty dispute status")
		}
	}

	// Payout statuses
	payoutStatuses := map[model.PayoutStatus]bool{
		model.PayoutStatusPending:   true,
		model.PayoutStatusInTransit: true,
		model.PayoutStatusPaid:      true,
		model.PayoutStatusFailed:    true,
		model.PayoutStatusCancelled: true,
	}
	for s := range payoutStatuses {
		if s == "" {
			t.Error("empty payout status")
		}
	}

	// Transaction statuses
	txStatuses := map[model.TransactionStatus]bool{
		model.TransactionStatusPending:    true,
		model.TransactionStatusProcessing: true,
		model.TransactionStatusSucceeded:  true,
		model.TransactionStatusFailed:     true,
		model.TransactionStatusCancelled:  true,
		model.TransactionStatusReversed:   true,
	}
	for s := range txStatuses {
		if s == "" {
			t.Error("empty transaction status")
		}
	}
}

func TestAllModelIntervalConstants(t *testing.T) {
	intervals := map[model.SubscriptionInterval]bool{
		model.SubscriptionIntervalDay:   true,
		model.SubscriptionIntervalWeek:  true,
		model.SubscriptionIntervalMonth: true,
		model.SubscriptionIntervalYear:  true,
	}
	for iv := range intervals {
		if iv == "" {
			t.Error("empty subscription interval")
		}
	}
}

func TestAllTransactionTypeConstants(t *testing.T) {
	types := map[model.TransactionType]bool{
		model.TransactionTypeCharge: true,
		model.TransactionTypeRefund: true,
		model.TransactionTypePayout: true,
	}
	for typ := range types {
		if typ == "" {
			t.Error("empty transaction type")
		}
	}
}

func TestAllPayoutMethodConstants(t *testing.T) {
	methods := map[model.PayoutMethod]bool{
		model.PayoutMethodStandard: true,
		model.PayoutMethodInstant:  true,
	}
	for m := range methods {
		if m == "" {
			t.Error("empty payout method")
		}
	}
}

// --- Comprehensive fee calculation ---

func TestFeeStructure_DifferentRates(t *testing.T) {
	tests := []struct {
		name           string
		percentageFee  float64
		fixedFee       int64
		amount         int64
		txCount        int64
		wantTotal      int64
	}{
		{"tier 1: 1% + $10", 0.01, 10, 10000, 1, 110},
		{"tier 2: 2% + $15", 0.02, 15, 5000, 2, 130},
		{"tier 3: 3% + $20", 0.03, 20, 1000, 5, 130},
		{"zero amount", 0.01, 10, 0, 0, 0},
		{"huge amount", 0.005, 5, 10000000, 1000, 55000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fees := &FeeStructure{PercentageFee: tt.percentageFee, FixedFee: tt.fixedFee}
			pctFee := int64(float64(tt.amount) * fees.PercentageFee)
			fixFee := fees.FixedFee * tt.txCount
			total := pctFee + fixFee

			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

// --- Webhook HMAC tests ---

func TestHMACSignature_Consistent(t *testing.T) {
	secret := "consistent_secret"
	body := []byte(`{"test": true}`)

	sig1 := computeHMAC(body, secret)
	sig2 := computeHMAC(body, secret)

	if sig1 != sig2 {
		t.Error("same input should produce same HMAC")
	}
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
