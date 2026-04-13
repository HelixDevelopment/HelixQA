// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds pre-registered Prometheus collectors for the cheaper vision
// subsystem. Each Metrics instance uses a private prometheus.Registry so that
// multiple instances (e.g. in tests) never conflict with each other or with
// the global default registry.
type Metrics struct {
	// RequestsTotal counts vision analysis calls, labelled by provider name.
	RequestsTotal *prometheus.CounterVec

	// CacheHitsTotal counts cache hits by cache layer. Valid label values for
	// "layer" are "exact", "differential", and "vector".
	CacheHitsTotal *prometheus.CounterVec

	// RequestDuration observes the wall-clock duration of each vision analysis
	// call, labelled by provider name.
	RequestDuration *prometheus.HistogramVec

	// CircuitBreakerState tracks the current state of each provider's circuit
	// breaker as a gauge. Conventional state values: 0 = closed, 1 = half-open,
	// 2 = open.
	CircuitBreakerState *prometheus.GaugeVec

	registry *prometheus.Registry
}

// requestDurationBuckets are the histogram bucket boundaries (in seconds) used
// for RequestDuration.
var requestDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// NewMetrics creates and registers all Prometheus collectors under the given
// namespace (e.g. "cheaper_vision"). A fresh, isolated prometheus.Registry is
// created for each call so that instances never share state.
func NewMetrics(namespace string) *Metrics {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of vision analysis requests dispatched to a provider.",
		},
		[]string{"provider"},
	)

	cacheHitsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits by cache layer (exact, differential, vector).",
		},
		[]string{"layer"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_seconds",
			Help:      "Wall-clock duration of vision analysis calls in seconds.",
			Buckets:   requestDurationBuckets,
		},
		[]string{"provider"},
	)

	circuitBreakerState := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "circuit_breaker_state",
			Help:      "Current circuit breaker state per provider: 0=closed, 1=half-open, 2=open.",
		},
		[]string{"provider"},
	)

	reg.MustRegister(
		requestsTotal,
		cacheHitsTotal,
		requestDuration,
		circuitBreakerState,
	)

	return &Metrics{
		RequestsTotal:       requestsTotal,
		CacheHitsTotal:      cacheHitsTotal,
		RequestDuration:     requestDuration,
		CircuitBreakerState: circuitBreakerState,
		registry:            reg,
	}
}

// Registry returns the private prometheus.Registry that holds all collectors
// for this Metrics instance. Callers may use it to gather metric families for
// testing or to expose a dedicated /metrics endpoint scoped to this subsystem.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// RecordRequest increments the requests_total counter for the given provider
// and records the call duration in the request_duration histogram.
func (m *Metrics) RecordRequest(provider string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(provider).Inc()
	m.RequestDuration.WithLabelValues(provider).Observe(duration.Seconds())
}

// RecordCacheHit increments the cache_hits_total counter for the given cache
// layer. Expected layer values are "exact", "differential", and "vector".
func (m *Metrics) RecordCacheHit(layer string) {
	m.CacheHitsTotal.WithLabelValues(layer).Inc()
}

// SetCircuitBreakerState sets the circuit_breaker_state gauge for the given
// provider to state. Conventional state values are 0 (closed), 1 (half-open),
// and 2 (open).
func (m *Metrics) SetCircuitBreakerState(provider string, state float64) {
	m.CircuitBreakerState.WithLabelValues(provider).Set(state)
}
