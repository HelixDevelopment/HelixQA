# OCU Baseline Benchmarks — 2026-04-17

## P0 Foundation

### BenchmarkProbeLocal

| Metric | Value |
|---|---|
| ns/op | 133570 |
| allocs/op | 366 |
| bytes/op | 46272 |

Host: nezha / linux amd64 / kernel 6.12.61-6.12-alt1
CPU: 11th Gen Intel(R) Core(TM) i7-1165G7 @ 2.80GHz

Regression gate: +25% on any of ns/op, allocs/op, bytes/op blocks PR.

P1–P7 append their own baseline rows to this file as their benches land.
