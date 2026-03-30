// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCostTracker(t *testing.T) {
	ct := NewCostTracker()
	require.NotNil(t, ct)
	assert.Equal(t, 0, ct.CallCount())
	assert.Equal(t, 0.0, ct.TotalCost())
}

func TestCostTracker_Record_CalculatesCost(t *testing.T) {
	ct := NewCostTracker()

	// OpenAI: $0.005/1K input, $0.015/1K output
	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 500, true,
	)

	assert.Equal(t, 1, ct.CallCount())

	// Expected: (1000/1000)*0.005 + (500/1000)*0.015
	//         = 0.005 + 0.0075 = 0.0125
	assert.InDelta(t, 0.0125, ct.TotalCost(), 0.0001)
}

func TestCostTracker_Record_MultipleProviders(t *testing.T) {
	ct := NewCostTracker()

	// OpenAI call
	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		2000, 1000, true,
	)
	// Google call
	ct.Record(
		ProviderGoogle, "gemini-2.0-flash", "execute",
		"vision", 5000, 2000, true,
	)
	// Ollama call (free)
	ct.Record(
		ProviderOllama, "llava:7b", "curiosity", "vision",
		3000, 1000, true,
	)

	assert.Equal(t, 3, ct.CallCount())

	// OpenAI: (2000/1000)*0.005 + (1000/1000)*0.015 = 0.025
	// Google: (5000/1000)*0.0001 + (2000/1000)*0.0004
	//       = 0.0005 + 0.0008 = 0.0013
	// Ollama: 0
	expected := 0.025 + 0.0013
	assert.InDelta(t, expected, ct.TotalCost(), 0.0001)
}

func TestCostTracker_CostByProvider(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 1000, true,
	)
	ct.Record(
		ProviderGoogle, "gemini-2.0-flash", "execute",
		"vision", 1000, 1000, true,
	)
	ct.Record(
		ProviderOpenAI, "gpt-4o", "analyze", "chat",
		1000, 1000, true,
	)

	byProvider := ct.CostByProvider()
	assert.Len(t, byProvider, 2)

	// OpenAI: 2 calls * ((1000/1000)*0.005 + (1000/1000)*0.015)
	//       = 2 * 0.020 = 0.040
	assert.InDelta(t, 0.040, byProvider[ProviderOpenAI], 0.0001)

	// Google: (1000/1000)*0.0001 + (1000/1000)*0.0004 = 0.0005
	assert.InDelta(t, 0.0005, byProvider[ProviderGoogle], 0.0001)
}

func TestCostTracker_CostByModel(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 500, true,
	)
	ct.Record(
		ProviderOpenAI, "gpt-4o-mini", "execute", "chat",
		1000, 500, true,
	)

	byModel := ct.CostByModel()
	assert.Len(t, byModel, 2)
	assert.Contains(t, byModel, "gpt-4o")
	assert.Contains(t, byModel, "gpt-4o-mini")
}

func TestCostTracker_CostByPhase(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 1000, true,
	)
	ct.Record(
		ProviderGoogle, "gemini", "execute", "vision",
		2000, 1000, true,
	)
	ct.Record(
		ProviderGoogle, "gemini", "curiosity", "vision",
		2000, 1000, true,
	)
	ct.Record(
		ProviderOpenAI, "gpt-4o", "analyze", "chat",
		1000, 1000, true,
	)

	byPhase := ct.CostByPhase()
	assert.Len(t, byPhase, 4)
	assert.Contains(t, byPhase, "plan")
	assert.Contains(t, byPhase, "execute")
	assert.Contains(t, byPhase, "curiosity")
	assert.Contains(t, byPhase, "analyze")

	// Plan: OpenAI 1K/1K = 0.020
	assert.InDelta(t, 0.020, byPhase["plan"], 0.0001)
}

func TestCostTracker_Summary(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 500, true,
	)
	ct.Record(
		ProviderGoogle, "gemini", "execute", "vision",
		2000, 1000, true,
	)
	ct.Record(
		ProviderOllama, "llava:7b", "curiosity", "vision",
		3000, 1500, false,
	)

	summary := ct.Summary()

	assert.Equal(t, 3, summary.TotalCalls)
	assert.Equal(t, 6000, summary.TotalInputTokens)
	assert.Equal(t, 3000, summary.TotalOutputTokens)
	assert.Greater(t, summary.TotalCostUSD, 0.0)

	// By provider.
	assert.Len(t, summary.ByProvider, 3)
	assert.Equal(t, 1, summary.ByProvider[ProviderOpenAI].Calls)
	assert.Equal(
		t, "gpt-4o",
		summary.ByProvider[ProviderOpenAI].Model,
	)

	// By phase.
	assert.Len(t, summary.ByPhase, 3)

	// By call type.
	assert.Len(t, summary.ByCallType, 2)
	assert.Contains(t, summary.ByCallType, "chat")
	assert.Contains(t, summary.ByCallType, "vision")

	// Records included in full summary.
	assert.Len(t, summary.Records, 3)
}

func TestCostTracker_SummaryCompact(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 500, true,
	)

	compact := ct.SummaryCompact()
	assert.Equal(t, 1, compact.TotalCalls)
	assert.Nil(t, compact.Records)
}

func TestCostTracker_ZeroCost_FreeProviders(t *testing.T) {
	ct := NewCostTracker()

	// Ollama is free.
	ct.Record(
		ProviderOllama, "llava:7b", "curiosity", "vision",
		10000, 5000, true,
	)

	assert.Equal(t, 0.0, ct.TotalCost())

	summary := ct.Summary()
	assert.Equal(t, 0.0, summary.TotalCostUSD)
	assert.Equal(t, 10000, summary.TotalInputTokens)
	assert.Equal(t, 5000, summary.TotalOutputTokens)
	pc := summary.ByProvider[ProviderOllama]
	assert.Equal(t, 0.0, pc.TotalCostUSD)
	assert.Equal(t, 1, pc.Calls)
}

func TestCostTracker_UnknownProvider_ZeroCost(t *testing.T) {
	ct := NewCostTracker()

	// Unknown provider should have zero cost (not panic).
	ct.Record(
		"unknown-provider", "some-model", "execute",
		"vision", 5000, 2000, true,
	)

	assert.Equal(t, 1, ct.CallCount())
	assert.Equal(t, 0.0, ct.TotalCost())
}

func TestCostTracker_SetRate(t *testing.T) {
	ct := NewCostTracker()

	// Set a custom rate for a new provider.
	ct.SetRate("custom", CostRate{
		InputPer1k:  0.01,
		OutputPer1k: 0.02,
	})

	ct.Record(
		"custom", "custom-v1", "plan", "chat",
		1000, 1000, true,
	)

	// Expected: (1000/1000)*0.01 + (1000/1000)*0.02 = 0.03
	assert.InDelta(t, 0.03, ct.TotalCost(), 0.0001)
}

func TestCostTracker_SetRate_OverridesExisting(t *testing.T) {
	ct := NewCostTracker()

	// Override OpenAI rate.
	ct.SetRate(ProviderOpenAI, CostRate{
		InputPer1k:  0.001,
		OutputPer1k: 0.002,
	})

	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 1000, true,
	)

	// Expected with new rate: 0.001 + 0.002 = 0.003
	assert.InDelta(t, 0.003, ct.TotalCost(), 0.0001)
}

func TestCostTracker_ConcurrentAccess(t *testing.T) {
	ct := NewCostTracker()
	var wg sync.WaitGroup

	// Spawn 100 goroutines writing and reading concurrently.
	for i := 0; i < 50; i++ {
		wg.Add(2)

		// Writer goroutine.
		go func(n int) {
			defer wg.Done()
			provider := ProviderOpenAI
			if n%2 == 0 {
				provider = ProviderGoogle
			}
			ct.Record(
				provider, "model", "execute", "vision",
				100, 50, true,
			)
		}(i)

		// Reader goroutine.
		go func() {
			defer wg.Done()
			_ = ct.TotalCost()
			_ = ct.CostByProvider()
			_ = ct.CostByModel()
			_ = ct.CostByPhase()
			_ = ct.CallCount()
			_ = ct.SummaryCompact()
		}()
	}

	wg.Wait()

	assert.Equal(t, 50, ct.CallCount())
	assert.Greater(t, ct.TotalCost(), 0.0)
}

func TestCostTracker_EmptySummary(t *testing.T) {
	ct := NewCostTracker()

	summary := ct.Summary()

	assert.Equal(t, 0.0, summary.TotalCostUSD)
	assert.Equal(t, 0, summary.TotalCalls)
	assert.Equal(t, 0, summary.TotalInputTokens)
	assert.Equal(t, 0, summary.TotalOutputTokens)
	assert.Empty(t, summary.ByProvider)
	assert.Empty(t, summary.ByPhase)
	assert.Empty(t, summary.ByCallType)
	assert.Empty(t, summary.Records)
}

func TestCostTracker_FailedCallsStillTracked(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderOpenAI, "gpt-4o", "execute", "vision",
		500, 0, false,
	)

	assert.Equal(t, 1, ct.CallCount())
	// Input cost still counts even on failure.
	// (500/1000)*0.005 = 0.0025
	assert.InDelta(t, 0.0025, ct.TotalCost(), 0.0001)

	summary := ct.Summary()
	assert.Equal(t, 1, summary.TotalCalls)
	assert.False(t, summary.Records[0].Success)
}

func TestCostTracker_AnthropicRates(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		ProviderAnthropic, "claude-sonnet-4", "analyze",
		"chat", 10000, 4000, true,
	)

	// (10000/1000)*0.003 + (4000/1000)*0.015
	// = 0.030 + 0.060 = 0.090
	assert.InDelta(t, 0.090, ct.TotalCost(), 0.0001)
}

func TestCostTracker_KimiRates(t *testing.T) {
	ct := NewCostTracker()

	ct.Record(
		"kimi", "kimi-k2.5", "execute", "vision",
		10000, 5000, true,
	)

	// (10000/1000)*0.0003 + (5000/1000)*0.0006
	// = 0.003 + 0.003 = 0.006
	assert.InDelta(t, 0.006, ct.TotalCost(), 0.0001)
}

func TestCostTracker_SummaryProviderCostAccumulates(
	t *testing.T,
) {
	ct := NewCostTracker()

	// Two calls to same provider with different models.
	ct.Record(
		ProviderOpenAI, "gpt-4o", "plan", "chat",
		1000, 500, true,
	)
	ct.Record(
		ProviderOpenAI, "gpt-4o-mini", "analyze", "chat",
		1000, 500, true,
	)

	summary := ct.Summary()
	pc := summary.ByProvider[ProviderOpenAI]
	assert.Equal(t, 2, pc.Calls)
	assert.Equal(t, 2000, pc.InputTokens)
	assert.Equal(t, 1000, pc.OutputTokens)
	// Model should be the last one recorded.
	assert.Equal(t, "gpt-4o-mini", pc.Model)
}
