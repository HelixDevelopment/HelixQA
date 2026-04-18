# OCU P2 — GPU Vision Pipeline Implementation Plan

> **For agentic workers:** Use `superpowers:subagent-driven-development`. Fresh subagent per group. TDD rigid.

**Goal:** Implement the `contracts.VisionPipeline` interface with CPU-local and thinker-CUDA-remote paths. P2 scope ships the pipeline *plumbing* + one trivial CPU-fallback `Analyze`/`Match`/`Diff`/`OCR` that works on raw pixel bytes. Real OpenCV CUDA + TensorRT OCR + DMA-BUF integrations land in P2.5 via the injectable backend pattern.

**Architecture:** `pkg/nexus/vision/` hosts a `Pipeline` struct implementing `contracts.VisionPipeline`. Constructor accepts an `ocuremote.Dispatcher` + fallback `LocalBackend` interface. For each method, the pipeline first tries to `Resolve(Capability{Kind:KindCUDAOpenCV})`; if that returns a remote Worker, dispatch. Otherwise use the Local backend. P2 local backend is deliberately simple (image-bytes-in-image-bytes-out), no cgo.

**Contracts locked in P0** — DO NOT touch `pkg/nexus/native/contracts/vision.go`.

**Invariants:** SPDX, `go 1.25.3`, `GOTOOLCHAIN=local`, commits on main no push, no sudo, no CGO in P2 (deferred to P2.5).

## Groups

### Group A — Pipeline struct + Local backend interface

Create `pkg/nexus/vision/pipeline.go`:
- `LocalBackend` interface with methods matching `VisionPipeline` (Analyze/Match/Diff/OCR) so injection is straightforward.
- `Pipeline` struct (dispatcher, local LocalBackend).
- `NewPipeline(d Dispatcher, local LocalBackend) *Pipeline`.
- Implements all four `VisionPipeline` methods. Each tries `d.Resolve(Capability{Kind:KindCUDAOpenCV|KindTensorRTOCR, PreferLocal: local != nil && isLocalPath})`. If Worker returned is remote, for P2 scope return `ErrNotWired` (real gRPC in P2.5). If local, delegate to `LocalBackend`.

Create `pipeline_test.go` with a fake LocalBackend + fake Dispatcher and 6-8 tests covering each method's dispatch paths. Commit `feat(ocu/vision): pipeline + LocalBackend interface`.

### Group B — CPU local backend (stub implementation)

Create `pkg/nexus/vision/cpu/backend.go`:
- `Backend` struct implementing `vision.LocalBackend`
- `Analyze` returns `Analysis{DispatchedTo: "local-cpu", LatencyMs: N}` with empty slices (real CV lands in P2.5)
- `Match` returns empty Match slice
- `Diff` computes a naive byte-diff region (non-empty for different inputs)
- `OCR` returns empty OCRResult

All methods respect Frame format; reject unsupported formats with a descriptive error.

Tests: 5 tests (one per method + one rejecting unsupported PixelFormat).

Commit `feat(ocu/vision/cpu): minimal CPU local backend`.

### Group C — Remote-dispatch gluing

In `pipeline.go`, add: when `d.Resolve(Capability{...})` returns a non-local Worker, the pipeline records which host was selected (via `ocuremote.Unwrap`) and returns `Analysis{DispatchedTo: "thinker-cuda", ...}` with empty slices + no error (the real compute lands in P2.5). This proves the dispatch path end-to-end without needing CUDA locally.

Add tests to `pipeline_test.go` asserting the Unwrap path works with a stub Dispatcher that returns `*ocuremote.remoteWorker`-like mock.

Commit `feat(ocu/vision): remote dispatch plumbing via ocuremote`.

### Group D — Bench, stress, security, challenge bank

Mirror P1 Group F exactly:
- `bench_test.go` measures Analyze throughput on CPU backend
- `stress_test.go` — 100 concurrent Analyze calls, race-clean
- `docs/security/ocu-p2-audit.md`
- `banks/ocu-vision.json` — 12+ entries

Commit `test(ocu/vision): bench + stress + security + bank`.

### Group E — Integration test

`tests/integration/ocu_vision_test.go` with `//go:build integration` — constructs a Pipeline with CPU backend, runs each method on a synthetic 800×600 BGRA frame, asserts no error, reasonable latency.

Commit `test(ocu/vision): integration smoke`.

### Group F — Close + docs + push

Full suite verification → update `docs/nexus/ocu-roadmap.md` P2 row → bump submodule pointer → closure brief tick → push all upstreams.

## Exit gate

P2 closed when: every test + bench green under `-race`, all 4 `VisionPipeline` methods callable on CPU backend, remote dispatch returns Analysis with DispatchedTo=`"thinker-cuda"`, integration smoke green, bench baseline appended, audit filed, bank ≥12 entries.

---

*Plan written Opus 4.7, 2026-04-17. Contracts frozen by P0.*
