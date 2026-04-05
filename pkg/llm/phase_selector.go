// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"fmt"
	"sort"
)

// PhaseStrategy describes the capability requirements for
// a pipeline phase. The selector uses these to score
// providers dynamically — no provider names are hardcoded.
type PhaseStrategy struct {
	// Name identifies this strategy (e.g. "navigation",
	// "analysis", "planning").
	Name string

	// PreferVision is true for phases that analyze
	// screenshots (execute, curiosity, analyze).
	PreferVision bool

	// PreferJSON is true for phases that need structured
	// JSON output (execute, curiosity, plan).
	PreferJSON bool

	// PreferReasoning is true for phases that benefit
	// from strong reasoning (learn, plan).
	PreferReasoning bool
}

// defaultPhaseStrategies maps pipeline phase names to
// their capability requirements.
var defaultPhaseStrategies = map[string]PhaseStrategy{
	"learn": {
		Name:            "planning",
		PreferReasoning: true,
	},
	"plan": {
		Name:            "planning",
		PreferReasoning: true,
		PreferJSON:      true,
	},
	"execute": {
		Name:         "navigation",
		PreferVision: true,
		PreferJSON:   true,
	},
	"curiosity": {
		Name:         "navigation",
		PreferVision: true,
		PreferJSON:   true,
	},
	"analyze": {
		Name:         "analysis",
		PreferVision: true,
	},
}

// PhaseModelSelector dynamically selects the best LLM
// provider for each pipeline phase based on the phase's
// capability requirements and each provider's scored
// attributes from the vision registry.
//
// Scoring is entirely data-driven: PhaseStrategy defines
// what capabilities to look for, and the vision model
// registry + API key availability determine provider
// scores. No provider-specific preferences are hardcoded.
type PhaseModelSelector struct {
	allProviders []Provider
	strategies   map[string]PhaseStrategy
}

// NewPhaseModelSelector creates a selector with the given
// providers and default phase strategies.
func NewPhaseModelSelector(
	providers []Provider,
) *PhaseModelSelector {
	return &PhaseModelSelector{
		allProviders: providers,
		strategies:   copyStrategies(defaultPhaseStrategies),
	}
}

// copyStrategies returns a shallow copy of the strategy
// map so callers cannot mutate the defaults.
func copyStrategies(
	src map[string]PhaseStrategy,
) map[string]PhaseStrategy {
	dst := make(map[string]PhaseStrategy, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SetStrategy overrides the strategy for a phase. This
// allows custom phases or tuning existing ones.
func (s *PhaseModelSelector) SetStrategy(
	phase string, strat PhaseStrategy,
) {
	s.strategies[phase] = strat
}

// SelectForPhase returns the best-scoring provider for the
// given pipeline phase. Returns nil if no providers are
// available.
func (s *PhaseModelSelector) SelectForPhase(
	phase string,
) Provider {
	ranked := s.SelectRankedForPhase(phase)
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0]
}

// SelectRankedForPhase returns all providers sorted by their
// score for the given pipeline phase (highest first). This
// enables callers to build fallback chains — if the primary
// provider fails, the next one in the list is the best
// alternative. Returns nil if no providers are available.
func (s *PhaseModelSelector) SelectRankedForPhase(
	phase string,
) []Provider {
	if len(s.allProviders) == 0 {
		return nil
	}

	strat, ok := s.strategies[phase]
	if !ok {
		strat = PhaseStrategy{Name: "default"}
	}

	scored := make([]scoredProvider, 0, len(s.allProviders))
	for _, p := range s.allProviders {
		scored = append(scored, scoredProvider{
			provider: p,
			score:    s.scoreProvider(p, strat),
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]Provider, len(scored))
	for i, sp := range scored {
		result[i] = sp.provider
	}

	if len(result) > 0 {
		var names []string
		for _, sp := range scored {
			names = append(names,
				fmt.Sprintf("%s(%.3f)", sp.provider.Name(), sp.score),
			)
		}
		fmt.Printf(
			"  [phase-selector] %s -> %v\n",
			phase, names,
		)
	}

	return result
}

// SelectAdaptiveForPhase returns an AdaptiveProvider wrapping
// all available providers ordered by their phase-specific
// scores. This gives automatic fallback: if the primary
// provider fails, the next best is tried immediately, with no
// single point of failure. Returns nil if no providers exist.
func (s *PhaseModelSelector) SelectAdaptiveForPhase(
	phase string,
) *AdaptiveProvider {
	ranked := s.SelectRankedForPhase(phase)
	if len(ranked) == 0 {
		return nil
	}
	ap := NewAdaptiveProvider(ranked...)
	ap.SetPhase(phase)
	return ap
}

// scoreProvider computes a composite score for a provider
// given the phase strategy requirements. The score is
// built from:
//   - Base quality/reliability from visionRegistryByProvider
//   - API key availability
//   - Vision capability match
//   - JSON production capability (penalizes known non-JSON
//     producers via low quality scores, NOT hardcoded names)
//   - Reasoning quality (boosted by high quality scores)
//
// All scoring is registry-driven — no provider names are
// tested directly.
func (s *PhaseModelSelector) scoreProvider(
	p Provider, strat PhaseStrategy,
) float64 {
	score := 0.5 // base score for unknown providers

	entry, known := visionRegistryByProvider[p.Name()]

	if known {
		// Replace base with registry-derived score.
		// Blend quality and reliability.
		score = 0.5*entry.QualityScore +
			0.3*entry.ReliabilityScore
	}

	// ── Vision capability match ─────────────────────
	if strat.PreferVision {
		if p.SupportsVision() {
			score += 0.3
		} else {
			// Non-vision provider is a poor fit for a
			// vision phase.
			score -= 0.2
		}
	}

	// ── JSON production capability ──────────────────
	// Providers with very high quality scores tend to
	// produce well-structured JSON. Providers with low
	// quality or specialized APIs (description-only)
	// are penalized when JSON is needed.
	if strat.PreferJSON && known {
		if entry.QualityScore < 0.70 {
			// Low-quality models struggle with JSON.
			score -= 0.15
		} else if entry.QualityScore >= 0.90 {
			// Premium models produce reliable JSON.
			score += 0.1
		}
		// Specialized vision APIs (very cheap, high
		// quality but description-only) get a penalty
		// because their output is free-form text, not
		// structured JSON for navigation.
		if entry.CostPer1kTokens < 0.002 &&
			entry.QualityScore > 0.95 &&
			!isGeneralPurposeLLM(entry) {
			score -= 0.3
		}
	}

	// ── Reasoning quality ───────────────────────────
	if strat.PreferReasoning && known {
		// Higher quality models are better reasoners.
		score += entry.QualityScore * 0.2
	}

	// ── Availability multiplier ─────────────────────
	if isProviderAvailable(p.Name()) {
		score *= 1.0
	} else {
		score *= 0.5
	}

	return score
}

// isGeneralPurposeLLM heuristically determines whether a
// registry entry represents a general-purpose LLM (vs a
// specialized vision API). General-purpose LLMs have
// higher per-token costs because they do full text
// generation, not just image description.
func isGeneralPurposeLLM(entry visionModelScore) bool {
	// Providers with output cost >= $0.005/1k are
	// definitely general-purpose LLMs (OpenAI, Anthropic,
	// xAI). Specialized APIs (astica) cost < $0.002/1k.
	// Free-tier providers are ambiguous but are generally
	// API wrappers, not specialized vision-only services.
	return entry.CostPer1kTokens >= 0.005 ||
		entry.CostPer1kTokens == 0.0
}

// Providers returns the full list of providers known to
// the selector.
func (s *PhaseModelSelector) Providers() []Provider {
	return s.allProviders
}

// Strategy returns the strategy for the given phase, or
// an empty PhaseStrategy if the phase is unknown.
func (s *PhaseModelSelector) Strategy(
	phase string,
) PhaseStrategy {
	return s.strategies[phase]
}
