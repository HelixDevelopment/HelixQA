// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"sync"
)

// FallbackChain tries providers in order until one succeeds.
// Used for vision provider fallback when the primary fails.
type FallbackChain[T any] struct {
	providers []NamedProvider[T]
	mu        sync.RWMutex
}

// NamedProvider pairs a name with a function that attempts
// an operation.
type NamedProvider[T any] struct {
	Name string
	Fn   func(ctx context.Context) (T, error)
}

// NewFallbackChain creates a FallbackChain with the given
// providers, tried in order.
func NewFallbackChain[T any](
	providers ...NamedProvider[T],
) *FallbackChain[T] {
	return &FallbackChain[T]{
		providers: providers,
	}
}

// Execute tries each provider in order and returns the first
// successful result. If all providers fail, returns the last
// error wrapped with context.
func (fc *FallbackChain[T]) Execute(
	ctx context.Context,
) (T, error) {
	fc.mu.RLock()
	providers := make([]NamedProvider[T], len(fc.providers))
	copy(providers, fc.providers)
	fc.mu.RUnlock()

	var zero T
	var lastErr error

	for _, p := range providers {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, err := p.Fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("%s: %w", p.Name, err)
	}

	if lastErr == nil {
		return zero, fmt.Errorf("no providers configured")
	}
	return zero, fmt.Errorf(
		"all %d providers failed, last: %w",
		len(providers), lastErr,
	)
}

// AddProvider adds a provider to the end of the chain.
func (fc *FallbackChain[T]) AddProvider(p NamedProvider[T]) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.providers = append(fc.providers, p)
}

// Len returns the number of providers in the chain.
func (fc *FallbackChain[T]) Len() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.providers)
}
