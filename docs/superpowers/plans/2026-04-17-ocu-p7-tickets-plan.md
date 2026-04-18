# OCU P7 — Magical Tickets + v4.0.0 Release Plan

**Phase:** P7 (final OCU phase)
**Date:** 2026-04-17
**Depends on:** P6 Automation (CLOSED 2026-04-18)

## Scope

P7 closes the full OpenClaw Ultimate program. It extends `pkg/ticket` with
rich OCU evidence types, bridges the automation result into the ticket
pipeline, generates a replay-script DSL, delivers four cross-cutting
challenge banks, a 10-category campaign runner, and the v4.0.0 release.

---

## Group A — Evidence kinds in `pkg/ticket`

File: `pkg/ticket/ocu_evidence.go`

Add 12 new `EvidenceKind*` string constants that the automation pipeline
produces. Constants are additive — no existing constant changes.

| Constant | Value |
|---|---|
| EvidenceKindClip | `clip` |
| EvidenceKindDiffOverlay | `diff_overlay` |
| EvidenceKindOCRDump | `ocr_dump` |
| EvidenceKindElementTree | `element_tree` |
| EvidenceKindHookTrace | `hook_trace` |
| EvidenceKindReplayScript | `replay_script` |
| EvidenceKindLLMReasoning | `llm_reasoning` |
| EvidenceKindPerfMetrics | `perf_metrics` |
| EvidenceKindAXTreeDiff | `ax_tree_diff` |
| EvidenceKindHAR | `har` |
| EvidenceKindWebRTCStream | `webrtc_stream` |
| EvidenceKindRawDMA | `raw_dma` |

Tests: all 12 constants non-empty and globally unique.

---

## Group B — FromAutomationResult + BuildReplayScript

Files:
- `pkg/ticket/ocu_mapping.go` — `FromAutomationResult(res automation.Result) []Evidence`
- `pkg/ticket/ocu_replay.go` — `BuildReplayScript(actions []automation.Action) string`
- `docs/ocu-replay-format.md` — DSL specification

### Evidence struct check

`ticket.Ticket` uses `Screenshots []string` and `Logs []string` rather than
a typed `Evidence` struct. The mapping file must add an `Evidence` struct
(if not already present) with `Kind` and `Ref` fields, and return
`[]Evidence` from `FromAutomationResult`.

### Replay DSL

One line per action, colon-separated:
```
<kind>:<field1>=<val1>:<field2>=<val2>
```

Example:
```
click:at=10,20
type:text="hello world"
scroll:at=100,200:dx=0:dy=-10
key:key=Return
drag:from=10,20:to=50,80
```

Tests: ≥ 8 tests covering each ActionKind and a combined multi-action script.

---

## Group C — 10-category campaign script

File: `scripts/ocu-full-campaign.sh`

Runs in sequence:
1. Unit — `go test -race ./pkg/nexus/...`
2. Integration — `go test -tags=integration -race ./tests/integration/...`
3. Stress — `go test -race -run TestStress ./pkg/nexus/...`
4. Security vuln — `govulncheck -mode source ./...`
5. Security vet — `go vet ./...`
6. Bench — `go test -bench=. -benchmem -run '^$' ./pkg/nexus/...`
7. Gofmt — `gofmt -l pkg/`
8. Challenges — `go test -race ./pkg/testbank/...`

Aggregates exit codes; prints a summary table; exits 1 if any category failed.

---

## Group D — Cross-cutting challenge banks

| File | Entries | Focus |
|---|---|---|
| `banks/ocu-tickets.json` | 36 | 12 evidence kinds × 3 cases each |
| `banks/ocu-adversarial.json` | 20 | Malformed sources, bad LLM, thinker unreachable, etc. |
| `banks/ocu-cross-platform.json` | 15 | Same flows on web/linux/android/androidtv |
| `banks/ocu-fixes-validation.json` | 10 | Regression entries citing earlier fix commits |

Total: 81 entries.

---

## Group E — Release notes + roadmap close

Files:
- `docs/releases/v4.0.0.md` — full program summary (P0–P7)
- `docs/nexus/ocu-roadmap.md` — P7 row flipped to CLOSED

---

## Group F — Tags

```
git tag -a v4.0.0-dev.p7 -m "OCU P7 exit gate: magical tickets, 10-cat campaign, cross-cutting banks"
git tag -a v4.0.0 -m "OpenClaw Ultimate v4.0.0 — full program (P0-P7) closed"
git push origin main --tags
```

---

## Success criteria

- `go build ./...` clean
- `go test -race ./pkg/ticket/... ./pkg/nexus/...` all pass
- `go vet ./...` zero diagnostics
- `gofmt -l pkg/` outputs nothing
- All 4 new JSON banks load without error via testbank loader
- `scripts/ocu-full-campaign.sh` runs end to end
- v4.0.0 tag pushed to all remotes
