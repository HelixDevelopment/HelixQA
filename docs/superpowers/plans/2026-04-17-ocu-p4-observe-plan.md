# OCU P4 — Observation Engine Implementation Plan

Goal: Implement `contracts.Observer` with 5 pluggable backends behind a single factory.
Scope P4: plumbing + injectable production-not-wired sentinel + shared ring-buffer helper.
Real LD_PRELOAD shim install, PLT/GOT hooking, D-Bus subscription, CDP event tap, and
AT-SPI tree walking are all deferred to P4.5 via injectable producer pattern.

## Groups

- A — factory + ring-buffer helper
  - `pkg/nexus/observe/factory.go`  — `Factory func(ctx, cfg Config) (Observer, error)`, `Config{BufferSize int}`,
    `Register/Open/Kinds` + `sync.RWMutex`
  - `pkg/nexus/observe/ring.go`     — bounded `RingBuffer[contracts.Event]` with `Push(e)` / `Snapshot(at, window)`;
    evicts oldest on overflow; goroutine-safe via `sync.Mutex`
  - `pkg/nexus/observe/factory_test.go` — 4 tests (register, open, unknown-kind, kinds-listing, concurrent)
  - `pkg/nexus/observe/ring_test.go`    — ≥5 tests (happy push+snapshot, timestamp window, eviction on full,
    empty snapshot, concurrent push under -race)

- B — five Observer backends (one sub-package each)

  | Kind        | Package                              | Production stub sentinel        |
  |---|---|---|
  | `ld_preload`| `pkg/nexus/observe/ld_preload/`      | ErrNotWired (P4.5: .so install) |
  | `plthook`   | `pkg/nexus/observe/plthook/`         | ErrNotWired (P4.5: GOT patch)   |
  | `dbus`      | `pkg/nexus/observe/dbus/`            | ErrNotWired (P4.5: gdbus)       |
  | `cdp`       | `pkg/nexus/observe/cdp/`             | ErrNotWired (P4.5: CDP socket)  |
  | `ax_tree`   | `pkg/nexus/observe/ax_tree/`         | ErrNotWired (P4.5: AT-SPI)      |

  Each sub-package:
  - `observer.go` — `Observer` struct satisfying `contracts.Observer`; composes `baseObserver` from
    `pkg/nexus/observe/base.go`; injectable `producer` func var
  - `observer_test.go` — 3 tests (mock produces events / factory registered in init / production returns ErrNotWired)

  Shared `base.go` in `pkg/nexus/observe/` factors:
  - `baseObserver` struct: holds `ring *RingBuffer`, `events chan contracts.Event`, `stopCh chan struct{}`,
    `started sync.Once`, `stopped sync.Once`
  - `startLoop(producer producerFunc)` — goroutine that calls producer, routes events to ring + channel
  - `Events() <-chan Event` — returns the read-only channel
  - `Snapshot(at, window)` — delegates to ring
  - `Stop() error` — closes stopCh via Once; drains; closes channel

- C — bench + stress + security audit + bank + integration + close

  - `pkg/nexus/observe/stress_test.go`   — 100 concurrent Start+Push+Snapshot+Stop cycles across all 5 backends
  - `pkg/nexus/observe/bench_test.go`    — BenchmarkRing_Push / BenchmarkRing_Snapshot
  - `docs/security/ocu-p4-audit.md`      — security checklist
  - `banks/ocu-observe.json`             — ≥15 entries
  - `tests/integration/ocu_observe_test.go` — tag-gated; blank-imports all 5 backends; asserts factory sees all 5 kinds
  - `docs/nexus/ocu-roadmap.md`          — flip P4 row to CLOSED

## Contracts (frozen P0)

```
Observer.Start(ctx, Target) error
Observer.Events() <-chan Event
Observer.Snapshot(at time.Time, window time.Duration) ([]Event, error)
Observer.Stop() error
```

EventKind constants already defined: EventKindSyscall, EventKindDBus, EventKindCDP,
EventKindAXTree, EventKindHook — used by each backend for emitted events.

## Deferred to P4.5

- Real LD_PRELOAD .so shim compilation + install into user-owned path (no sudo)
- PLT/GOT symbol allowlist + runtime patch
- gdbus D-Bus signal subscription
- CDP WebSocket event subscription
- AT-SPI2 accessibility tree walking + event subscription
