// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"sync"
	"time"
)

// ProviderMetrics holds performance statistics for a single vision provider.
type ProviderMetrics struct {
	ProviderName       string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatency       time.Duration
	AvgLatency         time.Duration
	LastUsed           time.Time
	ButtonAccuracy     float64
	TextFieldAccuracy  float64
	ImageAccuracy      float64
	GeneralAccuracy    float64
}

// ProviderOptimizer tracks per-provider metrics and selects the best provider
// for a given UI element type using exponential moving averages.
type ProviderOptimizer struct {
	metrics    map[string]*ProviderMetrics
	mu         sync.RWMutex
	windowSize time.Duration
}

// NewProviderOptimizer creates a ProviderOptimizer with a 1-hour staleness window.
func NewProviderOptimizer() *ProviderOptimizer {
	return &ProviderOptimizer{
		metrics:    make(map[string]*ProviderMetrics),
		windowSize: time.Hour,
	}
}

// getOrCreate retrieves an existing metrics entry or initialises a new one.
// Must be called with po.mu write-locked.
func (po *ProviderOptimizer) getOrCreate(provider string) *ProviderMetrics {
	m, ok := po.metrics[provider]
	if !ok {
		m = &ProviderMetrics{
			ProviderName:      provider,
			ButtonAccuracy:    0.5,
			TextFieldAccuracy: 0.5,
			ImageAccuracy:     0.5,
			GeneralAccuracy:   0.5,
		}
		po.metrics[provider] = m
	}
	return m
}

// updateAccuracy applies one step of an exponential moving average.
// signal is 1.0 for success, 0.0 for failure.
func updateAccuracy(current, signal float64) float64 {
	return 0.9*current + 0.1*signal
}

// applyAccuracy updates the uiType-specific accuracy field with the given signal.
// Must be called with po.mu write-locked.
func applyAccuracy(m *ProviderMetrics, uiType string, signal float64) {
	switch uiType {
	case "button":
		m.ButtonAccuracy = updateAccuracy(m.ButtonAccuracy, signal)
	case "text":
		m.TextFieldAccuracy = updateAccuracy(m.TextFieldAccuracy, signal)
	case "image":
		m.ImageAccuracy = updateAccuracy(m.ImageAccuracy, signal)
	default:
		m.GeneralAccuracy = updateAccuracy(m.GeneralAccuracy, signal)
	}
}

// accuracyFor returns the accuracy field that corresponds to uiType.
func accuracyFor(m *ProviderMetrics, uiType string) float64 {
	switch uiType {
	case "button":
		return m.ButtonAccuracy
	case "text":
		return m.TextFieldAccuracy
	case "image":
		return m.ImageAccuracy
	default:
		return m.GeneralAccuracy
	}
}

// RecordSuccess records a successful request for provider, updating latency
// statistics and the per-uiType accuracy via an EMA step toward 1.0.
func (po *ProviderOptimizer) RecordSuccess(provider string, latency time.Duration, uiType string) {
	po.mu.Lock()
	defer po.mu.Unlock()

	m := po.getOrCreate(provider)
	m.TotalRequests++
	m.SuccessfulRequests++
	m.TotalLatency += latency
	m.AvgLatency = m.TotalLatency / time.Duration(m.TotalRequests)
	m.LastUsed = time.Now()
	applyAccuracy(m, uiType, 1.0)
}

// RecordFailure records a failed request for provider, updating the per-uiType
// accuracy via an EMA step toward 0.0.
func (po *ProviderOptimizer) RecordFailure(provider string, uiType string) {
	po.mu.Lock()
	defer po.mu.Unlock()

	m := po.getOrCreate(provider)
	m.TotalRequests++
	m.FailedRequests++
	m.LastUsed = time.Now()
	applyAccuracy(m, uiType, 0.0)
}

// GetBestProvider returns the provider name with the highest score for the
// given uiType. Providers whose LastUsed is more than 10 minutes ago are
// skipped. Score = successRate * uiTypeAccuracy. When prioritizeSpeed is true,
// latency is factored in so that faster providers score higher.
// Returns an empty string when no eligible provider exists.
func (po *ProviderOptimizer) GetBestProvider(uiType string, prioritizeSpeed bool) string {
	po.mu.RLock()
	defer po.mu.RUnlock()

	staleCutoff := time.Now().Add(-10 * time.Minute)

	var bestName string
	var bestScore float64 = -1

	for name, m := range po.metrics {
		if m.LastUsed.Before(staleCutoff) {
			continue
		}
		if m.TotalRequests == 0 {
			continue
		}

		successRate := float64(m.SuccessfulRequests) / float64(m.TotalRequests)
		uiAcc := accuracyFor(m, uiType)
		score := successRate * uiAcc

		if prioritizeSpeed && m.AvgLatency > 0 {
			// Normalise: subtract a small fraction proportional to latency.
			// 1 second of avg latency reduces score by 0.1.
			latencyPenalty := m.AvgLatency.Seconds() * 0.1
			score -= latencyPenalty
		}

		if score > bestScore {
			bestScore = score
			bestName = name
		}
	}

	return bestName
}

// GetMetrics returns a copy of the metrics for provider, or nil if the
// provider has not been seen.
func (po *ProviderOptimizer) GetMetrics(provider string) *ProviderMetrics {
	po.mu.RLock()
	defer po.mu.RUnlock()

	m, ok := po.metrics[provider]
	if !ok {
		return nil
	}
	copy := *m
	return &copy
}

// GetAllMetrics returns a map of copies of all tracked provider metrics.
func (po *ProviderOptimizer) GetAllMetrics() map[string]*ProviderMetrics {
	po.mu.RLock()
	defer po.mu.RUnlock()

	result := make(map[string]*ProviderMetrics, len(po.metrics))
	for name, m := range po.metrics {
		copy := *m
		result[name] = &copy
	}
	return result
}
