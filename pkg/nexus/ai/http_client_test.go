package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient returns an HTTPLLMClient that allows private
// networks so httptest-backed tests (served from 127.0.0.1) are not
// blocked by the SSRF guard. Production clients keep the guard's
// default (private + loopback rejected).
func newTestClient(endpoint, apiKey, model string) *HTTPLLMClient {
	c := NewHTTPLLMClient(endpoint, apiKey, model)
	c.SSRFGuard.AllowPrivateNetworks = true
	return c
}

func TestHTTPLLMClient_RequiresEndpoint(t *testing.T) {
	c := &HTTPLLMClient{}
	if _, err := c.Chat(context.Background(), ChatRequest{Model: "m", UserPrompt: "x"}); err == nil {
		t.Fatal("missing endpoint must error")
	}
}

func TestHTTPLLMClient_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"test-model"`) {
			t.Errorf("request body missing model: %s", string(body))
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("auth header missing: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{
				{"message": map[string]any{"content": "hello"}},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 2, "cost_usd": 0.001},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "secret", "test-model")
	resp, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.TokensIn != 5 || resp.TokensOut != 2 {
		t.Errorf("tokens: in=%d out=%d", resp.TokensIn, resp.TokensOut)
	}
	if resp.CostUSD != 0.001 {
		t.Errorf("cost = %f", resp.CostUSD)
	}
}

func TestHTTPLLMClient_ErrorStatusSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limit"}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, "", "m")
	_, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("expected 429 in error, got %v", err)
	}
}

func TestHTTPLLMClient_NoChoicesRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, "", "m")
	_, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected no-choices error, got %v", err)
	}
}

func TestHTTPLLMClient_ImageAttachmentSerialized(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, "", "m")
	_, err := c.Chat(context.Background(), ChatRequest{
		UserPrompt:  "describe",
		ImageBase64: []string{"AAAA"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedBody, "image_url") {
		t.Errorf("image part missing: %q", capturedBody)
	}
	if !strings.Contains(capturedBody, "data:image/png;base64,AAAA") {
		t.Error("base64 data URI missing")
	}
}

func TestHTTPLLMClient_ModelDefaultFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"default-m"`) {
			t.Errorf("default model not applied: %s", body)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, "", "default-m")
	_, _ = c.Chat(context.Background(), ChatRequest{UserPrompt: "x"}) // no model override
}

func TestHTTPLLMClient_ModelRequiredWhenNoDefault(t *testing.T) {
	c := NewHTTPLLMClient("https://example.com", "", "")
	_, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err == nil {
		t.Fatal("expected model-required error")
	}
}

func TestHTTPLLMClient_BadJSONRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, "", "m")
	if _, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"}); err == nil {
		t.Fatal("malformed JSON should error")
	}
}
