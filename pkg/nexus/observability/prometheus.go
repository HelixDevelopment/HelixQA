package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusBridge adapts our narrow Registry into the
// prometheus/client_golang collector interface so operators can scrape
// Nexus metrics without writing their own exporter. Every Counter,
// Gauge, and Histogram in the bridge's source Registry becomes a
// matching prometheus.Metric.
type PrometheusBridge struct {
	source     *Registry
	counters   map[string]*prometheus.Desc
	gauges     map[string]*prometheus.Desc
	histograms map[string]*prometheus.Desc
}

// NewPrometheusBridge builds a bridge rooted at src.
func NewPrometheusBridge(src *Registry) *PrometheusBridge {
	b := &PrometheusBridge{
		source:     src,
		counters:   map[string]*prometheus.Desc{},
		gauges:     map[string]*prometheus.Desc{},
		histograms: map[string]*prometheus.Desc{},
	}
	for _, c := range src.Counters() {
		b.counters[c.Name()] = prometheus.NewDesc(c.Name(), c.Help(), nil, nil)
	}
	for _, g := range src.Gauges() {
		b.gauges[g.Name()] = prometheus.NewDesc(g.Name(), g.Help(), nil, nil)
	}
	for _, h := range src.Histograms() {
		b.histograms[h.Name()] = prometheus.NewDesc(h.Name(), h.Help(), nil, nil)
	}
	return b
}

// Describe satisfies prometheus.Collector.
func (b *PrometheusBridge) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range b.counters {
		ch <- d
	}
	for _, d := range b.gauges {
		ch <- d
	}
	for _, d := range b.histograms {
		ch <- d
	}
}

// Collect walks the source Registry and emits one prometheus.Metric
// per counter / gauge / histogram. Histograms are expanded into
// bucket-count + sum + count samples.
func (b *PrometheusBridge) Collect(ch chan<- prometheus.Metric) {
	for _, c := range b.source.Counters() {
		desc, ok := b.counters[c.Name()]
		if !ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, float64(c.Value()))
	}
	for _, g := range b.source.Gauges() {
		desc, ok := b.gauges[g.Name()]
		if !ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(g.Value()))
	}
	for _, h := range b.source.Histograms() {
		desc, ok := b.histograms[h.Name()]
		if !ok {
			continue
		}
		buckets := map[float64]uint64{}
		cumulative := uint64(0)
		counts := h.Counts()
		for i, le := range h.Buckets() {
			cumulative += uint64(counts[i])
			buckets[le] = cumulative
		}
		// +Inf bucket accumulates the last count.
		cumulative += uint64(counts[len(counts)-1])
		ch <- prometheus.MustNewConstHistogram(desc, cumulative, h.Sum(), buckets)
	}
}

// Handler returns a Prometheus scrape handler serving the Nexus metric
// set plus any standard collectors operators want to include. P5 fix
// (docs/nexus/remaining-work.md): the default bundle now includes
// collectors.NewGoCollector() + collectors.NewProcessCollector() so
// scrapes of the Nexus bridge expose Go-runtime gauges (goroutines,
// gc pause, heap alloc) and process gauges (rss, fds, cpu) alongside
// the custom Nexus metrics. Operators can still pass additional
// collectors via `extra`.
func Handler(r *Registry, extra ...prometheus.Collector) http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewPrometheusBridge(r))
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	for _, c := range extra {
		reg.MustRegister(c)
	}
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
