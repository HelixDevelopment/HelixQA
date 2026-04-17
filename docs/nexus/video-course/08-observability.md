---
module: 08
title: "Observability you can ship"
length: 12 minutes
---

# 08 — Observability you can ship

## Shot list

1. **00:00 — Cold open.** Grafana dashboard showing live Nexus
   metrics.
2. **01:00 — InMemoryTracer.** Show `Instrument(ctx, name, fn)` in
   action; inspect `Finished()` in tests.
3. **03:00 — Swap to OTel.** `SetDefault(otelTracer)`; nothing else
   changes.
4. **05:00 — Import Grafana dashboard.** Upload
   `monitoring/grafana/helix-nexus-dashboard.json`; filter by platform.
5. **07:30 — Panel tour.** Sessions, snapshot p95, a11y violations,
   Core Web Vitals, evidence vault growth, RBAC denials.
6. **10:30 — RBAC denials panel.** Trigger a denial in the live demo;
   watch the panel.
7. **11:30 — Outro + closing.** The whole Nexus stack is green.

## Exercise

Add a new span around `Engine.Snapshot` that records the resulting
element count as an attribute. Bonus: extend the dashboard with a
panel charting element-count trends over time.
