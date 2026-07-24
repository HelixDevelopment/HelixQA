package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helix-seller/helix-seller/internal/config"
	"github.com/helix-seller/helix-seller/internal/model"
)

// --- auth.go: VerifyPassword hash-length-mismatch path (lines 49-50) ---
// VerifyPassword hex-decodes the salt and stored hash. The stored hash must
// be 64 hex chars (32 bytes). If the hex string decodes to fewer/more bytes
// than argon2's 32-byte output, VerifyPassword should return false.

func TestVerifyPassword_HashLengthMismatch(t *testing.T) {
	svc := &AuthService{}

	salt := make([]byte, 16)
	rand.Read(salt)

	// Only 4 bytes (8 hex chars) — valid hex but wrong length.
	shortHash := hex.EncodeToString([]byte{1, 2, 3, 4})
	entry := fmt.Sprintf("%s:%s", hex.EncodeToString(salt), shortHash)

	ok, err := svc.VerifyPassword("any-password", entry)
	require.NoError(t, err)
	require.False(t, ok, "should return false when stored hash length != argon2 output length")
}

func TestVerifyPassword_ExtraLongStoredHash(t *testing.T) {
	svc := &AuthService{}

	salt := make([]byte, 16)
	rand.Read(salt)

	// 64 bytes (128 hex chars) — longer than argon2's 32-byte output.
	longHash := hex.EncodeToString(make([]byte, 64))
	entry := fmt.Sprintf("%s:%s", hex.EncodeToString(salt), longHash)

	ok, err := svc.VerifyPassword("any-password", entry)
	require.NoError(t, err)
	require.False(t, ok, "should return false when stored hash is longer than argon2 output")
}

// --- jwt.go: ValidateToken non-RSA signing method (line 93) ---
// ValidateToken's keyfunc only accepts *jwt.SigningMethodRSA. Passing an
// HMAC-signed token should trigger the fmt.Errorf on line 93.

func TestValidateToken_HMACRejected(t *testing.T) {
	svc := newTestJWTService(t)

	// Create a token signed with HMAC (not RSA).
	hmacToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	tokenString, err := hmacToken.SignedString([]byte("shared-secret"))
	require.NoError(t, err)

	claims, err := svc.ValidateToken(tokenString)
	require.Error(t, err)
	require.Nil(t, claims)
	require.Contains(t, err.Error(), "unexpected signing method")
}

// --- jwt.go: ValidateToken expired token (line 102-103) ---
// ValidateToken should reject tokens with expired timestamps.

func TestValidateToken_ExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privKey)
	require.NoError(t, err)

	// Build a JWTService with this key.
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "key.pem")
	pubPath := filepath.Join(tmpDir, "pub.pem")

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	require.NoError(t, os.WriteFile(privPath, privPEM, 0600))

	pubASN1, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0644))

	cfg := &config.Config{
		JWTAccessExpiry:   time.Hour,
		JWTRefreshExpiry:  24 * time.Hour,
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
	}
	js, err := NewJWTService(cfg)
	require.NoError(t, err)

	claims2, err := js.ValidateToken(tokenString)
	require.Error(t, err)
	require.Nil(t, claims2)
	require.Contains(t, err.Error(), "token is expired")
}

// --- jwt.go: NewJWTService non-RSA public key (line 52) ---
// NewJWTService parses the public key and asserts *rsa.PublicKey. An ECDSA
// key passes x509.ParsePKIXPublicKey but fails the type assertion.

func TestNewJWTService_ECDSAPublicKey(t *testing.T) {
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "key.pem")
	pubPath := filepath.Join(tmpDir, "pub.pem")

	// RSA private key (required for private key parsing).
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	require.NoError(t, os.WriteFile(privPath, privPEM, 0600))

	// ECDSA public key — passes ParsePKIXPublicKey but fails the RSA type assertion.
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pubASN1, err := x509.MarshalPKIXPublicKey(&ecdsaKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0644))

	cfg := &config.Config{
		JWTAccessExpiry:   time.Hour,
		JWTRefreshExpiry:  24 * time.Hour,
		JWTPrivateKeyPath: privPath,
		JWTPublicKeyPath:  pubPath,
	}
	js, err := NewJWTService(cfg)
	require.Error(t, err)
	require.Nil(t, js)
	require.Contains(t, err.Error(), "public key is not RSA")
}

// --- webhook.go: send http.NewRequest error path (line 107) ---
// When the webhook URL is so malformed that http.NewRequest fails,
// send should return the parse error without reaching client.Do.

func TestWebhookService_SendNewRequestFails(t *testing.T) {
	svc := &WebhookService{
		client: &http.Client{},
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:       uuid.New(),
		URL:      "://bad-url",
		IsActive: true,
	}

	err := svc.send(config, []byte(`{"event":"test"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing protocol scheme")
}

// --- webhook.go: Deliver with nil configRepo panic path (line 60) ---
// Deliver panics when configRepo is nil. Test that the panic is caught
// and re-panicked properly (the deferred recover handles it).

func TestWebhookService_Deliver_NilConfigRepo(t *testing.T) {
	svc := &WebhookService{
		configRepo: nil,
		logger:     zap.NewNop(),
		client:     &http.Client{},
	}

	require.Panics(t, func() {
		svc.Deliver(nil, uuid.New(), "test.event", map[string]string{"key": "value"})
	})
}

// --- webhook.go: send empty body (line 112-117 when secret is set) ---
// When a webhook config has a secret and body is empty bytes,
// the HMAC branch should still execute without error.

func TestWebhookService_Send_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &WebhookService{
		client: server.Client(),
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:     uuid.New(),
		URL:    server.URL,
		Secret: "test_secret",
	}

	err := svc.send(config, []byte{})
	require.NoError(t, err)
}

// --- Additional VerifyPassword edge cases ---

func TestVerifyPassword_InvalidHexSalt(t *testing.T) {
	svc := &AuthService{}

	_, err := svc.VerifyPassword("password", "zzzz:notahex")
	require.Error(t, err)
}

func TestVerifyPassword_InvalidHexHash(t *testing.T) {
	svc := &AuthService{}

	salt := make([]byte, 16)
	rand.Read(salt)

	_, err := svc.VerifyPassword("password", hex.EncodeToString(salt)+":zzzz")
	require.Error(t, err)
}
