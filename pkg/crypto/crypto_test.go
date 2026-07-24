package crypto

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"simple text", "hello world"},
		{"json payload", `{"email":"user@example.com","amount":1000}`},
		{"unicode", "Привет мир 你好世界"},
		{"special chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"long text", strings.Repeat("abcdefghijklmnop", 1000)},
		{"binary-like", "\x00\x01\x02\x03\xff\xfe\xfd"},
		{"single char", "a"},
		{"newlines and tabs", "line1\nline2\ttab\rcarriage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := Encrypt([]byte(tt.plaintext), key)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if bytes.Equal(ciphertext, []byte(tt.plaintext)) {
				t.Error("ciphertext should not equal plaintext")
			}

			decrypted, err := Decrypt(ciphertext, key)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("decrypted = %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	plaintext := "same input"

	ct1, err := Encrypt([]byte(plaintext), key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ct2, err := Encrypt([]byte(plaintext), key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	ciphertext, err := Encrypt([]byte("secret data"), key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}

func TestDecryptWithInvalidKey(t *testing.T) {
	ciphertext, _ := Encrypt([]byte("test"), mustGenerateKey(t))

	tests := []struct {
		name string
		key  string
	}{
		{"not hex", "not-a-hex-string"},
		{"too short", "deadbeef"},
		{"empty", ""},
		{"odd length", "abc"},
		{"invalid chars", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(ciphertext, tt.key)
			if err == nil {
				t.Errorf("Decrypt with invalid key %q should fail", tt.key)
			}
		})
	}
}

func TestDecryptWithCorruptedCiphertext(t *testing.T) {
	key := mustGenerateKey(t)
	ciphertext, err := Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = Decrypt(corrupted, key)
	if err == nil {
		t.Error("Decrypt with corrupted ciphertext should fail")
	}
}

func TestDecryptWithTruncatedCiphertext(t *testing.T) {
	key := mustGenerateKey(t)
	ciphertext, err := Encrypt([]byte("data"), key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	truncated := ciphertext[:len(ciphertext)/2]
	_, err = Decrypt(truncated, key)
	if err == nil {
		t.Error("Decrypt with truncated ciphertext should fail")
	}
}

func TestEncryptWithInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"not hex", "not-a-hex-string"},
		{"too short", "deadbeef"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt([]byte("test"), tt.key)
			if err == nil {
				t.Errorf("Encrypt with invalid key %q should fail", tt.key)
			}
		})
	}
}

func TestDecryptTooShortCiphertext(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext []byte
	}{
		{"empty", []byte{}},
		{"one byte", []byte{0x01}},
		{"half nonce size", make([]byte, 6)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext, key)
			if err == nil {
				t.Error("Decrypt with too short ciphertext should fail")
			}
			if !strings.Contains(err.Error(), "ciphertext too short") {
				t.Errorf("error should mention 'ciphertext too short', got: %q", err.Error())
			}
		})
	}
}

func TestGenerateKeyFormat(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	decoded, err := hex.DecodeString(key)
	if err != nil {
		t.Fatalf("GenerateKey returned invalid hex: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("key length = %d bytes, want 32 (AES-256)", len(decoded))
	}

	if len(key) != 64 {
		t.Errorf("hex key length = %d, want 64", len(key))
	}
}

func TestGenerateKeyUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey failed: %v", err)
		}
		if seen[key] {
			t.Fatalf("duplicate key generated: %s", key)
		}
		seen[key] = true
	}
}

func TestEncryptEncryptKeyLength(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"8 bytes", 8, true},
		{"16 bytes (AES-128)", 16, false},
		{"24 bytes (AES-192)", 24, false},
		{"32 bytes (AES-256)", 32, false},
		{"48 bytes", 48, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := hex.EncodeToString(make([]byte, tt.keyLen))
			_, err := Encrypt([]byte("test"), key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt with %s key: err = %v, wantErr = %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func mustGenerateKey(t *testing.T) string {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	return key
}
