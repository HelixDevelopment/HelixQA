// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig configures rate limiting for LLM providers
type RateLimitConfig struct {
	// Requests per minute
	RequestsPerMinute int
	// Tokens per minute
	TokensPerMinute int
	// Burst size for requests
	BurstSize int
	// Retry after header parsing
	RespectRetryAfter bool
	// Max consecutive failures before circuit opens
	MaxConsecutiveFailures int
	// Circuit breaker timeout
	CircuitBreakerTimeout time.Duration
}

// DefaultRateLimitConfig returns sensible defaults
type DefaultRateLimitConfig func() RateLimitConfig

// ProviderRateLimits defines rate limits for known providers
var ProviderRateLimits = map[string]RateLimitConfig{
	"google": {
		RequestsPerMinute:      60,
		TokensPerMinute:        1000000,
		BurstSize:              10,
		RespectRetryAfter:      true,
		MaxConsecutiveFailures: 3,
		CircuitBreakerTimeout:  30 * time.Second,
	},
	"githubmodels": {
		RequestsPerMinute:      15,
		TokensPerMinute:        8000, // Low token limit for GPT-4o
		BurstSize:              3,
		RespectRetryAfter:      true,
		MaxConsecutiveFailures: 3,
		CircuitBreakerTimeout:  60 * time.Second,
	},
	"groq": {
		RequestsPerMinute:      30,
		TokensPerMinute:        12000,
		BurstSize:              5,
		RespectRetryAfter:      true,
		MaxConsecutiveFailures: 3,
		CircuitBreakerTimeout:  30 * time.Second,
	},
	"anthropic": {
		RequestsPerMinute:      50,
		TokensPerMinute:        100000,
		BurstSize:              10,
		RespectRetryAfter:      true,
		MaxConsecutiveFailures: 3,
		CircuitBreakerTimeout:  30 * time.Second,
	},
	"openai": {
		RequestsPerMinute:      60,
		TokensPerMinute:        150000,
		BurstSize:              10,
		RespectRetryAfter:      true,
		MaxConsecutiveFailures: 3,
		CircuitBreakerTimeout:  30 * time.Second,
	},
}

// tokenBucket implements a token bucket rate limiter
type tokenBucket struct {
	rate       float64
	burst      float64
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// newTokenBucket creates a new token bucket
func newTokenBucket(ratePerMinute float64, burst int) *tokenBucket {
	return &tokenBucket{
		rate:       ratePerMinute / 60.0, // Convert to per second
		burst:      float64(burst),
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// wait blocks until a token is available
func (tb *tokenBucket) wait(ctx context.Context) error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.tokens = min(tb.burst, tb.tokens+elapsed*tb.rate)
	tb.lastUpdate = now

	if tb.tokens >= 1 {
		tb.tokens--
		return nil
	}

	// Calculate wait time
	waitTime := time.Duration((1 - tb.tokens) / tb.rate * float64(time.Second))

	tb.mu.Unlock()
	select {
	case <-ctx.Done():
		tb.mu.Lock()
		return ctx.Err()
	case <-time.After(waitTime):
		tb.mu.Lock()
		tb.tokens = max(0, tb.tokens-1)
		return nil
	}
}

// ProviderRateLimiter manages rate limiting for a provider
type ProviderRateLimiter struct {
	providerName   string
	requestBucket  *tokenBucket
	tokenBucket    *tokenBucket
	config         RateLimitConfig
	failures       int
	lastFailure    time.Time
	circuitOpen    bool
	circuitOpensAt time.Time
	mu             sync.RWMutex
}

// NewProviderRateLimiter creates a rate limiter for a provider
func NewProviderRateLimiter(providerName string) *ProviderRateLimiter {
	config, ok := ProviderRateLimits[providerName]
	if !ok {
		// Default config for unknown providers
		config = RateLimitConfig{
			RequestsPerMinute:      30,
			TokensPerMinute:        50000,
			BurstSize:              5,
			RespectRetryAfter:      true,
			MaxConsecutiveFailures: 3,
			CircuitBreakerTimeout:  30 * time.Second,
		}
	}

	return &ProviderRateLimiter{
		providerName:  providerName,
		requestBucket: newTokenBucket(float64(config.RequestsPerMinute), config.BurstSize),
		tokenBucket:   newTokenBucket(float64(config.TokensPerMinute), config.BurstSize*1000),
		config:        config,
	}
}

// Wait blocks until the request can proceed respecting rate limits
func (rl *ProviderRateLimiter) Wait(ctx context.Context, estimatedTokens int) error {
	// Check circuit breaker
	if rl.isCircuitOpen() {
		return fmt.Errorf("circuit breaker open for %s", rl.providerName)
	}

	// Wait for request bucket
	if err := rl.requestBucket.wait(ctx); err != nil {
		return err
	}

	// Wait for token bucket if we have an estimate
	if estimatedTokens > 0 {
		// Token bucket waits based on token count
		for i := 0; i < estimatedTokens/1000; i++ {
			if err := rl.tokenBucket.wait(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// RecordSuccess records a successful request
func (rl *ProviderRateLimiter) RecordSuccess() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.failures = 0
}

// RecordFailure records a failed request
func (rl *ProviderRateLimiter) RecordFailure(err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.failures++
	rl.lastFailure = time.Now()

	// Check if we should open the circuit
	if rl.failures >= rl.config.MaxConsecutiveFailures {
		// Check for rate limit errors - these shouldn't open circuit
		if !isRateLimitError(err) {
			rl.circuitOpen = true
			rl.circuitOpensAt = time.Now()
			fmt.Printf("  [rate-limiter] Circuit opened for %s (%d consecutive failures)\n",
				rl.providerName, rl.failures)
		}
	}
}

// isCircuitOpen checks if the circuit breaker is open
func (rl *ProviderRateLimiter) isCircuitOpen() bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if !rl.circuitOpen {
		return false
	}

	// Check if we should close the circuit
	if time.Since(rl.circuitOpensAt) > rl.config.CircuitBreakerTimeout {
		rl.mu.RUnlock()
		rl.mu.Lock()
		rl.circuitOpen = false
		rl.failures = 0
		rl.mu.Unlock()
		rl.mu.RLock()
		fmt.Printf("  [rate-limiter] Circuit closed for %s\n", rl.providerName)
		return false
	}

	return true
}

// ParseRetryAfter extracts retry delay from error or headers
func ParseRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}

	errStr := err.Error()

	// Check for retry-after in error message
	if idx := strings.Index(errStr, "retry after"); idx != -1 {
		// Try to parse seconds
		var seconds int
		if _, err := fmt.Sscanf(errStr[idx:], "retry after %d", &seconds); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}

	// Default backoff for rate limits
	if isRateLimitError(err) {
		return 5 * time.Second
	}

	return 0
}

// Global rate limiter registry
var rateLimiterRegistry = make(map[string]*ProviderRateLimiter)
var rateLimiterMu sync.RWMutex

// GetRateLimiter gets or creates a rate limiter for a provider
func GetRateLimiter(providerName string) *ProviderRateLimiter {
	rateLimiterMu.RLock()
	if rl, ok := rateLimiterRegistry[providerName]; ok {
		rateLimiterMu.RUnlock()
		return rl
	}
	rateLimiterMu.RUnlock()

	rateLimiterMu.Lock()
	defer rateLimiterMu.Unlock()

	// Double-check
	if rl, ok := rateLimiterRegistry[providerName]; ok {
		return rl
	}

	rl := NewProviderRateLimiter(providerName)
	rateLimiterRegistry[providerName] = rl
	return rl
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
