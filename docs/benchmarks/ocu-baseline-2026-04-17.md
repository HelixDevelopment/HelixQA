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

## P1 Capture

### BenchmarkSource_FrameChannelThroughput

Measures the in-process plumbing cost: `Open` + `Start` + read-1-frame + `Stop` + `Close`
with a mock producer that emits frames immediately. Real subprocess cost (chromedp /
xwd / adb screenrecord) is excluded — that lands in P1.5.

| Source | ns/op | allocs/op | bytes/op |
|---|---|---|---|
| web (CDP) | 10230295 | 10 | 4758 |
| linux-x11 (xwd) | 10500195 | 10 | 4696 |
| android / androidtv (ADB) | 10349545 | 12 | 5095 |

Host: nezha / linux amd64 / kernel 6.12.61-6.12-alt1
CPU: 11th Gen Intel(R) Core(TM) i7-1165G7 @ 2.80GHz

Regression gate: +25% on any of ns/op, allocs/op, bytes/op blocks PR.

Dominant cost: ~10 ms per Open→Start→frame→Stop→Close cycle is expected at this
stage because `Start()` includes a 10 ms sleep to surface immediate errors. The
sleep is intentional (avoids a blocking time.Sleep in production paths) and will
be removed once real producers use a readiness signal in P1.5.
