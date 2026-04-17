---
module: 06
title: "Core Web Vitals in tests"
length: 10 minutes
---

# 06 — Core Web Vitals in tests

## Shot list

1. **00:00 — Cold open.** LCP regression caught in CI.
2. **00:45 — Metrics envelope.** Explain the `Metrics` struct; why
   zero thresholds skip.
3. **02:30 — Generate a k6 script.** Live-code `GenerateScript`; show
   the baked-in thresholds.
4. **04:30 — Run k6.** `k6 run --out json=results.json`; parse with
   `ParseK6JSON`; `metrics.Assert(DefaultThresholds())`.
5. **07:00 — Baseline management.** Commit baselines under
   `tests/benchmark/baselines/`; compare deltas.
6. **09:00 — Outro.** Link to Grafana panel.

## Exercise

Add a k6 browser flow that logs in + browses 3 pages; store the
resulting Metrics as the first baseline; simulate a 30% LCP degrade
and watch Assert fail.
