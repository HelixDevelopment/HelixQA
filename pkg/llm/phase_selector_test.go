// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPhaseModelSelector_SelectForPhase_Execute prefers
// a vision-capable provider for the execute phase.
func TestPhaseModelSelector_SelectForPhase_Execute(
	t *testing.T,
) {
	t.Setenv("GEMINI_API_KEY", "k")
	t.Setenv("OPENAI_API_KEY", "k")

	providers := []Provider{
		&mockProvider{name: "text-only", vision: false},
		&mockProvider{name: ProviderGoogle, vision: true},
		&mockProvider{name: ProviderOpenAI, vision: true},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("execute")
	require.NotNil(t, best)
	assert.True(t, best.SupportsVision(),
		"Execute phase should select a vision provider")
}

// TestPhaseModelSelector_SelectForPhase_Plan prefers a
// high-quality reasoning provider.
func TestPhaseModelSelector_SelectForPhase_Plan(
	t *testing.T,
) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("HELIX_OLLAMA_URL", "http://localhost:11434")

	providers := []Provider{
		&mockProvider{
			name: ProviderOpenAI, vision: true,
		}, // quality 0.95
		&mockProvider{
			name: ProviderOllama, vision: true,
		}, // quality 0.65
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("plan")
	require.NotNil(t, best)
	assert.Equal(t, ProviderOpenAI, best.Name(),
		"Plan phase should prefer higher-quality model "+
			"for reasoning")
}

// TestPhaseModelSelector_SelectForPhase_Learn works like
// plan — reasoning-focused.
func TestPhaseModelSelector_SelectForPhase_Learn(
	t *testing.T,
) {
	t.Setenv("ANTHROPIC_API_KEY", "k")

	providers := []Provider{
		&mockProvider{
			name: ProviderAnthropic, vision: true,
		},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("learn")
	require.NotNil(t, best)
	assert.Equal(t, ProviderAnthropic, best.Name())
}

// TestPhaseModelSelector_SelectForPhase_Curiosity prefers
// vision + JSON (same as execute).
func TestPhaseModelSelector_SelectForPhase_Curiosity(
	t *testing.T,
) {
	t.Setenv("GEMINI_API_KEY", "k")

	providers := []Provider{
		&mockProvider{name: "chat-only", vision: false},
		&mockProvider{name: ProviderGoogle, vision: true},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("curiosity")
	require.NotNil(t, best)
	assert.True(t, best.SupportsVision(),
		"Curiosity phase should select vision provider")
}

// TestPhaseModelSelector_SelectForPhase_Analyze prefers
// vision.
func TestPhaseModelSelector_SelectForPhase_Analyze(
	t *testing.T,
) {
	t.Setenv("GEMINI_API_KEY", "k")

	providers := []Provider{
		&mockProvider{name: "chat-only", vision: false},
		&mockProvider{name: ProviderGoogle, vision: true},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("analyze")
	require.NotNil(t, best)
	assert.True(t, best.SupportsVision(),
		"Analyze phase should select vision provider")
}

// TestPhaseModelSelector_SelectForPhase_UnknownPhase
// returns a reasonable default.
func TestPhaseModelSelector_SelectForPhase_UnknownPhase(
	t *testing.T,
) {
	t.Setenv("OPENAI_API_KEY", "k")

	providers := []Provider{
		&mockProvider{name: ProviderOpenAI, vision: true},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("custom-phase")
	require.NotNil(t, best,
		"Unknown phase should still return a provider")
}

// TestPhaseModelSelector_SelectForPhase_NoProviders
// returns nil when no providers are available.
func TestPhaseModelSelector_SelectForPhase_NoProviders(
	t *testing.T,
) {
	sel := NewPhaseModelSelector(nil)
	best := sel.SelectForPhase("execute")
	assert.Nil(t, best)
}

// TestPhaseModelSelector_SelectForPhase_EmptyProviders
// returns nil.
func TestPhaseModelSelector_SelectForPhase_EmptyProviders(
	t *testing.T,
) {
	sel := NewPhaseModelSelector([]Provider{})
	best := sel.SelectForPhase("plan")
	assert.Nil(t, best)
}

// TestPhaseModelSelector_VisionBeatsNonVision verifies
// that for vision phases, a vision provider scores higher
// than a non-vision provider even if the non-vision
// provider has a higher base quality.
func TestPhaseModelSelector_VisionBeatsNonVision(
	t *testing.T,
) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("ANTHROPIC_API_KEY", "k")

	// Both available, similar quality. But one has no
	// vision.
	providers := []Provider{
		&mockProvider{
			name: ProviderAnthropic, vision: false,
		},
		&mockProvider{
			name: ProviderOpenAI, vision: true,
		},
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("execute")
	require.NotNil(t, best)
	assert.Equal(t, ProviderOpenAI, best.Name(),
		"Vision provider should win for execute phase")
}

// TestPhaseModelSelector_AvailabilityMatters verifies
// that an unavailable provider is penalized.
func TestPhaseModelSelector_AvailabilityMatters(
	t *testing.T,
) {
	t.Setenv("GEMINI_API_KEY", "k")
	t.Setenv("OPENAI_API_KEY", "")

	providers := []Provider{
		&mockProvider{
			name: ProviderOpenAI, vision: true,
		}, // unavailable
		&mockProvider{
			name: ProviderGoogle, vision: true,
		}, // available
	}
	sel := NewPhaseModelSelector(providers)
	best := sel.SelectForPhase("execute")
	require.NotNil(t, best)
	assert.Equal(t, ProviderGoogle, best.Name(),
		"Available provider should beat unavailable one")
}

// TestPhaseModelSelector_ScoreProvider_BaseScore verifies
// the base score for an unknown provider.
func TestPhaseModelSelector_ScoreProvider_BaseScore(
	t *testing.T,
) {
	sel := NewPhaseModelSelector(nil)
	p := &mockProvider{name: "unknown", vision: false}
	strat := PhaseStrategy{Name: "test"}
	score := sel.scoreProvider(p, strat)
	// Unknown provider, available, no phase preferences.
	assert.InDelta(t, 0.5, score, 0.01,
		"Unknown provider base score should be ~0.5")
}

// TestPhaseModelSelector_ScoreProvider_VisionBoost
// verifies the vision capability bonus.
func TestPhaseModelSelector_ScoreProvider_VisionBoost(
	t *testing.T,
) {
	t.Setenv("GEMINI_API_KEY", "k")

	sel := NewPhaseModelSelector(nil)
	p := &mockProvider{name: ProviderGoogle, vision: true}
	withVision := PhaseStrategy{PreferVision: true}
	withoutVision := PhaseStrategy{PreferVision: false}

	scoreV := sel.scoreProvider(p, withVision)
	scoreNV := sel.scoreProvider(p, withoutVision)
	assert.Greater(t, scoreV, scoreNV,
		"Vision boost should increase score")
}

// TestPhaseModelSelector_ScoreProvider_ReasoningBoost
// verifies the reasoning quality bonus.
func TestPhaseModelSelector_ScoreProvider_ReasoningBoost(
	t *testing.T,
) {
	t.Setenv("OPENAI_API_KEY", "k")

	sel := NewPhaseModelSelector(nil)
	p := &mockProvider{name: ProviderOpenAI, vision: true}
	withReasoning := PhaseStrategy{PreferReasoning: true}
	withoutReasoning := PhaseStrategy{
		PreferReasoning: false,
	}

	scoreR := sel.scoreProvider(p, withReasoning)
	scoreNR := sel.scoreProvider(p, withoutReasoning)
	assert.Greater(t, scoreR, scoreNR,
		"Reasoning boost should increase score")
}

// TestPhaseModelSelector_SetStrategy allows overriding
// the phase strategy.
func TestPhaseModelSelector_SetStrategy(t *testing.T) {
	sel := NewPhaseModelSelector(nil)
	custom := PhaseStrategy{
		Name:         "custom",
		PreferVision: true,
		PreferJSON:   true,
	}
	sel.SetStrategy("my-phase", custom)
	got := sel.Strategy("my-phase")
	assert.Equal(t, custom, got)
}

// TestPhaseModelSelector_Strategy_DefaultPhases verifies
// all five default phases have strategies.
func TestPhaseModelSelector_Strategy_DefaultPhases(
	t *testing.T,
) {
	sel := NewPhaseModelSelector(nil)
	for _, phase := range []string{
		"learn", "plan", "execute", "curiosity", "analyze",
	} {
		strat := sel.Strategy(phase)
		assert.NotEmpty(t, strat.Name,
			"Phase %q should have a strategy", phase)
	}
}

// TestPhaseModelSelector_Strategy_UnknownPhase returns
// zero-value strategy.
func TestPhaseModelSelector_Strategy_UnknownPhase(
	t *testing.T,
) {
	sel := NewPhaseModelSelector(nil)
	strat := sel.Strategy("nonexistent")
	assert.Empty(t, strat.Name)
}

// TestPhaseModelSelector_Providers returns the provider
// list.
func TestPhaseModelSelector_Providers(t *testing.T) {
	providers := []Provider{
		&mockProvider{name: "a"},
		&mockProvider{name: "b"},
	}
	sel := NewPhaseModelSelector(providers)
	assert.Len(t, sel.Providers(), 2)
}

// TestPhaseModelSelector_AsticaPenalizedForJSON verifies
// that astica (high quality, very cheap, specialized) is
// penalized for JSON-requiring phases via the registry-
// driven heuristic.
func TestPhaseModelSelector_AsticaPenalizedForJSON(
	t *testing.T,
) {
	t.Setenv("ASTICA_API_KEY", "k")
	t.Setenv("GEMINI_API_KEY", "k")

	sel := NewPhaseModelSelector(nil)
	astica := &mockProvider{
		name: "astica", vision: true,
	}
	google := &mockProvider{
		name: ProviderGoogle, vision: true,
	}

	// Execute phase requires JSON.
	strat := sel.Strategy("execute")
	asticaScore := sel.scoreProvider(astica, strat)
	googleScore := sel.scoreProvider(google, strat)

	assert.Greater(t, googleScore, asticaScore,
		"Google should score higher than astica for "+
			"JSON-requiring execute phase")
}

// TestPhaseModelSelector_HighQualityBoostedForJSON
// verifies that premium models get a JSON bonus.
func TestPhaseModelSelector_HighQualityBoostedForJSON(
	t *testing.T,
) {
	t.Setenv("OPENAI_API_KEY", "k")

	sel := NewPhaseModelSelector(nil)
	p := &mockProvider{name: ProviderOpenAI, vision: true}

	jsonStrat := PhaseStrategy{PreferJSON: true}
	noJSONStrat := PhaseStrategy{PreferJSON: false}

	scoreJSON := sel.scoreProvider(p, jsonStrat)
	scoreNoJSON := sel.scoreProvider(p, noJSONStrat)

	assert.Greater(t, scoreJSON, scoreNoJSON,
		"High-quality provider should get JSON bonus")
}

// TestIsGeneralPurposeLLM verifies the heuristic.
func TestIsGeneralPurposeLLM(t *testing.T) {
	tests := []struct {
		name string
		entry visionModelScore
		want  bool
	}{
		{
			name: "openai (expensive)",
			entry: visionModelScore{
				CostPer1kTokens: 0.020,
			},
			want: true,
		},
		{
			name: "astica (cheap specialized)",
			entry: visionModelScore{
				CostPer1kTokens: 0.001,
			},
			want: false,
		},
		{
			name: "free tier (nvidia)",
			entry: visionModelScore{
				CostPer1kTokens: 0.0,
			},
			want: true,
		},
		{
			name: "kimi (very cheap)",
			entry: visionModelScore{
				CostPer1kTokens: 0.0009,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want,
				isGeneralPurposeLLM(tt.entry))
		})
	}
}
