// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"image"
	"os"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testProvider struct{ name string }

func (p *testProvider) Analyze(_ context.Context, _ image.Image, _ string) (*cheaper.VisionResult, error) {
	return &cheaper.VisionResult{Text: "test", Provider: p.name, Model: "test-model", Duration: time.Millisecond, Timestamp: time.Now()}, nil
}
func (p *testProvider) Name() string                               { return p.name }
func (p *testProvider) HealthCheck(_ context.Context) error        { return nil }
func (p *testProvider) GetCapabilities() cheaper.ProviderCapabilities { return cheaper.ProviderCapabilities{} }
func (p *testProvider) GetCostEstimate(_ int, _ int) float64       { return 0 }

func TestBuild_NoProviders(t *testing.T) {
	_, err := Build(nil, Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers")
}

func TestBuild_WithoutLearning(t *testing.T) {
	providers := []cheaper.VisionProvider{&testProvider{name: "test"}}
	cfg := Config{
		FallbackChain:  []string{"test"},
		Strategy:       cheaper.StrategyFallback,
		Timeout:        5 * time.Second,
		Learning:       false,
	}
	bp, err := Build(providers, cfg)
	require.NoError(t, err)
	assert.Equal(t, "cheaper-cheaper-executor", bp.Name())
	assert.True(t, bp.SupportsVision())
}

func TestBuild_WithLearning(t *testing.T) {
	providers := []cheaper.VisionProvider{&testProvider{name: "test"}}
	cfg := Config{
		FallbackChain:       []string{"test"},
		Strategy:            cheaper.StrategyFallback,
		Timeout:             5 * time.Second,
		Learning:            true,
		SimilarityThreshold: 0.85,
	}
	bp, err := Build(providers, cfg)
	require.NoError(t, err)
	assert.Equal(t, "cheaper-cheaper-learning", bp.Name())
}

func TestEnabled(t *testing.T) {
	os.Setenv("HELIX_VISION_CHEAPER_ENABLED", "true")
	defer os.Unsetenv("HELIX_VISION_CHEAPER_ENABLED")
	assert.True(t, Enabled())
}

func TestEnabled_False(t *testing.T) {
	os.Unsetenv("HELIX_VISION_CHEAPER_ENABLED")
	assert.False(t, Enabled())
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, cheaper.StrategyFallback, cfg.Strategy)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 0.05, cfg.ChangeThreshold)
	assert.Len(t, cfg.FallbackChain, 4)
}
