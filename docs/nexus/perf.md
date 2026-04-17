---
title: Helix Nexus — Performance
phase: 5
status: ready
---

# Helix Nexus — Performance

Two collectors share the same `Metrics` envelope:

- `CoreWebVitals` (chromedp performance traces)
- `K6Runner` (generates + parses k6 browser scripts)

Default thresholds match the Core Web Vitals "good" bar. Zero values
skip the check so callers can relax one metric without editing others.

## Thresholds

| Metric | Default cap |
|---|---|
| LCP  | 2500 ms |
| INP  | 200 ms  |
| CLS  | 0.1     |
| FCP  | 1800 ms |
| TTFB | 800 ms  |

## Workflow

1. `GenerateScript(Scenario)` emits a self-contained k6 browser script
   with the above thresholds baked in.
2. Run `k6 run --out json=results.json <script>`.
3. `ParseK6JSON(raw)` folds the point stream into a `Metrics{}` struct.
4. `metrics.Assert(thresholds)` returns a non-nil error on any breach.
