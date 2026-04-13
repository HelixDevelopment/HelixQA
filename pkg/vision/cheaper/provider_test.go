// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cheaper provides access to cost-effective vision providers for
// HelixQA autonomous QA sessions.
package cheaper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVisionResult_Fields(t *testing.T) {
	now := time.Now()
	result := VisionResult{
		Text:        "detected button at center",
		RawResponse: map[string]interface{}{"raw": "data"},
		Metadata: map[string]interface{}{
			"source": "test",
		},
		Duration:   250 * time.Millisecond,
		Model:      "ui-tars-7b",
		Provider:   "local",
		Timestamp:  now,
		CacheHit:   false,
		Confidence: 0.92,
	}

	assert.Equal(t, "detected button at center", result.Text)
	assert.Equal(t, map[string]interface{}{"raw": "data"}, result.RawResponse)
	assert.Equal(t, "test", result.Metadata["source"])
	assert.Equal(t, 250*time.Millisecond, result.Duration)
	assert.Equal(t, "ui-tars-7b", result.Model)
	assert.Equal(t, "local", result.Provider)
	assert.Equal(t, now, result.Timestamp)
	assert.False(t, result.CacheHit)
	assert.InDelta(t, 0.92, result.Confidence, 0.0001)
}

func TestVisionResult_ZeroValue(t *testing.T) {
	var result VisionResult

	assert.Equal(t, "", result.Text)
	assert.Nil(t, result.RawResponse)
	assert.Nil(t, result.Metadata)
	assert.Equal(t, time.Duration(0), result.Duration)
	assert.Equal(t, "", result.Model)
	assert.Equal(t, "", result.Provider)
	assert.True(t, result.Timestamp.IsZero())
	assert.False(t, result.CacheHit)
	assert.InDelta(t, 0.0, result.Confidence, 0.0001)
}

func TestVisionResult_CacheHit(t *testing.T) {
	result := VisionResult{
		Text:       "cached result",
		CacheHit:   true,
		Confidence: 1.0,
		Duration:   0,
	}

	assert.True(t, result.CacheHit)
	assert.InDelta(t, 1.0, result.Confidence, 0.0001)
	assert.Equal(t, time.Duration(0), result.Duration)
}

func TestProviderCapabilities_Defaults(t *testing.T) {
	caps := ProviderCapabilities{}

	assert.False(t, caps.SupportsStreaming)
	assert.Equal(t, 0, caps.MaxImageSize)
	assert.Nil(t, caps.SupportedFormats)
	assert.Equal(t, time.Duration(0), caps.AverageLatency)
	assert.False(t, caps.SupportsBatch)
	assert.InDelta(t, 0.0, caps.CostPer1MTokens, 0.0001)
}

func TestProviderCapabilities_FullyPopulated(t *testing.T) {
	caps := ProviderCapabilities{
		SupportsStreaming: true,
		MaxImageSize:      5 * 1024 * 1024, // 5 MB
		SupportedFormats:  []string{"png", "jpeg", "webp"},
		AverageLatency:    500 * time.Millisecond,
		SupportsBatch:     true,
		CostPer1MTokens:   0.15,
	}

	assert.True(t, caps.SupportsStreaming)
	assert.Equal(t, 5*1024*1024, caps.MaxImageSize)
	assert.Equal(t, []string{"png", "jpeg", "webp"}, caps.SupportedFormats)
	assert.Equal(t, 500*time.Millisecond, caps.AverageLatency)
	assert.True(t, caps.SupportsBatch)
	assert.InDelta(t, 0.15, caps.CostPer1MTokens, 0.0001)
}

func TestProviderConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config ProviderConfig
	}{
		{
			name: "minimal config",
			config: ProviderConfig{
				Name:    "ui-tars",
				Enabled: true,
			},
		},
		{
			name: "full config",
			config: ProviderConfig{
				Name:       "showui",
				Enabled:    true,
				Priority:   2,
				Config:     map[string]interface{}{"endpoint": "http://localhost:8080", "timeout": 30},
				FallbackTo: []string{"ui-tars", "glm4v"},
			},
		},
		{
			name: "disabled config",
			config: ProviderConfig{
				Name:    "omniparser",
				Enabled: false,
			},
		},
		{
			name: "high priority config",
			config: ProviderConfig{
				Name:     "qwen-vl",
				Enabled:  true,
				Priority: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Name)

			if tt.config.FallbackTo != nil {
				assert.Greater(t, len(tt.config.FallbackTo), 0)
			}

			if tt.config.Config != nil {
				assert.NotEmpty(t, tt.config.Config)
			}
		})
	}
}

func TestProviderConfig_ZeroValue(t *testing.T) {
	var cfg ProviderConfig

	assert.Equal(t, "", cfg.Name)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 0, cfg.Priority)
	assert.Nil(t, cfg.Config)
	assert.Nil(t, cfg.FallbackTo)
}

func TestProviderFactory_Type(t *testing.T) {
	// ProviderFactory is a function type — verify a conforming function can be assigned.
	var factory ProviderFactory = func(_ map[string]interface{}) (VisionProvider, error) {
		return nil, nil
	}

	assert.NotNil(t, factory)

	// Call the factory to verify the signature works.
	provider, err := factory(map[string]interface{}{"key": "value"})
	assert.NoError(t, err)
	assert.Nil(t, provider)
}
