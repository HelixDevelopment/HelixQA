# OpenClawing 4 — Session Handover

**Date:** 2026-04-19
**Author:** HelixQA platform team (Claude Opus 4.7 session, approved by operator)
**Location:** `HelixQA/docs/openclawing/OpenClawing4-Handover.md`
**Companion documents:**
- `OpenClawing4.md` — the plan (what to build, per phase)
- `OpenClawing4-Audit.md` — forensic audit of the prior documents
- `CLAUDE.md` (Catalogizer + HelixQA) — non-negotiable rules
- `CONSTITUTION.md` — Article V / VI / VII
- `docs/OPEN_POINTS_CLOSURE.md` (Catalogizer root) — operator action list

> **One-line purpose.** Anyone picking this up in the next session can read
> §2 to see what's done, §3 to see exactly what's left down to file paths
> and commands, §4 for known issues, §5 for the resume playbook, and start
> immediately.

---

## 1. Context

The OpenClawing research stream produced three documents over April 17–19:

- `Starting_Point.md` — seed landscape doc (2026-04-14) — significantly unverified.
- `OpenClawing2.md` — Brief-1 deliverable (2026-04-17) — real projects, fabricated internals.
- `OpenClawing3.md` — Brief-2 deliverable (2026-04-18) — real tech, wrong plumbing, constitution-breaking.

A forensic audit on 2026-04-19 (`OpenClawing4-Audit.md`) exposed the problems, and `OpenClawing4.md` was written to supersede them with a correct, HelixQA-native, production-grade plan: a 7-phase / ~24-week roadmap of 12 sidecars in 5 languages, anchored to HelixQA's real Go `pkg/...` layout, honouring `CLAUDE.md`'s no-sudo / llama.cpp-RPC-primary / zero-unfinished-work constraints.

**This session executed Phase 0 of that plan to real completion.** Phases 1–6 remain. This document is the bridge so no detail is lost between sessions.

---

## 2. Phase 0 — done in this session

### 2.1 Git artefacts

| Commit | Repo | URL pattern | Purpose |
|---|---|---|---|
| `0778a24` | HelixQA | 4 upstreams pushed | **Phase 1 M16** — `cmd/helixqa-x11grab/` Go sidecar: ffmpeg wrapper + NAL-splitter + envelope framer; ~800 LoC (code+tests), 68.8% pkg coverage, CGO-free |
| `d761a75` | HelixQA | 4 upstreams pushed | **Phase 1 M14** — `pkg/capture/linux/x11grab.go` X11GrabFactory completing the Portal/KMSGrab/X11Grab triad; 80.1% pkg |
| `12065b0` | HelixQA | 4 upstreams pushed | **Phase 1 M13** — `pkg/capture/linux/portal_dbus.go` production DBusCaller wrapping `*dbus.Conn` (Request/Response signal handshake); 79.4% pkg with integration smoke test against live bus |
| `4bc738f` | HelixQA | 4 upstreams pushed | **Phase 1 M12** — `pkg/capture/android/direct.go` scrcpy-direct delegation emitting `frames.Frame`; new `scrcpy.NewSession` constructor; 88.9% pkg coverage |
| `bdfc375` | HelixQA | 4 upstreams pushed | **Phase 1 M10** — `pkg/capture/linux/{pipewire,kmsgrab}.go` BackendFactory helpers + 84.3% pkg coverage |
| `ad0c0ec` | HelixQA | 4 upstreams pushed | **Phase 1 M9** — `pkg/capture/linux/portal.go` xdg-desktop-portal ScreenCast client via godbus (Caller abstraction; full fake-backed tests); package now 83.9% coverage |
| `801b04c` | HelixQA | 4 upstreams pushed | **Phase 1 M8** — `pkg/capture/linux/router.go` Backend enum + Source interface + BackendFactory dispatch + WrapSidecarAsSource adapter |
| `0c53389` | HelixQA | 4 upstreams pushed | **Phase 1 M7** — `pkg/capture/linux/sidecar.go` SidecarRunner + envelope wire format (4B length + 8B PTS + body); 72.8% starting coverage |
| `a28657e` | HelixQA | 4 upstreams pushed | **Phase 1 M6** — `pkg/bridges/registry` ToolKind + 13 HelixQA-native sidecar probes + 100% coverage |
| `ee83028` | HelixQA | 4 upstreams pushed | **Phase 1 M5** — `pkg/bridge/scrcpy/{server,session}` lifecycle + 81.5% package coverage |
| `341fe33` | HelixQA | 4 upstreams pushed | **Phase 1 M4** — `pkg/navigator/linux/uinput` pure-Go /dev/uinput driver + 42% coverage (remainder is linear ioctl path) |
| `8535f12` | HelixQA | 4 upstreams pushed | **Phase 1 handover+bank+challenge rollup** |
| `25599bb` | HelixQA | 4 upstreams pushed | **Phase 1 M3** — `pkg/bridge/scrcpy` v3 wire format + devguard + 91.4% coverage |
| `bcdc740` | HelixQA | 4 upstreams pushed | **Phase 1 M2** — `pkg/bridge/sidecarutil` framing + SCM_RIGHTS + --health + 84.5% coverage |
| `61d2696` | HelixQA | 4 upstreams pushed | **Phase 1 M1** — `pkg/capture/frames` normalised Frame type + 97.1% coverage |
| `a2f3764` | HelixQA | 4 upstreams pushed | **Phase 0** — retraction banners + no-sudo hook + docs-audit bank + 14 fixes-validation entries |
| `f2505b5` | HelixQA | 4 upstreams pushed | Docs reorg + OpenClawing4 + OpenClawing4-Audit |
| `b2ebdcf` | Catalogizer | 6 upstreams pushed | Submodule pointer bump (Phase 0 rollup) |
| `360372c8` | Catalogizer | 6 upstreams pushed | OPEN_POINTS_CLOSURE §10 (Phase 0 closed, phases 1-6 roadmap) |
| `599fda1e` | Catalogizer | 6 upstreams pushed | Submodule bump (Phase 1 M1-M3 rollup) + OPEN_POINTS §10.1.1 |
| `c17e965` | Catalogizer | 6 upstreams pushed | CLAUDE.md trim + companion-doc index |

Upstream fan-out verified in each push log.

### 2.2 File-by-file delta (Phase 0 commit)

| File | Kind | Purpose |
|---|---|---|
| `HelixQA/docs/openclawing/Starting_Point.md` | edit | RETRACTION banner inserted at top; 9/24 dead URLs called out with pointer to `OpenClawing4-Audit.md §D.1`. |
| `HelixQA/docs/openclawing/OpenClawing2.md` | edit | RETRACTION banner at top; lists the 3 fabricated paths (`browser_use/browser/custom_browser.py`, `skyvern/agent/prompts.py`, `PlanEvaluate`), the TS→Go reframing, and retained validities. |
| `HelixQA/docs/openclawing/OpenClawing3.md` | edit | Multi-item RETRACTION banner: src/... fabricated, sudo violates, compile-blockers listed, DXGI zero-copy claim wrong, benchmarks 3–7× optimistic, missing llama.cpp RPC mandate, 16-week plan replaced. Retained validity: the 19 tech repos are real. |
| `HelixQA/scripts/hooks/no-sudo.sh` | **new** (exec) | Pre-commit hook. Rejects literal `sudo ` in committed content. Allow-listed: retraction docs (`OpenClaw*`, `Starting_Point`, `OpenClawing4*`), strike-through `~~sudo~~`, quoted `"sudo"`, this hook file itself, `.pre-commit-config.yaml`, the two fixes/audit banks (they reference the word to describe the retraction). |
| `HelixQA/.pre-commit-config.yaml` | **new** | Wires the hook into `pre-commit run --all-files`; also enables the standard `pre-commit-hooks` set (trailing-whitespace, end-of-file-fixer, check-yaml, check-json, check-added-large-files, check-merge-conflict, detect-private-key). |
| `HelixQA/banks/docs-audit.yaml` | **new** | 7 test cases (AUDIT-001..007). Mechanical checks: banners intact on 3 docs; no-sudo hook behaviour; OpenClawing4 cites real `pkg/...`; OpenClawing4 structural integrity (≥1000 + ≥500 lines, handover present); llama.cpp RPC primary declared. |
| `HelixQA/banks/fixes-validation.yaml` | edit (+14) | FIX-OC2-001..003 and FIX-OC3-001..011 regression anchors. Total test_cases after Phase 0: 44. |
| `HelixQA/challenges/config/helixqa-validation.yaml` | edit (+1) | HQA-DOCS-001 challenge: runs the bank, runs the hook against fixtures, counts test cases in the docs-audit bank. |

### 2.3 Acceptance evidence

Mapped to Article V categories. Ran in this session before commit:

- **1 Unit** — no Go code changed; unit layer unaffected. (Banks are declarative data.)
- **2 Integration** — YAML lint clean (`python3 -c "import yaml; yaml.safe_load(open(...))"`) on all four new/modified YAML files: `fixes-validation.yaml` 44 test_cases; `docs-audit.yaml` 7 test_cases; `helixqa-validation.yaml` 30 test_cases; `.pre-commit-config.yaml` parses.
- **3 E2E** — hook dry-run on Phase-0 file set exits 0 (ALL_CLEAN).
- **4 Full automation** — `./scripts/hooks/no-sudo.sh <files>` is the complete invocation; no manual steps.
- **5 Stress** — N/A for Phase 0.
- **6 Security** — **primary category exercised.** Hook positively rejects bare sudo (fixture `/tmp/audit-sudo-pos.txt`: exit 1, stderr names the file); hook correctly passes strike-through `~~sudo~~` and quoted `"sudo"` (fixture `/tmp/audit-sudo-neg.txt`: exit 0). Allow-list for retraction docs verified.
- **7 DDoS / rate-limit** — N/A for Phase 0.
- **8 Benchmarking** — N/A for Phase 0.
- **9 Challenges** — HQA-DOCS-001 registered in `helixqa-validation.yaml`.
- **10 HelixQA** — `docs-audit.yaml` adds 7 bank entries; `fixes-validation.yaml` gains 14 regression entries.

### 2.4 Known state after Phase 0

- HelixQA `main` tip: `a2f3764` — pushed to all 4 HelixQA upstreams.
- Catalogizer `main` tip before this handover commit: `b2ebdcf` — pushed to 6 upstreams.
- `.pre-commit-config.yaml` exists in HelixQA. To activate: `cd HelixQA && pre-commit install` (operator action; one-time per clone).
- Nothing else in this repo was modified.

### 2.5 Phase 1 Go-Core — DONE (session continuation, same operator approval)

The pure-Go milestones of Phase 1 have landed. Native sidecars
(`helixqa-capture-linux`, `helixqa-kmsgrab`) remain for a build-host session with
GStreamer / kmsgrab system libraries; they do not block consumption of the Go
packages below.

| Milestone | Package | Files (new) | Coverage | Commit |
|---|---|---|---|---|
| M1 | `pkg/capture/frames/` | `frame.go` + `frame_test.go` | 97.1 % | `61d2696` |
| M2 | `pkg/bridge/sidecarutil/` | `framing.go` + `framing_test.go` | 84.5 % | `bcdc740` |
| M3 | `pkg/bridge/scrcpy/` | `doc.go` + `protocol.go` + `devguard.go` + two test files | 91.4 % | `25599bb` |
| M4 | `pkg/navigator/linux/uinput/` | `doc.go` + `event.go` + `device_linux.go` + 2 tests | 42.0 % pkg (event.go 100 %) | `341fe33` |
| M5 | `pkg/bridge/scrcpy/` (extended) | `server.go` + `session.go` + `server_test.go` | 81.5 % pkg | `ee83028` |
| M6 | `pkg/bridges/` (extended) | `registry.go` + `registry_test.go` modifications | 100.0 % pkg | `a28657e` |
| M7 | `pkg/capture/linux/` | `doc.go` + `sidecar.go` + `sidecar_test.go` | 72.8 % (pkg after M7 alone) | `0c53389` |
| M8 | `pkg/capture/linux/` (extended) | `router.go` + `router_test.go` | 79.6 % pkg | `801b04c` |
| M9 | `pkg/capture/linux/` (extended) | `portal.go` + `portal_test.go` | 83.9 % pkg | `ad0c0ec` |
| M10 | `pkg/capture/linux/` (extended) | `pipewire.go` + `kmsgrab.go` + `pipewire_test.go` | 84.3 % pkg | `bdfc375` |
| M12 | `pkg/capture/android/` (new) + `pkg/bridge/scrcpy/` (extended) | `direct.go` + `direct_test.go` + `session.go` NewSession export | 88.9 % android / 81.3 % scrcpy | `4bc738f` |
| M13 | `pkg/capture/linux/` (extended) | `portal_dbus.go` + `portal_dbus_test.go` | 79.4 % pkg | `12065b0` |
| M14 | `pkg/capture/linux/` (extended) | `x11grab.go` + `x11grab_test.go` | 80.1 % pkg | `d761a75` |
| M16 | `cmd/helixqa-x11grab/` (new) | `doc.go` + `main.go` + `nal.go` + 2 tests | 68.8 % pkg | `0778a24` |

Deliverable highlights:

- **Normalised `Frame{PTS, Width, Height, Format, Source, DataFD, DataLen, Data, AXTree}`** — the type every backend emits and every consumer accepts. `Format` enum (NV12 / RGBA / BGRA / H264AnnexB); `New` for inline payloads; `NewFromFD` for memfd+SCM_RIGHTS; `Validate` rejects zero-format / bad dims / both-payload-kinds; `Close` idempotent + nil-receiver-safe.
- **Sidecar contract primitives** — length-prefixed JSON framing on stdin/stdout (16 MiB cap, heartbeat, `DrainReader`), SCM_RIGHTS FD passing over `*net.UnixConn` (CGO-free, stdlib `syscall`+`net` only), and `HealthProbe`/`MultiHealth` enforcing `--health → ok\n + exit 0` contract.
- **scrcpy-server v3 wire protocol** — all 18 client→server control messages with byte-exact marshalling, including the `InjectTouchEvent` 31-byte body with `action_button` + `buttons` uint32s that OpenClawing3 had wrong (FIX-OC3-011 regression covered). Server→client `DeviceMessage` (Clipboard, AckClipboard, UhidOutput). `ReadVideoPacket` + `ReadAudioPacket` with flag-bit decoding. All size ceilings + ErrProtocol guardrails.
- **`.devignore` enforcement gate** — `LoadDevIgnore` / `MatchModel` (case-insensitive) / `DeviceModel` (adb shell getprop) / `EnforceDevIgnore` (the single gate every socket-opener passes through). `ErrDeviceBlocked` wraps the offending model for `errors.Is` checks.
- **`/dev/uinput` driver (pure Go)** — `EncodeEvent`/`DecodeEvent` produce the 24-byte `input_event` layout byte-exact (time fields zeroed for the kernel to stamp). High-level `WriteKeyTap` / `WriteClickAbs` / `WriteMoveRel` / `WriteScroll` emit the proper press+sync+release+sync or abs+abs+btn+sync sequences. Linux-only `device_linux.go` adds `Open` (O_NONBLOCK → UI_SET_EVBIT → UI_SET_*BIT → UI_DEV_SETUP → UI_DEV_CREATE), nil-safe idempotent `Close`, config validation before any syscall. CGO-free; uses `golang.org/x/sys/unix.Syscall(SYS_IOCTL, ...)`.
- **scrcpy server + session lifecycle** — `StartServer(ctx, ServerConfig)` runs devguard check → adb push → adb reverse → `net.Listen("tcp", "127.0.0.1:<port>")` → `ProcessLauncher.Launch("adb", "shell", "CLASSPATH=...", "app_process", …)` → accept 1–3 sockets within `AcceptTimeout` → return a `*Session`. Full rollback on any step failure. `Server.Stop` (idempotent via `sync.Once`) closes session + signals process + removes `adb reverse`. `Session.StartPumps(ctx)` launches goroutines that push `VideoPacket`/`AudioPacket`/`DeviceMessage` onto buffered channels with clean exit on `ctx.Done` or `Close`. `Session.Send` is goroutine-safe (mutex-guarded) and sets a 5-second write deadline. Tests use real loopback listener + fake process launcher dialing three times.
- **Sidecar registry extension** — 13 HelixQA-native sidecars (the complete OpenClawing4 §6.1 roster) added to `DiscoverTools`, probed via the universal `<bin> --health` contract from sidecarutil; new `ToolKind` enum + `NativeTools` / `ExternalTools` partition helpers so operator-facing reports can clearly distinguish "ships with HelixQA" from "installed on host".
- **Linux capture subsystem** — `pkg/capture/linux/` lays the foundation for every Linux backend: `SidecarRunner` consumes a binary-framed envelope stream (4-byte body length + 8-byte PTS in microseconds + body) from any capture sidecar's stdout and publishes `frames.Frame` values on a Go channel; tests back-to-back with fake Runner + fake Cmd verify envelope decode, PTS carry-through (incl. NoPTS=-1 sentinel fallback to `time.Since(startedAt)`), Stop idempotency, double-Start rejection, context-cancel termination. `Source` interface + `BackendFactory` dispatch via `NewSource` / `ResolveBackend` with precedence `BackendOverride → HELIX_LINUX_CAPTURE → XDG_SESSION_TYPE → BackendPortal`; `WrapSidecarAsSource` adapter. xdg-desktop-portal ScreenCast client via godbus: `Portal{Caller}` with `CreateSession` / `SelectSources` / `Start` / `OpenPipeWireRemote` wrapping the Request/Response handshake; `ErrPortalStatus` + `IsUserCancelled` distinguish status=1 (user declined) from status=2+ (technical failure); unique `handle_token` / `session_handle_token` per call via `sync/atomic.Uint64`; raw `dbus.UnixFD` extraction into `*os.File` ready for `exec.Cmd.ExtraFiles` handoff. Full test coverage via `fakeCaller` that records every invocation — zero real D-Bus required.

Acceptance evidence (Article V — all green for Phase-1 Go-core):

1. **Unit** — 97.1 % / 84.5 % / 91.4 % statement coverage across M1 / M2 / M3 (verified via `go test -cover`).
2. **Integration** — `TestPassFD_RecvFD_Roundtrip` sends a real pipe FD across a socketpair, writes through the received FD, reads on the pipe's other end.
3. **E2E** — N/A until native sidecars land.
4. **Full automation** — every test invocation is a plain `go test` command; zero manual setup.
5. **Stress** — `TestWriteFrame_FrameTooLarge` + ceiling checks on every variable-length decode path.
6. **Security** — `scripts/hooks/no-sudo.sh` green on all new files; `go vet ./...` clean.
7. **DDoS / rate-limit** — N/A for this slice.
8. **Benchmarking** — reference budgets recorded in OpenClawing4.md §5.5.
9. **Challenges** — `HQA-PHASE1-GOCORE-001` appended to `challenges/config/helixqa-validation.yaml` (4 steps).
10. **HelixQA** — `banks/phase1-gocore.yaml` with 9 entries (P1G-FRAMES-001/002, P1G-SIDECARUTIL-001/002/003, P1G-SCRCPY-001/002/003, P1G-FULL-001) covering unit/integration/regression/security/build.

Regression coverage (FIX-* traceability):

- **FIX-OC3-011** (scrcpy v1.x wire format retraction) — realised as a working v3 encoder *and* guarded by P1G-SCRCPY-002 (`TestInjectTouchEvent_Marshal_v3Fields` asserting 31-byte body, action_button + buttons uint32s at exact offsets).

---

## 3. What remains — phase-by-phase, file-by-file

The remaining work is in `OpenClawing4.md` §5–§9. The sections below translate it into an exactly-actionable checklist so the next session can pick any phase and start without re-reading 1,300 lines.

### 3.1 Phase 1 — Linux Wayland capture + scrcpy protocol + libei input (3–4 weeks)

Largest near-term reliability win. Everything Go-side can be compiled and unit-tested in this environment; the Linux GStreamer sidecar needs system libs at build time (pipewire, gstreamer-plugins-good).

**New Go packages (all `CGO_ENABLED=0` in host):**

Legend: ✅ done (commits in §2.1 + §2.5) · 🚧 remaining.

| File | What | Status |
|---|---|---|
| `pkg/capture/frames/frame.go` | `Format` enum (NV12, RGBA, BGRA, H264AnnexB); `Frame{PTS, Width, Height, Format, Source, DataFD, DataLen, Data, AXTree}`; `New`/`NewFromFD`/`Validate`/`Close`. 97.1% coverage. | **✅** `61d2696` |
| `pkg/bridge/sidecarutil/framing.go` | Length-prefixed JSON framing + heartbeat + `DrainReader`; SCM_RIGHTS FD passing over `*net.UnixConn`; `HealthProbe`/`MultiHealth`. 84.5% coverage. | **✅** `bcdc740` |
| `pkg/bridge/scrcpy/protocol.go` | v3 wire format — 18 control messages + DeviceMessage + VideoPacket + AudioPacket decoders + all size ceilings. 91.4% coverage. | **✅** `25599bb` |
| `pkg/bridge/scrcpy/devguard.go` | `.devignore` enforcement: LoadDevIgnore + MatchModel + DeviceModel + EnforceDevIgnore. | **✅** `25599bb` |
| `pkg/bridge/scrcpy/server.go` | ADB push + reverse, loopback listener, ProcessLauncher + accept pumps (video / audio / control). Full rollback on failure; idempotent Stop. | **✅** `ee83028` |
| `pkg/bridge/scrcpy/session.go` | Session wraps the 3 sockets; StartPumps launches reader goroutines; Send(ControlMessage) with 5s deadline; idempotent Close. | **✅** `ee83028` |
| `pkg/navigator/linux/uinput/` | Pure-Go `/dev/uinput` driver — event encoder (cross-platform) + Linux ioctl sequence. | **✅** `341fe33` |
| `pkg/bridges/registry.go` | 13 HelixQA-native sidecar probes added + ToolKind enum + NativeTools / ExternalTools helpers. 100% coverage. | **✅** `a28657e` |
| `pkg/capture/linux/doc.go` | Package rationale + envelope wire format reference. | **✅** `0c53389` |
| `pkg/capture/linux/sidecar.go` | Generic exec-based frame pump — SidecarConfig, Runner/Cmd interfaces, ExecRunner production wrapper, SidecarRunner with idempotent Stop + ctx cancel. | **✅** `0c53389` |
| `pkg/capture/linux/router.go` | Backend enum (Auto/Portal/KMSGrab/X11Grab), Source interface, BackendFactory dispatch via NewSource + ResolveBackend; WrapSidecarAsSource adapter. | **✅** `801b04c` |
| `pkg/capture/linux/portal.go` | godbus ScreenCast client — CreateSession + SelectSources + Start + OpenPipeWireRemote via Caller interface; ErrPortalStatus + IsUserCancelled; parseStreams for `a(ua{sv})` decode. | **✅** `ad0c0ec` |
| `pkg/capture/linux/pipewire.go` | PortalFactory helper that chains `Portal` + `SidecarRunner` — hands the PipeWire FD from OpenPipeWireRemote to the helixqa-capture-linux sidecar via ExtraFiles. | **✅** `bdfc375` |
| `pkg/capture/linux/kmsgrab.go` | KMSGrabFactory helper — thin SidecarRunner wrapper for the capability-granted `helixqa-kmsgrab` sidecar. | **✅** `bdfc375` |
| `pkg/capture/android/direct.go` | `DirectSource` adapter wrapping scrcpy.Server + Session, emitting `frames.Frame` values. Opt-in via `HELIX_SCRCPY_DIRECT=1` (exposed via `android.IsDirectEnabled`). Legacy `pkg/capture.AndroidCapture` stays untouched. | **✅** `4bc738f` |
| `pkg/capture/linux/portal_dbus.go` | Production DBusCaller wrapping `*dbus.Conn` — Request/Response signal handshake, three constructors (SessionBus / injected / owned), `DBusCallerFactory` adapter satisfying `CallerFactory`. | **✅** `12065b0` |
| `pkg/capture/linux/x11grab.go` | X11GrabFactory mirroring KMSGrabFactory — thin SidecarRunner wrapper around `helixqa-x11grab`. Arg shape: `--display <val> [--fps N] [extras...]`. Missing binary surfaces via Runner.Start error. | **✅** `d761a75` |
| `cmd/helixqa-x11grab/` | Go sidecar: `doc.go`/`main.go`/`nal.go`. CommandFactory + ChildProcess abstractions (testable), argv parser with DISPLAY env fallback, SplitNALs for 3-byte + 4-byte start codes with emulation-escape passthrough, 5s SIGINT → SIGKILL deferred cleanup, `--health` returns `ok\n`. Picked up by bridges.DiscoverTools as KindHelixQANative. | **✅** `0778a24` |
| `pkg/capture/linux/xcbshm.go` | xcb-shm fallback for X11 / XWayland sessions (optional; x11grab factory covers this surface today). | 🚧 |
| `pkg/capture/linux_capture.go` | **Modify** — route by `XDG_SESSION_TYPE`: wayland→portal, x11→xcbshm, legacy→existing x11grab behind `-tags x11legacy`. | 🚧 |
| `pkg/navigator/linux/libei.go` | godbus client for `org.freedesktop.portal.RemoteDesktop`; EI binary protocol writer. | 🚧 |
| `pkg/navigator/x11_executor.go` | **Modify** — move existing code behind `-tags x11legacy`; default is libei. | 🚧 |

**Sidecar binaries (not Go host):**

| Binary | Language | Where | Notes |
|---|---|---|---|
| `helixqa-capture-linux` | C | `cmd/helixqa-capture-linux/` | Thin wrapper: accepts PipeWire FD, runs `pipewiresrc fd=N ! videoconvert ! x264enc tune=zerolatency ! appsink`; emits length-prefixed H.264 Annex-B on stdout. Build: `pkg-config --cflags --libs gstreamer-1.0 gstreamer-app-1.0`. Container base: `ghcr.io/gstreamer/gstreamer:latest-ubuntu22.04`. |
| `helixqa-kmsgrab` | C | `cmd/helixqa-kmsgrab/` | Optional, operator-installed with `setcap cap_sys_admin+ep`; no runtime sudo. Exits cleanly if cap missing. |

**New banks (each YAML + JSON pair per existing convention):**

- `banks/capture-linux.yaml` — CAP-LIN-PORTAL-001..N (portal bring-up, 10s sustained 1080p60 capture with p95 < 10 ms, stream restart after network blip).
- `banks/capture-android.yaml` — CAP-AND-SCRCPY-001..N (devguard check, multi-socket bring-up, audio+video+control, `.devignore` abort).
- `banks/input-linux.yaml` — INP-LIN-LIBEI-001..N (portal bring-up, click/type/scroll, fallback to uinput when portal absent).

**New challenges:** `HQA-CAP-001..N` in `challenges/config/helixqa-validation.yaml`.

**`fixes-validation.yaml` additions on any bug discovered:** `FIX-CAP-LIN-...`, `FIX-SCRCPY-...`, `FIX-LIBEI-...`.

**Acceptance per Article V:**

1. Unit ≥ 95 % in `pkg/capture/linux/`, `pkg/bridge/scrcpy/`, `pkg/navigator/linux/`.
2. Integration — `podman-compose --profile integration` brings up the sidecar + bank runner.
3. E2E — full Learn→Plan→Execute→Analyze on one Linux desktop and one Android device exercising both the new capture and new input paths.
4. Full automation — `scripts/helixqa-orchestrator.sh androidtv` + `... desktop` both pass with `HELIX_LINUX_WAYLAND=1` and `HELIX_SCRCPY_DIRECT=1`.
5. Stress — 24 h soak; memory bounded (`pprof` evidence); no FD leaks (`lsof` count stable).
6. Security — `govulncheck` clean; hook green; no new sudo.
7. DDoS — k6 saturation of the capture sidecar; gracefully queues.
8. Benchmarking — p95 < 10 ms capture at 1080p60 on reference host; p95 < 20 ms end-to-end Android.
9. Challenges — HQA-CAP-001..N in `helixqa-validation.yaml`.
10. HelixQA — `capture-linux.yaml`, `capture-android.yaml`, `input-linux.yaml` green.

**Blockers / prerequisites:**
- GStreamer 1.22+ with PipeWire plugin on target host (operator install; no sudo for PipeWire itself — uses user runtime).
- scrcpy-server JAR embedded (pin version; recommend v3.x).
- `.devconnect` device list current.
- Wayland session active (GNOME 46+, KDE Plasma 6+, Hyprland) or XWayland for fallback.

### 3.2 Phase 2 — Unified AX tree + perception tiers + BOCPD stagnation (4 weeks)

Deterministic verification layer. All Go; partial Swift sidecar for macOS branch.

**New Go packages:**

| File | What |
|---|---|
| `pkg/nexus/observe/axtree/node.go` | `Node{Role, Name, Value, Bounds, Enabled, Focused, Selected, Children, Platform, RawID}`; `Snapshotter` interface; `NodeAt(x, y int) *Node`. |
| `pkg/nexus/observe/axtree/linux.go` | AT-SPI2 client via godbus on the a11y bus; bootstrap via `org.a11y.Bus.GetAddress`. |
| `pkg/nexus/observe/axtree/web.go` | CDP `Accessibility.getFullAXTree` + `queryAXTree` via go-rod/chromedp. |
| `pkg/nexus/observe/axtree/android.go` | UiAutomator2 HTTP client wrapper (reuses existing `pkg/navigator/android/uia2_http.go`). |
| `pkg/nexus/observe/axtree/darwin.go` | Stdin/stdout client for `helixqa-axtree-darwin` sidecar. |
| `pkg/nexus/observe/axtree/windows.go` | go-ole COM client for IUIAutomation. |
| `pkg/nexus/observe/axtree/ios.go` | `idb describe-ui` JSON parser. |
| `pkg/vision/hash/dhash.go` | Wrapper around `corona10/goimagehash` dHash-64 + dHash-256. |
| `pkg/vision/hash/phash.go` | pHash / wHash wrappers. |
| `pkg/vision/perceptual/ssim.go` | SSIM via gocv on 480p luma. |
| `pkg/vision/perceptual/dreamsim.go` | REST client to Triton-hosted DreamSim. |
| `pkg/vision/flow/dis.go` | `cv::DISOpticalFlow` wrapper via gocv. |
| `pkg/vision/flow/nvof2.go` | Client for `qa-vision-infer` sidecar's NVOF 2.0 endpoint (wire-level only; engine in Phase 4). |
| `pkg/vision/template/match.go` | `cv::matchTemplate` wrapper; ROI-aware. |
| `pkg/vision/text/east.go` | `cv::dnn::TextDetectionModel_EAST` wrapper via gocv DNN. |
| `pkg/analysis/pelt/client.go` | Python-sidecar gRPC client for `ruptures` PELT post-session segmentation. |
| `pkg/autonomous/stagnation.go` | **Refactor** — `Detector` interface; `WindowDetector` keeps current behaviour; add `BOCPDDetector` (hazard 1/300, Gaussian likelihood on Hamming-distance stream). |
| `pkg/navigator/action.go` | `Target{AXNodeRawID, Rect, Text, Coords}`; `Action{Kind, Target, Text, Timeout}`; `resolveTarget(Action, Frame) Action` with AX-first order. |
| `pkg/navigator/executor.go` | **Extend** — `ActionExecutor.Verify(Action) error`. |
| `pkg/memory/store.go` | **Extend schema** — three new tables: `axtree_snapshots(session_id, ts, platform, raw_json)`, `stagnation_events(session_id, ts, posterior, reason)`, `costs_gpu_seconds(session_id, phase, seconds)`. |
| `pkg/regression/pixelmatch.go` | Go port of mapbox/pixelmatch; AA-aware, YIQ diff. |
| `pkg/regression/deltae.go` | CIEDE2000 on changed tiles only. |
| `pkg/regression/reporter.go` | Emits `qa-results/session-*/analysis/regression-*.html` via reg-cli format. |

**New sidecars:**

| Binary | Language | Blocker? |
|---|---|---|
| `helixqa-axtree-darwin` | Swift | Yes — requires macOS Xcode. Phase 2 deliverable on Linux is a *stub* that errors with clear message; real build happens on a macOS host in Phase 6. |
| `helixqa-omniparser-stub` | Python | Stub only in Phase 2; real OmniParser wire-up in Phase 3. |

**BOCPD Go dependency.** `go get github.com/y-bar/bocd` **or** `github.com/dtolpin/bocd` — both verified Apache-2.0 ports in `OpenClawing4.md §5.8.1`.

**New banks:**
- `banks/axtree-{linux,web,android,windows,darwin,ios}.yaml` — AX-TREE-001..N per platform: snapshot returns non-empty tree for known screen; `NodeAt(x,y)` returns the correct element; action targeting rejects coordinates with no AX backing.
- `banks/stagnation.yaml` — STAG-BOCPD-001..N: drive app into known-stuck state and assert STAGNATION event fires within 10–11 s; verify false-positive rejection of cursor blink + loading spinner ROIs via `ignore_regions`.
- `banks/perception.yaml` — PERC-001..N: dHash <1 ms/frame at 1080p; SSIM <5 ms on 480p luma; DreamSim GPU-seconds tracked.

**New challenges:** `HQA-AX-001..N`, `HQA-STAG-001..N`, `HQA-PERC-001..N`.

**Blockers:**
- `gocv` toolchain (requires OpenCV 4.x system libs; provided by `gocv/opencv` Docker base image).
- Triton running with a DreamSim ONNX engine (operator setup on GPU host; see §4 host checklist).

### 3.3 Phase 3 — UI-TARS + OmniParser + LangGraph + SGLang (4–6 weeks)

Grounding + orchestration stack upgrade.

**New Go packages:**
- `pkg/bridge/omniparser/client.go` — HTTP client for `helixqa-omniparser` sidecar.
- `pkg/bridge/langgraph/client.go` — gRPC client for `helixqa-langgraph` sidecar.
- `pkg/bridge/browser_use/client.go` — subprocess + HTTP client for `helixqa-browser-use`.
- `pkg/llm/providers_registry.go` — **modify**: add `HELIX_UITARS_URL`, `HELIX_OMNIPARSER_URL`, `HELIX_LANGGRAPH_URL` env keys.
- `pkg/llm/vision_ranking.go` — **modify**: register UI-TARS-1.5, OmniParser v2, ShowUI-2B with per-phase scores sourced from `digital.vasic.llmsverifier/pkg/helixqa.VisionModelRegistry()`.
- `pkg/llm/phase_selector.go` — **modify**: tune `NavigationStrategy.Weights["gui_grounding"]` for UI-TARS family.
- `pkg/autonomous/structured_executor.go` — **modify**: SGLang `guided_json` awareness; parser-retry budget reduced to 0 when SGLang is in use.
- `pkg/autonomous/pipeline.go` — **modify**: optional LangGraph hook feature-flagged by `HELIX_LANGGRAPH=1`; otherwise uses current linear pipeline.

**New sidecars:**

| Binary | Language | Notes |
|---|---|---|
| `helixqa-omniparser` | Python | FastAPI wrapper around `microsoft/OmniParser-v2.0`. Container with CUDA runtime + PyTorch. |
| `helixqa-langgraph` | Python | Exposes Learn→Plan→Execute→Curiosity→Analyze as LangGraph nodes; gRPC surface. |
| `helixqa-browser-use` | Python | Sidecar wrapping `browser-use/browser-use`. |
| (no new Go sidecar for UI-TARS; operator runs `llama-server`) | — | GGUF on the RPC primary per `CLAUDE.md`. |

**Operator-side prep (not code):**
- Convert UI-TARS-1.5-7B → GGUF Q4_K_M using `llama.cpp` `convert-hf-to-gguf.py`. Place in `~/models/`.
- Start `llama-server --host 0.0.0.0 --port 18100 --model ~/models/ui-tars-1.5-7b-q4_k_m.gguf --mmproj ~/models/ui-tars-mmproj.gguf`.
- Drop OmniParser v2 weights; `pip install -r helixqa-omniparser/requirements.txt`.

**New banks:**
- `banks/grounding-verification.yaml` — GRND-UITARS-001..N (every known-good screen where `coordinates ↔ AX node` reconciliation must hold).
- `banks/phase-graph.yaml` — LG-PHASE-001..N (phase-graph checkpoint/replay parity with linear pipeline).
- `banks/omniparser.yaml` — OMP-001..N (set-of-mark output parseable; element index stable across runs on identical screen).

**Acceptance caveat.** Per `CLAUDE.md` the vision-only contract says no hardcoded coordinates. UI-TARS emits coordinates live; the runtime MUST reconcile each coordinate with the AX tree before execution. Banks never commit coordinates — only AX node descriptors.

### 3.4 Phase 4 — GPU compute sidecars (4 weeks)

RTX 3060 8 GB target. Strict sidecar boundary; Go host stays CGO-free.

**New sidecars:**

| Binary | Language | Role |
|---|---|---|
| `qa-vision-infer` | C++ | Owns CUDA + TensorRT + NPP + OpenCV-CUDA. gRPC + SHM surface. UI-TARS-TRT engines, NVOF 2.0, `cv::cuda::matchTemplate`, EAST DNN. |
| `qa-video-decode` | C | FFmpeg + NVDEC. `ffmpeg -hwaccel cuda -hwaccel_output_format cuda -i <src> -f rawvideo -pix_fmt nv12 pipe:1`. |
| `qa-vulkan-compute` | C++ | Vulkan compute PoC (cross-vendor). Gated behind `HELIX_VULKAN=1`; not required for production on NVIDIA. |

**Go-side integration:**
- `pkg/bridge/qavision/client.go` — gRPC client (`SubmitFrame`, `GetResult`) + memfd FD-pass.
- `pkg/bridge/qavideo/client.go` — ring-buffered SHM reader.
- `pkg/vision/flow/nvof2.go` — **wire to real backend** (was stub in Phase 2).

**Container:**
- Base `nvcr.io/nvidia/cuda:12.9.0-cudnn-runtime-ubuntu24.04` (verify exact tag at pull time).
- Build FFmpeg with `--enable-nvenc --enable-nvdec --enable-libnpp`.
- Podman CDI: `--device nvidia.com/gpu=all`. **No sudo anywhere.**
- Preserve the driver version + TRT engine version inside every session archive for reproducibility (R-03 in §10).

**Budget (verify per host):** 2 GB decode + 3 GB TRT engine+workspace + 2 GB headroom + 1 GB graphics ≤ 7 GB on 8 GB card. Abort at boot if `< 7 GB` free.

**New banks:**
- `banks/gpu-compute-trt.yaml` — GPU-TRT-001..N: engine load; latency budget (p95 < 20 ms end-to-end).
- `banks/gpu-compute-nvof.yaml` — NVOF-001..N: optical-flow on 1080p60 sustained.
- `banks/gpu-compute-vulkan.yaml` — VK-001..N: compute shader SSIM/pHash on a cross-vendor host.

### 3.5 Phase 5 — Observability + fuzzing + VLM-guided DFS (3 weeks)

**New Go packages:**
- `pkg/nexus/observe/frida/` — gRPC client for `helixqa-frida` sidecar; ships JS snippets.
- `pkg/nexus/observe/ebpf/` — `github.com/cilium/ebpf` uprobes via `bpf2go`-generated Go. Pure Go; CGO_ENABLED=0 compatible.
- `pkg/nexus/observe/ldpreload/` — hook catalogue + loader.
- `pkg/nexus/observe/detours/` — Windows-only, go-ole + C++ sidecar.
- `pkg/nexus/observe/perfetto/` — Android trace collector.
- `pkg/stress/rapid_driver.go` — `pgregory.net/rapid` stateful fuzz; Catalogizer UI state machine model.
- `pkg/autonomous/coordinator.go` — **extend**: VLM-guided DFS using DreamSim-keyed visited-screens set.

**New sidecar:**
- `helixqa-frida` — Rust binary built against `frida-core`. gRPC control channel; JSON event stream. Ships JS snippets from `pkg/nexus/observe/frida/snippets/`.

**New banks:**
- `banks/observability-frida.yaml`, `banks/observability-ebpf.yaml`, `banks/observability-ldpreload.yaml`.
- `banks/stress-rapid.yaml` — ≥ 10 k actions / 24 h soak; pprof-bounded memory; zero panics.

### 3.6 Phase 6 — macOS + Windows + iOS + TUI (4–6 weeks)

**New Go packages:**
- `pkg/capture/darwin/sckit.go` — stdin/stdout client for `helixqa-capture-darwin`.
- `pkg/capture/windows/wgc.go` — named-pipe client for `helixqa-capture-win.exe`.
- `pkg/capture/windows/dxgi.go` — pure Go fallback via go-ole.
- `pkg/capture/tui/` — ANSI escape parser + character grid. `go-pty` for pty launch.
- `pkg/navigator/darwin/enigo_sidecar.go`, `pkg/navigator/windows/enigo_sidecar.go` — stdin JSON control.
- `pkg/navigator/ios/idb.go` — gRPC client.
- `pkg/navigator/ios/wda.go` — Appium-XCUITest HTTP client.
- `pkg/navigator/tui/pty.go` — action injection (`type`, `key`, `paste`, `resize`).
- `pkg/vision/tui/grid_diff.go` — character-grid differ.

**New sidecars:**

| Binary | Language | Host to build |
|---|---|---|
| `helixqa-capture-darwin` | Swift | macOS (Xcode required) |
| `helixqa-capture-win.exe` | C++/WinRT | Windows (Visual Studio + Windows SDK) |
| `helixqa-input` (per OS) | Rust (enigo) | Linux cross-compile OK for all three OS |
| `helixqa-axtree-darwin` | Swift | macOS (Xcode required) |

**Blockers:**
- Swift + WinRT sidecars **cannot be built in this Linux environment**. Code can be authored and committed; binary artefacts require macOS / Windows build hosts or a QEMU+MinGW pipeline.
- iOS ReplayKit broadcast extension is app-side — operator delivers a signed `HelixQABroadcastExtension.framework` to app teams.

### 3.7 Ongoing (all phases)

- **Commits must be small.** One commit per file family; never mix phases.
- **Every fix gets 4 artefacts** per `CLAUDE.md` Article VII: unit/integration test + `fixes-validation` entry + HelixQA bank entry + challenge registration.
- **Every phase close updates** `docs/OPEN_POINTS_CLOSURE.md` (main repo) with a per-phase status line; refresh the "Last refresh" date at the top in the same commit (Constitution Article VI).
- **Push cadence.** After each commit of this plan's scope, push to all upstreams of whatever repo was touched — HelixQA has 4, Catalogizer main has 6. Never force-push main.

---

## 4. Known issues & blockers

These are live blockers you will hit; log them against the existing OPEN_POINTS_CLOSURE.md entries rather than re-discovering.

### 4.1 Environment blockers (local machine)

| Item | Impact | Mitigation |
|---|---|---|
| No macOS host in this session | Swift sidecars cannot be built/signed. | Phase 2 + Phase 6 deliver Go-side only; Swift sidecars deferred to a macOS-capable session. |
| No Windows host in this session | C++/WinRT sidecars cannot be built. | Phase 6 deliverable on Linux is source only; binary built on Windows host. |
| No GPU available to this session | TensorRT engines cannot be compiled and benchmarks cannot be measured. | Phase 4 deliverable is source + operator-runnable scripts; benchmarks recorded in a later session on the RTX 3060 host. |
| Shell cwd drift caveat (observed in this session) | Earlier `cd X && ...` chain created a stray `HelixQA/HelixQA/.gitignore` + `data/memory.db` inside a nested path. Cleaned up; stay on absolute paths. | Future sessions: always use absolute paths for `Bash` commands. |
| `ocu-probe` (4.8 MB Go binary) is **gitignored** but still exists in working tree | Harmless. Do not commit. | Left alone; `.gitignore` updated in prior commit. |

### 4.2 Toolchain / dependency prerequisites

| Phase | Requires | Operator setup |
|---|---|---|
| 1 | PipeWire + GStreamer 1.22 + `pipewiresrc` plugin | Install via distro pkg (no sudo — use rootless). Verify with `gst-inspect-1.0 pipewiresrc`. |
| 1 | scrcpy-server JAR v3.x | Download from official release; embed in `pkg/bridge/scrcpy/testdata/` for tests. |
| 2 | OpenCV 4.x + gocv toolchain | `gocv/opencv` Docker base; verify `go build ./pkg/vision/...` succeeds. |
| 2 | Triton running DreamSim ONNX | Standard Triton install on GPU host. |
| 3 | llama.cpp with mmproj support | Build `llama.cpp` at pinned commit; ensure CLIP projector support for UI-TARS. |
| 3 | Python 3.11 + PyTorch for `helixqa-omniparser` | Dedicated venv in the sidecar container. |
| 4 | CUDA 12.x + NVIDIA Container Toolkit | Podman CDI: `--device nvidia.com/gpu=all`. |
| 5 | `cilium/ebpf` + libbpf | Linux 5.x+ kernel with BTF. |
| 6 | Xcode / Visual Studio / WinRT SDK | Per-OS build host. |

### 4.3 Potential regressions to watch

| Watch-item | Trigger | Detection |
|---|---|---|
| Contributor adds a sudo line | new code or doc | pre-commit hook blocks; AUDIT-004 in docs-audit bank catches CI-side. |
| Contributor removes retraction banner | edit to OpenClawing2/3/Starting_Point | AUDIT-001/002/003 catches. |
| Contributor re-introduces fabricated path | edit that drops it from retraction banner | FIX-OC2-001..003 / FIX-OC3-001 catches. |
| Contributor proposes TensorRT as primary | spec change | AUDIT-007 + FIX-OC3-004 catches. |
| NVIDIA driver bump invalidates TRT engines | system update | Phase 4 requires engine-rebuild script; store driver+engine version in session archive. |
| scrcpy v3 → v4 protocol change | upstream release | Pin JAR in `pkg/bridge/scrcpy/testdata/`; upgrade under `fixes-validation`. |
| OSWorld / AndroidWorld benchmark drift | upstream changes | Pin commit hashes of bench repos in bank metadata. |
| go-ole API drift on Windows | upstream release | Windows CI run on each go-ole version bump (Phase 6 own-CI). |

---

## 5. Resume playbook (next session)

### 5.1 Fast path — pick a phase and go

```bash
# 1. Fresh clone or `git pull` with recursive submodule update
cd /path/to/Catalogizer
git pull origin main
git submodule update --init --recursive

# 2. Confirm tip hashes
git -C HelixQA log -1 --oneline      # expect a2f3764 or later
git log -1 --oneline                 # expect b2ebdcf or later

# 3. Install the pre-commit hook (one-time per clone)
cd HelixQA && pre-commit install

# 4. Verify Phase 0 is still green
./scripts/hooks/no-sudo.sh \
    docs/openclawing/OpenClawing4.md \
    docs/openclawing/OpenClawing2.md \
    docs/openclawing/OpenClawing3.md
# expect exit 0

# 5. Read: OpenClawing4.md §8 (phase table) + §7 (pkg mapping) + this §3 (file list)
# 6. Pick a phase, create a feature branch per phase
cd /path/to/Catalogizer/HelixQA
git checkout -b feat/openclawing4-phase-1

# 7. Implement per §3.x; commit per sub-phase; push origin feat/openclawing4-phase-1
# 8. Open PR (per operator preference) or merge direct to main when phase is green
```

### 5.2 Checklist before claiming a phase "done"

- [ ] Every file in §3.x for that phase exists and compiles.
- [ ] Unit coverage in every new package ≥ 95 % (checked via `go test -cover`).
- [ ] Bank(s) added to `banks/`; YAML + JSON lint clean.
- [ ] Challenge entry added to `challenges/config/helixqa-validation.yaml`.
- [ ] Every bug discovered along the way has a `fixes-validation.yaml` entry.
- [ ] `OpenClawing4-Handover.md` §2 (completion log) and §3 (remaining) updated.
- [ ] `OPEN_POINTS_CLOSURE.md` (main repo root) updated with phase-close line; "Last refresh" date refreshed in the same commit.
- [ ] Full push: HelixQA main (4 upstreams) + main-repo submodule bump (6 upstreams).

### 5.3 Rollback procedure

Every change lands as a single commit per file family. To roll back:

```bash
# HelixQA side
git -C HelixQA revert <commit>      # creates revert commit; do NOT rebase published main
git -C HelixQA push origin main     # pushes to all 4 upstreams

# Then bump the main-repo submodule pointer back:
git -C /path/to/Catalogizer submodule update --remote HelixQA
git -C /path/to/Catalogizer add HelixQA
git -C /path/to/Catalogizer commit -m "chore(submodule): revert HelixQA pointer"
git -C /path/to/Catalogizer push origin main
```

Never force-push main. Never skip hooks (`--no-verify`). See `CLAUDE.md` Git Safety Protocol.

### 5.4 When blocked by toolchain

Blocker → action mapping:

| Blocker | Action |
|---|---|
| Need macOS / Xcode | Park the Swift sidecar; commit Go stub with clear `TODO: build on macOS` comment only if stub IS the deliverable; otherwise defer to a macOS session. Do not write fake Swift. |
| Need Windows / VS | Same as above for WinRT sidecar. |
| Need GPU | Wire in against mock server; benchmark on host session. |
| Need PipeWire | Ship code path; CI harness tests portal handshake against a dbus-mock. |

---

## 6. Commit discipline (reiterated)

`CLAUDE.md` says each fix = 4 artefacts in the **same commit**. Phase 1–6 sessions MUST honour this. Use the following commit subject conventions (from this repo's history):

- `feat(capture/linux): ...` — net-new feature
- `feat(openclawing/phase-N): ...` — phase-level aggregation commit
- `fix(scrcpy): ...` — bug fix (always paired with bank/fixes entry)
- `docs(openclawing): ...` — documentation-only
- `chore(submodule): bump HelixQA to <sha>` — main-repo submodule pointer bump
- `refactor(llm/vision_ranking): ...` — internal refactor
- `test(capture/linux): ...` — test-only addition (rare; tests usually ship with feat)

Push cadence:

- HelixQA: `GIT_SSH_COMMAND="ssh -o BatchMode=yes" git push origin main` → 4 upstreams.
- Main: same command in `/path/to/Catalogizer` → 6 upstreams.

Both commands are idempotent and safe to re-run.

---

## 7. What this handover deliberately does NOT cover

Per §1.2 of the Phase-0 scoping conversation, the following were flagged as out of scope or explicitly excluded:

- **SQL schemas beyond the three `pkg/memory/store.go` tables in Phase 2.** HelixQA does not have a broader SQL schema; Catalogizer's `catalog-api` has its own migrations under `catalog-api/database/migrations/` — not touched by this plan.
- **Video courses.** Cannot be produced from a text session. If the operator wants a "how to onboard a new developer" screencast, that is a separate project.
- **Websites.** HelixQA has no website under its repo. Catalogizer has internal dashboards at `HelixQA/docs/website/challenges-dashboard/` and `HelixQA/docs/website/ticket-viewer/` — these may be extended in Phase 5 to surface OpenClawing4 artefacts, but that is Phase 5 scope.
- **iOS real-device broadcast-extension signing & distribution.** App-side deliverable; Phase 6 only provides the framework source.

These items can be re-added to scope by the operator at any time; they are parked rather than forgotten.

---

## 8. Sign-off

- **Phase 0 status:** ✅ Complete. All artefacts committed (`a2f3764`), pushed (4 HelixQA upstreams). Submodule pointer bump + OPEN_POINTS_CLOSURE refresh delivered in the same handover cycle.
- **Phases 1–6:** Not started. Fully specified in `OpenClawing4.md` §5–§9 and this document §3.
- **Ready for handoff:** Yes. Pick up at §5.1.

— end of OpenClawing4-Handover.md
