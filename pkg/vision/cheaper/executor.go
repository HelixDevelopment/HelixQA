// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/circuitbreaker"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/failsafe-go/failsafe-go/timeout"
)

// ExecutionStrategy controls how ResilientExecutor dispatches calls across
// multiple VisionProviders.
type ExecutionStrategy string

const (
	// StrategyFirstSuccess fires all providers concurrently and returns the
	// first successful result, cancelling the rest.
	StrategyFirstSuccess ExecutionStrategy = "first_success"

	// StrategyParallel fires all providers concurrently, waits for all of them
	// to finish, and returns the fastest successful result.
	StrategyParallel ExecutionStrategy = "parallel"

	// StrategyFallback tries providers in the order defined by FallbackChain,
	// moving to the next only when the current one fails.
	StrategyFallback ExecutionStrategy = "fallback"

	// StrategyWeighted tries providers in the order they appear in
	// ExecutorConfig.Providers (highest priority first) and returns the first
	// success.
	StrategyWeighted ExecutionStrategy = "weighted"
)

// ExecutorConfig holds all settings for a ResilientExecutor.
type ExecutorConfig struct {
	// Strategy selects the dispatch algorithm.
	Strategy ExecutionStrategy

	// Providers is the ordered list of VisionProviders available to the
	// executor. For StrategyWeighted the order is the priority order.
	Providers []VisionProvider

	// Timeout is the wall-clock deadline for a single top-level Execute call.
	// A zero value means no timeout is applied.
	Timeout time.Duration

	// RetryAttempts is the total number of attempts (including the first) when
	// the provider returns a transient error. 0 or 1 means no retries.
	RetryAttempts int

	// RetryDelay is the base delay between retry attempts.
	RetryDelay time.Duration

	// CircuitBreaker enables per-provider circuit breakers.
	CircuitBreaker bool

	// CBFailureThreshold is the number of consecutive failures required to open
	// the circuit breaker.
	CBFailureThreshold int

	// CBSuccessThreshold is the number of consecutive successes in the
	// half-open state required to close the circuit breaker.
	CBSuccessThreshold int

	// CBTimeout is how long the circuit breaker stays open before transitioning
	// to half-open.
	CBTimeout time.Duration

	// FallbackChain lists provider names in the order they should be tried when
	// using StrategyFallback.
	FallbackChain []string

	// MaxConcurrency limits how many provider calls may be in-flight
	// simultaneously. 0 means unlimited.
	MaxConcurrency int
}

// ResilientExecutor dispatches vision analysis calls across multiple
// VisionProviders according to the configured ExecutionStrategy. It wraps each
// individual provider call with failsafe-go policies (timeout, retry, circuit
// breaker). All methods are safe for concurrent use.
type ResilientExecutor struct {
	config          ExecutorConfig
	providerMap     map[string]VisionProvider
	circuitBreakers map[string]circuitbreaker.CircuitBreaker[*VisionResult]
	mu              sync.RWMutex
}

// NewResilientExecutor creates a new ResilientExecutor from the given config.
// It builds the provider map from config.Providers and, when
// config.CircuitBreaker is true, initialises per-provider circuit breakers.
func NewResilientExecutor(config ExecutorConfig) *ResilientExecutor {
	e := &ResilientExecutor{
		config:          config,
		providerMap:     make(map[string]VisionProvider, len(config.Providers)),
		circuitBreakers: make(map[string]circuitbreaker.CircuitBreaker[*VisionResult]),
	}

	for _, p := range config.Providers {
		e.providerMap[p.Name()] = p
	}

	if config.CircuitBreaker {
		for _, p := range config.Providers {
			ft := uint(config.CBFailureThreshold)
			if ft == 0 {
				ft = 5
			}
			st := uint(config.CBSuccessThreshold)
			if st == 0 {
				st = 1
			}
			cbTimeout := config.CBTimeout
			if cbTimeout == 0 {
				cbTimeout = 30 * time.Second
			}

			cb := circuitbreaker.NewBuilder[*VisionResult]().
				WithFailureThreshold(ft).
				WithSuccessThreshold(st).
				WithDelay(cbTimeout).
				Build()

			e.circuitBreakers[p.Name()] = cb
		}
	}

	return e
}

// Execute dispatches the vision analysis call according to the configured
// strategy and returns the first successful VisionResult.
func (e *ResilientExecutor) Execute(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	switch e.config.Strategy {
	case StrategyFirstSuccess:
		return e.executeFirstSuccess(ctx, img, prompt)
	case StrategyParallel:
		return e.executeParallel(ctx, img, prompt)
	case StrategyFallback:
		return e.executeFallbackChain(ctx, img, prompt)
	case StrategyWeighted:
		return e.executeWeighted(ctx, img, prompt)
	default:
		return nil, fmt.Errorf("cheaper: unknown execution strategy %q", e.config.Strategy)
	}
}

// executeFirstSuccess fires all providers concurrently and returns the first
// successful result. All in-flight calls are cancelled once a winner is found.
func (e *ResilientExecutor) executeFirstSuccess(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	if len(e.config.Providers) == 0 {
		return nil, errors.New("cheaper: no providers configured")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type outcome struct {
		result *VisionResult
		err    error
	}

	ch := make(chan outcome, len(e.config.Providers))
	for _, p := range e.config.Providers {
		p := p
		go func() {
			res, err := e.executeWithResilience(ctx, p, img, prompt)
			ch <- outcome{res, err}
		}()
	}

	var lastErr error
	for range e.config.Providers {
		out := <-ch
		if out.err == nil && out.result != nil {
			cancel()
			return out.result, nil
		}
		lastErr = out.err
	}
	return nil, fmt.Errorf("cheaper: all providers failed: %w", lastErr)
}

// executeParallel fires all providers concurrently, waits for every result,
// and returns the one with the shortest duration that succeeded.
func (e *ResilientExecutor) executeParallel(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	if len(e.config.Providers) == 0 {
		return nil, errors.New("cheaper: no providers configured")
	}

	type outcome struct {
		result *VisionResult
		err    error
	}

	ch := make(chan outcome, len(e.config.Providers))
	for _, p := range e.config.Providers {
		p := p
		go func() {
			res, err := e.executeWithResilience(ctx, p, img, prompt)
			ch <- outcome{res, err}
		}()
	}

	var (
		best    *VisionResult
		lastErr error
	)
	for range e.config.Providers {
		out := <-ch
		if out.err == nil && out.result != nil {
			if best == nil || out.result.Duration < best.Duration {
				best = out.result
			}
		} else if out.err != nil {
			lastErr = out.err
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, fmt.Errorf("cheaper: all providers failed: %w", lastErr)
}

// executeFallbackChain tries providers in FallbackChain order, returning the
// first successful result. Falls back to config.Providers order when
// FallbackChain is empty.
func (e *ResilientExecutor) executeFallbackChain(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	chain := e.config.FallbackChain
	if len(chain) == 0 {
		for _, p := range e.config.Providers {
			chain = append(chain, p.Name())
		}
	}

	var lastErr error
	for _, name := range chain {
		e.mu.RLock()
		p, ok := e.providerMap[name]
		e.mu.RUnlock()
		if !ok {
			lastErr = fmt.Errorf("cheaper: provider %q not found", name)
			continue
		}

		res, err := e.executeWithResilience(ctx, p, img, prompt)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("cheaper: fallback chain exhausted: %w", lastErr)
}

// executeWeighted tries providers in the order they appear in
// config.Providers (highest priority first) and returns the first success.
func (e *ResilientExecutor) executeWeighted(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	if len(e.config.Providers) == 0 {
		return nil, errors.New("cheaper: no providers configured")
	}

	var lastErr error
	for _, p := range e.config.Providers {
		res, err := e.executeWithResilience(ctx, p, img, prompt)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("cheaper: all weighted providers failed: %w", lastErr)
}

// executeWithResilience wraps a single provider call with failsafe-go policies:
//  1. A timeout policy (when config.Timeout > 0).
//  2. A retry policy (when config.RetryAttempts > 1).
//  3. A circuit breaker (when config.CircuitBreaker is true and a breaker
//     exists for the provider).
//
// Policies are composed in the order: Timeout → Retry → CircuitBreaker → Call,
// so the timeout governs the whole retry loop.
func (e *ResilientExecutor) executeWithResilience(
	ctx context.Context,
	p VisionProvider,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	// Build the policy chain. Policies are collected in innermost→outermost
	// order and reversed before passing to failsafe.With, so the timeout (last
	// appended) ends up as the outermost wrapper and governs the entire retry
	// loop.
	var policies []failsafe.Policy[*VisionResult]

	// Innermost: circuit breaker (guards the raw call).
	if e.config.CircuitBreaker {
		e.mu.RLock()
		cb, hasCB := e.circuitBreakers[p.Name()]
		e.mu.RUnlock()
		if hasCB {
			policies = append(policies, cb)
		}
	}

	// Middle: retry policy.
	if e.config.RetryAttempts > 1 {
		delay := e.config.RetryDelay
		if delay <= 0 {
			delay = 10 * time.Millisecond
		}
		rp := retrypolicy.NewBuilder[*VisionResult]().
			WithMaxAttempts(e.config.RetryAttempts).
			WithDelay(delay).
			Build()
		policies = append(policies, rp)
	}

	// Outermost: timeout (wraps everything including retries).
	if e.config.Timeout > 0 {
		tp := timeout.New[*VisionResult](e.config.Timeout)
		policies = append(policies, tp)
	}

	// callWithExec uses the Execution's context so that the timeout policy's
	// child-context cancellation reaches the provider's select loop.
	callWithExec := func(exec failsafe.Execution[*VisionResult]) (*VisionResult, error) {
		execCtx := exec.Context()
		if execCtx == nil {
			execCtx = ctx
		}
		start := time.Now()
		res, err := p.Analyze(execCtx, img, prompt)
		if err != nil {
			return nil, err
		}
		res.Duration = time.Since(start)
		return res, nil
	}

	if len(policies) == 0 {
		// No policies — call directly with the outer context.
		start := time.Now()
		res, err := p.Analyze(ctx, img, prompt)
		if err != nil {
			return nil, err
		}
		res.Duration = time.Since(start)
		return res, nil
	}

	// Reverse so that the last appended (timeout) becomes policies[0], i.e.
	// the outermost wrapper in failsafe.With(policies[0], ..., policies[n-1]).
	reversed := make([]failsafe.Policy[*VisionResult], len(policies))
	for i, pol := range policies {
		reversed[len(policies)-1-i] = pol
	}

	return failsafe.With(reversed...).WithContext(ctx).GetWithExecution(callWithExec)
}

// GetCircuitBreakerState returns the current state string ("closed",
// "open", "half-open") of the circuit breaker for the named provider. Returns
// "unknown" when either circuit breakers are disabled or no breaker exists for
// the name.
func (e *ResilientExecutor) GetCircuitBreakerState(name string) string {
	if !e.config.CircuitBreaker {
		return "unknown"
	}
	e.mu.RLock()
	cb, ok := e.circuitBreakers[name]
	e.mu.RUnlock()
	if !ok {
		return "unknown"
	}
	return cb.State().String()
}

// GetProviderStats returns a map keyed by provider name. Each value is a
// map[string]interface{} with the provider's name and (when circuit breakers
// are enabled) the current circuit breaker state.
func (e *ResilientExecutor) GetProviderStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := make(map[string]interface{}, len(e.providerMap))
	for name := range e.providerMap {
		entry := map[string]interface{}{
			"name": name,
		}
		if e.config.CircuitBreaker {
			if cb, ok := e.circuitBreakers[name]; ok {
				entry["circuit_breaker_state"] = cb.State().String()
			}
		}
		stats[name] = entry
	}
	return stats
}
