// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig holds configuration for retry with exponential backoff.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (default 2.0).
	Multiplier float64
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// RetryFunc is a function that can be retried.
type RetryFunc func(ctx context.Context) error

// Retry executes fn with exponential backoff retries.
func Retry(
	ctx context.Context,
	cfg RetryConfig,
	fn RetryFunc,
) error {
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt < cfg.MaxRetries {
			// Wait with backoff before next attempt.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff.
			delay = time.Duration(
				float64(delay) * cfg.Multiplier,
			)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return fmt.Errorf(
		"failed after %d retries: %w",
		cfg.MaxRetries, lastErr,
	)
}

// RetryWithResult executes a function that returns a result
// with exponential backoff.
func RetryWithResult[T any](
	ctx context.Context,
	cfg RetryConfig,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}

	var zero T
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if attempt < cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(
				float64(delay) * cfg.Multiplier,
			)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return zero, fmt.Errorf(
		"failed after %d retries: %w",
		cfg.MaxRetries, lastErr,
	)
}

// CalculateBackoff computes the delay for a given attempt.
func CalculateBackoff(
	attempt int,
	initial time.Duration,
	multiplier float64,
	maxDelay time.Duration,
) time.Duration {
	delay := time.Duration(
		float64(initial) * math.Pow(multiplier, float64(attempt)),
	)
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}
