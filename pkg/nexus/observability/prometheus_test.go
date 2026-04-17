package observability

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrometheusBridge_ExposesEveryMetric(t *testing.T) {
	reg := NewRegistry()
	m := DefaultMetrics(reg)
	m.BrowserActiveSessions.Set(2)
	m.SessionOpensTotal.Add(5)
	m.ObserveSnapshotDuration(0)

	h := Handler(reg)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	resp.Body.Close()

	for _, want := range []string{
		"helix_nexus_browser_active_sessions",
		"helix_nexus_session_opens_total",
		"helix_nexus_snapshot_duration_ms",
		"helix_nexus_snapshot_duration_ms_bucket",
		"helix_nexus_snapshot_duration_ms_sum",
		"helix_nexus_snapshot_duration_ms_count",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Prometheus output missing %q\n-- got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "helix_nexus_browser_active_sessions 2") {
		t.Errorf("gauge value not exposed, output:\n%s", out)
	}
	if !strings.Contains(out, "helix_nexus_session_opens_total 5") {
		t.Errorf("counter value not exposed, output:\n%s", out)
	}
}

func TestPrometheusBridge_HistogramBuckets(t *testing.T) {
	reg := NewRegistry()
	h := reg.NewHistogram("helix_test_latency", "latency", []float64{10, 100})
	h.Observe(5)
	h.Observe(50)
	h.Observe(500)

	handler := Handler(reg)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, _ := srv.Client().Get(srv.URL)
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	resp.Body.Close()

	if !strings.Contains(out, `helix_test_latency_bucket{le="10"} 1`) {
		t.Errorf("bucket routing wrong\n%s", out)
	}
	if !strings.Contains(out, `helix_test_latency_bucket{le="100"} 2`) {
		t.Errorf("cumulative bucket wrong\n%s", out)
	}
	if !strings.Contains(out, `helix_test_latency_count 3`) {
		t.Errorf("count wrong\n%s", out)
	}
}
