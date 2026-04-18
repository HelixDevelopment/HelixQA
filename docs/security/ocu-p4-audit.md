# OCU P4 — Security Audit (2026-04-18)

| Item | Status | Note |
|---|---|---|
| No sudo/root | PASS | All five backends are pure Go stubs in P4. No subprocess, no privileged syscall. LD_PRELOAD shim install (P4.5) targets a user-owned directory (e.g. `~/.local/lib/`) — no root required. |
| No CGO in P4 | PASS | Real LD_PRELOAD .so compilation, PLT/GOT patching, gdbus binding, CDP WebSocket, and AT-SPI2 wiring are all deferred to P4.5. P4 is 100% pure Go stubs with injectable producers. |
| No secrets in source | PASS | No credentials, tokens, device serials, or bus addresses hardcoded anywhere in `pkg/nexus/observe/`. |
| govulncheck clean | PASS | Verified at Group C (same dependency set as P0–P3; no new imports added in P4). |
| go vet clean | PASS | Zero warnings across `pkg/nexus/observe/...` |
| Injectable production sentinel | PASS | Every backend exposes a package-level `newProducer` var typed as a local `producer` interface. Production path (`productionProducer`) returns `ErrNotWired` without any syscall or subprocess. Tests swap the var for a `mockProducer`. |
| Ring buffer concurrency safety | PASS | `RingBuffer` uses `sync.Mutex` on all reads and writes. `TestRing_ConcurrentPush` (64 goroutines × 32 pushes) passes under -race. |
| BaseObserver lifecycle safety | PASS | `StartLoop` is guarded by `sync.Once`; `BaseStop` is guarded by a second `sync.Once`. `wg.Wait()` in `BaseStop` ensures the goroutine has exited before returning. No goroutine leak possible. |
| 100-goroutine -race stress clean | PASS | `TestStress_Observe_100Concurrent` exercises all five backends (20 goroutines each) via `StartLoop→drain→Snapshot→BaseStop`. Passes under -race. |
| Compile-time contracts.Observer satisfaction | PASS | All 5 backends implement `Start`, `Events`, `Snapshot`, `Stop`. `Events` and `Snapshot` are promoted from `*BaseObserver`; `Start` and `Stop` are defined on each backend struct. |
| LD_PRELOAD user-path install (P4.5 detail) | DEFERRED | Shim `.so` will be placed under `$HOME/.local/lib/ocu/` (user-writable, no sudo). Operator must ensure `LD_PRELOAD` is set in the target process environment — documented as an operator-action item. |
| PLT/GOT symbol allowlist (P4.5 detail) | DEFERRED | Hook patches will be restricted to an explicit allowlist of symbol names to prevent accidental hooking of security-sensitive libc functions. List defined in P4.5 design doc. |
| D-Bus session bus access | PASS | Session bus is user-accessible via `DBUS_SESSION_BUS_ADDRESS`. No system bus (root) access planned. |
| CDP local endpoint | PASS | Chrome DevTools Protocol exposes a local WebSocket on a user-controlled port. No network-facing exposure required. |
| AT-SPI2 accessibility bus | PASS | AT-SPI2 is a session service accessible to the logged-in user via `AT_SPI_BUS_ADDRESS`. No root required. |
