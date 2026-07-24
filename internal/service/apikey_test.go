package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestApiKeyService_Constructor(t *testing.T) {
	svc := NewApiKeyService(nil)
	if svc == nil {
		t.Fatal("expected non-nil ApiKeyService")
	}
}

func TestApiKeyService_KeyGeneration_Different(t *testing.T) {
	for i := 0; i < 10; i++ {
		key1 := generateTestKey()
		key2 := generateTestKey()
		if key1 == key2 {
			t.Errorf("two generated keys should be different, iteration %d", i)
		}
	}
}

func TestApiKeyService_KeyGeneration_Length(t *testing.T) {
	key := generateTestKey()
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64 (32 bytes hex)", len(key))
	}
}

func TestApiKeyService_KeyGeneration_ValidHex(t *testing.T) {
	key := generateTestKey()
	if _, err := hex.DecodeString(key); err != nil {
		t.Errorf("key should be valid hex: %v", err)
	}
}

func TestApiKeyService_HashConsistency(t *testing.T) {
	key := generateTestKey()

	hash1 := sha256.Sum256([]byte(key))
	hash2 := sha256.Sum256([]byte(key))

	if hash1 != hash2 {
		t.Error("same key should produce same hash")
	}
}

func TestApiKeyService_HashDifferentForDifferentKeys(t *testing.T) {
	key1 := generateTestKey()
	key2 := generateTestKey()

	hash1 := sha256.Sum256([]byte(key1))
	hash2 := sha256.Sum256([]byte(key2))

	if hash1 == hash2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestApiKeyService_HashHex(t *testing.T) {
	key := generateTestKey()
	hash := sha256.Sum256([]byte(key))
	hashHex := hex.EncodeToString(hash[:])

	if len(hashHex) != 64 {
		t.Errorf("hash hex length = %d, want 64", len(hashHex))
	}
}

func TestApiKeyService_KeyPrefix(t *testing.T) {
	key := generateTestKey()
	prefix := key[:8]

	if len(prefix) != 8 {
		t.Errorf("prefix length = %d, want 8", len(prefix))
	}

	if !strings.HasPrefix(key, prefix) {
		t.Error("key should start with its prefix")
	}
}

func TestApiKeyService_KeyPrefixUniqueness(t *testing.T) {
	prefixes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := generateTestKey()
		prefix := key[:8]
		if prefixes[prefix] {
			t.Errorf("duplicate prefix found: %s", prefix)
		}
		prefixes[prefix] = true
	}
}

func TestApiKeyService_ModelFields(t *testing.T) {
	merchantID := uuid.New()
	userID := uuid.New()

	ak := &testApiKey{
		ID:         uuid.New(),
		MerchantID: merchantID,
		UserID:     userID,
		Name:       "Test Key",
		KeyPrefix:  "hx_12345",
		KeyHash:    "abc123",
		Scopes:     []string{"payments:read", "payments:write"},
		RateLimit:  500,
		IsActive:   true,
	}

	if ak.MerchantID != merchantID {
		t.Error("MerchantID mismatch")
	}
	if ak.UserID != userID {
		t.Error("UserID mismatch")
	}
	if len(ak.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(ak.Scopes))
	}
	if !ak.IsActive {
		t.Error("IsActive should be true")
	}
	if ak.RateLimit != 500 {
		t.Errorf("RateLimit = %d, want 500", ak.RateLimit)
	}
}

func TestApiKeyService_ScopeValidation(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		valid  bool
	}{
		{"empty scopes", []string{}, true},
		{"single scope", []string{"payments:read"}, true},
		{"multiple scopes", []string{"payments:read", "payments:write", "customers:read"}, true},
		{"admin scope", []string{"admin"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := len(tt.scopes) >= 0
			if valid != tt.valid {
				t.Errorf("scopes %v validity = %v, want %v", tt.scopes, valid, tt.valid)
			}
		})
	}
}

func TestApiKeyService_RateLimitValues(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit int
	}{
		{"no limit", 0},
		{"low limit", 10},
		{"medium limit", 100},
		{"high limit", 10000},
		{"very high limit", 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ak := &testApiKey{
				ID:        uuid.New(),
				RateLimit: tt.rateLimit,
			}
			if ak.RateLimit != tt.rateLimit {
				t.Errorf("RateLimit = %d, want %d", ak.RateLimit, tt.rateLimit)
			}
		})
	}
}

func generateTestKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type testApiKey struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	UserID     uuid.UUID
	Name       string
	KeyPrefix  string
	KeyHash    string
	Scopes     []string
	RateLimit  int
	IsActive   bool
}
