package observability

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRegistry_CounterIncOnly(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("x", "")
	c.Inc()
	c.Add(4)
	c.Add(-3) // rejected
	if c.Value() != 5 {
		t.Errorf("counter = %d, want 5", c.Value())
	}
}

func TestRegistry_GaugeSetAddValue(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("g", "")
	g.Set(10)
	g.Add(-4)
	g.Add(2)
	if g.Value() != 8 {
		t.Errorf("gauge = %d, want 8", g.Value())
	}
}

func TestRegistry_HistogramObserveBucketRouting(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("h", "", []float64{10, 100})
	h.Observe(5)
	h.Observe(50)
	h.Observe(200)
	if h.Count() != 3 {
		t.Errorf("count = %d", h.Count())
	}
	counts := h.Counts()
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 1 {
		t.Errorf("bucket routing: %+v", counts)
	}
	if got := h.Sum(); got != 255 {
		t.Errorf("sum = %g", got)
	}
}

func TestRegistry_ReuseExistingInstrument(t *testing.T) {
	r := NewRegistry()
	c1 := r.NewCounter("c", "")
	c2 := r.NewCounter("c", "")
	if c1 != c2 {
		t.Error("same name must return the same counter")
	}
}

func TestDefaultMetrics_AllRegistered(t *testing.T) {
	r := NewRegistry()
	m := DefaultMetrics(r)
	m.BrowserActiveSessions.Set(3)
	m.SessionOpensTotal.Inc()
	m.SnapshotDurationMs.Observe(123)
	m.ObserveFlowDuration(2 * time.Second)
	m.AiCostCents.Add(200)

	if m.BrowserActiveSessions.Value() != 3 {
		t.Error("gauge not wired")
	}
	if m.SessionOpensTotal.Value() != 1 {
		t.Error("counter not wired")
	}
	if m.SnapshotDurationMs.Count() != 1 {
		t.Error("histogram not wired")
	}
	if len(r.Counters()) < 4 || len(r.Gauges()) < 4 || len(r.Histograms()) < 3 {
		t.Errorf("registry population: c=%d g=%d h=%d", len(r.Counters()), len(r.Gauges()), len(r.Histograms()))
	}
}

func TestRegistry_ExposeTextFormat(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("helix_test_counter", "test counter")
	c.Add(7)
	g := r.NewGauge("helix_test_gauge", "test gauge")
	g.Set(42)
	h := r.NewHistogram("helix_test_hist", "test hist", []float64{1, 10})
	h.Observe(0.5)
	h.Observe(5)
	h.Observe(50)

	out := string(r.Expose())
	for _, want := range []string{
		"# HELP helix_test_counter test counter",
		"# TYPE helix_test_counter counter",
		"helix_test_counter 7",
		"# TYPE helix_test_gauge gauge",
		"helix_test_gauge 42",
		"helix_test_hist_bucket",
		"helix_test_hist_sum",
		"helix_test_hist_count 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposition missing %q\n-- got:\n%s", want, out)
		}
	}
}

func TestRegistry_ConcurrentObserve(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("h", "", []float64{10})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Observe(5)
		}()
	}
	wg.Wait()
	if h.Count() != 100 {
		t.Errorf("expected 100 observations, got %d", h.Count())
	}
}
