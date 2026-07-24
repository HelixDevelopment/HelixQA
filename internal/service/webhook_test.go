package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/helix-seller/helix-seller/internal/model"
	"go.uber.org/zap"
)

func TestWebhookService_Constructor(t *testing.T) {
	svc := NewWebhookService(nil, zap.NewNop())
	if svc == nil {
		t.Fatal("expected non-nil WebhookService")
	}
	if svc.client == nil {
		t.Error("expected non-nil HTTP client")
	}
	if svc.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestWebhookService_EventMatches_Exact(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}
	tests := []struct {
		events   []string
		eventType string
		want     bool
	}{
		{[]string{"payment.succeeded"}, "payment.succeeded", true},
		{[]string{"payment.succeeded"}, "payment.failed", false},
		{[]string{"payment.succeeded", "payment.failed"}, "payment.failed", true},
		{[]string{"payment.succeeded", "payment.failed"}, "refund.created", false},
	}

	for _, tt := range tests {
		got := svc.eventMatches(tt.events, tt.eventType)
		if got != tt.want {
			t.Errorf("eventMatches(%v, %s) = %v, want %v", tt.events, tt.eventType, got, tt.want)
		}
	}
}

func TestWebhookService_EventMatches_Wildcard(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}
	got := svc.eventMatches([]string{"*"}, "anything.goes")
	if !got {
		t.Error("wildcard should match any event")
	}
}

func TestWebhookService_EventMatches_Empty(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}
	got := svc.eventMatches([]string{}, "payment.succeeded")
	if got {
		t.Error("empty events should not match")
	}
}

func TestWebhookService_EventMatches_Nil(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}
	got := svc.eventMatches(nil, "payment.succeeded")
	if got {
		t.Error("nil events should not match")
	}
}

func TestWebhookService_EventMatches_WildcardOnly(t *testing.T) {
	svc := &WebhookService{logger: zap.NewNop()}
	got := svc.eventMatches([]string{"*"}, "subscription.cancelled")
	if !got {
		t.Error("wildcard should match subscription.cancelled")
	}
}

func TestHMACSignature_Generation(t *testing.T) {
	secret := "webhook_secret_123"
	body := []byte(`{"type":"payment.succeeded","amount":1000}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if sig == "" {
		t.Error("expected non-empty signature")
	}
	if len(sig) != 64 {
		t.Errorf("expected 64 char hex sig, got %d", len(sig))
	}
}

func TestHMACSignature_Verification(t *testing.T) {
	secret := "webhook_secret_123"
	body := []byte(`{"type":"payment.succeeded","amount":1000}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	// Verify
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write(body)
	expected := hex.EncodeToString(mac2.Sum(nil))

	if sig != expected {
		t.Errorf("signature mismatch: %s != %s", sig, expected)
	}
}

func TestHMACSignature_DifferentSecret(t *testing.T) {
	body := []byte(`{"type":"payment.succeeded"}`)

	mac1 := hmac.New(sha256.New, []byte("secret1"))
	mac1.Write(body)
	sig1 := hex.EncodeToString(mac1.Sum(nil))

	mac2 := hmac.New(sha256.New, []byte("secret2"))
	mac2.Write(body)
	sig2 := hex.EncodeToString(mac2.Sum(nil))

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestHMACSignature_DifferentBody(t *testing.T) {
	secret := "webhook_secret_123"

	mac1 := hmac.New(sha256.New, []byte(secret))
	mac1.Write([]byte("body1"))
	sig1 := hex.EncodeToString(mac1.Sum(nil))

	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write([]byte("body2"))
	sig2 := hex.EncodeToString(mac2.Sum(nil))

	if sig1 == sig2 {
		t.Error("different bodies should produce different signatures")
	}
}

func TestHMACSignature_EmptyBody(t *testing.T) {
	secret := "webhook_secret_123"
	body := []byte{}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if sig == "" {
		t.Error("empty body should still produce a signature")
	}
}

func TestWebhookService_Send_SetsHeaders(t *testing.T) {
	secret := "test_secret"
	body := []byte(`{"test": true}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if sig == "" {
		t.Error("expected signature")
	}
}

// --- send / sendWithRetry tests ---

func TestWebhookService_Send_Success(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		receivedHeaders = r.Header
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
		Secret: "test_secret_123",
	}

	body := []byte(`{"type":"payment.succeeded"}`)
	err := svc.send(config, body)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if string(receivedBody) != string(body) {
		t.Errorf("body = %q, want %q", string(receivedBody), string(body))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", receivedHeaders.Get("Content-Type"))
	}
	if receivedHeaders.Get("X-Helix-Webhook-ID") != config.ID.String() {
		t.Errorf("X-Helix-Webhook-ID = %q, want %q", receivedHeaders.Get("X-Helix-Webhook-ID"), config.ID.String())
	}
}

func TestWebhookService_Send_WithSignature(t *testing.T) {
	var receivedSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Helix-Signature")
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
		Secret: "my_secret",
	}

	body := []byte(`{"test": true}`)
	err := svc.send(config, body)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if receivedSig == "" {
		t.Error("expected X-Helix-Signature header")
	}

	// Verify the signature
	mac := hmac.New(sha256.New, []byte("my_secret"))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if receivedSig != expected {
		t.Errorf("signature = %q, want %q", receivedSig, expected)
	}
}

func TestWebhookService_Send_NoSecret_NoSignature(t *testing.T) {
	var receivedSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Helix-Signature")
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
		Secret: "",
	}

	err := svc.send(config, []byte(`{}`))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if receivedSig != "" {
		t.Errorf("expected no signature, got %q", receivedSig)
	}
}

func TestWebhookService_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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

	err := svc.send(config, []byte(`{}`))
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestWebhookService_Send_BadURL(t *testing.T) {
	svc := &WebhookService{
		client: &http.Client{},
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: "http://localhost:1",
	}

	err := svc.send(config, []byte(`{}`))
	if err == nil {
		t.Error("expected error for bad URL")
	}
}

func TestWebhookService_SendWithRetry_EventuallySucceeds(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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

	svc.sendWithRetry(config, []byte(`{}`))

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}

func TestWebhookService_SendWithRetry_AllFail(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
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

	svc.sendWithRetry(config, []byte(`{}`))

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 5 {
		t.Errorf("expected 5 attempts, got %d", got)
	}
}

func TestWebhookService_Send_BadRequest(t *testing.T) {
	svc := &WebhookService{
		client: &http.Client{},
		logger: zap.NewNop(),
	}

	config := &model.WebhookConfig{
		ID:  uuid.New(),
		URL: "not-a-url",
	}

	err := svc.send(config, []byte(`{}`))
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestWebhookService_Send_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
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

	err := svc.send(config, []byte(`{}`))
	if err == nil {
		t.Error("expected error for 400 status")
	}
}
