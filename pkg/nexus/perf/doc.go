// Package perf drives browser-level performance validation. Two
// collectors ship:
//
//   CoreWebVitals — parses chromedp performance traces into LCP / INP /
//                   CLS / FCP / TTFB metrics.
//   K6Runner      — generates and runs k6 browser scripts from
//                   pkg/nexus test scenarios, producing a normalised
//                   Metrics struct.
//
// Both collectors return the same Metrics type so callers can compare
// them against baselines recorded under tests/benchmark/baselines/.
package perf
