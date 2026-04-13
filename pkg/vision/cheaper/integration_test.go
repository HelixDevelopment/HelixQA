// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper/cache"
)

// TestIntegration_FullPipeline creates a Registry, registers a stub provider,
// creates a ResilientExecutor with fallback strategy, and verifies that Analyze
// returns a valid result end-to-end.
func TestIntegration_FullPipeline(t *testing.T) {
	reg := NewRegistry()
	reg.Register("integration-stub", stubFactory("integration-stub"))

	provider, err := reg.Create("integration-stub", nil)
	require.NoError(t, err)
	require.NotNil(t, provider)

	exec := NewResilientExecutor(ExecutorConfig{
		Strategy:      StrategyFallback,
		Providers:     []VisionProvider{provider},
		FallbackChain: []string{"integration-stub"},
		Timeout:       2 * time.Second,
	})

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	result, err := exec.Execute(context.Background(), img, "describe this image")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "integration-stub", result.Provider)
}

// TestIntegration_CacheLayers uses ExactCache directly: puts an entry and
// verifies a cache hit on the second call without invoking the provider.
func TestIntegration_CacheLayers(t *testing.T) {
	c := cache.NewExactCache(1000)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	prompt := "tap the login button"

	// Cold cache — must miss.
	got, ok := c.Get(img, prompt)
	assert.False(t, ok, "cold cache must miss")
	assert.Nil(t, got)

	// Populate the cache.
	resp := &cache.CachedResponse{
		Text:      "cached text",
		Model:     "stub-model",
		Duration:  5 * time.Millisecond,
		Timestamp: time.Now(),
	}
	c.Put(img, prompt, resp)

	// Warm cache — must hit.
	got, ok = c.Get(img, prompt)
	require.True(t, ok, "warm cache must hit after Put")
	require.NotNil(t, got)
	assert.Equal(t, "cached text", got.Text)
	assert.Equal(t, "stub-model", got.Model)
}

// TestIntegration_RegistryToExecutor registers multiple stub providers, creates
// an executor, and verifies that first_success strategy returns a valid result
// from one of the registered providers.
func TestIntegration_RegistryToExecutor(t *testing.T) {
	reg := NewRegistry()
	names := []string{"alpha-int", "beta-int", "gamma-int"}
	for _, n := range names {
		n := n
		reg.Register(n, stubFactory(n))
	}

	providers := make([]VisionProvider, 0, len(names))
	for _, n := range names {
		p, err := reg.Create(n, nil)
		require.NoError(t, err)
		providers = append(providers, p)
	}

	exec := NewResilientExecutor(ExecutorConfig{
		Strategy:  StrategyFirstSuccess,
		Providers: providers,
		Timeout:   2 * time.Second,
	})

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	result, err := exec.Execute(context.Background(), img, "identify elements")

	require.NoError(t, err)
	require.NotNil(t, result)

	// The result must come from one of the registered providers.
	found := false
	for _, n := range names {
		if result.Provider == n {
			found = true
			break
		}
	}
	assert.True(t, found, "result.Provider %q must be one of the registered names", result.Provider)
}
