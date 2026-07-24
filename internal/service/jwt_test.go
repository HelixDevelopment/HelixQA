package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/config"
)

func generateTestKeys(t *testing.T) (privPath, pubPath string, cleanup func()) {
	t.Helper()

	dir := t.TempDir()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	privPath = filepath.Join(dir, "private.pem")
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	pubPath = filepath.Join(dir, "public.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	cleanup = func() {}
	return privPath, pubPath, cleanup
}

func newTestJWTService(t *testing.T) *JWTService {
	t.Helper()
	privPath, pubPath, cleanup := generateTestKeys(t)
	t.Cleanup(cleanup)

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

func TestNewJWTService(t *testing.T) {
	svc := newTestJWTService(t)
	if svc == nil {
		t.Fatal("expected non-nil JWTService")
	}
	if svc.privateKey == nil {
		t.Error("privateKey not set")
	}
	if svc.publicKey == nil {
		t.Error("publicKey not set")
	}
}

func TestNewJWTService_MissingPrivateKey(t *testing.T) {
	cfg := &config.Config{
		JWTPrivateKeyPath: "/nonexistent/private.pem",
		JWTPublicKeyPath:  "/nonexistent/public.pem",
	}
	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("expected error for missing private key")
	}
}

func TestGenerateAccessToken(t *testing.T) {
	svc := newTestJWTService(t)

	userID := uuid.New()
	merchantID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, "test@example.com", "admin", merchantID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := newTestJWTService(t)

	userID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	svc := newTestJWTService(t)

	userID := uuid.New()
	merchantID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, "test@example.com", "user", merchantID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims["email"] != "test@example.com" {
		t.Errorf("email = %v, want test@example.com", claims["email"])
	}
	if claims["role"] != "user" {
		t.Errorf("role = %v, want user", claims["role"])
	}
	if claims["token_type"] != "access" {
		t.Errorf("token_type = %v, want access", claims["token_type"])
	}
}

func TestValidateToken_RefreshToken(t *testing.T) {
	svc := newTestJWTService(t)

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
}

func TestValidateToken_InvalidToken(t *testing.T) {
	svc := newTestJWTService(t)

	_, err := svc.ValidateToken("invalid.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateToken_TamperedToken(t *testing.T) {
	svc := newTestJWTService(t)

	userID := uuid.New()
	merchantID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, "test@example.com", "admin", merchantID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	tampered := token + "x"
	_, err = svc.ValidateToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestPublicKey(t *testing.T) {
	svc := newTestJWTService(t)

	pub := svc.PublicKey()
	if pub == nil {
		t.Fatal("expected non-nil public key")
	}
}
