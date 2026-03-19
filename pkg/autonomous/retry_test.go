// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), DefaultRetryConfig(), func(_ context.Context) error {
		calls++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	calls := 0
	cfg := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := Retry(context.Background(), cfg, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return fmt.Errorf("transient error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetry_AllAttemptsFail(t *testing.T) {
	calls := 0
	cfg := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := Retry(context.Background(), cfg, func(_ context.Context) error {
		calls++
		return fmt.Errorf("persistent error")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 2 retries")
	assert.Contains(t, err.Error(), "persistent error")
	assert.Equal(t, 3, calls) // initial + 2 retries
}

func TestRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, DefaultRetryConfig(), func(_ context.Context) error {
		return fmt.Errorf("should not reach")
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetry_ContextCanceledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cfg := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, cfg, func(_ context.Context) error {
		calls++
		return fmt.Errorf("error")
	})
	assert.Error(t, err)
	assert.LessOrEqual(t, calls, 2)
}

func TestRetry_ZeroMultiplier(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:   1,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   0, // should default to 2.0
	}

	calls := 0
	err := Retry(context.Background(), cfg, func(_ context.Context) error {
		calls++
		if calls < 2 {
			return fmt.Errorf("error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestRetryWithResult_Success(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}

	result, err := RetryWithResult(context.Background(), cfg, func(_ context.Context) (string, error) {
		return "hello", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestRetryWithResult_SuccessAfterRetry(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}
	calls := 0

	result, err := RetryWithResult(context.Background(), cfg, func(_ context.Context) (int, error) {
		calls++
		if calls < 2 {
			return 0, fmt.Errorf("not yet")
		}
		return 42, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestRetryWithResult_AllFail(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:   1,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
	}

	_, err := RetryWithResult(context.Background(), cfg, func(_ context.Context) (string, error) {
		return "", fmt.Errorf("fail")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 1 retries")
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		attempt    int
		initial    time.Duration
		multiplier float64
		maxDelay   time.Duration
		expected   time.Duration
	}{
		{0, time.Second, 2.0, 30 * time.Second, time.Second},
		{1, time.Second, 2.0, 30 * time.Second, 2 * time.Second},
		{2, time.Second, 2.0, 30 * time.Second, 4 * time.Second},
		{10, time.Second, 2.0, 30 * time.Second, 30 * time.Second}, // capped
		{0, 500 * time.Millisecond, 3.0, 10 * time.Second, 500 * time.Millisecond},
	}

	for _, tc := range tests {
		result := CalculateBackoff(
			tc.attempt, tc.initial, tc.multiplier, tc.maxDelay,
		)
		assert.Equal(t, tc.expected, result,
			"attempt=%d", tc.attempt)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, time.Second, cfg.InitialDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}
