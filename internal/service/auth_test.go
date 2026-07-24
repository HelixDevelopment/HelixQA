package service

import (
	"testing"
	"strings"
)

func TestHashPassword(t *testing.T) {
	svc := &AuthService{}

	hash, err := svc.HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Hash should contain separator
	if !strings.Contains(hash, ":") {
		t.Fatal("hash should contain ':' separator")
	}

	// Salt and hash parts should be hex
	parts := strings.Split(hash, ":")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	// Salt should be 32 hex chars (16 bytes)
	if len(parts[0]) != 32 {
		t.Errorf("salt hex length = %d, want 32", len(parts[0]))
	}

	// Hash should be 64 hex chars (32 bytes)
	if len(parts[1]) != 64 {
		t.Errorf("hash hex length = %d, want 64", len(parts[1]))
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	svc := &AuthService{}
	password := "mysecretpassword"

	hash, err := svc.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	ok, err := svc.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword should return true for correct password")
	}
}

func TestVerifyPassword_Incorrect(t *testing.T) {
	svc := &AuthService{}

	hash, err := svc.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	ok, err := svc.VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword should return false for incorrect password")
	}
}

func TestVerifyPassword_InvalidFormat(t *testing.T) {
	svc := &AuthService{}

	tests := []struct {
		name string
		hash string
	}{
		{"no separator", "abc123"},
		{"empty", ""},
		{"single part", "abc123def456"},
		{"invalid hex salt", "xyz:abc123"},
		{"invalid hex hash", "abc123:xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.VerifyPassword("password", tt.hash)
			if err == nil {
				t.Fatal("expected error for invalid hash format")
			}
		})
	}
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	svc := &AuthService{}
	password := "samepassword"

	hash1, _ := svc.HashPassword(password)
	hash2, _ := svc.HashPassword(password)

	if hash1 == hash2 {
		t.Fatal("two hashes of same password should differ (different salts)")
	}

	// But both should verify correctly
	ok1, _ := svc.VerifyPassword(password, hash1)
	ok2, _ := svc.VerifyPassword(password, hash2)
	if !ok1 || !ok2 {
		t.Fatal("both hashes should verify correctly")
	}
}
