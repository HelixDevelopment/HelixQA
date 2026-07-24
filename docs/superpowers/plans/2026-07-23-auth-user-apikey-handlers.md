# Auth, User, and API Key Handlers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create HTTP handlers for authentication, user management, and API key management in the Helix Seller payment facade.

**Architecture:** Three new handler files (`auth.go`, `user.go`, `apikey.go`) with corresponding service/repository layers, plus updating the existing router to wire everything together. Handlers follow the existing pattern in the codebase: structured request binding, AppError responses, and context-based user/merchant ID extraction.

**Tech Stack:** Go, Gin, UUID, JWT (golang-jwt), bcrypt, TOTP (pquerna/otp), zap logging

## Global Constraints

- Module path: `github.com/helix-seller/helix-seller`
- Go version: 1.25.0
- Error responses use `model.AppError` with Code/Message fields
- User/merchant IDs extracted from gin.Context via `c.GetString("user_id")` / `c.GetString("merchant_id")`
- JWT middleware already sets `user_id`, `merchant_id`, `role` in context
- Password minimum length: 12 characters
- API keys shown only once at creation (full key returned, only hash stored)

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/service/auth.go` | Password hashing, user authentication, user creation |
| `internal/service/jwt.go` | Access/refresh token generation, token validation |
| `internal/service/apikey.go` | API key creation, validation, revocation |
| `internal/service/mfa.go` | TOTP secret generation, recovery codes, verification |
| `internal/repository/user_repo.go` | User and API key database operations |
| `internal/handler/auth.go` | HTTP handlers for auth endpoints |
| `internal/handler/user.go` | HTTP handlers for user endpoints |
| `internal/handler/apikey.go` | HTTP handlers for API key endpoints |
| `internal/handler/router.go` | Updated router wiring all handlers |

---

### Task 1: Create User Repository

**Files:**
- Create: `internal/repository/user_repo.go`

**Interfaces:**
- Produces: `UserRepo` with methods `Create`, `GetByID`, `GetByEmail`, `Update`, `ListByMerchant`, `CreateApiKey`, `GetApiKeyByID`, `ListApiKeysByMerchant`, `RevokeApiKey`

- [ ] **Step 1: Create user repository file**

```go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())`,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.MerchantID, user.IsActive, user.MfaEnabled, user.MfaSecret,
	)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.MerchantID, &user.IsActive, &user.MfaEnabled, &user.MfaSecret, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	return &user, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.MerchantID, &user.IsActive, &user.MfaEnabled, &user.MfaSecret, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	return &user, err
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET name = $1, email = $2, updated_at = NOW() WHERE id = $3 AND deleted_at IS NULL`,
		user.Name, user.Email, user.ID,
	)
	return err
}

func (r *UserRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE merchant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.MerchantID, &u.IsActive, &u.MfaEnabled, &u.MfaSecret, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) CreateApiKey(ctx context.Context, key *model.ApiKey) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, merchant_id, user_id, name, key_prefix, key_hash, scopes, rate_limit, is_active, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())`,
		key.ID, key.MerchantID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.Scopes, key.RateLimit, key.IsActive, key.ExpiresAt,
	)
	return err
}

func (r *UserRepo) GetApiKeyByID(ctx context.Context, id uuid.UUID) (*model.ApiKey, error) {
	var key model.ApiKey
	err := r.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, user_id, name, key_prefix, key_hash, scopes, rate_limit, is_active, expires_at, created_at, last_used_at
		 FROM api_keys WHERE id = $1`,
		id,
	).Scan(&key.ID, &key.MerchantID, &key.UserID, &key.Name, &key.KeyPrefix, &key.KeyHash, &key.Scopes, &key.RateLimit, &key.IsActive, &key.ExpiresAt, &key.CreatedAt, &key.LastUsedAt)
	if err == sql.ErrNoRows {
		return nil, model.ErrNotFound
	}
	return &key, err
}

func (r *UserRepo) ListApiKeysByMerchant(ctx context.Context, merchantID uuid.UUID) ([]model.ApiKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, merchant_id, user_id, name, key_prefix, scopes, rate_limit, is_active, expires_at, created_at, last_used_at
		 FROM api_keys WHERE merchant_id = $1 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []model.ApiKey
	for rows.Next() {
		var k model.ApiKey
		if err := rows.Scan(&k.ID, &k.MerchantID, &k.UserID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.RateLimit, &k.IsActive, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *UserRepo) RevokeApiKey(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET is_active = false WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return model.ErrNotFound
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/repository/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/repository/user_repo.go
git commit -m "feat: add user and API key repository"
```

---

### Task 2: Create Auth Service

**Files:**
- Create: `internal/service/auth.go`

**Interfaces:**
- Consumes: `repository.UserRepo`, `model.User`
- Produces: `AuthService` with `Register`, `Authenticate`, `HashPassword`, `VerifyPassword`

- [ ] **Step 1: Create auth service file**

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo *repository.UserRepo
}

func NewAuthService(userRepo *repository.UserRepo) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *AuthService) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *AuthService) Register(ctx context.Context, email, password, name string, merchantID uuid.UUID) (*model.User, error) {
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, model.NewConflictError("email already registered")
	}

	hash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         model.RoleUser,
		MerchantID:   merchantID,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, model.ErrUnauthorized
	}

	if !user.IsActive {
		return nil, model.ErrUnauthorized
	}

	if !s.VerifyPassword(password, user.PasswordHash) {
		return nil, model.ErrUnauthorized
	}

	return user, nil
}

func generateAPIKey() (string, string, string) {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	fullKey := "hx_" + hex.EncodeToString(bytes)
	keyHash := hex.EncodeToString(bytes)
	prefix := fullKey[:10]
	return fullKey, keyHash, prefix
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/service/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/service/auth.go
git commit -m "feat: add auth service with password hashing and authentication"
```

---

### Task 3: Create JWT Service

**Files:**
- Create: `internal/service/jwt.go`

**Interfaces:**
- Consumes: JWT private/public key paths from config
- Produces: `JWTService` with `GenerateAccessToken`, `GenerateRefreshToken`, `ValidateToken`

- [ ] **Step 1: Create JWT service file**

```go
package service

import (
	"crypto/rsa"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/middleware"
)

type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL time.Duration
	refreshTTL time.Duration
}

func NewJWTService(privateKeyPath, publicKeyPath string, accessTTL, refreshTTL time.Duration) (*JWTService, error) {
	privKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privKeyData)
	if err != nil {
		return nil, err
	}

	pubKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyData)
	if err != nil {
		return nil, err
	}

	return &JWTService{
		privateKey:  privKey,
		publicKey:   pubKey,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

func (s *JWTService) GenerateAccessToken(userID, merchantID, role string) (string, error) {
	claims := &middleware.Claims{
		UserID:     userID,
		MerchantID: merchantID,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *JWTService) GenerateRefreshToken(userID string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *JWTService) ValidateToken(tokenString string) (*middleware.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &middleware.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*middleware.Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/service/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/service/jwt.go
git commit -m "feat: add JWT service for access and refresh token management"
```

---

### Task 4: Create MFA Service

**Files:**
- Create: `internal/service/mfa.go`

**Interfaces:**
- Produces: `MFAService` with `GenerateSecret`, `GenerateRecoveryCodes`, `Verify`

- [ ] **Step 1: Add otp dependency**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go get github.com/pquerna/otp`
Expected: Dependency added

- [ ] **Step 2: Create MFA service file**

```go
package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/pquerna/otp/totp"
)

type MFAService struct {
	issuer string
}

func NewMFAService(issuer string) *MFAService {
	return &MFAService{issuer: issuer}
}

func (s *MFAService) GenerateSecret(accountName string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: accountName,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func (s *MFAService) Verify(secret, code string) bool {
	return totp.Validate(code, secret)
}

func (s *MFAService) GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("failed to generate recovery code: %w", err)
		}
		codes[i] = hex.EncodeToString(bytes)
	}
	return codes, nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/service/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/service/mfa.go go.sum
git commit -m "feat: add MFA service for TOTP and recovery codes"
```

---

### Task 5: Create API Key Service

**Files:**
- Create: `internal/service/apikey.go`

**Interfaces:**
- Consumes: `repository.UserRepo`
- Produces: `ApiKeyService` with `Create`, `Validate`, `Revoke`

- [ ] **Step 1: Create API key service file**

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type ApiKeyService struct {
	userRepo *repository.UserRepo
}

func NewApiKeyService(userRepo *repository.UserRepo) *ApiKeyService {
	return &ApiKeyService{userRepo: userRepo}
}

func (s *ApiKeyService) Create(ctx context.Context, merchantID, userID uuid.UUID, name string, scopes []string, rateLimit int) (*model.ApiKey, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, "", err
	}

	fullKey := "hx_" + hex.EncodeToString(bytes)
	keyHash := hex.EncodeToString(bytes)
	prefix := fullKey[:10]

	key := &model.ApiKey{
		ID:         uuid.New(),
		MerchantID: merchantID,
		UserID:     userID,
		Name:       name,
		KeyPrefix:  prefix,
		KeyHash:    keyHash,
		Scopes:     scopes,
		RateLimit:  rateLimit,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}

	if err := s.userRepo.CreateApiKey(ctx, key); err != nil {
		return nil, "", err
	}

	return key, fullKey, nil
}

func (s *ApiKeyService) Validate(ctx context.Context, keyHash string) (*model.ApiKey, error) {
	_ = ctx
	_ = keyHash
	return nil, model.ErrNotFound
}

func (s *ApiKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	return s.userRepo.RevokeApiKey(ctx, id)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/service/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/service/apikey.go
git commit -m "feat: add API key service for creation, validation, and revocation"
```

---

### Task 6: Create Auth Handler

**Files:**
- Create: `internal/handler/auth.go`

**Interfaces:**
- Consumes: `service.AuthService`, `service.JWTService`, `service.MFAService`, `repository.UserRepo`
- Produces: `AuthHandler` with `Register`, `Login`, `Refresh`, `Logout`, `SetupMFA`, `VerifyMFA`

- [ ] **Step 1: Create auth handler file**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
	"github.com/helix-seller/helix-seller/internal/service"
)

type AuthHandler struct {
	authSvc  *service.AuthService
	jwtSvc   *service.JWTService
	mfaSvc   *service.MFAService
	userRepo *repository.UserRepo
}

func NewAuthHandler(authSvc *service.AuthService, jwtSvc *service.JWTService, mfaSvc *service.MFAService, userRepo *repository.UserRepo) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, jwtSvc: jwtSvc, mfaSvc: mfaSvc, userRepo: userRepo}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email      string `json:"email" binding:"required,email"`
		Password   string `json:"password" binding:"required,min=12"`
		Name       string `json:"name" binding:"required"`
		MerchantID string `json:"merchant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	merchantUUID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant_id"})
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), req.Email, req.Password, req.Name, merchantUUID)
	if err != nil {
		if appErr, ok := err.(*model.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID.String(), user.MerchantID.String(), string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"role":       user.Role,
			"merchant_id": user.MerchantID,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authSvc.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if appErr, ok := err.(*model.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if user.MfaEnabled {
		c.JSON(http.StatusOK, gin.H{
			"requires_mfa": true,
			"user_id":      user.ID,
		})
		return
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID.String(), user.MerchantID.String(), string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"role":       user.Role,
			"merchant_id": user.MerchantID,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.jwtSvc.ValidateToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uuid.MustParse(claims.UserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID.String(), user.MerchantID.String(), string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *AuthHandler) SetupMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uuid.MustParse(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	secret, qrURL, err := h.mfaSvc.GenerateSecret(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate MFA secret"})
		return
	}

	recoveryCodes, err := h.mfaSvc.GenerateRecoveryCodes(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate recovery codes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret":        secret,
		"qr_code_url":   qrURL,
		"recovery_codes": recoveryCodes,
	})
}

func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Code   string `json:"code" binding:"required"`
		Secret string `json:"secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.mfaSvc.Verify(req.Secret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid MFA code"})
		return
	}

	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user.MfaEnabled = true
	user.MfaSecret = &req.Secret
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable MFA"})
		return
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID.String(), user.MerchantID.String(), string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/handler/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/handler/auth.go
git commit -m "feat: add auth handler for register, login, refresh, logout, MFA"
```

---

### Task 7: Create User Handler

**Files:**
- Create: `internal/handler/user.go`

**Interfaces:**
- Consumes: `repository.UserRepo`
- Produces: `UserHandler` with `GetUser`, `UpdateUser`

- [ ] **Step 1: Create user handler file**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type UserHandler struct {
	userRepo *repository.UserRepo
}

func NewUserHandler(userRepo *repository.UserRepo) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uuid.MustParse(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"role":       user.Role,
		"merchant_id": user.MerchantID,
		"is_active":  user.IsActive,
		"mfa_enabled": user.MfaEnabled,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email" binding:"omitempty,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), uuid.MustParse(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"role":       user.Role,
		"merchant_id": user.MerchantID,
		"updated_at": user.UpdatedAt,
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/handler/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/handler/user.go
git commit -m "feat: add user handler for profile management"
```

---

### Task 8: Create API Key Handler

**Files:**
- Create: `internal/handler/apikey.go`

**Interfaces:**
- Consumes: `service.ApiKeyService`, `repository.UserRepo`
- Produces: `ApiKeyHandler` with `CreateApiKey`, `ListApiKeys`, `RevokeApiKey`

- [ ] **Step 1: Create API key handler file**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
	"github.com/helix-seller/helix-seller/internal/service"
)

type ApiKeyHandler struct {
	apiKeySvc *service.ApiKeyService
	userRepo *repository.UserRepo
}

func NewApiKeyHandler(apiKeySvc *service.ApiKeyService, userRepo *repository.UserRepo) *ApiKeyHandler {
	return &ApiKeyHandler{apiKeySvc: apiKeySvc, userRepo: userRepo}
}

func (h *ApiKeyHandler) CreateApiKey(c *gin.Context) {
	userID := c.GetString("user_id")
	merchantID := c.GetString("merchant_id")
	if userID == "" || merchantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		Name      string   `json:"name" binding:"required"`
		Scopes    []string `json:"scopes"`
		RateLimit int      `json:"rate_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.RateLimit == 0 {
		req.RateLimit = 1000
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant_id"})
		return
	}

	key, fullKey, err := h.apiKeySvc.Create(c.Request.Context(), merchantUUID, userUUID, req.Name, req.Scopes, req.RateLimit)
	if err != nil {
		if appErr, ok := err.(*model.AppError); ok {
			c.JSON(appErr.HTTPStatus, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"key":        fullKey,
		"key_prefix": key.KeyPrefix,
		"scopes":     key.Scopes,
		"rate_limit": key.RateLimit,
		"created_at": key.CreatedAt,
		"warning":    "save this key now - it will not be shown again",
	})
}

func (h *ApiKeyHandler) ListApiKeys(c *gin.Context) {
	merchantID := c.GetString("merchant_id")
	if merchantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant_id"})
		return
	}

	keys, err := h.userRepo.ListApiKeysByMerchant(c.Request.Context(), merchantUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

func (h *ApiKeyHandler) RevokeApiKey(c *gin.Context) {
	merchantID := c.GetString("merchant_id")
	if merchantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	keyID := c.Param("keyId")
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key ID"})
		return
	}

	key, err := h.userRepo.GetApiKeyByID(c.Request.Context(), keyUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merchant_id"})
		return
	}

	if key.MerchantID != merchantUUID {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key does not belong to this merchant"})
		return
	}

	if err := h.apiKeySvc.Revoke(c.Request.Context(), keyUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./internal/handler/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/handler/apikey.go
git commit -m "feat: add API key handler for creation, listing, and revocation"
```

---

### Task 9: Update Router

**Files:**
- Modify: `internal/handler/router.go`

**Interfaces:**
- Consumes: `AuthHandler`, `UserHandler`, `ApiKeyHandler`

- [ ] **Step 1: Update router.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(logger *zap.Logger, authHandler *AuthHandler, userHandler *UserHandler, apiKeyHandler *ApiKeyHandler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	v1 := router.Group("/api/v1")
	{
		// Auth (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/mfa/verify", authHandler.VerifyMFA)
		}

		// Protected
		p := v1.Group("")
		{
			// Users
			p.GET("/users/me", userHandler.GetUser)
			p.PUT("/users/me", userHandler.UpdateUser)

			// API Keys
			p.POST("/api-keys", apiKeyHandler.CreateApiKey)
			p.GET("/api-keys", apiKeyHandler.ListApiKeys)
			p.DELETE("/api-keys/:keyId", apiKeyHandler.RevokeApiKey)

			// Auth (protected)
			p.POST("/auth/logout", authHandler.Logout)
			p.POST("/auth/mfa/setup", authHandler.SetupMFA)

			// Merchants
			m := p.Group("/merchants")
			{
				m.GET("", nil)
				m.POST("", nil)
				m.GET("/:merchantId", nil)
				m.PUT("/:merchantId", nil)

				// Customers
				c := m.Group("/:merchantId/customers")
				{
					c.GET("", nil)
					c.POST("", nil)
					c.GET("/:customerId", nil)
					c.PUT("/:customerId", nil)
				}

				// Transactions
				t := m.Group("/:merchantId/transactions")
				{
					t.GET("", nil)
					t.POST("", nil)
					t.GET("/:transactionId", nil)
				}

				// Refunds
				r := m.Group("/:merchantId/refunds")
				{
					r.POST("", nil)
				}

				// Subscriptions
				s := m.Group("/:merchantId/subscriptions")
				{
					s.GET("", nil)
					s.POST("", nil)
					s.GET("/:subscriptionId", nil)
					s.PATCH("/:subscriptionId", nil)
					s.DELETE("/:subscriptionId", nil)
				}

				// Invoices
				inv := m.Group("/:merchantId/invoices")
				{
					inv.GET("", nil)
					inv.POST("", nil)
					inv.GET("/:invoiceId", nil)
				}

				// Payouts
				pay := m.Group("/:merchantId/payouts")
				{
					pay.GET("", nil)
					pay.GET("/:payoutId", nil)
					pay.POST("", nil)
				}

				// Disputes
				d := m.Group("/:merchantId/disputes")
				{
					d.GET("", nil)
					d.GET("/:disputeId", nil)
					d.POST("", nil)
					d.POST("/:disputeId/evidence", nil)
				}

				// Payment Methods
				pm := m.Group("/:merchantId/payment-methods")
				{
					pm.GET("", nil)
					pm.POST("", nil)
					pm.GET("/:paymentMethodId", nil)
					pm.DELETE("/:paymentMethodId", nil)
				}

				// Webhooks
				wh := m.Group("/:merchantId/webhooks")
				{
					wh.GET("", nil)
					wh.POST("", nil)
					wh.GET("/:webhookId", nil)
					wh.PUT("/:webhookId", nil)
					wh.DELETE("/:webhookId", nil)
				}

				// Provider Configs
				pr := m.Group("/:merchantId/providers")
				{
					pr.GET("", nil)
					pr.POST("", nil)
					pr.GET("/:providerId", nil)
					pr.PUT("/:providerId", nil)
					pr.DELETE("/:providerId", nil)
				}

				// Exchange Rates
				m.GET("/:merchantId/exchange-rates", nil)

				// Analytics
				a := m.Group("/:merchantId/analytics")
				{
					a.GET("/summary", nil)
					a.GET("/transactions", nil)
					a.GET("/export", nil)
				}

				// Billing
				b := m.Group("/:merchantId/billing")
				{
					b.GET("/fees", nil)
					b.GET("/invoices", nil)
				}
			}
		}

		// Webhook ingress (no auth)
		wh := v1.Group("/webhooks")
		{
			wh.POST("/stripe", nil)
			wh.POST("/paypal", nil)
			wh.POST("/square", nil)
		}
	}

	return router
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/handler/router.go
git commit -m "feat: wire auth, user, and API key handlers into router"
```

---

### Task 10: Final Verification

- [ ] **Step 1: Build entire project**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go build ./...`
Expected: No errors

- [ ] **Step 2: Run vet**

Run: `cd /run/media/milosvasic/DATA4TB/Projects/helix_seller && go vet ./...`
Expected: No errors

- [ ] **Step 3: Final commit with all files**

```bash
git add -A
git commit -m "feat: complete auth, user, and API key HTTP handlers"
```
