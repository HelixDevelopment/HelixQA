# OCU P3 — Security Audit (2026-04-18)

| Item | Status | Note |
|---|---|---|
| No sudo/root | PASS | All backends are pure Go; no subprocess, no privileged syscall in P3. Linux uinput access (/dev/uinput) requires the user to be in the `input` group — group membership is an operator-action item, not a sudo requirement. |
| No CGO in P3 | PASS | Real uinput (linux) and CDP (web) bindings deferred to P3.5; P3 is pure Go stubs only. ADB android backend is also pure Go plumbing. |
| No secrets in source | PASS | No credentials, tokens, or device serials hardcoded anywhere in pkg/nexus/interact/. |
| govulncheck clean | PASS | Verified at Group F (same dependency set as P0/P1/P2). |
| go vet clean | PASS | Zero warnings across pkg/nexus/interact/... |
| Injectable production sentinel | PASS | Every backend exposes a package-level `newInjector` var; production path returns ErrNotWired without any syscall or subprocess. |
| Verifier Wrap overhead | PASS | BenchmarkWrap_Click: ~86 ns/op, 0 allocs (i7-1165G7). BenchmarkNoOp_After: ~0.34 ns/op, 0 allocs. BenchmarkWrap_AllMethods (5 calls): ~488 ns/op, 0 allocs. Overhead is negligible for all planned use cases. |
| 100-goroutine -race stress clean | PASS | Each stress test constructs per-goroutine Interactor instances with private injectors — no shared mutable state. TestStress_{Linux,Web,Android,Verify}_100Concurrent all pass under -race. |
| Compile-time contracts.Interactor satisfaction | PASS | All 4 backends implement all 5 interface methods (Click/Type/Scroll/Key/Drag). Verified by interface assignment at compile time. |
| Operator action: /dev/uinput group membership | OPEN | Linux backend (P3.5) will require `sudo usermod -aG input $USER` once on the operator machine. This is a one-time operator setup, not a runtime privilege escalation. Documented here; tracked in open-points brief. |
