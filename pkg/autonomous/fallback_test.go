// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackChain_FirstSucceeds(t *testing.T) {
	chain := NewFallbackChain(
		NamedProvider[string]{
			Name: "primary",
			Fn:   func(_ context.Context) (string, error) { return "primary-result", nil },
		},
		NamedProvider[string]{
			Name: "secondary",
			Fn:   func(_ context.Context) (string, error) { return "secondary-result", nil },
		},
	)

	result, err := chain.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "primary-result", result)
}

func TestFallbackChain_FirstFailsSecondSucceeds(t *testing.T) {
	chain := NewFallbackChain(
		NamedProvider[string]{
			Name: "primary",
			Fn:   func(_ context.Context) (string, error) { return "", fmt.Errorf("primary down") },
		},
		NamedProvider[string]{
			Name: "secondary",
			Fn:   func(_ context.Context) (string, error) { return "secondary-result", nil },
		},
	)

	result, err := chain.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "secondary-result", result)
}

func TestFallbackChain_AllFail(t *testing.T) {
	chain := NewFallbackChain(
		NamedProvider[string]{
			Name: "a",
			Fn:   func(_ context.Context) (string, error) { return "", fmt.Errorf("a failed") },
		},
		NamedProvider[string]{
			Name: "b",
			Fn:   func(_ context.Context) (string, error) { return "", fmt.Errorf("b failed") },
		},
	)

	_, err := chain.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all 2 providers failed")
	assert.Contains(t, err.Error(), "b failed")
}

func TestFallbackChain_NoProviders(t *testing.T) {
	chain := NewFallbackChain[string]()

	_, err := chain.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers configured")
}

func TestFallbackChain_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	chain := NewFallbackChain(
		NamedProvider[string]{
			Name: "a",
			Fn:   func(_ context.Context) (string, error) { return "ok", nil },
		},
	)

	_, err := chain.Execute(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFallbackChain_AddProvider(t *testing.T) {
	chain := NewFallbackChain[string]()
	assert.Equal(t, 0, chain.Len())

	chain.AddProvider(NamedProvider[string]{
		Name: "new",
		Fn:   func(_ context.Context) (string, error) { return "new-result", nil },
	})
	assert.Equal(t, 1, chain.Len())

	result, err := chain.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-result", result)
}

func TestFallbackChain_IntType(t *testing.T) {
	chain := NewFallbackChain(
		NamedProvider[int]{
			Name: "a",
			Fn:   func(_ context.Context) (int, error) { return 0, fmt.Errorf("fail") },
		},
		NamedProvider[int]{
			Name: "b",
			Fn:   func(_ context.Context) (int, error) { return 42, nil },
		},
	)

	result, err := chain.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

// Stress test: concurrent Execute calls.
func TestFallbackChain_Stress_ConcurrentExecute(t *testing.T) {
	chain := NewFallbackChain(
		NamedProvider[string]{
			Name: "provider",
			Fn:   func(_ context.Context) (string, error) { return "ok", nil },
		},
	)

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			result, err := chain.Execute(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, "ok", result)
		}()
	}
	wg.Wait()
}
