// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockProvider is a local test double for the Provider interface.
type mockProvider struct {
	name       string
	vision     bool
	chatResp   *Response
	chatErr    error
	visionResp *Response
	visionErr  error
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) SupportsVision() bool { return m.vision }

func (m *mockProvider) Chat(
	_ context.Context,
	_ []Message,
) (*Response, error) {
	return m.chatResp, m.chatErr
}

func (m *mockProvider) Vision(
	_ context.Context,
	_ []byte,
	_ string,
) (*Response, error) {
	return m.visionResp, m.visionErr
}

// TestAdaptiveProvider_SelectsFirst verifies that when both providers
// succeed, the first one's response is returned.
func TestAdaptiveProvider_SelectsFirst(t *testing.T) {
	first := &mockProvider{
		name:     "first",
		chatResp: &Response{Content: "from first", Model: "m1"},
	}
	second := &mockProvider{
		name:     "second",
		chatResp: &Response{Content: "from second", Model: "m2"},
	}

	ap := NewAdaptiveProvider(first, second)
	resp, err := ap.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Content != "from first" {
		t.Errorf("expected response from first provider, got: %q", resp.Content)
	}
}

// TestAdaptiveProvider_FallsBack verifies that when the first provider
// fails, the second provider is tried and its response is returned.
func TestAdaptiveProvider_FallsBack(t *testing.T) {
	first := &mockProvider{
		name:    "first",
		chatErr: errors.New("first provider unavailable"),
	}
	second := &mockProvider{
		name:     "second",
		chatResp: &Response{Content: "from second", Model: "m2"},
	}

	ap := NewAdaptiveProvider(first, second)
	resp, err := ap.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("expected no error after fallback, got: %v", err)
	}
	if resp.Content != "from second" {
		t.Errorf("expected response from second provider, got: %q", resp.Content)
	}
}

// TestAdaptiveProvider_AllFail verifies that when all providers fail,
// the returned error contains "all providers failed".
func TestAdaptiveProvider_AllFail(t *testing.T) {
	first := &mockProvider{
		name:    "first",
		chatErr: errors.New("first error"),
	}
	second := &mockProvider{
		name:    "second",
		chatErr: errors.New("second error"),
	}

	ap := NewAdaptiveProvider(first, second)
	_, err := ap.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "all providers failed") {
		t.Errorf("expected error to contain %q, got: %v",
			"all providers failed", err)
	}
}

// TestAdaptiveProvider_VisionSelectsCapable verifies that Vision()
// skips providers without vision support and uses the first
// vision-capable one.
func TestAdaptiveProvider_VisionSelectsCapable(t *testing.T) {
	noVision := &mockProvider{
		name:   "no-vision",
		vision: false,
	}
	withVision := &mockProvider{
		name:       "with-vision",
		vision:     true,
		visionResp: &Response{Content: "vision result", Model: "vm1"},
	}

	ap := NewAdaptiveProvider(noVision, withVision)
	resp, err := ap.Vision(context.Background(), []byte("img"), "describe")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Content != "vision result" {
		t.Errorf("expected vision result from capable provider, got: %q",
			resp.Content)
	}
}

// TestAdaptiveProvider_NoProviders verifies that constructing with no
// providers and calling Chat returns an error.
func TestAdaptiveProvider_NoProviders(t *testing.T) {
	ap := NewAdaptiveProvider()
	_, err := ap.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for empty provider list, got nil")
	}
}

// TestNewAdaptiveFromConfigs verifies that factory construction from
// a valid ProviderConfig slice succeeds and returns an adaptive
// provider with Name() == "adaptive".
func TestNewAdaptiveFromConfigs(t *testing.T) {
	configs := []ProviderConfig{
		{
			Name:    ProviderOllama,
			BaseURL: "http://localhost:11434",
			Model:   "qwen2.5",
		},
	}

	ap, err := NewAdaptiveFromConfigs(configs)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ap.Name() != "adaptive" {
		t.Errorf("expected Name() == %q, got: %q", "adaptive", ap.Name())
	}
}

// TestAdaptiveProvider_SupportsVision_AnyCapable verifies SupportsVision
// returns true when at least one wrapped provider supports vision.
func TestAdaptiveProvider_SupportsVision_AnyCapable(t *testing.T) {
	noVision := &mockProvider{name: "a", vision: false}
	withVision := &mockProvider{name: "b", vision: true}

	ap := NewAdaptiveProvider(noVision, withVision)
	if !ap.SupportsVision() {
		t.Error("expected SupportsVision() == true when one provider has vision")
	}
}

// TestAdaptiveProvider_SupportsVision_NoneCapable verifies SupportsVision
// returns false when no provider supports vision.
func TestAdaptiveProvider_SupportsVision_NoneCapable(t *testing.T) {
	a := &mockProvider{name: "a", vision: false}
	b := &mockProvider{name: "b", vision: false}

	ap := NewAdaptiveProvider(a, b)
	if ap.SupportsVision() {
		t.Error("expected SupportsVision() == false when no provider has vision")
	}
}

// TestAdaptiveProvider_Vision_NoCapableProvider verifies that Vision()
// returns a specific error when no vision-capable provider is available.
func TestAdaptiveProvider_Vision_NoCapableProvider(t *testing.T) {
	noVision := &mockProvider{name: "a", vision: false}

	ap := NewAdaptiveProvider(noVision)
	_, err := ap.Vision(context.Background(), []byte("img"), "describe")
	if err == nil {
		t.Fatal("expected error when no vision-capable provider, got nil")
	}
	if !strings.Contains(err.Error(), "no vision-capable providers") {
		t.Errorf("expected error to contain %q, got: %v",
			"no vision-capable providers", err)
	}
}

// TestNewAdaptiveFromConfigs_AllInvalid verifies that an error is
// returned when all supplied configs are invalid.
func TestNewAdaptiveFromConfigs_AllInvalid(t *testing.T) {
	configs := []ProviderConfig{
		{Name: ""},         // empty name — invalid
		{Name: "unknown"},  // unknown type — silently skipped
	}

	_, err := NewAdaptiveFromConfigs(configs)
	if err == nil {
		t.Fatal("expected error when all configs are invalid, got nil")
	}
}

// TestAdaptiveProvider_SkipsUnavailableProviders verifies that
// providers returning auth/credit errors are marked unavailable
// and skipped on subsequent calls.
func TestAdaptiveProvider_SkipsUnavailableProviders(t *testing.T) {
	authErr := &mockProvider{
		name:    "no-credits",
		chatErr: errors.New("API error status 401: unauthorized - invalid api key"),
	}
	good := &mockProvider{
		name:     "good",
		chatResp: &Response{Content: "success", Model: "good-model"},
	}

	ap := NewAdaptiveProvider(authErr, good)

	ctx := context.Background()
	msgs := []Message{{Role: "user", Content: "test"}}

	// First call: authErr fails with 401, marked unavailable, good succeeds
	resp, err := ap.Chat(ctx, msgs)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.Content != "success" {
		t.Errorf("expected 'success', got %q", resp.Content)
	}

	// authErr should be marked unavailable
	if !ap.isUnavailable("no-credits") {
		t.Error("expected no-credits to be unavailable")
	}
	unavail := ap.GetUnavailableProviders()
	if _, ok := unavail["no-credits"]; !ok {
		t.Error("expected no-credits in unavailable map")
	}

	// Second call: should skip authErr entirely
	resp2, err := ap.Chat(ctx, msgs)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if resp2.Content != "success" {
		t.Errorf("expected 'success', got %q", resp2.Content)
	}
}

// TestIsAuthOrCreditError verifies detection of auth/credit
// errors across various provider error message formats.
func TestIsAuthOrCreditError(t *testing.T) {
	tests := []struct {
		msg    string
		expect bool
	}{
		{"API error status 401: unauthorized", true},
		{"error 403: forbidden", true},
		{"insufficient credits remaining", true},
		{"quota exceeded for this billing period", true},
		{"payment required", true},
		{"invalid api key provided", true},
		{"rate limit exceeded", true},
		{"access denied", true},
		{"subscription plan limit reached", true},
		{"connection timeout", false},
		{"context deadline exceeded", false},
		{"internal server error 500", false},
		{"model not found", false},
		{"EOF", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isAuthOrCreditError(errors.New(tt.msg))
			if got != tt.expect {
				t.Errorf("isAuthOrCreditError(%q) = %v, want %v",
					tt.msg, got, tt.expect)
			}
		})
	}
	// nil error should return false
	if isAuthOrCreditError(nil) {
		t.Error("nil error should return false")
	}
}
