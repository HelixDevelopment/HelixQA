# OpenClaw Ultimate — Program Roadmap

Living status doc for the 8 OCU sub-projects. Program spec at
`docs/superpowers/specs/2026-04-17-openclaw-ultimate-program-design.md`.

## Status table

| Sub-project | Status | Spec | Plan | Notes |
|---|---|---|---|---|
| P0 Foundation | **CLOSED 2026-04-17** | [spec](../superpowers/specs/2026-04-17-openclaw-ultimate-program-design.md) | [plan](../superpowers/plans/2026-04-17-ocu-p0-foundation-plan.md) | Contracts + Containers GPU extension + vertical-slice CLIs shipped. All ten P0-applicable test categories green. |
| P1 Capture | pending | — | — | Waits on P0 contracts (✅ available). Can start. |
| P2 Vision | pending | — | — | Waits on P0 contracts (✅). Can start in parallel with P1. |
| P3 Interact | pending | — | — | Waits on P0 contracts (✅). Can start in parallel with P1/P2. |
| P4 Observe | pending | — | — | Waits on P0 contracts (✅). Can start in parallel with P1/P2/P3. |
| P5 Record | pending | — | — | Waits on Wave 2 (P1–P4). |
| P6 Automation | pending | — | — | Waits on P5. |
| P7 Tickets+tests | pending | — | — | Waits on P6. |

## Contract versions

| Contract | Version | Locked by |
|---|---|---|
| capture.go | v1 | P0 |
| vision.go | v1 | P0 |
| interact.go | v1 | P0 |
| observe.go | v1 | P0 |
| record.go | v1 | P0 |
| remote.go | v1 | P0 |

## Budgets vs actuals

| Budget | Limit | P0 actual | Status |
|---|---|---|---|
| ProbeLocal | n/a (not budgeted) | ~133 µs / 366 allocs / 46 KB (laptop i7-1165G7) | informational |

P1–P7 append their own actuals as benches land.

## Risk register

Live copy of program-spec §5.5. Update in-place whenever likelihood or impact changes.

## Maintenance

Per Constitution Article VI: every commit that changes sub-project state must update this table in the SAME commit.
