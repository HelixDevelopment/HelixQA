// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionserver

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	// Ensure none of the HELIX_VISION_* vars are set so we exercise pure
	// defaults.
	keys := []string{
		"HELIX_VISION_PROVIDER",
		"HELIX_VISION_FALLBACK_ENABLED",
		"HELIX_VISION_FALLBACK_CHAIN",
		"HELIX_VISION_PARALLEL",
		"HELIX_VISION_TIMEOUT",
		"HELIX_VISION_LEARNING",
		"HELIX_VISION_EXACT_CACHE",
		"HELIX_VISION_DIFFERENTIAL",
		"HELIX_VISION_VECTOR_MEMORY",
		"HELIX_VISION_FEW_SHOT",
		"HELIX_VISION_PERSIST_PATH",
		"HELIX_VISION_MAX_MEMORIES",
		"HELIX_VISION_CHANGE_THRESHOLD",
		"HELIX_VISION_LISTEN_ADDR",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}

	cfg := LoadConfig()
	require.NotNil(t, cfg)

	assert.Equal(t, "auto", cfg.Provider)
	assert.True(t, cfg.FallbackEnabled)
	assert.Equal(t, []string{"qwen25vl", "glm4v", "uitars", "showui"}, cfg.FallbackChain)
	assert.True(t, cfg.ParallelExecution)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.True(t, cfg.LearningEnabled)
	assert.True(t, cfg.ExactCache)
	assert.True(t, cfg.Differential)
	assert.True(t, cfg.VectorMemory)
	assert.True(t, cfg.FewShot)
	assert.Equal(t, "", cfg.PersistPath)
	assert.Equal(t, 100000, cfg.MaxMemories)
	assert.InDelta(t, 0.05, cfg.ChangeThreshold, 1e-9)
	assert.Equal(t, ":8090", cfg.ListenAddr)
}

func TestConfig_LoadFromEnv(t *testing.T) {
	t.Setenv("HELIX_VISION_PROVIDER", "glm4v")
	t.Setenv("HELIX_VISION_FALLBACK_ENABLED", "false")
	t.Setenv("HELIX_VISION_FALLBACK_CHAIN", "showui,uitars")
	t.Setenv("HELIX_VISION_PARALLEL", "false")
	t.Setenv("HELIX_VISION_TIMEOUT", "60s")
	t.Setenv("HELIX_VISION_LEARNING", "false")
	t.Setenv("HELIX_VISION_EXACT_CACHE", "false")
	t.Setenv("HELIX_VISION_DIFFERENTIAL", "false")
	t.Setenv("HELIX_VISION_VECTOR_MEMORY", "false")
	t.Setenv("HELIX_VISION_FEW_SHOT", "false")
	t.Setenv("HELIX_VISION_PERSIST_PATH", "/tmp/helix-persist")
	t.Setenv("HELIX_VISION_MAX_MEMORIES", "5000")
	t.Setenv("HELIX_VISION_CHANGE_THRESHOLD", "0.10")
	t.Setenv("HELIX_VISION_LISTEN_ADDR", ":9999")

	cfg := LoadConfig()
	require.NotNil(t, cfg)

	assert.Equal(t, "glm4v", cfg.Provider)
	assert.False(t, cfg.FallbackEnabled)
	assert.Equal(t, []string{"showui", "uitars"}, cfg.FallbackChain)
	assert.False(t, cfg.ParallelExecution)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.False(t, cfg.LearningEnabled)
	assert.False(t, cfg.ExactCache)
	assert.False(t, cfg.Differential)
	assert.False(t, cfg.VectorMemory)
	assert.False(t, cfg.FewShot)
	assert.Equal(t, "/tmp/helix-persist", cfg.PersistPath)
	assert.Equal(t, 5000, cfg.MaxMemories)
	assert.InDelta(t, 0.10, cfg.ChangeThreshold, 1e-9)
	assert.Equal(t, ":9999", cfg.ListenAddr)
}

func TestConfig_FallbackChainParsing(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected []string
	}{
		{
			name:     "single provider",
			envVal:   "qwen25vl",
			expected: []string{"qwen25vl"},
		},
		{
			name:     "multiple providers",
			envVal:   "qwen25vl,glm4v,uitars",
			expected: []string{"qwen25vl", "glm4v", "uitars"},
		},
		{
			name:     "providers with spaces",
			envVal:   " qwen25vl , glm4v , showui ",
			expected: []string{"qwen25vl", "glm4v", "showui"},
		},
		{
			name:     "empty string falls back to default",
			envVal:   "",
			expected: []string{"qwen25vl", "glm4v", "uitars", "showui"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVal == "" {
				os.Unsetenv("HELIX_VISION_FALLBACK_CHAIN")
			} else {
				t.Setenv("HELIX_VISION_FALLBACK_CHAIN", tc.envVal)
			}
			cfg := LoadConfig()
			assert.Equal(t, tc.expected, cfg.FallbackChain)
		})
	}
}

func TestConfig_BoolParsing(t *testing.T) {
	truthy := []string{"1", "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON"}
	for _, v := range truthy {
		t.Run("truthy_"+v, func(t *testing.T) {
			t.Setenv("HELIX_VISION_LEARNING", v)
			cfg := LoadConfig()
			assert.True(t, cfg.LearningEnabled, "expected true for %q", v)
		})
	}

	falsy := []string{"0", "false", "no", "off", "anything-else"}
	for _, v := range falsy {
		t.Run("falsy_"+v, func(t *testing.T) {
			t.Setenv("HELIX_VISION_LEARNING", v)
			cfg := LoadConfig()
			assert.False(t, cfg.LearningEnabled, "expected false for %q", v)
		})
	}
}

func TestConfig_InvalidNumericsFallToDefaults(t *testing.T) {
	t.Setenv("HELIX_VISION_MAX_MEMORIES", "not-a-number")
	t.Setenv("HELIX_VISION_CHANGE_THRESHOLD", "nan")
	t.Setenv("HELIX_VISION_TIMEOUT", "invalid-duration")

	cfg := LoadConfig()
	assert.Equal(t, 100000, cfg.MaxMemories)
	assert.InDelta(t, 0.05, cfg.ChangeThreshold, 1e-9)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
}
