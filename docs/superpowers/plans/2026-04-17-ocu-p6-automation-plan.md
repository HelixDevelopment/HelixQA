# OCU P6 — Unified Automation Surface Implementation Plan

**Date**: 2026-04-17
**Status**: IN PROGRESS
**Spec**: `docs/superpowers/specs/2026-04-17-openclaw-ultimate-program-design.md`

## Overview

P6 delivers the unified automation surface (`pkg/nexus/automation/`) that
composes P1–P5 primitives behind a single `Engine`. The Engine accepts
`Action` values produced by the LLM / Agent state machine, dispatches each
to the right sub-engine (capture, vision, interact, observe, record), runs
post-action verification, and returns a structured `Result` the Agent can
fold into its next planning turn.

### Core principle: LLM remains sole decider

The Engine is a pure dispatcher. It never synthesises, guesses, or invents
actions. Every `Action` it receives was decided by the LLM via the Agent
state machine. The Bridge adapter in `agent_bridge/` is a one-liner that
wires `Engine.Perform` into the Agent without adding any decision logic.

### No CGO, no sudo

P6 has zero CGO. All file paths are user-writable. No elevated privileges
are required. Engine delegates all real work to P1–P5 backends, which have
their own privilege contracts.

---

## Groups

### A — Engine + Action + Result

- `pkg/nexus/automation/action.go`  — `ActionKind` constants + `Action` struct
- `pkg/nexus/automation/result.go`  — `Result` struct + `EvidenceRef` struct
- `pkg/nexus/automation/engine.go`  — `Engine` struct + `New()` + `Perform()`
- `pkg/nexus/automation/engine_test.go` — ≥10 tests covering all ActionKinds
- `pkg/nexus/automation/action_test.go` — Action zero-value safety tests

### B — Verifier composition

- `pkg/nexus/automation/verifier/verifier.go` — `Verifier` interface,
  `PixelVerifier` (uses `VisionPipeline.Diff`), `MultiVerifier` (AND-chain)
- `pkg/nexus/automation/verifier/verifier_test.go` — threshold, false,
  multi-false, error-propagation tests

### C — Agent bridge

- `pkg/nexus/automation/agent_bridge/bridge.go` — `Bridge` adapter:
  receives `automation.Action` from Agent, calls `Engine.Perform`,
  returns `automation.Result`. Zero decision logic.
- `pkg/nexus/automation/agent_bridge/bridge_test.go` — nil-engine, happy
  path, error propagation, compile-time interface check.

### D — Bench + stress + security + bank + integration + close + push

- `pkg/nexus/automation/bench_test.go` — benchmark `Perform` across all
  ActionKinds on stub Engine
- `pkg/nexus/automation/stress_test.go` — 100 concurrent `Perform` calls,
  `-race` clean
- `docs/security/ocu-p6-audit.md` — privilege, CGO, Bridge decision-logic
  proof, PixelVerifier safety
- `banks/ocu-automation.json` — ≥18 test bank entries covering all
  ActionKinds + verifier variants + Bridge + stress
- `tests/integration/ocu_automation_test.go` — tag-gated; end-to-end
  sequence (Capture → Click → Analyze → RecordClip) against stub backends
- Update `docs/nexus/ocu-roadmap.md` P6 row → CLOSED

---

## ActionKind → dispatch table

| Kind            | Sub-engine call              | Evidence produced              |
|-----------------|------------------------------|--------------------------------|
| `click`         | `Interactor.Click`           | —                              |
| `type`          | `Interactor.Type`            | —                              |
| `scroll`        | `Interactor.Scroll`          | —                              |
| `key`           | `Interactor.Key`             | —                              |
| `drag`          | `Interactor.Drag`            | —                              |
| `capture`       | `CaptureSource.Frames()`     | `screenshot_before` EvidenceRef|
| `analyze`       | `VisionPipeline.Analyze`     | `DispatchedTo` in Result       |
| `record_clip`   | `Recorder.Clip`              | `clip` EvidenceRef             |

---

## Verification chain

After every mutating action (click / type / scroll / key / drag) the Engine
can optionally run `verifier.MultiVerifier` if provided. Verification takes
a before-frame snapshot (pre-action) and after-frame snapshot (post-action)
and calls `VisionPipeline.Diff`. `PixelVerifier` passes when
`DiffResult.TotalDelta >= Threshold`. `MultiVerifier` short-circuits on the
first inner-verifier failure.

In P6 the Engine wires the verifier into `Result.VerificationPassed`; the
Agent uses this signal to detect stuck screens without any LLM roundtrip.

---

## Agent bridge contract

```
LLM → AgentStep.Actions → [nexus.Action] → Bridge.ExecuteAction(automation.Action)
        → Engine.Perform → Result → perception signal for next Agent turn
```

Bridge maps `nexus.Action.Kind` → `automation.ActionKind` and populates the
`automation.Action` from the corresponding `nexus.Action` fields. The bridge
is the only place where `nexus.Action` and `automation.Action` types meet.

---

## Security invariants

1. No `sudo`, no `setuid`, no capability-raising.
2. Zero CGO.
3. Bridge has no decision logic — `ExecuteAction` is a single delegating call.
4. `PixelVerifier` returns an error (not a silent `false`) when `Vision.Diff`
   fails — no silent swallows.
5. `MultiVerifier` propagates the first error immediately.

---

## Exit gate

P6 is CLOSED when:

- `GOTOOLCHAIN=local go test -mod=vendor -race ./pkg/nexus/automation/... -count=1` passes
- `go vet ./pkg/nexus/automation/...` clean
- `gofmt -l pkg/nexus/automation/` empty
- `govulncheck -mode source ./...` reports no new findings
- All ≥18 bank entries present in `banks/ocu-automation.json`
- Integration test `tests/integration/ocu_automation_test.go` compiles cleanly
- `docs/nexus/ocu-roadmap.md` P6 row shows CLOSED
