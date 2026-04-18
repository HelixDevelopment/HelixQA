# OCU P1 — Security Audit (2026-04-18)

| Item | Status | Note |
|---|---|---|
| No sudo/root requirements | ✅ | All three backends (web/linux/android) are pure Go structs. No subprocess spawning in P1 scope. Production subprocess wiring (chromedp / xwd / adb screenrecord) is deferred to P1.5 and will use `exec.CommandContext` with hardcoded binary names and hardcoded arg lists — no shell-string expansion. |
| No CGO in P1 | ✅ | Zero CGO in any P1 file. All three sub-packages are pure Go. `go vet` and `go build -mod=vendor` confirm no cgo directives. |
| Production subprocess args are hardcoded | ✅ | Not yet wired (P1.5 scope). The injectable `newFrameProducer` pattern ensures production wiring will be a single, auditable function — arg lists will be hardcoded slices, never `sh -c` with user-supplied strings. |
| `newFrameProducer` injection is test-only | ✅ | The package-level `newFrameProducer` variable is unexported. Production pathway returns `ErrNotWired` via `productionFrameProducer`. Tests restore the original via `defer func() { newFrameProducer = original }()`. No silent fake data is emitted in production. |
| `Close()` / `Stop()` idempotent | ✅ | Both methods use `sync.Once` (Stop) and `atomic.Bool.Swap` (Close) to guarantee safe double-invocation. Verified by `-race` stress test with 100 concurrent goroutines. |
| `-race` clean under 100-goroutine stress | ✅ | `TestStress_Source_100Clients` passes under `go test -race` on all three sub-packages (web, linux, android). |
| No secrets hardcoded | ✅ | No API keys, tokens, device serials, or credentials in any P1 source file. `.env.example` placeholders only. |
| SSH / ADB credentials not logged | ✅ | P1 introduces no logging paths. Error values are wrapped with `fmt.Errorf` and do not echo config or credentials. |
| `govulncheck` clean | ✅ | Run: `GOTOOLCHAIN=local govulncheck -mode source ./...` — no new vulnerable symbols introduced by P1. |
| `go vet` clean | ✅ | `go vet ./pkg/nexus/capture/...` passes with zero diagnostics. |
