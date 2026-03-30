// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRankVisionProviders_ExcludesNonVision verifies that providers
// without vision support are excluded from the ranked list.
func TestRankVisionProviders_ExcludesNonVision(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "text-only", vision: false},
		&mockProvider{name: ProviderGoogle, vision: true},
	}
	ranked := rankVisionProviders(providers)
	require.Len(t, ranked, 1)
	assert.Equal(t, ProviderGoogle, ranked[0].Name())
}

// TestRankVisionProviders_EmptyInput returns empty slice.
func TestRankVisionProviders_EmptyInput(t *testing.T) {
	ranked := rankVisionProviders(nil)
	assert.Empty(t, ranked)
}

// TestRankVisionProviders_AllNonVision returns empty slice.
func TestRankVisionProviders_AllNonVision(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "a", vision: false},
		&mockProvider{name: "b", vision: false},
	}
	ranked := rankVisionProviders(providers)
	assert.Empty(t, ranked)
}

// TestRankVisionProviders_HigherQualityFirst verifies that a
// provider with a higher quality score ranks above one with a
// lower score when both have API keys available.
func TestRankVisionProviders_HigherQualityFirst(t *testing.T) {
	// Set env keys so both are "available".
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("GEMINI_API_KEY", "test-key")

	providers := []Provider{
		&mockProvider{name: ProviderGoogle, vision: true},  // quality 0.88
		&mockProvider{name: ProviderOpenAI, vision: true},  // quality 0.95
	}
	ranked := rankVisionProviders(providers)
	require.Len(t, ranked, 2)
	assert.Equal(t, ProviderOpenAI, ranked[0].Name(),
		"OpenAI (quality 0.95) should rank above Google (quality 0.88)")
	assert.Equal(t, ProviderGoogle, ranked[1].Name())
}

// TestRankVisionProviders_AvailableBeatsUnavailable verifies that
// a provider with an API key configured ranks above a higher-quality
// provider without one.
func TestRankVisionProviders_AvailableBeatsUnavailable(t *testing.T) {
	// Only Google has a key; OpenAI does not.
	t.Setenv("GEMINI_API_KEY", "test-key")
	// Ensure OpenAI key is unset.
	t.Setenv("OPENAI_API_KEY", "")

	providers := []Provider{
		&mockProvider{name: ProviderOpenAI, vision: true},  // quality 0.95 but unavailable
		&mockProvider{name: ProviderGoogle, vision: true},   // quality 0.88, available
	}
	ranked := rankVisionProviders(providers)
	require.Len(t, ranked, 2)
	assert.Equal(t, ProviderGoogle, ranked[0].Name(),
		"Available Google should rank above unavailable OpenAI")
}

// TestRankVisionProviders_OllamaAlwaysAvailable verifies that
// Ollama is treated as available even without HELIX_OLLAMA_URL set.
func TestRankVisionProviders_OllamaAlwaysAvailable(t *testing.T) {
	t.Setenv("HELIX_OLLAMA_URL", "")

	available := isProviderAvailable(ProviderOllama)
	assert.True(t, available,
		"Ollama should be available without HELIX_OLLAMA_URL")
}

// TestRankVisionProviders_UnknownProviderGetsLowScore verifies
// that an unknown vision-capable provider gets a low baseline
// score and appears after known providers.
func TestRankVisionProviders_UnknownProviderGetsLowScore(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")

	providers := []Provider{
		&mockProvider{name: "mystery-vision", vision: true},
		&mockProvider{name: ProviderGoogle, vision: true},
	}
	ranked := rankVisionProviders(providers)
	require.Len(t, ranked, 2)
	assert.Equal(t, ProviderGoogle, ranked[0].Name(),
		"Known Google should rank above unknown provider")
	assert.Equal(t, "mystery-vision", ranked[1].Name())
}

// TestRankVisionProviders_FreeCostBonus verifies that free
// providers (cost == 0) get a score bump over similarly-rated
// paid providers.
func TestRankVisionProviders_FreeCostBonus(t *testing.T) {
	// Both providers need keys to be "available".
	t.Setenv("NVIDIA_API_KEY", "test-key")
	t.Setenv("XAI_API_KEY", "test-key")

	// nvidia: quality 0.80, cost 0.0 (free)
	// xai:    quality 0.80, cost 0.005 (paid)
	providers := []Provider{
		&mockProvider{name: "xai", vision: true},
		&mockProvider{name: "nvidia", vision: true},
	}
	ranked := rankVisionProviders(providers)
	require.Len(t, ranked, 2)

	nvidiaScore := scoreVisionProvider(
		&mockProvider{name: "nvidia", vision: true},
	)
	xaiScore := scoreVisionProvider(
		&mockProvider{name: "xai", vision: true},
	)
	assert.Greater(t, nvidiaScore, xaiScore,
		"Free nvidia should score higher than paid xai at same quality")
}

// TestScoreVisionProvider_KnownProvider verifies that a known
// provider gets a score derived from registry data, not the
// unknown baseline.
func TestScoreVisionProvider_KnownProvider(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")

	score := scoreVisionProvider(
		&mockProvider{name: ProviderGoogle, vision: true},
	)
	// Google: base = 0.6*0.88 + 0.4*0.96 = 0.912
	// Available: *1.0, cost < 0.002: *1.05 = ~0.9576
	assert.Greater(t, score, 0.90,
		"Google with key should score above 0.90")
}

// TestScoreVisionProvider_UnavailableProvider verifies that a
// provider without its API key gets a halved availability factor.
func TestScoreVisionProvider_UnavailableProvider(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	score := scoreVisionProvider(
		&mockProvider{name: ProviderAnthropic, vision: true},
	)
	// Anthropic: base = 0.6*0.94 + 0.4*0.97 = 0.952
	// Unavailable: *0.5 = 0.476
	assert.Less(t, score, 0.50,
		"Unavailable Anthropic should score below 0.50")
}

// TestIsProviderAvailable_WithKey verifies that a provider with
// its env var set is reported as available.
func TestIsProviderAvailable_WithKey(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "test-key")
	assert.True(t, isProviderAvailable("nvidia"))
}

// TestIsProviderAvailable_WithoutKey verifies that a provider
// without its env var set is reported as unavailable.
func TestIsProviderAvailable_WithoutKey(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "")
	assert.False(t, isProviderAvailable("nvidia"))
}

// TestIsProviderAvailable_UnknownProvider verifies that a
// provider not in ProviderEnvKeys is treated as available.
func TestIsProviderAvailable_UnknownProvider(t *testing.T) {
	assert.True(t, isProviderAvailable("totally-unknown-provider"))
}

// TestRankVisionProviders_PreservesAllVisionProviders verifies
// that ranking does not drop any vision-capable provider.
func TestRankVisionProviders_PreservesAllVisionProviders(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("ANTHROPIC_API_KEY", "k")
	t.Setenv("GEMINI_API_KEY", "k")
	t.Setenv("KIMI_API_KEY", "k")
	t.Setenv("NVIDIA_API_KEY", "k")

	providers := []Provider{
		&mockProvider{name: ProviderOpenAI, vision: true},
		&mockProvider{name: ProviderAnthropic, vision: true},
		&mockProvider{name: ProviderGoogle, vision: true},
		&mockProvider{name: "kimi", vision: true},
		&mockProvider{name: "nvidia", vision: true},
		&mockProvider{name: ProviderOllama, vision: true},
		&mockProvider{name: "text-only", vision: false},
	}
	ranked := rankVisionProviders(providers)
	assert.Len(t, ranked, 6,
		"All 6 vision-capable providers should be in the ranked list")
}

// TestVisionRegistryByProvider_Indexed verifies the init()
// function correctly indexes all registry entries.
func TestVisionRegistryByProvider_Indexed(t *testing.T) {
	assert.Len(t, visionRegistryByProvider, len(visionModelRegistry),
		"Index should contain every registry entry")
	for _, m := range visionModelRegistry {
		_, ok := visionRegistryByProvider[m.Provider]
		assert.True(t, ok, "Provider %q should be indexed", m.Provider)
	}
}
