# OCU P6 — Security Audit

**Date**: 2026-04-18
**Scope**: `pkg/nexus/automation/` (Engine, Action, Result, verifier/, agent_bridge/)
**Author**: vasic-digital
**Status**: PASSED

---

## 1. Privilege escalation

**Finding**: No `sudo`, no `setuid`, no capability-raising calls anywhere in
`pkg/nexus/automation/`. The Engine delegates all real work to P1–P5 backends
which carry their own privilege contracts. No new syscall surface is
introduced in P6.
**Risk**: None in P6.
**Action**: None required.

---

## 2. CGO surface

**Finding**: Zero CGO in P6. `engine.go`, `action.go`, `result.go`,
`verifier/verifier.go`, and `agent_bridge/bridge.go` are all pure Go. CGO
is only present in P1 (capture backends) and future P2.5/P5.5 encoder
bindings, which are out of scope for this audit.
**Risk**: None in P6.
**Action**: None required.

---

## 3. Bridge decision-logic proof

**Finding**: `agent_bridge.Bridge.ExecuteAction` is a single delegating call:

```go
func (b *Bridge) ExecuteAction(ctx context.Context, a automation.Action) (automation.Result, error) {
    if b.Engine == nil {
        return automation.Result{}, errors.New("agent_bridge: Engine is nil")
    }
    return b.Engine.Perform(ctx, a)
}
```

There are no conditionals that alter the Action, no field mutations, no
strategy selection, no inference. The LLM's decision passes through
byte-for-byte. The nil-check is a safety guard, not a decision.
**Risk**: None. Bridge is provably decision-free.
**Action**: None required.

---

## 4. Engine decision-logic proof

**Finding**: `Engine.Perform` dispatches on `Action.Kind` (a `switch`)
and routes to the appropriate P1–P5 sub-engine method. No branch alters
the semantics of the action or substitutes a different action. The only
synthesised content is the `EvidenceRef.Ref` string (a frame sequence
number or byte count) — cosmetic labels, not instructions.
**Risk**: None. Engine is a pure dispatcher.
**Action**: None required.

---

## 5. PixelVerifier error handling

**Finding**: `PixelVerifier.Verify` returns an error (not a silent `false`)
when `Vision.Diff` fails or returns a nil result. `MultiVerifier.Verify`
propagates the first inner error immediately, wrapping it with the inner
index for debuggability. Neither implementation swallows errors silently.
**Risk**: None.
**Action**: None required.

---

## 6. Secrets in evidence

**Finding**: `EvidenceRef.Ref` is populated only with:
- A frame sequence number string (`"seq-<uint64>"`), or
- A clip byte-count string (`"<N> bytes"`).

No pixel data, no API keys, no file system paths, no environment variables
are written into any Result or EvidenceRef field.
**Risk**: Low. Callers must not populate `Action.Expected` with secrets, as
it is forwarded verbatim to Verifier implementations.
**Action**: Documented in `action.go` godoc: `Expected` is free-form and
passed to verifiers without sanitisation.

---

## 7. Race safety

**Finding**: `Engine.Perform` is stateless across calls — every invocation
operates on its own stack-local `Result` and does not mutate any Engine
field. The `benchCapture` and `stressCapture` stubs allocate a new channel
per `Frames()` call, preventing shared-state races in tests. The stress test
(`TestStress_Engine_100Concurrent`) passes cleanly under `-race`.
**Risk**: None.
**Action**: None.

---

## 8. Panic surface

**Finding**: `automation.New` panics when any of the five required
sub-engines is nil. This is an intentional programmer-error guard
(fail-fast at construction, not at runtime inside a concurrent Perform).
The panic message identifies the nil argument by name. No other panic
sites exist in P6.
**Risk**: None beyond the intentional design.
**Action**: Callers that build sub-engines lazily must use `NewBridge`
(which defers the nil check to `ExecuteAction`) rather than `New` directly.
This is documented in `agent_bridge/bridge.go` godoc.

---

## 9. Denial of service

**Finding**: `Engine.Perform` is bounded by the sub-engine calls it
delegates to. No unbounded loops, no goroutine spawning, no unbounded
channel allocation inside Perform itself. The `ActionCapture` and
`ActionAnalyze` paths use a non-blocking `select { case …; default: }`
so a slow capture source does not stall the caller.
**Risk**: None in P6. DoS surface lies in P1–P5 backends.
**Action**: None required for P6.

---

## Summary

| Check | Result |
|---|---|
| No sudo / no root | PASS |
| No CGO in P6 | PASS |
| Bridge has zero decision logic (one-liner proof) | PASS |
| Engine is a pure dispatcher (no action synthesis) | PASS |
| PixelVerifier does not swallow errors | PASS |
| MultiVerifier propagates first error immediately | PASS |
| No secrets in EvidenceRef / Result fields | PASS |
| Race-safe under -race | PASS |
| No unbounded loops or goroutine leaks in Perform | PASS |
| Panic surface limited to nil-guard in New() | PASS |
