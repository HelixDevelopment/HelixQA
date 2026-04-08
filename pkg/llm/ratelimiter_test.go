// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTokenBucket_wait(t *testing.T) {
	tb := newTokenBucket(60, 10) // 60 per minute, burst 10

	// Should not block for first 10 tokens (burst)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if err := tb.wait(ctx); err != nil {
			t.Fatalf("wait %d: unexpected error: %v", i, err)
		}
	}

	// Context cancellation should return error
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tb.wait(cancelCtx); err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestProviderRateLimiter_Wait(t *testing.T) {
	rl := NewProviderRateLimiter("google")

	ctx := context.Background()

	// First request should succeed immediately
	if err := rl.Wait(ctx, 100); err != nil {
		t.Fatalf("first wait: unexpected error: %v", err)
	}

	// Record success
	rl.RecordSuccess()
	if rl.failures != 0 {
		t.Errorf("expected 0 failures after success, got: %d", rl.failures)
	}
}

func TestProviderRateLimiter_RecordFailure(t *testing.T) {
	rl := NewProviderRateLimiter("test")
	rl.config.MaxConsecutiveFailures = 3

	// Record non-rate-limit failures
	for i := 0; i < 3; i++ {
		rl.RecordFailure(errors.New("some error"))
	}

	if !rl.isCircuitOpen() {
		t.Error("expected circuit to be open after 3 failures")
	}

	// Rate limit errors should not open circuit
	rl2 := NewProviderRateLimiter("test2")
	rl2.config.MaxConsecutiveFailures = 3

	for i := 0; i < 5; i++ {
		rl2.RecordFailure(errors.New("429 rate limit exceeded"))
	}

	if rl2.isCircuitOpen() {
		t.Error("circuit should not open for rate limit errors")
	}
}

func TestProviderRateLimiter_CircuitBreakerTimeout(t *testing.T) {
	rl := NewProviderRateLimiter("test")
	rl.config.MaxConsecutiveFailures = 1
	rl.config.CircuitBreakerTimeout = 50 * time.Millisecond

	// Open circuit
	rl.RecordFailure(errors.New("failure"))
	if !rl.isCircuitOpen() {
		t.Fatal("expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Circuit should be closed now
	if rl.isCircuitOpen() {
		t.Error("expected circuit to be closed after timeout")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected time.Duration
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "retry after seconds",
			err:      errors.New("rate limited, retry after 30"),
			expected: 30 * time.Second,
		},
		{
			name:     "rate limit error default",
			err:      errors.New("429 Too Many Requests"),
			expected: 5 * time.Second,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseRetryAfter(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetRateLimiter(t *testing.T) {
	// Get rate limiter for same provider twice
	rl1 := GetRateLimiter("google")
	rl2 := GetRateLimiter("google")

	if rl1 != rl2 {
		t.Error("expected same rate limiter instance for same provider")
	}

	// Different providers should get different limiters
	rl3 := GetRateLimiter("anthropic")
	if rl1 == rl3 {
		t.Error("expected different rate limiter instances for different providers")
	}
}

func TestProviderRateLimits(t *testing.T) {
	// Verify all known providers have rate limits
	knownProviders := []string{
		"google", "githubmodels", "groq", "anthropic", "openai",
	}

	for _, name := range knownProviders {
		config, ok := ProviderRateLimits[name]
		if !ok {
			t.Errorf("provider %s not found in ProviderRateLimits", name)
			continue
		}

		if config.RequestsPerMinute <= 0 {
			t.Errorf("provider %s: RequestsPerMinute should be > 0", name)
		}
		if config.TokensPerMinute <= 0 {
			t.Errorf("provider %s: TokensPerMinute should be > 0", name)
		}
		if config.BurstSize <= 0 {
			t.Errorf("provider %s: BurstSize should be > 0", name)
		}
		if config.MaxConsecutiveFailures <= 0 {
			t.Errorf("provider %s: MaxConsecutiveFailures should be > 0", name)
		}
	}
}

func TestNewProviderRateLimiter_Defaults(t *testing.T) {
	// Unknown provider should get defaults
	rl := NewProviderRateLimiter("unknown-provider")

	if rl.config.RequestsPerMinute != 30 {
		t.Errorf("expected default RequestsPerMinute=30, got: %d", rl.config.RequestsPerMinute)
	}
	if rl.config.TokensPerMinute != 50000 {
		t.Errorf("expected default TokensPerMinute=50000, got: %d", rl.config.TokensPerMinute)
	}
}

func TestProviderRateLimiter_Wait_CircuitOpen(t *testing.T) {
	rl := NewProviderRateLimiter("test")
	rl.config.MaxConsecutiveFailures = 1
	rl.circuitOpen = true
	rl.circuitOpensAt = time.Now()

	ctx := context.Background()
	err := rl.Wait(ctx, 100)

	if err == nil {
		t.Error("expected error when circuit is open")
	}
	if err.Error() != "circuit breaker open for test" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMinMax(t *testing.T) {
	if min(1.0, 2.0) != 1.0 {
		t.Error("min(1, 2) should be 1")
	}
	if min(2.0, 1.0) != 1.0 {
		t.Error("min(2, 1) should be 1")
	}
	if max(1.0, 2.0) != 2.0 {
		t.Error("max(1, 2) should be 2")
	}
	if max(2.0, 1.0) != 2.0 {
		t.Error("max(2, 1) should be 2")
	}
}

func BenchmarkTokenBucket_wait(b *testing.B) {
	tb := newTokenBucket(60000, 1000) // High rate
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.wait(ctx)
	}
}

func BenchmarkGetRateLimiter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetRateLimiter("benchmark-provider")
	}
}
