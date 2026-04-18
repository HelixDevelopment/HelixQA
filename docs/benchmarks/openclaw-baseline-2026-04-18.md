# OpenClawing2 Benchmark Baseline — 2026-04-18

Captured on: Intel Core i7-1165G7 @ 2.80GHz, linux/amd64, `GOMAXPROCS=3`.
Command: `go test -run '^$' -bench . -benchtime 100x ./pkg/nexus/agent/... ./pkg/nexus/primitives/... ./pkg/nexus/coordinate/...`

These numbers are the reference point for future regression
detection. Future OpenClawing2 campaign runs should report
against this table + flag any bench whose `ns/op` grows > 25%
without an accompanying justification in the release notes.

## Phase 2 — Agent state machine

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkAgent_Step` | 1 682 | — | — |
| `BenchmarkParsePlannerJSON` | 2 464 | 648 | 14 |
| `BenchmarkAgentState_RecentSteps` | 18 | — | — |

Agent step orchestration sits at ≈2 μs per iteration in the
pure-fixture path (fake planner + fake adapter) — a real LLM call
typically dominates the end-to-end wall-clock.

## Phase 3 — MessageManager

| Benchmark | ns/op |
|---|---:|
| `BenchmarkMessageManager_Compact` | 20 737 |

~21 μs per Compact on a 20-step history feels right — the
dominant cost is the token-count + default-summariser walk.

## Phase 4 — Retry / LoopDetector

| Benchmark | ns/op |
|---|---:|
| `BenchmarkLoopDetector_Record` | 108 |
| `BenchmarkLoopDetector_IsLoop` | 20 |
| `BenchmarkRetryWithBackoff_HappyPath` | 20 |

Retry happy path + loop-detector checks are under 25 ns each —
adding these to every Agent.Step is effectively free.

## Phase 5 — Stagehand primitives

| Benchmark | ns/op |
|---|---:|
| `BenchmarkPromptCache_PutGet` | 238 |
| `BenchmarkEngine_Act_Cached` | 1 352 |

Cached Act lands under 1.4 μs including the snapshot + cache
lookup — cache hits really are free in steady-state.

## Phase 6 — Coordinate scaling

| Benchmark | ns/op |
|---|---:|
| `BenchmarkScaleCoordinates_NormalizedPath` | 16 |
| `BenchmarkScaleCoordinates_AspectMatch` | 20 |

Normalised coordinate scaling under 20 ns — no reason not to
leave it on for every coord_* dispatch.

## Regression thresholds

Future campaigns flag a regression when a benchmark on this
hardware class crosses the following ceiling:

| Benchmark | Ceiling (ns/op) |
|---|---:|
| `BenchmarkAgent_Step` | 2 100 |
| `BenchmarkParsePlannerJSON` | 3 100 |
| `BenchmarkAgentState_RecentSteps` | 25 |
| `BenchmarkMessageManager_Compact` | 26 000 |
| `BenchmarkLoopDetector_Record` | 140 |
| `BenchmarkLoopDetector_IsLoop` | 25 |
| `BenchmarkRetryWithBackoff_HappyPath` | 25 |
| `BenchmarkPromptCache_PutGet` | 300 |
| `BenchmarkEngine_Act_Cached` | 1 700 |
| `BenchmarkScaleCoordinates_NormalizedPath` | 21 |
| `BenchmarkScaleCoordinates_AspectMatch` | 26 |

The ceiling is +25% over the 2026-04-18 figure, rounded. Operators
running the campaign on different hardware classes should re-
capture their own baseline + commit it alongside the release
evidence.
