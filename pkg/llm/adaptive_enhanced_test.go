// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/learning"
)

// enhancedMockProvider is a test double for Provider
type enhancedMockProvider struct {
	name           string
	supportsVision bool
	chatFunc       func(ctx context.Context, messages []Message) (*Response, error)
	visionFunc     func(ctx context.Context, image []byte, prompt string) (*Response, error)
}

func (m *enhancedMockProvider) Name() string         { return m.name }
func (m *enhancedMockProvider) SupportsVision() bool { return m.supportsVision }
func (m *enhancedMockProvider) Chat(ctx context.Context, messages []Message) (*Response, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages)
	}
	return &Response{Content: "mock response"}, nil
}
func (m *enhancedMockProvider) Vision(ctx context.Context, image []byte, prompt string) (*Response, error) {
	if m.visionFunc != nil {
		return m.visionFunc(ctx, image, prompt)
	}
	return &Response{Content: "mock vision response"}, nil
}

func TestNewEnhancedAdaptiveProvider(t *testing.T) {
	configs := []ProviderConfig{
		{Name: "google", APIKey: "test-key"},
		{Name: "anthropic", APIKey: "test-key"},
	}

	eap, err := NewEnhancedAdaptiveProvider(configs)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if eap.Name() != "adaptive-enhanced" {
		t.Errorf("expected name 'adaptive-enhanced', got: %s", eap.Name())
	}

	if len(eap.providers) != 2 {
		t.Errorf("expected 2 providers, got: %d", len(eap.providers))
	}
}

func TestEnhancedAdaptiveProvider_SupportsVision(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{name: "text-only", supportsVision: false},
			&enhancedMockProvider{name: "vision", supportsVision: true},
		},
	}

	if !eap.SupportsVision() {
		t.Error("expected SupportsVision to return true when at least one provider supports it")
	}

	eap2 := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{name: "text-only", supportsVision: false},
		},
	}

	if eap2.SupportsVision() {
		t.Error("expected SupportsVision to return false when no providers support it")
	}
}

func TestEnhancedAdaptiveProvider_Chat_Success(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name: "google",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					return &Response{Content: "success", Model: "gemini"}, nil
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "google", APIKey: "key"},
		},
	}

	messages := []Message{{Role: RoleUser, Content: "test"}}
	resp, err := eap.Chat(context.Background(), messages)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "success" {
		t.Errorf("expected 'success', got: %s", resp.Content)
	}
}

func TestEnhancedAdaptiveProvider_Chat_Fallback(t *testing.T) {
	callCount := 0
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name: "failing",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					callCount++
					return nil, errors.New("failure")
				},
			},
			&enhancedMockProvider{
				name: "working",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					return &Response{Content: "fallback success"}, nil
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "failing", APIKey: "key"},
			{Name: "working", APIKey: "key"},
		},
	}

	messages := []Message{{Role: RoleUser, Content: "test"}}
	resp, err := eap.Chat(context.Background(), messages)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback success" {
		t.Errorf("expected 'fallback success', got: %s", resp.Content)
	}
	if callCount != 1 {
		t.Errorf("expected failing provider to be called once, got: %d", callCount)
	}
}

func TestEnhancedAdaptiveProvider_Chat_AllFail(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name: "p1",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					return nil, errors.New("error1")
				},
			},
			&enhancedMockProvider{
				name: "p2",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					return nil, errors.New("error2")
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "p1", APIKey: "key"},
			{Name: "p2", APIKey: "key"},
		},
	}

	messages := []Message{{Role: RoleUser, Content: "test"}}
	_, err := eap.Chat(context.Background(), messages)

	if err == nil {
		t.Error("expected error when all providers fail")
	}
	if !contains(err.Error(), "all providers failed") {
		t.Errorf("expected 'all providers failed' in error, got: %v", err)
	}
}

func TestEnhancedAdaptiveProvider_Chat_ContextCancellation(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name: "slow",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "slow", APIKey: "key"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []Message{{Role: RoleUser, Content: "test"}}
	_, err := eap.Chat(ctx, messages)

	// Error should contain context.Canceled or be wrapped
	if err == nil || (!strings.Contains(err.Error(), "context canceled") && err != context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

func TestEnhancedAdaptiveProvider_Vision_Success(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name:           "vision",
				supportsVision: true,
				visionFunc: func(ctx context.Context, image []byte, prompt string) (*Response, error) {
					return &Response{Content: "vision success"}, nil
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "vision", APIKey: "key"},
		},
	}

	resp, err := eap.Vision(context.Background(), []byte("image"), "prompt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "vision success" {
		t.Errorf("expected 'vision success', got: %s", resp.Content)
	}
}

func TestEnhancedAdaptiveProvider_Vision_NoProviders(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name:           "text-only",
				supportsVision: false,
			},
		},
		configs: []ProviderConfig{
			{Name: "text-only", APIKey: "key"},
		},
	}

	_, err := eap.Vision(context.Background(), []byte("image"), "prompt")

	if err == nil {
		t.Error("expected error when no vision providers available")
	}
}

func TestEnhancedAdaptiveProvider_RateLimitRetry(t *testing.T) {
	// This test verifies the rate limit detection works
	// The actual retry behavior depends on the retryAfter value
	rl := GetRateLimiter("rate-limit-test")

	// Reset state
	rl.failures = 0
	rl.circuitOpen = false

	// Record rate limit failures - these should NOT open the circuit
	for i := 0; i < 10; i++ {
		rl.RecordFailure(errors.New("429 rate limit exceeded"))
	}

	// Circuit should NOT be open for rate limit errors
	if rl.circuitOpen {
		t.Error("circuit should not open for rate limit errors")
	}

	// Reset
	rl.failures = 0
	rl.circuitOpen = false

	// Record non-rate-limit errors - these SHOULD open the circuit
	for i := 0; i < 3; i++ {
		rl.RecordFailure(errors.New("some other error"))
	}

	// Circuit should be open for non-rate-limit errors
	if !rl.circuitOpen {
		t.Error("circuit should open for non-rate-limit errors")
	}
}

func TestEnhancedAdaptiveProvider_SetKnowledgeBase(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{}
	kb := learning.NewKnowledgeBase()
	platforms := []string{"androidtv"}

	eap.SetKnowledgeBase(kb, platforms)

	if eap.learning != kb {
		t.Error("knowledge base not set correctly")
	}
	if len(eap.platforms) != 1 || eap.platforms[0] != "androidtv" {
		t.Error("platforms not set correctly")
	}
}

func TestEnhancedAdaptiveProvider_Providers(t *testing.T) {
	providers := []Provider{
		&enhancedMockProvider{name: "p1"},
		&enhancedMockProvider{name: "p2"},
	}
	eap := &EnhancedAdaptiveProvider{
		providers: providers,
	}

	returned := eap.Providers()
	if len(returned) != 2 {
		t.Errorf("expected 2 providers, got: %d", len(returned))
	}
}

func TestEnhancedAdaptiveProvider_rankProviders(t *testing.T) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{name: "google"},
			&enhancedMockProvider{name: "failing"},
			&enhancedMockProvider{name: "anthropic"},
		},
		configs: []ProviderConfig{
			{Name: "google", APIKey: "key"},
			{Name: "failing", APIKey: "key"},
			{Name: "anthropic", APIKey: "key"},
		},
	}

	// Simulate failures on "failing" provider
	rl := GetRateLimiter("failing")
	rl.RecordFailure(errors.New("error"))
	rl.RecordFailure(errors.New("error"))
	rl.RecordFailure(errors.New("error"))

	ranked := eap.rankProviders()

	// Failing provider should be ranked last
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked providers, got: %d", len(ranked))
	}

	// Check that failing provider is last
	lastProvider := eap.providers[ranked[2]]
	if lastProvider.Name() != "failing" {
		t.Errorf("expected failing provider to be ranked last, got: %s", lastProvider.Name())
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func BenchmarkEnhancedAdaptiveProvider_Chat(b *testing.B) {
	eap := &EnhancedAdaptiveProvider{
		providers: []Provider{
			&enhancedMockProvider{
				name: "fast",
				chatFunc: func(ctx context.Context, messages []Message) (*Response, error) {
					return &Response{Content: "response"}, nil
				},
			},
		},
		configs: []ProviderConfig{
			{Name: "fast", APIKey: "key"},
		},
	}

	messages := []Message{{Role: RoleUser, Content: "test"}}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eap.Chat(ctx, messages)
	}
}
