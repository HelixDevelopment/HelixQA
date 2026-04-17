package observability

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Registry is the narrow Prometheus-shaped metrics surface Nexus uses.
// It is intentionally not tied to the official client_golang so tests
// stay self-contained. A small adapter under cmd/ can bridge to
// prometheus/client_golang when operators wire it up.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry returns a fresh registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]*Counter{},
		gauges:     map[string]*Gauge{},
		histograms: map[string]*Histogram{},
	}
}

// Counter is a monotonically-increasing metric.
type Counter struct {
	name  string
	help  string
	value atomic.Int64
}

// Inc adds 1.
func (c *Counter) Inc() { c.value.Add(1) }

// Add adds n (rejects negative).
func (c *Counter) Add(n int64) {
	if n < 0 {
		return
	}
	c.value.Add(n)
}

// Value reports the current count.
func (c *Counter) Value() int64 { return c.value.Load() }

// Name returns the metric name.
func (c *Counter) Name() string { return c.name }

// Help returns the help string.
func (c *Counter) Help() string { return c.help }

// Gauge is a metric that may go up and down.
type Gauge struct {
	name  string
	help  string
	value atomic.Int64 // stored as microseconds or raw ints by convention
}

// Set stores n.
func (g *Gauge) Set(n int64) { g.value.Store(n) }

// Add adds n (may be negative).
func (g *Gauge) Add(n int64) { g.value.Add(n) }

// Value reports the current value.
func (g *Gauge) Value() int64 { return g.value.Load() }

// Name returns the metric name.
func (g *Gauge) Name() string { return g.name }

// Help returns the help string.
func (g *Gauge) Help() string { return g.help }

// Histogram is a simple linear-bucket histogram with min/max/count.
type Histogram struct {
	name    string
	help    string
	buckets []float64
	mu      sync.Mutex
	counts  []int64
	total   int64
	sum     float64
	minV    float64
	maxV    float64
}

// Observe records a single sample.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total++
	h.sum += v
	if h.total == 1 {
		h.minV = v
		h.maxV = v
	} else {
		if v < h.minV {
			h.minV = v
		}
		if v > h.maxV {
			h.maxV = v
		}
	}
	// Find the first bucket whose upper bound >= v. +Inf catches overflow.
	idx := sort.SearchFloat64s(h.buckets, v)
	if idx >= len(h.counts) {
		idx = len(h.counts) - 1
	}
	h.counts[idx]++
}

// Count reports total observations.
func (h *Histogram) Count() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.total
}

// Sum reports the running sum of observations.
func (h *Histogram) Sum() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sum
}

// Buckets returns the configured bucket upper bounds.
func (h *Histogram) Buckets() []float64 { return append([]float64(nil), h.buckets...) }

// Counts returns a snapshot of per-bucket counts.
func (h *Histogram) Counts() []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]int64, len(h.counts))
	copy(out, h.counts)
	return out
}

// Name returns the metric name.
func (h *Histogram) Name() string { return h.name }

// Help returns the help string.
func (h *Histogram) Help() string { return h.help }

// NewCounter registers and returns a Counter.
func (r *Registry) NewCounter(name, help string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{name: name, help: help}
	r.counters[name] = c
	return c
}

// NewGauge registers and returns a Gauge.
func (r *Registry) NewGauge(name, help string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name, help: help}
	r.gauges[name] = g
	return g
}

// NewHistogram registers and returns a Histogram with the given bucket
// upper bounds. A final "+Inf" bucket is appended automatically.
func (r *Registry) NewHistogram(name, help string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	b := append([]float64(nil), buckets...)
	sort.Float64s(b)
	h := &Histogram{name: name, help: help, buckets: b, counts: make([]int64, len(b)+1)}
	r.histograms[name] = h
	return h
}

// Counters returns a snapshot slice of all counters.
func (r *Registry) Counters() []*Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Counter, 0, len(r.counters))
	for _, c := range r.counters {
		out = append(out, c)
	}
	return out
}

// Gauges returns a snapshot slice of all gauges.
func (r *Registry) Gauges() []*Gauge {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Gauge, 0, len(r.gauges))
	for _, g := range r.gauges {
		out = append(out, g)
	}
	return out
}

// Histograms returns a snapshot slice of all histograms.
func (r *Registry) Histograms() []*Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Histogram, 0, len(r.histograms))
	for _, h := range r.histograms {
		out = append(out, h)
	}
	return out
}

// NexusMetrics is the canonical set of metrics the Grafana dashboard
// references. Constructing one registers every metric; Nexus code
// calls the typed methods to emit samples.
type NexusMetrics struct {
	BrowserActiveSessions *Gauge
	MobileActiveSessions  *Gauge
	DesktopActiveSessions *Gauge

	SessionOpensTotal  *Counter
	SessionClosesTotal *Counter

	SnapshotDurationMs *Histogram
	FlowDurationSecs   *Histogram
	CwvLCPMs           *Histogram

	A11yViolationsTotal *Counter
	RbacDenialsTotal    *Counter

	AiCostCents     *Counter
	EvidenceBytes   *Gauge
}

// DefaultMetrics returns the canonical Nexus metric set registered
// against r.
func DefaultMetrics(r *Registry) *NexusMetrics {
	return &NexusMetrics{
		BrowserActiveSessions: r.NewGauge("helix_nexus_browser_active_sessions", "Active browser sessions."),
		MobileActiveSessions:  r.NewGauge("helix_nexus_mobile_active_sessions", "Active mobile sessions."),
		DesktopActiveSessions: r.NewGauge("helix_nexus_desktop_active_sessions", "Active desktop sessions."),

		SessionOpensTotal:  r.NewCounter("helix_nexus_session_opens_total", "Sessions opened across all platforms."),
		SessionClosesTotal: r.NewCounter("helix_nexus_session_closes_total", "Sessions closed across all platforms."),

		SnapshotDurationMs: r.NewHistogram("helix_nexus_snapshot_duration_ms", "Snapshot latency in milliseconds.", []float64{50, 100, 250, 500, 1000, 2500, 5000}),
		FlowDurationSecs:   r.NewHistogram("helix_nexus_flow_duration_seconds", "Cross-platform flow duration in seconds.", []float64{1, 5, 15, 30, 60, 120, 300, 600}),
		CwvLCPMs:           r.NewHistogram("helix_nexus_cwv_lcp_ms", "LCP latency in milliseconds.", []float64{500, 1000, 1500, 2000, 2500, 4000}),

		A11yViolationsTotal: r.NewCounter("helix_nexus_a11y_violations_total", "Accessibility violations seen across sessions."),
		RbacDenialsTotal:    r.NewCounter("helix_nexus_rbac_denials_total", "RBAC denials recorded."),

		AiCostCents:   r.NewCounter("helix_nexus_ai_cost_cents", "Cumulative LLM spend in cents."),
		EvidenceBytes: r.NewGauge("helix_nexus_evidence_bytes", "Bytes currently stored in the evidence vault."),
	}
}

// ObserveSnapshotDuration is a convenience for the browser Engine.
func (m *NexusMetrics) ObserveSnapshotDuration(d time.Duration) {
	m.SnapshotDurationMs.Observe(float64(d.Milliseconds()))
}

// ObserveFlowDuration is a convenience for the orchestrator.
func (m *NexusMetrics) ObserveFlowDuration(d time.Duration) {
	m.FlowDurationSecs.Observe(d.Seconds())
}

// Expose formats every metric as a Prometheus text-exposition response.
// Minimal formatter — no labels, suitable for the narrow Nexus surface.
func (r *Registry) Expose() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b []byte
	appendLine := func(s string) {
		b = append(b, s...)
		b = append(b, '\n')
	}
	appendLinef := func(format string, args ...any) {
		// Keep formatting cheap and deterministic — this is the text
		// exposition so Grafana/Prometheus can scrape it.
		appendLine(sprintfLite(format, args...))
	}

	for _, c := range r.Counters() {
		appendLinef("# HELP %s %s", c.Name(), c.Help())
		appendLinef("# TYPE %s counter", c.Name())
		appendLinef("%s %d", c.Name(), c.Value())
	}
	for _, g := range r.Gauges() {
		appendLinef("# HELP %s %s", g.Name(), g.Help())
		appendLinef("# TYPE %s gauge", g.Name())
		appendLinef("%s %d", g.Name(), g.Value())
	}
	for _, h := range r.Histograms() {
		appendLinef("# HELP %s %s", h.Name(), h.Help())
		appendLinef("# TYPE %s histogram", h.Name())
		cumulative := int64(0)
		counts := h.Counts()
		bounds := h.Buckets()
		for i, le := range bounds {
			cumulative += counts[i]
			appendLinef("%s_bucket{le=\"%g\"} %d", h.Name(), le, cumulative)
		}
		cumulative += counts[len(counts)-1]
		appendLinef("%s_bucket{le=\"+Inf\"} %d", h.Name(), cumulative)
		appendLinef("%s_sum %g", h.Name(), h.Sum())
		appendLinef("%s_count %d", h.Name(), h.Count())
	}
	_ = appendLine
	return b
}
