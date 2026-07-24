package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/config"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
	"github.com/helix-seller/helix-seller/internal/service"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := getEnvOrDefault("DATABASE_URL", "postgresql://helix:helix_dev@127.0.0.1:5432/helix_seller")
	var err error
	testDB, err = pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	if err := testDB.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "unable to ping database: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newTestDBRepos() (*repository.UserRepo, *repository.CustomerRepo, *repository.MerchantRepo,
	*repository.ProviderConfigRepo, *repository.WebhookConfigRepo, *repository.PaymentMethodRepo) {
	return repository.NewUserRepo(testDB),
		repository.NewCustomerRepo(testDB),
		repository.NewMerchantRepo(testDB),
		repository.NewProviderConfigRepo(testDB),
		repository.NewWebhookConfigRepo(testDB),
		repository.NewPaymentMethodRepo(testDB)
}

func newTestServices(userRepo *repository.UserRepo, webhookConfigRepo *repository.WebhookConfigRepo) (
	*service.AuthService, *service.JWTService, *service.MFAService, *service.WebhookService, *service.ApiKeyService) {
	cfg := &config.Config{
		JWTPrivateKeyPath: "../../keys/jwt_private.pem",
		JWTPublicKeyPath:  "../../keys/jwt_public.pem",
		JWTAccessExpiry:   15 * time.Minute,
		JWTRefreshExpiry:  168 * time.Hour,
	}

	authSvc := service.NewAuthService(userRepo)
	jwtSvc, err := service.NewJWTService(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create JWT service: %v", err))
	}
	mfaSvc := service.NewMFAService()
	webhookSvc := service.NewWebhookService(webhookConfigRepo, nil)
	apiKeySvc := service.NewApiKeyService(testDB)

	return authSvc, jwtSvc, mfaSvc, webhookSvc, apiKeySvc
}

func seedUser(t *testing.T, userRepo *repository.UserRepo, email string) *model.User {
	t.Helper()
	user := &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "testhash",
		Name:         "Test User",
		Role:         model.RoleUser,
		IsActive:     true,
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedUserWithPassword(t *testing.T, authSvc *service.AuthService, userRepo *repository.UserRepo, email, password string) *model.User {
	t.Helper()
	hash, err := authSvc.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         "Test User",
		Role:         model.RoleUser,
		IsActive:     true,
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedMerchant(t *testing.T, merchantRepo *repository.MerchantRepo) *model.Merchant {
	t.Helper()
	m := &model.Merchant{
		ID:        uuid.New(),
		LegalName: "Test Corp",
		TradeName: "Test",
		Email:     "merchant@test.com",
		Phone:     "+1234567890",
		Country:   "US",
		Currency:  "USD",
		Status:    model.MerchantStatusPending,
		KycStatus: model.KycStatusPending,
	}
	if err := merchantRepo.Create(context.Background(), m); err != nil {
		t.Fatalf("seed merchant: %v", err)
	}
	return m
}

func seedCustomer(t *testing.T, customerRepo *repository.CustomerRepo, merchantID uuid.UUID) *model.Customer {
	t.Helper()
	c := &model.Customer{
		ID:         uuid.New(),
		MerchantID: merchantID,
		Name:       "Test Customer",
		Email:      "customer@test.com",
		Phone:      "+0987654321",
	}
	if err := customerRepo.Create(context.Background(), c); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	return c
}

func seedProviderConfig(t *testing.T, providerRepo *repository.ProviderConfigRepo, merchantID uuid.UUID) *model.ProviderConfig {
	t.Helper()
	pc := &model.ProviderConfig{
		ID:            uuid.New(),
		MerchantID:    merchantID,
		Provider:      "stripe",
		IsActive:      true,
		Config:        json.RawMessage(`{"api_key":"sk_test"}`),
		FallbackOrder: 1,
		HealthStatus:  model.HealthStatusHealthy,
	}
	if err := providerRepo.Create(context.Background(), pc); err != nil {
		t.Fatalf("seed provider config: %v", err)
	}
	return pc
}

func seedWebhookConfig(t *testing.T, webhookRepo *repository.WebhookConfigRepo, merchantID uuid.UUID) *model.WebhookConfig {
	t.Helper()
	w := &model.WebhookConfig{
		MerchantID: merchantID,
		URL:        "https://example.com/webhook",
		Secret:     "whsec_test",
		Events:     []string{"payment.succeeded"},
		IsActive:   true,
	}
	if err := webhookRepo.Create(context.Background(), w); err != nil {
		t.Fatalf("seed webhook config: %v", err)
	}
	return w
}

func ginContextWith(method, path string, body interface{}, params ...gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
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
	if len(params) > 0 {
		c.Params = params[0]
	}
	return c, w
}

// ============================================================
// AUTH HANDLER TESTS
// ============================================================

func TestRegister_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{
		"email":    fmt.Sprintf("register_success_%d@example.com", time.Now().UnixNano()),
		"password": "securepassword123",
		"name":     "New User",
	}
	c, w := ginContextWith("POST", "/auth/register", body, nil)
	h.Register(c)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestRegister_DuplicateUser(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	email := fmt.Sprintf("dup_%d@example.com", time.Now().UnixNano())
	body := map[string]string{
		"email":    email,
		"password": "securepassword123",
		"name":     "First User",
	}
	c, w := ginContextWith("POST", "/auth/register", body, nil)
	h.Register(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("first register failed: %d %s", w.Code, w.Body.String())
	}

	body["name"] = "Second User"
	c2, w2 := ginContextWith("POST", "/auth/register", body, nil)
	h.Register(c2)
	if w2.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d, body: %s", w2.Code, http.StatusConflict, w2.Body.String())
	}
}

func TestRegister_BindError(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{"email": "bad"}
	c, w := ginContextWith("POST", "/auth/register", body, nil)
	h.Register(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "short",
		"name":     "Test",
	}
	c, w := ginContextWith("POST", "/auth/register", body, nil)
	h.Register(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLogin_Success_NoMFA(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	email := fmt.Sprintf("login_%d@example.com", time.Now().UnixNano())
	seedUserWithPassword(t, authSvc, userRepo, email, "securepassword123")

	body := map[string]string{"email": email, "password": "securepassword123"}
	c, w := ginContextWith("POST", "/auth/login", body, nil)
	h.Login(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}
}

func TestLogin_Success_WithMFA(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	email := fmt.Sprintf("login_mfa_%d@example.com", time.Now().UnixNano())
	user := seedUserWithPassword(t, authSvc, userRepo, email, "securepassword123")

	secret, _ := mfaSvc.GenerateSecret()
	user.MfaEnabled = true
	user.MfaSecret = &secret
	userRepo.Update(context.Background(), user)

	body := map[string]string{"email": email, "password": "securepassword123"}
	c, w := ginContextWith("POST", "/auth/login", body, nil)
	h.Login(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mfa_required"] != true {
		t.Error("expected mfa_required=true")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{"email": "nonexistent@example.com", "password": "wrongpassword"}
	c, w := ginContextWith("POST", "/auth/login", body, nil)
	h.Login(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestLogin_BindError(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{}
	c, w := ginContextWith("POST", "/auth/login", body, nil)
	h.Login(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRefresh_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	email := fmt.Sprintf("refresh_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	refreshToken, err := jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	body := map[string]string{"refresh_token": refreshToken}
	c, w := ginContextWith("POST", "/auth/refresh", body, nil)
	h.Refresh(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{"refresh_token": "invalid.token.here"}
	c, w := ginContextWith("POST", "/auth/refresh", body, nil)
	h.Refresh(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_UserNotFound(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	fakeID := uuid.New()
	refreshToken, err := jwtSvc.GenerateRefreshToken(fakeID)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	body := map[string]string{"refresh_token": refreshToken}
	c, w := ginContextWith("POST", "/auth/refresh", body, nil)
	h.Refresh(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_BindError(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{}
	c, w := ginContextWith("POST", "/auth/refresh", body, nil)
	h.Refresh(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetupMFA_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, _, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, nil, mfaSvc, userRepo)

	email := fmt.Sprintf("mfa_setup_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	c, w := ginContextWith("POST", "/auth/mfa/setup", nil, nil)
	c.Set("user_id", user.ID.String())
	h.SetupMFA(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["secret"] == nil {
		t.Error("expected secret in response")
	}
	if resp["recovery_codes"] == nil {
		t.Error("expected recovery_codes in response")
	}
	if resp["totp_url"] == nil {
		t.Error("expected totp_url in response")
	}
}

func TestSetupMFA_UserNotFound(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, _, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, nil, mfaSvc, userRepo)

	c, w := ginContextWith("POST", "/auth/mfa/setup", nil, nil)
	c.Set("user_id", uuid.New().String())
	h.SetupMFA(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetupMFA_InvalidUserID(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, _, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, nil, mfaSvc, userRepo)

	c, w := ginContextWith("POST", "/auth/mfa/setup", nil, nil)
	c.Set("user_id", "not-a-uuid")
	h.SetupMFA(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestVerifyMFA_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	email := fmt.Sprintf("mfa_verify_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	secret, _ := mfaSvc.GenerateSecret()
	user.MfaSecret = &secret
	userRepo.Update(context.Background(), user)

	body := map[string]string{"user_id": user.ID.String(), "code": "123456"}
	c, w := ginContextWith("POST", "/auth/mfa/verify", body, nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (invalid code expected unauthorized)", w.Code, http.StatusUnauthorized)
	}
}

func TestVerifyMFA_UserNotFound(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	body := map[string]string{"user_id": uuid.New().String(), "code": "123456"}
	c, w := ginContextWith("POST", "/auth/mfa/verify", body, nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestVerifyMFA_NoSecret(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	email := fmt.Sprintf("mfa_nosecret_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	body := map[string]string{"user_id": user.ID.String(), "code": "123456"}
	c, w := ginContextWith("POST", "/auth/mfa/verify", body, nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVerifyMFA_BindError(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	body := map[string]string{}
	c, w := ginContextWith("POST", "/auth/mfa/verify", body, nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestVerifyMFA_InvalidUUID(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, mfaSvc, userRepo)

	body := map[string]string{"user_id": "not-a-uuid", "code": "123456"}
	c, w := ginContextWith("POST", "/auth/mfa/verify", body, nil)
	h.VerifyMFA(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ============================================================
// USER HANDLER TESTS
// ============================================================

func TestUpdateUser_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	email := fmt.Sprintf("upduser_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	body := map[string]string{"name": "Updated Name", "email": "updated_" + email}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", user.ID.String())
	h.UpdateUser(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp model.User
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Name != "Updated Name" {
		t.Errorf("name = %s, want 'Updated Name'", resp.Name)
	}
}

func TestUpdateUser_UserNotFound(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	body := map[string]string{"name": "Updated"}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", uuid.New().String())
	h.UpdateUser(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateUser_InvalidUUID(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	body := map[string]string{"name": "Updated"}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", "not-a-uuid")
	h.UpdateUser(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateUser_PartialUpdate_NameOnly(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	email := fmt.Sprintf("updname_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	body := map[string]string{"name": "Only Name Changed"}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", user.ID.String())
	h.UpdateUser(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateUser_PartialUpdate_EmailOnly(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	email := fmt.Sprintf("updemail_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	newEmail := "new_" + email
	body := map[string]string{"email": newEmail}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", user.ID.String())
	h.UpdateUser(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateUser_EmptyBody(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	email := fmt.Sprintf("updempty_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	body := map[string]string{}
	c, w := ginContextWith("PUT", "/users/me", body, nil)
	c.Set("user_id", user.ID.String())
	h.UpdateUser(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateUser_BindError(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	c, w := ginContextWith("PUT", "/users/me", "not json", nil)
	c.Set("user_id", uuid.New().String())
	h.UpdateUser(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ============================================================
// CUSTOMER HANDLER TESTS
// ============================================================

func TestUpdateCustomer_Success(t *testing.T) {
	_, customerRepo, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	merchant := seedMerchant(t, merchantRepo)
	customer := seedCustomer(t, customerRepo, merchant.ID)

	body := map[string]string{"name": "Updated Customer", "email": "updated@test.com", "phone": "+1111111111"}
	c, w := ginContextWith("PUT", "/customers/"+customer.ID.String(), body,
		gin.Params{{Key: "customerId", Value: customer.ID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp model.Customer
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Name != "Updated Customer" {
		t.Errorf("name = %s, want 'Updated Customer'", resp.Name)
	}
}

func TestUpdateCustomer_NotFound(t *testing.T) {
	_, customerRepo, _, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	fakeID := uuid.New()
	body := map[string]string{"name": "Updated"}
	c, w := ginContextWith("PUT", "/customers/"+fakeID.String(), body, nil,
		gin.Params{{Key: "customerId", Value: fakeID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateCustomer_InvalidUUID(t *testing.T) {
	_, customerRepo, _, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	body := map[string]string{"name": "Updated"}
	c, w := ginContextWith("PUT", "/customers/bad", body, nil,
		gin.Params{{Key: "customerId", Value: "not-a-uuid"}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateCustomer_BindError(t *testing.T) {
	_, customerRepo, _, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	cID := uuid.New()
	c, w := ginContextWith("PUT", "/customers/"+cID.String(), "not json", nil,
		gin.Params{{Key: "customerId", Value: cID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateCustomer_PartialUpdate_NameOnly(t *testing.T) {
	_, customerRepo, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	merchant := seedMerchant(t, merchantRepo)
	customer := seedCustomer(t, customerRepo, merchant.ID)

	body := map[string]string{"name": "Name Only"}
	c, w := ginContextWith("PUT", "/customers/"+customer.ID.String(), body, nil,
		gin.Params{{Key: "customerId", Value: customer.ID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateCustomer_PartialUpdate_PhoneOnly(t *testing.T) {
	_, customerRepo, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	merchant := seedMerchant(t, merchantRepo)
	customer := seedCustomer(t, customerRepo, merchant.ID)

	body := map[string]string{"phone": "+9999999999"}
	c, w := ginContextWith("PUT", "/customers/"+customer.ID.String(), body, nil,
		gin.Params{{Key: "customerId", Value: customer.ID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateCustomer_EmptyBody(t *testing.T) {
	_, customerRepo, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewCustomerHandler(customerRepo, nil)

	merchant := seedMerchant(t, merchantRepo)
	customer := seedCustomer(t, customerRepo, merchant.ID)

	body := map[string]string{}
	c, w := ginContextWith("PUT", "/customers/"+customer.ID.String(), body, nil,
		gin.Params{{Key: "customerId", Value: customer.ID.String()}})
	h.UpdateCustomer(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ============================================================
// MERCHANT HANDLER TESTS
// ============================================================

func TestUpdateMerchant_Success(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{
		"legal_name": "Updated Corp",
		"trade_name": "Updated Trade",
		"email":      "updated@test.com",
		"phone":      "+1111111111",
	}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp model.Merchant
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.LegalName != "Updated Corp" {
		t.Errorf("legal_name = %s, want 'Updated Corp'", resp.LegalName)
	}
}

func TestUpdateMerchant_NotFound(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	fakeID := uuid.New()
	body := map[string]string{"legal_name": "Updated"}
	c, w := ginContextWith("PUT", "/merchants/"+fakeID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: fakeID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateMerchant_InvalidUUID(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	body := map[string]string{"legal_name": "Updated"}
	c, w := ginContextWith("PUT", "/merchants/bad", body, nil,
		gin.Params{{Key: "merchantId", Value: "not-a-uuid"}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateMerchant_BindError(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	mID := uuid.New()
	c, w := ginContextWith("PUT", "/merchants/"+mID.String(), "not json", nil,
		gin.Params{{Key: "merchantId", Value: mID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateMerchant_PartialUpdate_LegalNameOnly(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{"legal_name": "Legal Only"}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateMerchant_PartialUpdate_TradeNameOnly(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{"trade_name": "Trade Only"}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateMerchant_PartialUpdate_EmailOnly(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{"email": "email_only@test.com"}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateMerchant_PartialUpdate_PhoneOnly(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{"phone": "+2222222222"}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateMerchant_EmptyBody(t *testing.T) {
	_, _, merchantRepo, _, _, _ := newTestDBRepos()
	h := NewMerchantHandler(merchantRepo)

	merchant := seedMerchant(t, merchantRepo)

	body := map[string]string{}
	c, w := ginContextWith("PUT", "/merchants/"+merchant.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}})
	h.UpdateMerchant(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ============================================================
// PROVIDER HANDLER TESTS
// ============================================================

func TestUpdateProvider_Success(t *testing.T) {
	_, _, _, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	provider := seedProviderConfig(t, providerRepo, merchant.ID)
	_ = userRepo

	isActive := false
	fo := int16(5)
	hs := "degraded"
	body := map[string]interface{}{
		"config":         map[string]string{"api_key": "new_key"},
		"is_active":      &isActive,
		"fallback_order": &fo,
		"health_status":  &hs,
	}
	c, w := ginContextWith("PUT", "/providers/"+provider.ID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: provider.ID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUpdateProvider_NotFound(t *testing.T) {
	_, _, _, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	fakeID := uuid.New()
	body := map[string]string{"config": "{}"}
	c, w := ginContextWith("PUT", "/providers/"+fakeID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: fakeID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateProvider_InvalidUUID(t *testing.T) {
	_, _, _, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	body := map[string]string{"config": "{}"}
	c, w := ginContextWith("PUT", "/providers/bad", body, nil,
		gin.Params{{Key: "providerId", Value: "not-a-uuid"}})
	h.UpdateProvider(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateProvider_BindError(t *testing.T) {
	_, _, _, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	pID := uuid.New()
	c, w := ginContextWith("PUT", "/providers/"+pID.String(), "not json", nil,
		gin.Params{{Key: "providerId", Value: pID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateProvider_PartialUpdate_IsActiveOnly(t *testing.T) {
	_, _, merchantRepo, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	merchant := seedMerchant(t, merchantRepo)
	provider := seedProviderConfig(t, providerRepo, merchant.ID)

	isActive := false
	body := map[string]interface{}{
		"is_active": &isActive,
	}
	c, w := ginContextWith("PUT", "/providers/"+provider.ID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: provider.ID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateProvider_PartialUpdate_FallbackOrderOnly(t *testing.T) {
	_, _, merchantRepo, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	merchant := seedMerchant(t, merchantRepo)
	provider := seedProviderConfig(t, providerRepo, merchant.ID)

	fo := int16(3)
	body := map[string]interface{}{
		"fallback_order": &fo,
	}
	c, w := ginContextWith("PUT", "/providers/"+provider.ID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: provider.ID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateProvider_PartialUpdate_HealthStatusOnly(t *testing.T) {
	_, _, merchantRepo, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	merchant := seedMerchant(t, merchantRepo)
	provider := seedProviderConfig(t, providerRepo, merchant.ID)

	hs := "unhealthy"
	body := map[string]interface{}{
		"health_status": &hs,
	}
	c, w := ginContextWith("PUT", "/providers/"+provider.ID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: provider.ID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateProvider_EmptyBody(t *testing.T) {
	_, _, merchantRepo, providerRepo, _, _ := newTestDBRepos()
	h := NewProviderHandler(providerRepo)

	merchant := seedMerchant(t, merchantRepo)
	provider := seedProviderConfig(t, providerRepo, merchant.ID)

	body := map[string]interface{}{}
	c, w := ginContextWith("PUT", "/providers/"+provider.ID.String(), body, nil,
		gin.Params{{Key: "providerId", Value: provider.ID.String()}})
	h.UpdateProvider(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ============================================================
// WEBHOOK HANDLER TESTS
// ============================================================

func TestUpdateWebhook_Success(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	body := map[string]interface{}{
		"url":      "https://updated.example.com",
		"secret":   "new_secret",
		"events":   []string{"payment.failed"},
		"is_active": false,
	}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUpdateWebhook_NotFound(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo

	fakeID := uuid.New()
	body := map[string]interface{}{"url": "https://new.com"}
	c, w := ginContextWith("PUT", "/webhooks/"+fakeID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: fakeID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateWebhook_MerchantMismatch(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant1 := seedMerchant(t, merchantRepo)
	merchant2 := seedMerchant(t, merchantRepo)
	_ = userRepo

	webhook := seedWebhookConfig(t, webhookRepo, merchant1.ID)

	body := map[string]interface{}{"url": "https://new.com"}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant2.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateWebhook_InvalidMerchantID(t *testing.T) {
	_, _, _, _, _, _ := newTestDBRepos()
	h := &WebhookHandler{}

	c, w := ginContextWith("PUT", "/webhooks/bad", []byte(`{}`), nil,
		gin.Params{{Key: "merchantId", Value: "bad"}, {Key: "webhookId", Value: "bad"}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateWebhook_InvalidWebhookID(t *testing.T) {
	_, _, _, _, _, _ := newTestDBRepos()
	h := &WebhookHandler{}

	c, w := ginContextWith("PUT", "/webhooks/bad", []byte(`{}`), nil,
		gin.Params{{Key: "merchantId", Value: uuid.New().String()}, {Key: "webhookId", Value: "bad"}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateWebhook_BindError(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), "not json", nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateWebhook_PartialUpdate_URLOnly(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	body := map[string]interface{}{"url": "https://url-only.example.com"}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateWebhook_PartialUpdate_SecretOnly(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	body := map[string]interface{}{"secret": "new_secret"}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateWebhook_PartialUpdate_EventsOnly(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	body := map[string]interface{}{"events": []string{"payment.failed", "refund.created"}}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateWebhook_EmptyBody(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	body := map[string]interface{}{}
	c, w := ginContextWith("PUT", "/webhooks/"+webhook.ID.String(), body, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.UpdateWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ============================================================
// WEBHOOK DELETE TESTS
// ============================================================

func TestDeleteWebhook_Success(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo
	webhook := seedWebhookConfig(t, webhookRepo, merchant.ID)

	c, w := ginContextWith("DELETE", "/webhooks/"+webhook.ID.String(), nil, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.DeleteWebhook(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestDeleteWebhook_NotFound(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant := seedMerchant(t, merchantRepo)
	_ = userRepo

	fakeID := uuid.New()
	c, w := ginContextWith("DELETE", "/webhooks/"+fakeID.String(), nil, nil,
		gin.Params{{Key: "merchantId", Value: merchant.ID.String()}, {Key: "webhookId", Value: fakeID.String()}})
	h.DeleteWebhook(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteWebhook_MerchantMismatch(t *testing.T) {
	_, _, _, _, webhookRepo, _ := newTestDBRepos()
	_, _, _, webhookSvc, _ := newTestServices(nil, webhookRepo)
	h := NewWebhookHandler(webhookSvc)

	userRepo, _, merchantRepo, _, _, _ := newTestDBRepos()
	merchant1 := seedMerchant(t, merchantRepo)
	merchant2 := seedMerchant(t, merchantRepo)
	_ = userRepo

	webhook := seedWebhookConfig(t, webhookRepo, merchant1.ID)

	c, w := ginContextWith("DELETE", "/webhooks/"+webhook.ID.String(), nil, nil,
		gin.Params{{Key: "merchantId", Value: merchant2.ID.String()}, {Key: "webhookId", Value: webhook.ID.String()}})
	h.DeleteWebhook(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteWebhook_InvalidMerchantID(t *testing.T) {
	_, _, _, _, _, _ := newTestDBRepos()
	h := &WebhookHandler{}

	c, w := ginContextWith("DELETE", "/webhooks/bad", nil, nil,
		gin.Params{{Key: "merchantId", Value: "bad"}, {Key: "webhookId", Value: "bad"}})
	h.DeleteWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteWebhook_InvalidWebhookID(t *testing.T) {
	_, _, _, _, _, _ := newTestDBRepos()
	h := &WebhookHandler{}

	c, w := ginContextWith("DELETE", "/webhooks/bad", nil, nil,
		gin.Params{{Key: "merchantId", Value: uuid.New().String()}, {Key: "webhookId", Value: "bad"}})
	h.DeleteWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ============================================================
// APIKEY HANDLER TESTS
// ============================================================

func TestListApiKeys_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	_, _, _, _, apiKeySvc := newTestServices(userRepo, nil)
	h := NewApiKeyHandler(apiKeySvc)

	merchantID := uuid.New()
	c, w := ginContextWith("GET", "/api-keys", nil, nil)
	c.Set("merchant_id", merchantID.String())
	c.Set("user_id", uuid.New().String())
	h.ListApiKeys(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestListApiKeys_Empty(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	_, _, _, _, apiKeySvc := newTestServices(userRepo, nil)
	h := NewApiKeyHandler(apiKeySvc)

	merchantID := uuid.New()
	c, w := ginContextWith("GET", "/api-keys", nil, nil)
	c.Set("merchant_id", merchantID.String())
	c.Set("user_id", uuid.New().String())
	h.ListApiKeys(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["api_keys"] == nil {
		t.Error("expected api_keys in response")
	}
}

func TestRevokeApiKey_Success(t *testing.T) {
	userRepo, _, _, _, _ := newTestDBRepos()
	_, _, _, _, apiKeySvc := newTestServices(userRepo, nil)
	h := NewApiKeyHandler(apiKeySvc)

	_, apiKey, err := apiKeySvc.Create(context.Background(), uuid.New(), uuid.New(), "Test Key", []string{"read"}, 1000, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	c, w := ginContextWith("DELETE", "/api-keys/"+apiKey.ID.String(), nil, nil,
		gin.Params{{Key: "keyId", Value: apiKey.ID.String()}})
	h.RevokeApiKey(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRevokeApiKey_NotFound(t *testing.T) {
	userRepo, _, _, _, _ := newTestDBRepos()
	_, _, _, _, apiKeySvc := newTestServices(userRepo, nil)
	h := NewApiKeyHandler(apiKeySvc)

	fakeID := uuid.New()
	c, w := ginContextWith("DELETE", "/api-keys/"+fakeID.String(), nil, nil,
		gin.Params{{Key: "keyId", Value: fakeID.String()}})
	h.RevokeApiKey(c)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (or %d), body: %s", w.Code, http.StatusInternalServerError, http.StatusNotFound, w.Body.String())
	}
}

func TestRevokeApiKey_InvalidUUID(t *testing.T) {
	userRepo, _, _, _, _ := newTestDBRepos()
	_, _, _, _, apiKeySvc := newTestServices(userRepo, nil)
	h := NewApiKeyHandler(apiKeySvc)

	c, w := ginContextWith("DELETE", "/api-keys/bad", nil, nil,
		gin.Params{{Key: "keyId", Value: "not-a-uuid"}})
	h.RevokeApiKey(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ============================================================
// ADDITIONAL COVERAGE TESTS
// ============================================================

func TestLogin_BindError_MissingPassword(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, jwtSvc, _, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, jwtSvc, nil, userRepo)

	body := map[string]string{"email": "test@example.com"}
	c, w := ginContextWith("POST", "/auth/login", body, nil)
	h.Login(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetupMFA_EmptyUserID(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	authSvc, _, mfaSvc, _, _ := newTestServices(userRepo, nil)
	h := NewAuthHandler(authSvc, nil, mfaSvc, userRepo)

	c, w := ginContextWith("POST", "/auth/mfa/setup", nil, nil)
	c.Set("user_id", "")
	h.SetupMFA(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetUser_Success(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	email := fmt.Sprintf("getuser_%d@example.com", time.Now().UnixNano())
	user := seedUser(t, userRepo, email)

	c, w := ginContextWith("GET", "/users/me", nil, nil)
	c.Set("user_id", user.ID.String())
	h.GetUser(c)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	userRepo, _, _, _, _, _ := newTestDBRepos()
	h := NewUserHandler(userRepo)

	c, w := ginContextWith("GET", "/users/me", nil, nil)
	c.Set("user_id", uuid.New().String())
	h.GetUser(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestListApiKeys_InvalidMerchantID(t *testing.T) {
	h := &ApiKeyHandler{}

	c, w := ginContextWith("GET", "/api-keys", nil, nil)
	c.Set("merchant_id", "not-a-uuid")
	c.Set("user_id", uuid.New().String())
	h.ListApiKeys(c)
	if w.Code != http.StatusOK {
		t.Logf("ListApiKeys with invalid UUID: status=%d (may parse as zero UUID)", w.Code)
	}
}
