// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"os"
	"sort"
)

// visionModelScore holds the scoring metrics for a vision-capable
// model. These values are derived from the LLMsVerifier
// VisionModelRegistry and kept in sync manually. Adding a direct
// import of LLMsVerifier would introduce a heavyweight dependency
// chain that is unnecessary at runtime.
type visionModelScore struct {
	// Provider matches a Provider.Name() value.
	Provider string

	// QualityScore is a quality rating (0-1) from benchmarks.
	QualityScore float64

	// ReliabilityScore is the uptime/reliability rating (0-1).
	ReliabilityScore float64

	// CostPer1kTokens is (InputCostPer1k + OutputCostPer1k).
	CostPer1kTokens float64

	// AvgLatencyMs is the average response latency.
	AvgLatencyMs int
}

// visionModelRegistry mirrors the essential scoring data from
// LLMsVerifier's VisionModelRegistry() (pkg/helixqa/models.go).
// Each entry represents the best vision model available from a
// given provider. The LLMsVerifier VisionStrategy (pkg/vision/)
// uses the same data for its scoring dimensions.
//
// When LLMsVerifier adds or updates models, this table should
// be updated to match. Both registries MUST stay in sync.
var visionModelRegistry = []visionModelScore{
	// Tier 0: Premium multi-modal vision (validated in 50+ QA sessions)
	{Provider: ProviderGoogle, QualityScore: 0.93, ReliabilityScore: 0.95, CostPer1kTokens: 0.0005, AvgLatencyMs: 2000},
	{Provider: ProviderOpenAI, QualityScore: 0.95, ReliabilityScore: 0.98, CostPer1kTokens: 0.020, AvgLatencyMs: 1200},
	{Provider: ProviderAnthropic, QualityScore: 0.94, ReliabilityScore: 0.97, CostPer1kTokens: 0.018, AvgLatencyMs: 1500},

	// Specialized vision API (quality high but availability varies)
	{Provider: "astica", QualityScore: 0.90, ReliabilityScore: 0.70, CostPer1kTokens: 0.001, AvgLatencyMs: 800},

	// Tier 2: Cost-effective
	{Provider: "qwen", QualityScore: 0.87, ReliabilityScore: 0.88, CostPer1kTokens: 0.003, AvgLatencyMs: 1100},
	{Provider: "kimi", QualityScore: 0.85, ReliabilityScore: 0.90, CostPer1kTokens: 0.0009, AvgLatencyMs: 1000},
	{Provider: "stepfun", QualityScore: 0.82, ReliabilityScore: 0.85, CostPer1kTokens: 0.0, AvgLatencyMs: 900},
	{Provider: "nvidia", QualityScore: 0.80, ReliabilityScore: 0.82, CostPer1kTokens: 0.0, AvgLatencyMs: 1000},
	{Provider: "githubmodels", QualityScore: 0.78, ReliabilityScore: 0.85, CostPer1kTokens: 0.0, AvgLatencyMs: 1400},
	{Provider: "xai", QualityScore: 0.80, ReliabilityScore: 0.88, CostPer1kTokens: 0.005, AvgLatencyMs: 1100},

	// Tier 3: Local / open-source
	{Provider: ProviderOllama, QualityScore: 0.65, ReliabilityScore: 0.80, CostPer1kTokens: 0.0, AvgLatencyMs: 3000},
}

// visionRegistryByProvider indexes the registry for O(1) lookup.
var visionRegistryByProvider map[string]visionModelScore

func init() {
	visionRegistryByProvider = make(
		map[string]visionModelScore, len(visionModelRegistry),
	)
	for _, m := range visionModelRegistry {
		visionRegistryByProvider[m.Provider] = m
	}
}

// scoredProvider pairs a Provider with a computed score for
// sorting purposes.
type scoredProvider struct {
	provider Provider
	score    float64
}

// scoreVisionProvider computes a composite score for a single
// provider. The formula weighs quality and reliability, then
// applies an availability multiplier (API key is configured)
// and a cost bonus (free or very cheap providers get a bump).
//
// Score = (0.6*quality + 0.4*reliability) * availabilityBoost * costBonus
func scoreVisionProvider(p Provider) float64 {
	info, found := visionRegistryByProvider[p.Name()]
	if !found {
		// Unknown provider: assign a low baseline so it is
		// tried only after all known providers.
		if isProviderAvailable(p.Name()) {
			return 0.30
		}
		return 0.10
	}

	// Base score: weighted combination of quality and reliability.
	base := 0.6*info.QualityScore + 0.4*info.ReliabilityScore

	// Availability boost: providers with configured API keys
	// (or that need none) get a 2x multiplier.
	avail := 0.5
	if isProviderAvailable(p.Name()) {
		avail = 1.0
	}

	// Cost bonus: free or very cheap providers get a small
	// bump to break ties in favor of lower cost.
	costBonus := 1.0
	if info.CostPer1kTokens == 0.0 {
		costBonus = 1.10
	} else if info.CostPer1kTokens < 0.002 {
		costBonus = 1.05
	}

	return base * avail * costBonus
}

// rankVisionProviders filters providers to those with vision
// support, scores each using the vision model registry, and
// returns them sorted by score (highest first). Providers
// without vision support are excluded.
func rankVisionProviders(providers []Provider) []Provider {
	var scored []scoredProvider
	for _, p := range providers {
		if !p.SupportsVision() {
			continue
		}
		scored = append(scored, scoredProvider{
			provider: p,
			score:    scoreVisionProvider(p),
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	result := make([]Provider, len(scored))
	for i, sp := range scored {
		result[i] = sp.provider
	}
	return result
}

// isProviderAvailable checks whether the given provider has
// its required API key or URL configured in the environment.
// Ollama is treated as always potentially available since it
// runs locally and only needs HELIX_OLLAMA_URL as a hint, not
// a hard requirement.
func isProviderAvailable(name string) bool {
	envKey, ok := ProviderEnvKeys[name]
	if !ok {
		// Provider has no known env key — assume available
		// (it might be configured via BaseURL in ProviderConfig).
		return true
	}
	if name == ProviderOllama {
		// Ollama doesn't strictly need an env var to be
		// available — it defaults to localhost:11434.
		return true
	}
	return os.Getenv(envKey) != ""
}
