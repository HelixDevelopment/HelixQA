// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatherMetricFamily collects all metric families from the registry and
// returns the one whose name matches familyName. Returns nil when not found.
func gatherMetricFamily(t *testing.T, reg *prometheus.Registry, familyName string) *dto.MetricFamily {
	t.Helper()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	for _, mf := range mfs {
		if mf.GetName() == familyName {
			return mf
		}
	}
	return nil
}

// counterValue returns the value of the first metric in mf whose labels match
// the provided label pairs, or -1 when no match is found.
func counterValue(mf *dto.MetricFamily, labels map[string]string) float64 {
	if mf == nil {
		return -1
	}
	for _, m := range mf.GetMetric() {
		if labelsMatch(m.GetLabel(), labels) {
			if c := m.GetCounter(); c != nil {
				return c.GetValue()
			}
		}
	}
	return -1
}

// gaugeValue returns the value of the first metric in mf whose labels match
// the provided label pairs, or -1 when no match is found.
func gaugeValue(mf *dto.MetricFamily, labels map[string]string) float64 {
	if mf == nil {
		return -1
	}
	for _, m := range mf.GetMetric() {
		if labelsMatch(m.GetLabel(), labels) {
			if g := m.GetGauge(); g != nil {
				return g.GetValue()
			}
		}
	}
	return -1
}

// labelsMatch reports whether the given label pairs contain all expected
// key/value entries.
func labelsMatch(pairs []*dto.LabelPair, expected map[string]string) bool {
	matched := 0
	for _, lp := range pairs {
		v, ok := expected[lp.GetName()]
		if ok && v == lp.GetValue() {
			matched++
		}
	}
	return matched == len(expected)
}

// TestMetrics_New verifies that NewMetrics registers all four collectors and
// that the custom registry exposes them.
func TestMetrics_New(t *testing.T) {
	m := NewMetrics("test_vision")

	require.NotNil(t, m)
	require.NotNil(t, m.RequestsTotal)
	require.NotNil(t, m.CacheHitsTotal)
	require.NotNil(t, m.RequestDuration)
	require.NotNil(t, m.CircuitBreakerState)
	require.NotNil(t, m.Registry())

	// Touch each collector so the registry emits metric families for them.
	// Counters and gauges appear after a single Add/Set call; histograms only
	// appear in gathered output after at least one Observe call.
	m.RequestsTotal.WithLabelValues("noop").Add(0)
	m.CacheHitsTotal.WithLabelValues("exact").Add(0)
	m.CircuitBreakerState.WithLabelValues("noop").Set(0)
	m.RequestDuration.WithLabelValues("noop").Observe(0)

	reg := m.Registry()
	mfs, err := reg.Gather()
	require.NoError(t, err)

	names := make(map[string]struct{})
	for _, mf := range mfs {
		names[mf.GetName()] = struct{}{}
	}

	assert.Contains(t, names, "test_vision_requests_total")
	assert.Contains(t, names, "test_vision_cache_hits_total")
	assert.Contains(t, names, "test_vision_request_duration_seconds")
	assert.Contains(t, names, "test_vision_circuit_breaker_state")
}

// TestMetrics_RecordRequest verifies that RecordRequest increments the
// requests_total counter for the given provider and also records an observation
// in the request_duration histogram.
func TestMetrics_RecordRequest(t *testing.T) {
	m := NewMetrics("rr_vision")

	m.RecordRequest("provider_a", 250*time.Millisecond)
	m.RecordRequest("provider_a", 100*time.Millisecond)
	m.RecordRequest("provider_b", 500*time.Millisecond)

	reg := m.Registry()

	// -- counter --
	mfCounter := gatherMetricFamily(t, reg, "rr_vision_requests_total")
	require.NotNil(t, mfCounter, "rr_vision_requests_total not found")

	assert.InDelta(t, 2.0, counterValue(mfCounter, map[string]string{"provider": "provider_a"}), 1e-9)
	assert.InDelta(t, 1.0, counterValue(mfCounter, map[string]string{"provider": "provider_b"}), 1e-9)

	// -- histogram --
	mfHist := gatherMetricFamily(t, reg, "rr_vision_request_duration_seconds")
	require.NotNil(t, mfHist, "rr_vision_request_duration_seconds not found")

	var histA *dto.Histogram
	for _, metric := range mfHist.GetMetric() {
		if labelsMatch(metric.GetLabel(), map[string]string{"provider": "provider_a"}) {
			histA = metric.GetHistogram()
		}
	}
	require.NotNil(t, histA, "histogram for provider_a not found")
	assert.EqualValues(t, 2, histA.GetSampleCount(), "expected 2 observations for provider_a")
}

// TestMetrics_RecordCacheHit verifies that RecordCacheHit increments the
// cache_hits_total counter for the correct layer label.
func TestMetrics_RecordCacheHit(t *testing.T) {
	m := NewMetrics("ch_vision")

	m.RecordCacheHit("exact")
	m.RecordCacheHit("exact")
	m.RecordCacheHit("exact")
	m.RecordCacheHit("differential")
	m.RecordCacheHit("vector")
	m.RecordCacheHit("vector")

	mf := gatherMetricFamily(t, m.Registry(), "ch_vision_cache_hits_total")
	require.NotNil(t, mf, "ch_vision_cache_hits_total not found")

	assert.InDelta(t, 3.0, counterValue(mf, map[string]string{"layer": "exact"}), 1e-9)
	assert.InDelta(t, 1.0, counterValue(mf, map[string]string{"layer": "differential"}), 1e-9)
	assert.InDelta(t, 2.0, counterValue(mf, map[string]string{"layer": "vector"}), 1e-9)
}

// TestMetrics_SetCircuitBreakerState verifies that SetCircuitBreakerState
// updates the gauge for the given provider to the expected state value.
func TestMetrics_SetCircuitBreakerState(t *testing.T) {
	m := NewMetrics("cb_vision")

	const (
		stateClosed   = 0.0
		stateHalfOpen = 1.0
		stateOpen     = 2.0
	)

	m.SetCircuitBreakerState("alpha", stateClosed)
	m.SetCircuitBreakerState("beta", stateOpen)

	mf := gatherMetricFamily(t, m.Registry(), "cb_vision_circuit_breaker_state")
	require.NotNil(t, mf, "cb_vision_circuit_breaker_state not found")

	assert.InDelta(t, stateClosed, gaugeValue(mf, map[string]string{"provider": "alpha"}), 1e-9)
	assert.InDelta(t, stateOpen, gaugeValue(mf, map[string]string{"provider": "beta"}), 1e-9)

	// Transition alpha to half-open.
	m.SetCircuitBreakerState("alpha", stateHalfOpen)
	mf = gatherMetricFamily(t, m.Registry(), "cb_vision_circuit_breaker_state")
	require.NotNil(t, mf)
	assert.InDelta(t, stateHalfOpen, gaugeValue(mf, map[string]string{"provider": "alpha"}), 1e-9)
}

// TestMetrics_IsolatedRegistries verifies that two Metrics instances with
// different namespaces do not share state via a global prometheus registry.
func TestMetrics_IsolatedRegistries(t *testing.T) {
	m1 := NewMetrics("iso1_vision")
	m2 := NewMetrics("iso2_vision")

	m1.RecordCacheHit("exact")
	m1.RecordCacheHit("exact")

	// m2 must be unaffected by m1's observations.
	mf := gatherMetricFamily(t, m2.Registry(), "iso2_vision_cache_hits_total")
	// The family may not exist yet if no observations were made; that is fine.
	if mf != nil {
		assert.InDelta(t, 0.0, counterValue(mf, map[string]string{"layer": "exact"}), 1e-9)
	}

	// m1 must have exactly 2 hits.
	mf1 := gatherMetricFamily(t, m1.Registry(), "iso1_vision_cache_hits_total")
	require.NotNil(t, mf1)
	assert.InDelta(t, 2.0, counterValue(mf1, map[string]string{"layer": "exact"}), 1e-9)
}
