// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"sync"
	"time"
)

// CostRecord tracks a single LLM API call's cost.
type CostRecord struct {
	// Provider is the provider name (e.g. "openai", "google").
	Provider string `json:"provider"`

	// Model is the model identifier used for the call.
	Model string `json:"model"`

	// Timestamp is when the call was made.
	Timestamp time.Time `json:"timestamp"`

	// InputTokens is the number of input/prompt tokens.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of output/completion tokens.
	OutputTokens int `json:"output_tokens"`

	// InputCost is the input token cost in USD.
	InputCost float64 `json:"input_cost_usd"`

	// OutputCost is the output token cost in USD.
	OutputCost float64 `json:"output_cost_usd"`

	// TotalCost is InputCost + OutputCost in USD.
	TotalCost float64 `json:"total_cost_usd"`

	// Phase is the pipeline phase ("learn", "plan", "execute",
	// "curiosity", "analyze").
	Phase string `json:"phase"`

	// Success indicates whether the API call succeeded.
	Success bool `json:"success"`

	// CallType is "chat" or "vision".
	CallType string `json:"call_type"`
}

// CostRate holds per-1K-token cost rates for a provider/model.
type CostRate struct {
	// InputPer1k is the cost per 1,000 input tokens in USD.
	InputPer1k float64

	// OutputPer1k is the cost per 1,000 output tokens in USD.
	OutputPer1k float64
}

// CostSummary is the complete cost breakdown for a session.
type CostSummary struct {
	// TotalCostUSD is the total session cost in USD.
	TotalCostUSD float64 `json:"total_cost_usd"`

	// TotalCalls is the number of LLM API calls made.
	TotalCalls int `json:"total_calls"`

	// TotalInputTokens across all calls.
	TotalInputTokens int `json:"total_input_tokens"`

	// TotalOutputTokens across all calls.
	TotalOutputTokens int `json:"total_output_tokens"`

	// ByProvider maps provider name to its cost breakdown.
	ByProvider map[string]ProviderCost `json:"by_provider"`

	// ByPhase maps phase name to total cost in USD.
	ByPhase map[string]float64 `json:"by_phase"`

	// ByCallType maps call type ("chat"/"vision") to cost.
	ByCallType map[string]float64 `json:"by_call_type"`

	// Records holds all individual cost records. Omitted from
	// JSON when empty to keep reports compact.
	Records []CostRecord `json:"records,omitempty"`
}

// ProviderCost holds the cost breakdown for a single provider.
type ProviderCost struct {
	// Provider name.
	Provider string `json:"provider"`

	// Model identifier (last used).
	Model string `json:"model"`

	// Calls is the number of API calls.
	Calls int `json:"calls"`

	// InputTokens total for this provider.
	InputTokens int `json:"input_tokens"`

	// OutputTokens total for this provider.
	OutputTokens int `json:"output_tokens"`

	// TotalCostUSD for this provider.
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// CostTracker accumulates LLM API call costs across a session.
// It is safe for concurrent use.
type CostTracker struct {
	mu      sync.RWMutex
	records []CostRecord
	rates   map[string]CostRate
}

// NewCostTracker creates a CostTracker pre-populated with cost
// rates from the visionModelRegistry and LLMsVerifier model
// data. Providers not in the registry are assumed free (zero
// cost).
func NewCostTracker() *CostTracker {
	rates := make(map[string]CostRate)

	// Populate from the vision model registry. The registry
	// stores CostPer1kTokens as input+output combined, so we
	// use the more precise per-direction rates from
	// LLMsVerifier where available.
	preciseRates := map[string]CostRate{
		"astica":       {InputPer1k: 0.0005, OutputPer1k: 0.0005},
		ProviderOpenAI: {InputPer1k: 0.005, OutputPer1k: 0.015},
		ProviderAnthropic: {
			InputPer1k:  0.003,
			OutputPer1k: 0.015,
		},
		ProviderGoogle: {
			InputPer1k:  0.0001,
			OutputPer1k: 0.0004,
		},
		"kimi":    {InputPer1k: 0.0003, OutputPer1k: 0.0006},
		"qwen":    {InputPer1k: 0.001, OutputPer1k: 0.002},
		"xai":     {InputPer1k: 0.0025, OutputPer1k: 0.0025},
		"stepfun": {InputPer1k: 0.0, OutputPer1k: 0.0},
		"nvidia":  {InputPer1k: 0.0, OutputPer1k: 0.0},
		"githubmodels": {
			InputPer1k:  0.0,
			OutputPer1k: 0.0,
		},
		ProviderOllama: {InputPer1k: 0.0, OutputPer1k: 0.0},
		ProviderUITars: {InputPer1k: 0.0, OutputPer1k: 0.0},
	}
	for provider, rate := range preciseRates {
		rates[provider] = rate
	}

	return &CostTracker{
		records: make([]CostRecord, 0, 64),
		rates:   rates,
	}
}

// SetRate sets or overrides the cost rate for a provider.
func (ct *CostTracker) SetRate(
	provider string, rate CostRate,
) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.rates[provider] = rate
}

// Record adds a cost record for an LLM API call. The cost is
// calculated from the provider's registered rate and the token
// counts. If the provider has no registered rate, cost is zero.
func (ct *CostTracker) Record(
	provider, model, phase, callType string,
	inputTokens, outputTokens int,
	success bool,
) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	rate := ct.rates[provider]
	inputCost := float64(inputTokens) / 1000.0 * rate.InputPer1k
	outputCost := float64(outputTokens) / 1000.0 *
		rate.OutputPer1k

	ct.records = append(ct.records, CostRecord{
		Provider:     provider,
		Model:        model,
		Timestamp:    time.Now(),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    inputCost + outputCost,
		Phase:        phase,
		Success:      success,
		CallType:     callType,
	})
}

// TotalCost returns the total session cost in USD.
func (ct *CostTracker) TotalCost() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	var total float64
	for _, r := range ct.records {
		total += r.TotalCost
	}
	return total
}

// CostByProvider returns total cost per provider name.
func (ct *CostTracker) CostByProvider() map[string]float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make(map[string]float64)
	for _, r := range ct.records {
		result[r.Provider] += r.TotalCost
	}
	return result
}

// CostByModel returns total cost per model identifier.
func (ct *CostTracker) CostByModel() map[string]float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make(map[string]float64)
	for _, r := range ct.records {
		result[r.Model] += r.TotalCost
	}
	return result
}

// CostByPhase returns total cost per pipeline phase.
func (ct *CostTracker) CostByPhase() map[string]float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	result := make(map[string]float64)
	for _, r := range ct.records {
		result[r.Phase] += r.TotalCost
	}
	return result
}

// CallCount returns the total number of recorded API calls.
func (ct *CostTracker) CallCount() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.records)
}

// Summary returns a complete cost summary for reporting.
// The returned CostSummary includes all records.
func (ct *CostTracker) Summary() CostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.summaryLocked(true)
}

// SummaryCompact returns a cost summary without individual
// records. Useful for progress reporting where full records
// would be too verbose.
func (ct *CostTracker) SummaryCompact() CostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.summaryLocked(false)
}

// summaryLocked builds the summary while the read lock is
// already held.
func (ct *CostTracker) summaryLocked(
	includeRecords bool,
) CostSummary {
	summary := CostSummary{
		ByProvider: make(map[string]ProviderCost),
		ByPhase:    make(map[string]float64),
		ByCallType: make(map[string]float64),
	}

	for _, r := range ct.records {
		summary.TotalCostUSD += r.TotalCost
		summary.TotalCalls++
		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens

		// By provider.
		pc := summary.ByProvider[r.Provider]
		pc.Provider = r.Provider
		pc.Model = r.Model
		pc.Calls++
		pc.InputTokens += r.InputTokens
		pc.OutputTokens += r.OutputTokens
		pc.TotalCostUSD += r.TotalCost
		summary.ByProvider[r.Provider] = pc

		// By phase.
		summary.ByPhase[r.Phase] += r.TotalCost

		// By call type.
		summary.ByCallType[r.CallType] += r.TotalCost
	}

	if includeRecords {
		// Return a copy to avoid races if the caller holds
		// the summary after the lock is released.
		summary.Records = make(
			[]CostRecord, len(ct.records),
		)
		copy(summary.Records, ct.records)
	}

	return summary
}
