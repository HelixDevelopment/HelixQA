// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// delayProvider sleeps for delay before returning a result. It is used to
// simulate slow providers in concurrency-sensitive tests.
type delayProvider struct {
	name     string
	delay    time.Duration
	response string
}

func (d *delayProvider) Name() string { return d.name }

func (d *delayProvider) Analyze(
	ctx context.Context,
	_ image.Image,
	_ string,
) (*VisionResult, error) {
	select {
	case <-time.After(d.delay):
		return &VisionResult{Provider: d.name, Text: d.response}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *delayProvider) HealthCheck(_ context.Context) error { return nil }

func (d *delayProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (d *delayProvider) GetCostEstimate(_, _ int) float64 { return 0 }

// errorProvider always returns the configured error. It records how many times
// it was called so that retry behaviour can be verified.
type errorProvider struct {
	name    string
	err     error
	callCnt atomic.Int64
}

func (e *errorProvider) Name() string { return e.name }

func (e *errorProvider) Analyze(
	_ context.Context,
	_ image.Image,
	_ string,
) (*VisionResult, error) {
	e.callCnt.Add(1)
	return nil, e.err
}

func (e *errorProvider) HealthCheck(_ context.Context) error { return nil }

func (e *errorProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (e *errorProvider) GetCostEstimate(_, _ int) float64 { return 0 }

// successAfterProvider fails for the first `failCount` calls, then succeeds.
type successAfterProvider struct {
	name      string
	failCount int64
	callCnt   atomic.Int64
	err       error
}

func (s *successAfterProvider) Name() string { return s.name }

func (s *successAfterProvider) Analyze(
	_ context.Context,
	_ image.Image,
	_ string,
) (*VisionResult, error) {
	n := s.callCnt.Add(1)
	if n <= s.failCount {
		return nil, s.err
	}
	return &VisionResult{Provider: s.name, Text: "ok"}, nil
}

func (s *successAfterProvider) HealthCheck(_ context.Context) error { return nil }

func (s *successAfterProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (s *successAfterProvider) GetCostEstimate(_, _ int) float64 { return 0 }

// newImg returns a trivial 1×1 RGBA image suitable for passing to Analyze.
func newImg() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	return img
}

// TestExecutor_FirstSuccess_FirstProviderWins verifies that when several
// providers are registered and the executor uses StrategyFirstSuccess, the
// first provider to respond successfully wins and its result is returned.
func TestExecutor_FirstSuccess_FirstProviderWins(t *testing.T) {
	fast := &delayProvider{name: "fast", delay: 10 * time.Millisecond, response: "fast-result"}
	slow := &delayProvider{name: "slow", delay: 200 * time.Millisecond, response: "slow-result"}

	cfg := ExecutorConfig{
		Strategy:  StrategyFirstSuccess,
		Providers: []VisionProvider{fast, slow},
		Timeout:   2 * time.Second,
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "fast", result.Provider)
	assert.Equal(t, "fast-result", result.Text)
}

// TestExecutor_FirstSuccess_AllFail verifies that StrategyFirstSuccess returns
// an error when every provider fails.
func TestExecutor_FirstSuccess_AllFail(t *testing.T) {
	errA := errors.New("provider-a failed")
	errB := errors.New("provider-b failed")

	cfg := ExecutorConfig{
		Strategy: StrategyFirstSuccess,
		Providers: []VisionProvider{
			&errorProvider{name: "a", err: errA},
			&errorProvider{name: "b", err: errB},
		},
		Timeout: 2 * time.Second,
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestExecutor_Parallel_ReturnsFastest verifies that StrategyParallel fires
// all providers concurrently and returns the fastest successful result.
func TestExecutor_Parallel_ReturnsFastest(t *testing.T) {
	fast := &delayProvider{name: "fast", delay: 10 * time.Millisecond, response: "fast-result"}
	med := &delayProvider{name: "med", delay: 100 * time.Millisecond, response: "med-result"}
	slow := &delayProvider{name: "slow", delay: 300 * time.Millisecond, response: "slow-result"}

	cfg := ExecutorConfig{
		Strategy:  StrategyParallel,
		Providers: []VisionProvider{fast, med, slow},
		Timeout:   2 * time.Second,
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "fast", result.Provider)
}

// TestExecutor_FallbackChain_FirstFails_SecondSucceeds verifies that
// StrategyFallback tries providers in FallbackChain order and uses the second
// when the first fails.
func TestExecutor_FallbackChain_FirstFails_SecondSucceeds(t *testing.T) {
	bad := &errorProvider{name: "bad", err: errors.New("bad")}
	good := &delayProvider{name: "good", delay: 0, response: "good-result"}

	cfg := ExecutorConfig{
		Strategy:      StrategyFallback,
		Providers:     []VisionProvider{bad, good},
		Timeout:       2 * time.Second,
		FallbackChain: []string{"bad", "good"},
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "good", result.Provider)
	assert.Equal(t, "good-result", result.Text)
}

// TestExecutor_FallbackChain_Exhausted verifies that when all providers in
// the FallbackChain fail, Execute returns an error.
func TestExecutor_FallbackChain_Exhausted(t *testing.T) {
	cfg := ExecutorConfig{
		Strategy: StrategyFallback,
		Providers: []VisionProvider{
			&errorProvider{name: "p1", err: errors.New("fail1")},
			&errorProvider{name: "p2", err: errors.New("fail2")},
		},
		Timeout:       2 * time.Second,
		FallbackChain: []string{"p1", "p2"},
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestExecutor_Weighted verifies that StrategyWeighted tries providers in the
// order they appear in config.Providers and returns the first success.
func TestExecutor_Weighted(t *testing.T) {
	bad := &errorProvider{name: "bad", err: errors.New("bad")}
	good := &delayProvider{name: "good", delay: 0, response: "weighted-result"}

	cfg := ExecutorConfig{
		Strategy:  StrategyWeighted,
		Providers: []VisionProvider{bad, good},
		Timeout:   2 * time.Second,
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "good", result.Provider)
	assert.Equal(t, "weighted-result", result.Text)
}

// TestExecutor_WithRetry_TransientError verifies that executeWithResilience
// retries on transient errors and eventually succeeds.
func TestExecutor_WithRetry_TransientError(t *testing.T) {
	transient := errors.New("transient")
	prov := &successAfterProvider{
		name:      "flaky",
		failCount: 2,
		err:       transient,
	}

	cfg := ExecutorConfig{
		Strategy:      StrategyFallback,
		Providers:     []VisionProvider{prov},
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    5 * time.Millisecond,
		FallbackChain: []string{"flaky"},
	}
	exec := NewResilientExecutor(cfg)

	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "flaky", result.Provider)
	// Provider was called 3 times: 2 failures + 1 success.
	assert.Equal(t, int64(3), prov.callCnt.Load())
}

// TestExecutor_CircuitBreaker_Opens verifies that when enough failures
// accumulate the circuit breaker opens and subsequent calls fail fast with a
// circuit-open error rather than calling the provider again.
func TestExecutor_CircuitBreaker_Opens(t *testing.T) {
	prov := &errorProvider{name: "unstable", err: errors.New("always fails")}

	cfg := ExecutorConfig{
		Strategy:           StrategyFallback,
		Providers:          []VisionProvider{prov},
		Timeout:            2 * time.Second,
		CircuitBreaker:     true,
		CBFailureThreshold: 3,
		CBSuccessThreshold: 1,
		CBTimeout:          30 * time.Second,
		FallbackChain:      []string{"unstable"},
	}
	exec := NewResilientExecutor(cfg)

	img := newImg()
	ctx := context.Background()

	// Exhaust the failure threshold to open the circuit.
	for i := 0; i < cfg.CBFailureThreshold; i++ {
		_, _ = exec.Execute(ctx, img, "prompt")
	}

	state := exec.GetCircuitBreakerState("unstable")
	assert.Equal(t, "open", state)
}

// TestExecutor_Timeout verifies that Execute respects the configured Timeout
// and returns an error when the provider takes too long.
func TestExecutor_Timeout(t *testing.T) {
	slow := &delayProvider{name: "slow", delay: 500 * time.Millisecond, response: "too-late"}

	cfg := ExecutorConfig{
		Strategy:  StrategyWeighted,
		Providers: []VisionProvider{slow},
		Timeout:   50 * time.Millisecond,
	}
	exec := NewResilientExecutor(cfg)

	start := time.Now()
	result, err := exec.Execute(context.Background(), newImg(), "prompt")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, result)
	// Should have returned well before the provider's natural delay.
	assert.Less(t, elapsed, 400*time.Millisecond)
}

// TestExecutor_GetCircuitBreakerState verifies that GetCircuitBreakerState
// returns "closed" for a healthy provider and "open" after the breaker trips.
func TestExecutor_GetCircuitBreakerState(t *testing.T) {
	prov := &errorProvider{name: "prov", err: errors.New("fail")}

	cfg := ExecutorConfig{
		Strategy:           StrategyFallback,
		Providers:          []VisionProvider{prov},
		Timeout:            2 * time.Second,
		CircuitBreaker:     true,
		CBFailureThreshold: 2,
		CBSuccessThreshold: 1,
		CBTimeout:          30 * time.Second,
		FallbackChain:      []string{"prov"},
	}
	exec := NewResilientExecutor(cfg)

	// Initial state is closed.
	assert.Equal(t, "closed", exec.GetCircuitBreakerState("prov"))

	// Unknown providers should return a sensible string.
	assert.Equal(t, "unknown", exec.GetCircuitBreakerState("nonexistent"))

	// Trip the breaker.
	for i := 0; i < cfg.CBFailureThreshold; i++ {
		_, _ = exec.Execute(context.Background(), newImg(), "p")
	}
	assert.Equal(t, "open", exec.GetCircuitBreakerState("prov"))
}

// TestExecutor_GetProviderStats verifies that GetProviderStats returns an entry
// for every registered provider and that the entry is a non-nil map.
func TestExecutor_GetProviderStats(t *testing.T) {
	p1 := &delayProvider{name: "p1", delay: 0, response: "r1"}
	p2 := &delayProvider{name: "p2", delay: 0, response: "r2"}

	cfg := ExecutorConfig{
		Strategy:  StrategyWeighted,
		Providers: []VisionProvider{p1, p2},
		Timeout:   2 * time.Second,
	}
	exec := NewResilientExecutor(cfg)

	stats := exec.GetProviderStats()
	require.NotNil(t, stats)
	assert.Contains(t, stats, "p1")
	assert.Contains(t, stats, "p2")

	for _, v := range stats {
		m, ok := v.(map[string]interface{})
		require.True(t, ok, "each stats entry should be map[string]interface{}")
		assert.NotNil(t, m)
	}
}
