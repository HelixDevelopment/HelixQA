# HelixQA — Test-Type Coverage Ledger (CONST-050(B))

**Status banner**

| | |
|---|---|
| Round | 219 (helix_qa deep-doc + test-matrix enrichment) |
| Date | 2026-05-19 |
| Authority | constitution submodule §11.4.27 / CONST-050(B) + §11.4.25 / CONST-048 + §11.4.41 / CONST-061 |
| Scope | HelixQA submodule (`digital.vasic.helixqa`) — all owned-by-us code under `pkg/`, `cmd/`, `tests/`, `challenges/`, `banks/` |
| Cascade | parent project `helix_code` (this submodule) + recursive own-org submodules (none — CONST-051(C) forbids nested own-org chains here) |

---

## 1. Why this ledger exists

Constitution submodule §11.4.27 (cascaded into this submodule's
`CONSTITUTION.md` / `CLAUDE.md` / `AGENTS.md` as **CONST-050(B)**) mandates
100% test-type coverage across every test type the domain warrants. The
verbatim 2026-05-19 operator mandate that triggered round 219 frames it
plainly:

> "all existing tests and Challenges do work in anti-bluff manner —
> they MUST confirm that all tested codebase really works as expected!
> We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completition
> and full usability by end users of the product!"

A test-type slot that exists in the matrix but contains no executable
asset is a **§11.4 PASS-bluff at the coverage layer**: the summary
says "covered" while the user-facing behaviour is unprotected. This
ledger names every slot, identifies the asset that fills it (or marks
it explicitly `NOT_YET_FILLED` with a tracked issue), and records the
captured-evidence pathway for each PASS.

## 2. The 14 test types HelixQA owns

Per CONST-050(B) the test types HelixQA must cover are: **unit,
integration, e2e, full-automation, security, ddos, scaling, chaos,
stress, performance, benchmarking, ui, ux, Challenges**. helix_qa
adds a 15th implicit slot — **autonomous-QA-session** — because
that is the orchestrator's reason for existing (§11.4.52 /
Autonomous-Validation Mandate).

## 3. Coverage matrix — current state (round 219 snapshot)

| # | Test type | Asset path | Runtime evidence shape | Status |
|---|-----------|------------|------------------------|--------|
| 1 | unit | `pkg/*/_test.go` + `cmd/helixqa/*_test.go` | `go test ./... -count=1` output | FILLED — mocks permitted only here per CONST-050(A) |
| 2 | integration | `tests/integration/` | real-process spawn + assertions against real binaries | FILLED |
| 3 | e2e | `tests/e2e/` | end-to-end pipeline run on real banks | FILLED |
| 4 | full-automation | `qa-results/` artefacts captured from `helixqa run --banks banks/ --platform <p>` | structured JSON results + Markdown report per run | FILLED — see §11.4.52 autonomous-validation invariant |
| 5 | security | `tests/security/` + `scripts/anti-bluff/` | scanner reports + `bluff-baseline.txt` reconciliation | FILLED |
| 6 | ddos | `challenges/scripts/ddos_health_flood_challenge.sh` | wire latencies p50/p95/p99 + pass-rate ≥ 95% | FILLED |
| 7 | scaling | `challenges/scripts/scaling_horizontal_challenge.sh` | wire evidence on horizontal scale-out | FILLED |
| 8 | chaos | `challenges/scripts/chaos_failure_injection_challenge.sh` | survival-under-injection evidence | FILLED |
| 9 | stress | `challenges/scripts/stress_sustained_load_challenge.sh` + `tests/stress/` | sustained-load wire evidence | FILLED |
| 10 | performance | `tests/benchmark/` + Go `Benchmark*` funcs | `go test -bench=. -benchmem` output | FILLED |
| 11 | benchmarking | `tests/benchmark/` (named explicitly to satisfy CONST-050(B) literal) | benchmark deltas vs `banks/benchmarking-baselines.yaml` | FILLED |
| 12 | ui | `challenges/scripts/ui_terminal_interaction_challenge.sh` | terminal-UI captured key/screen interactions | FILLED |
| 13 | ux | `challenges/scripts/ux_end_to_end_flow_challenge.sh` | end-user flow captured via journeyed assertions | FILLED |
| 14 | Challenges | `challenges/scripts/*.sh` (12 scripts now incl. orchestrator) | each script emits its own captured wire evidence | FILLED |
| 15 | autonomous-QA-session | `cmd/helixqa/main.go` `autonomous` subcommand + `pkg/autonomous/` + `pkg/issuedetector/` + `pkg/session/` | recorded video + timeline + LLM-discovered tickets per run | FILLED — §11.4.52 / CONST-052 path |

**Round 219 addition:** Challenge #14 row gained
`challenges/scripts/helixqa_orchestrator_challenge.sh` (8-phase
orchestrator-surface validator with built-in paired-mutation per
§1.1). The Challenge proves that the orchestrator's `version`,
`list`, `report`, `run` subcommands, i18n round-trip,
anti-bluff-vocab scan, and secret-leak gate all work for real
users — closing the §11.4 anchor at the orchestrator-entry-point
granularity.

## 4. Per-row evidence-capture pathway

Every row in §3 above MUST produce **positive captured evidence**
on every PASS per §11.4.2 (recorded-evidence requirement). The
pathways are:

- **unit / integration / e2e / performance / benchmarking** —
  `go test -v -count=1` stdout (test names + PASS line per test).
  No `t.Skip()` without `SKIP-OK: #<ticket>` marker per CONST-035.
- **security** — `scripts/anti-bluff/bluff-scanner.sh --mode all`
  report + `challenges/baselines/bluff-baseline.txt` diff.
- **ddos / scaling / chaos / stress** — challenge stdout containing
  `p50`/`p95`/`p99` latency values, throughput numbers, pass-rate
  percentages. Vibes-only PASS is forbidden.
- **ui / ux** — challenge stdout containing assertions about key
  presses, screen content, journey-step outcomes.
- **Challenges** — each `challenges/scripts/*.sh` emits a phase-
  by-phase `[PASS]`/`[FAIL]`/`[SKIP-OK: <reason>]` log; the new
  `helixqa_orchestrator_challenge.sh` formalises the pattern with
  8 phases.
- **autonomous-QA-session** — `qa-results/<session-id>/` directory
  with `qa-report.md`, `tickets/`, `videos/`, `timeline.json`.

## 5. Composition with adjacent constitutional anchors

This ledger composes with (does NOT replace):

- **§11.4.4 / CONST-035** — every PASS in the matrix carries
  positive runtime evidence.
- **§11.4.6 / CONST-035 no-guessing** — "covered" claims in this
  ledger are tied to concrete asset paths; ambiguous claims use
  `UNCONFIRMED:` / `NOT_YET_FILLED` markers.
- **§11.4.25 / CONST-048** — full automation across feature ×
  platform × invariant. This ledger is the test-type axis of that
  larger matrix.
- **§11.4.41 / CONST-061** — pre-force-push merge-first integration.
  When this matrix changes via cross-cutting work, the
  Force-push merge-first audit section in the changelog cites the
  affected rows.
- **§11.4.52 / autonomous-validation** — row 15 is the named asset
  for the §11.4.52 invariant on this submodule.

## 6. How to run the matrix end-to-end

```bash
# From this submodule's root.
make test                                    # rows 1, 2, 3, 10, 11
make qa-all                                  # rows 5, 6, 7, 8, 9, 12, 13, 14
bash challenges/scripts/helixqa_orchestrator_challenge.sh   # row 14 (orchestrator surface)
./bin/helixqa autonomous --project <path> --env .env        # row 15
```

Each invocation produces captured stdout suitable for pasting into
a commit message or changelog. Per §11.4 Definition of Done, no
row above is considered "covered" without that paste.

## 7. Change log

- **2026-05-19, round 219** — initial publication. 15 rows
  populated. Added `helixqa_orchestrator_challenge.sh` (paired
  mutation built in). Cited verbatim 2026-05-19 operator mandate
  per CONST-049 §11.4.17 classification.
- Subsequent rounds: update §3 row counts as new banks / packages
  / Challenges land. The ledger MUST stay in sync per §11.4.60 /
  CONST-063 documentation always-sync composite covenant.

---

*Per CONST-050(B): "A test-type slot that exists in the matrix but
contains no executable asset is a §11.4 PASS-bluff at the coverage
layer." This ledger names every slot.*
