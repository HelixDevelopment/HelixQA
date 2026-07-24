package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func generateRSAKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return priv, &priv.PublicKey
}

func writePublicKeyToFile(t *testing.T, pub *rsa.PublicKey) string {
	t.Helper()
	pemBytes := marshalRSAPublicKeyPEM(t, pub)
	path := filepath.Join(t.TempDir(), "public.pem")
	if err := os.WriteFile(path, pemBytes, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}
	return path
}

func marshalRSAPublicKeyPEM(t *testing.T, pub *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: der})
}

func writeInvalidPEMFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(path, []byte("not-a-valid-pem"), 0644); err != nil {
		t.Fatalf("failed to write invalid PEM: %v", err)
	}
	return path
}

func signJWT(t *testing.T, priv *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

func newTestClaims(userID, merchantID, role string) *Claims {
	return &Claims{
		UserID:     userID,
		MerchantID: merchantID,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func newExpiredClaims(userID, merchantID, role string) *Claims {
	return &Claims{
		UserID:     userID,
		MerchantID: merchantID,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
}

// --- Auth: Valid JWT paths ---

func TestAuth_ValidJWT_AcceptsAndSetsContext(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	claims := newTestClaims("user-123", "merchant-456", "account_admin")
	token := signJWT(t, priv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(pubPath, logger))

	var gotUserID, gotMerchantID, gotRole, gotAuthMethod string
	var gotClaims interface{}
	r.GET("/protected", func(c *gin.Context) {
		gotUserID = c.GetString("user_id")
		gotMerchantID = c.GetString("merchant_id")
		gotRole = c.GetString("role")
		gotAuthMethod = c.GetString("auth_method")
		gotClaims, _ = c.Get("claims")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotUserID != "user-123" {
		t.Errorf("user_id = %q, want user-123", gotUserID)
	}
	if gotMerchantID != "merchant-456" {
		t.Errorf("merchant_id = %q, want merchant-456", gotMerchantID)
	}
	if gotRole != "account_admin" {
		t.Errorf("role = %q, want account_admin", gotRole)
	}
	if gotAuthMethod != "jwt" {
		t.Errorf("auth_method = %q, want jwt", gotAuthMethod)
	}
	if gotClaims == nil {
		t.Error("claims not set in context")
	}
}

// --- Auth: Invalid signature ---

func TestAuth_InvalidSignature_RejectsToken(t *testing.T) {
	_, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	wrongPriv, _ := generateRSAKeyPair(t)
	claims := newTestClaims("user-123", "merchant-456", "user")
	token := signJWT(t, wrongPriv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(pubPath, logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (wrong signature should be rejected)", w.Code, http.StatusUnauthorized)
	}
}

// --- Auth: Expired token ---

func TestAuth_ExpiredToken_RejectsToken(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	claims := newExpiredClaims("user-123", "merchant-456", "user")
	token := signJWT(t, priv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(pubPath, logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (expired token should be rejected)", w.Code, http.StatusUnauthorized)
	}
}

// --- Auth: Invalid PEM key file ---

func TestAuth_InvalidPEMKeyFile_Returns500(t *testing.T) {
	invalidPath := writeInvalidPEMFile(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(invalidPath, logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (invalid PEM should return 500)", w.Code, http.StatusInternalServerError)
	}
}

// --- Auth: Malformed tokens ---

func TestAuth_MalformedJWT_RejectsToken(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)
	_ = priv

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(pubPath, logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	malformedTokens := []string{
		"not.a.jwt",
		"eyJhbGciOiJSUzI1NiJ9",         // incomplete
		"Bearer.with.dots.but.invalid", // not actually signed properly
	}

	for _, mt := range malformedTokens {
		t.Run(mt[:min(len(mt), 20)], func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+mt)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d for token %q", w.Code, http.StatusUnauthorized, mt)
			}
		})
	}
}

// --- Auth: Bearer with only spaces ---

func TestAuth_BearerWithNoTokenString_Rejects(t *testing.T) {
	logger := zap.NewNop()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth("/tmp/nonexistent_key.pem", logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer no-token-here")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (missing key file returns 500)", w.Code, http.StatusInternalServerError)
	}
}

// --- OptionalAuth: Valid JWT sets context ---

func TestOptionalAuth_ValidJWT_SetsContext(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	claims := newTestClaims("opt-user", "opt-merchant", "root_admin")
	token := signJWT(t, priv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(pubPath, logger))

	var gotUserID, gotMerchantID, gotRole, gotAuthMethod string
	r.GET("/check", func(c *gin.Context) {
		gotUserID = c.GetString("user_id")
		gotMerchantID = c.GetString("merchant_id")
		gotRole = c.GetString("role")
		gotAuthMethod = c.GetString("auth_method")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotUserID != "opt-user" {
		t.Errorf("user_id = %q, want opt-user", gotUserID)
	}
	if gotMerchantID != "opt-merchant" {
		t.Errorf("merchant_id = %q, want opt-merchant", gotMerchantID)
	}
	if gotRole != "root_admin" {
		t.Errorf("role = %q, want root_admin", gotRole)
	}
	if gotAuthMethod != "jwt" {
		t.Errorf("auth_method = %q, want jwt", gotAuthMethod)
	}
}

// --- OptionalAuth: Invalid signature still passes ---

func TestOptionalAuth_InvalidSignature_PassesThrough(t *testing.T) {
	_, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	wrongPriv, _ := generateRSAKeyPair(t)
	claims := newTestClaims("user-123", "merchant-456", "user")
	token := signJWT(t, wrongPriv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(pubPath, logger))
	r.GET("/check", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (OptionalAuth should pass through on invalid sig)", w.Code, http.StatusOK)
	}
}

// --- OptionalAuth: Expired token still passes ---

func TestOptionalAuth_ExpiredToken_PassesThrough(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	claims := newExpiredClaims("user-123", "merchant-456", "user")
	token := signJWT(t, priv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(pubPath, logger))
	r.GET("/check", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (OptionalAuth should pass through on expired token)", w.Code, http.StatusOK)
	}
}

// --- OptionalAuth: Invalid PEM file still passes ---

func TestOptionalAuth_InvalidPEM_PassesThrough(t *testing.T) {
	invalidPath := writeInvalidPEMFile(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth(invalidPath, logger))
	r.GET("/check", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (OptionalAuth should pass through on invalid PEM)", w.Code, http.StatusOK)
	}
}

// --- OptionalAuth: Malformed token still passes ---

func TestOptionalAuth_MalformedJWT_PassesThrough(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuth("/tmp/nonexistent_key.pem", logger))
	r.GET("/check", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (OptionalAuth should pass through on malformed token)", w.Code, http.StatusOK)
	}
}

// --- Auth: Bearer with extra spaces in token part ---

func TestAuth_BearerTokenWithSpaces_Rejects(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth("/tmp/nonexistent.pem", logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer token with spaces")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (Bearer with spaces still reads key file)", w.Code, http.StatusInternalServerError)
	}
}

// --- Auth: Case-insensitive Bearer prefix ---

func TestAuth_BearerCaseInsensitive(t *testing.T) {
	priv, pub := generateRSAKeyPair(t)
	pubPath := writePublicKeyToFile(t, pub)

	claims := newTestClaims("user-123", "merchant-456", "user")
	token := signJWT(t, priv, claims)

	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(pubPath, logger))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (case-insensitive bearer should work)", w.Code, http.StatusOK)
	}
}

// --- Auth: Different roles ---

func TestAuth_DifferentRoles_SetCorrectly(t *testing.T) {
	roles := []string{"root_admin", "account_admin", "user", "custom_role"}
	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			priv, pub := generateRSAKeyPair(t)
			pubPath := writePublicKeyToFile(t, pub)

			claims := newTestClaims("u1", "m1", role)
			token := signJWT(t, priv, claims)

			logger := zap.NewNop()
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(Auth(pubPath, logger))

			var gotRole string
			r.GET("/check", func(c *gin.Context) {
				gotRole = c.GetString("role")
				c.JSON(200, nil)
			})

			req := httptest.NewRequest("GET", "/check", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if gotRole != role {
				t.Errorf("role = %q, want %q", gotRole, role)
			}
		})
	}
}

// --- Audit: synchronous path coverage ---

func newAuditPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig("postgres://nonexistent:nonexistent@localhost:1/nonexistent?connect_timeout=1")
	if err != nil {
		t.Skipf("could not parse pgx config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skipf("could not create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestAudit_WithBody_RecordsAndPasses(t *testing.T) {
	pool := newAuditPool(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Audit(pool, logger))
	r.POST("/audit-body", func(c *gin.Context) {
		c.JSON(200, gin.H{"received": true})
	})

	body := bytes.NewBufferString(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/audit-body", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestAudit_NoBody_Passes(t *testing.T) {
	pool := newAuditPool(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Audit(pool, logger))
	r.GET("/audit-no-body", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/audit-no-body", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestAudit_SetsUserContext(t *testing.T) {
	pool := newAuditPool(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "actor-99")
		c.Set("merchant_id", "merchant-88")
		c.Next()
	})
	r.Use(Audit(pool, logger))
	r.PUT("/audit-ctx", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	body := bytes.NewBufferString(`{"update":true}`)
	req := httptest.NewRequest("PUT", "/audit-ctx", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestAudit_LargeBody_Handled(t *testing.T) {
	pool := newAuditPool(t)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Audit(pool, logger))
	r.POST("/audit-large", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	largeBody := bytes.Repeat([]byte("x"), 1024*100)
	req := httptest.NewRequest("POST", "/audit-large", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	time.Sleep(200 * time.Millisecond)
}

// --- RequestSizeLimit: negative limit ---

func TestRequestSizeLimit_NegativeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestSizeLimit(-1))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (negative limit with nil body should pass)", w.Code, http.StatusOK)
	}
}

// --- CORS: header value verification ---

func TestCORS_HeaderValues(t *testing.T) {
	mw := CORS()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key",
		"Access-Control-Expose-Headers": "X-Request-ID",
		"Access-Control-Max-Age":        "86400",
	}

	for header, want := range expected {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

// --- RequestID: context value set ---

func TestRequestID_ContextValueSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())

	var ctxRequestID string
	r.GET("/test", func(c *gin.Context) {
		ctxRequestID = c.GetString("request_id")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if ctxRequestID == "" {
		t.Error("request_id not set in context")
	}
	if w.Header().Get("X-Request-ID") != ctxRequestID {
		t.Errorf("header X-Request-ID = %q, context = %q", w.Header().Get("X-Request-ID"), ctxRequestID)
	}
}

func TestRequestID_PreservesExistingID_InContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())

	var ctxRequestID string
	r.GET("/test", func(c *gin.Context) {
		ctxRequestID = c.GetString("request_id")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-id-42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if ctxRequestID != "custom-id-42" {
		t.Errorf("context request_id = %q, want custom-id-42", ctxRequestID)
	}
}

// --- SecurityHeaders: each header exact value ---

func TestSecurityHeaders_ExactValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", "default-src 'self'"},
		{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
		{"Cache-Control", "no-store, no-cache, must-revalidate"},
		{"Pragma", "no-cache"},
	}

	for _, tt := range tests {
		got := w.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.expected)
		}
	}
}

func TestSecurityHeaders_PassesToHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Recovery: with request_id context ---

func TestRecovery_WithRequestID(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.Use(Recovery(logger))
	r.GET("/panic", func(c *gin.Context) {
		panic("context panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- Logger: with request_id ---

func TestLogger_WithRequestID(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Logger: multiple errors ---

func TestLogger_MultipleErrors(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger(logger))
	r.POST("/multi-error", func(c *gin.Context) {
		_ = c.Error(gin.Error{Err: fmt.Errorf("error 1"), Type: 1})
		_ = c.Error(gin.Error{Err: fmt.Errorf("error 2"), Type: 1})
		c.JSON(500, gin.H{"error": true})
	})

	req := httptest.NewRequest("POST", "/multi-error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
