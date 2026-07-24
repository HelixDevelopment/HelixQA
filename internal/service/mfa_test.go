package service

import (
	"testing"
	"strings"
)

func TestGenerateSecret(t *testing.T) {
	svc := NewMFAService()

	secret, err := svc.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if len(secret) == 0 {
		t.Fatal("secret should not be empty")
	}

	// Should be valid base32
	if strings.Contains(secret, " ") {
		t.Fatal("secret should not contain spaces")
	}
}

func TestGenerateSecret_Unique(t *testing.T) {
	svc := NewMFAService()

	s1, _ := svc.GenerateSecret()
	s2, _ := svc.GenerateSecret()

	if s1 == s2 {
		t.Fatal("two generated secrets should be different")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	svc := NewMFAService()

	codes, err := svc.GenerateRecoveryCodes(8)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes failed: %v", err)
	}

	if len(codes) != 8 {
		t.Fatalf("expected 8 codes, got %d", len(codes))
	}

	for i, code := range codes {
		if len(code) == 0 {
			t.Errorf("code %d should not be empty", i)
		}
	}
}

func TestGenerateRecoveryCodes_Unique(t *testing.T) {
	svc := NewMFAService()

	codes, _ := svc.GenerateRecoveryCodes(10)

	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Fatalf("duplicate recovery code: %s", code)
		}
		seen[code] = true
	}
}

func TestTotpURL(t *testing.T) {
	svc := NewMFAService()

	url := svc.TotpURL("HelixSeller", "user@example.com", "JBSWY3DPEHPK3PXP")

	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Fatal("URL should start with otpauth://totp/")
	}
	if !strings.Contains(url, "HelixSeller") {
		t.Fatal("URL should contain issuer")
	}
	if !strings.Contains(url, "user@example.com") {
		t.Fatal("URL should contain email")
	}
	if !strings.Contains(url, "JBSWY3DPEHPK3PXP") {
		t.Fatal("URL should contain secret")
	}
}
