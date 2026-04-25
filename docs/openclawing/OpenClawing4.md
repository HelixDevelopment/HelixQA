# OpenClawing 4 — Audit, Gap Analysis, Corrections, and the Ultimate HelixQA Integration Plan

**Author:** HelixQA platform team
**Date:** 2026-04-19
**Location:** `HelixQA/docs/openclawing/OpenClawing4.md`
**Supersedes / refines:** `Starting_Point.md`, `OpenClawing2.md`, `OpenClawing3.md`
**Companion audit:** `OpenClawing4-Audit.md` (forensic read of the three prior documents; cited throughout)

> **One-sentence purpose.** Turn the OpenClawing research stream — today a mix of real opportunities, fabricated code paths, constitution-breaking shortcuts, and overambitious benchmarks — into a rigorous, HelixQA-native, production-grade plan that satisfies every original brief **and** honours every non-negotiable constraint in the Catalogizer `CONSTITUTION.md` and `CLAUDE.md`.

---

## 0. How to read this document

This document is organised top-down: **big picture → subsystem → package → file/method → acceptance test**. Section numbers are stable references; internal cross-links use them. Every claim is either (a) verifiable against the current HelixQA Go codebase (file/method named inline), or (b) carries a canonical third-party URL. Where the original research was wrong or invented, the relevant cell in §2 flags it and §11 records the correction so downstream readers never re-introduce the error.

Readership, in order of need:

1. **Reviewers** who must decide whether to green-light the plan — read §1 (TL;DR), §2 (audit), §8 (phased plan).
2. **Engineers implementing phases** — read §3 (codebase ground truth), §5 (tech deep dive), §6 (integration architecture), §7 (package mapping).
3. **QA / release** — read §9 (acceptance) and §10 (risks).
4. **Everyone** should skim §11 (corrections) before citing any OpenClawing2/3 claim again.

The word **MUST** carries its RFC 2119 meaning. "Should" means "do this unless you have documented reason not to." "Never" is absolute.

---

## 1. Executive Summary

### 1.1 Audit verdict in four lines

- `Starting_Point.md` — **unsourced landscape document**; 9 of the 24 primary repository URLs return HTTP 404, and 10 projects have URLs that disagree between the list and the per-project write-ups. Treat as context only; cite **nothing** from it downstream.
- `OpenClawing2.md` (answering Brief-1) — real external projects (anthropic-quickstarts, browser-use, Skyvern, Stagehand, UI-TARS) but **fabricated internal paths** (e.g. `browser_use/browser/custom_browser.py`, a non-existent `skyvern/agent/prompts.py`, an invented `PlanEvaluate` class). Zero GitHub URLs in the body despite the brief explicitly demanding them. Frames the "port target" as TypeScript when HelixQA is Go — the entire porting premise is misaligned.
- `OpenClawing3.md` (answering Brief-2) — 4,204-line encyclopaedia. All 19 technology repositories it names are real; but it invents a `src/...` C++/TS tree that does not exist in HelixQA's Go `pkg/...` layout; contains compile-blocking code (TensorRT `enqueueV3` copy-paste, invalid `namespace{}` inside a `.c` file, deprecated `destroy()` calls); proposes `sudo dpkg -i`, `sudo apt-get install tensorrt`, `sudo usermod -aG input` — **direct violations of the no-sudo / no-root rule in CLAUDE.md**; asserts DXGI "zero-copy" that is actually a GPU↔GPU copy; and schedules 47 technologies across 16 weeks, which is 5–10× too optimistic.
- **Neither deliverable reflects HelixQA's mandatory llama.cpp RPC distributed vision stack.** OpenClawing3 proposes TensorRT as primary inference, contradicting the explicit "llama.cpp RPC distributed inference is the primary local backend" rule.

### 1.2 The recommended stack (TL;DR)

| Layer | Primary (production) | Fallback | Rationale |
|---|---|---|---|
| Grounding VLM (Navigate) | **UI-TARS-1.5-7B** on llama.cpp RPC | ShowUI-2B (low-VRAM nodes); Claude Computer Use (cloud) | 94 % ScreenSpot-V2 open-weight; fits the existing RPC fabric |
| UI-element parser | **OmniParser v2** (YOLO-v8 + Florence-2) sidecar | UGround-V1 7B | Decouples perception from reasoning; LLM consumes marks |
| Reasoning / Planning | Qwen2.5-VL-32B via vLLM **or** Claude Sonnet 5 REST | Qwen2.5-VL-7B local | Phase strategy picks per-run |
| Orchestration | **LangGraph** phase graph (sidecar, Python) | SmolAgents code-agent for Curiosity | Deterministic replay, checkpointing |
| Capture (Linux) | **xdg-desktop-portal ScreenCast + pipewiresrc** (GStreamer sidecar) | X11 xcb-shm fallback | Wayland-correct; zero-root |
| Capture (Android) | **scrcpy-server direct protocol** (pure-Go client) | `adb exec-out screencap` | Kills the x11grab latency, adds audio/clipboard |
| Capture (macOS / Windows) | **SCKit / WGC sidecars** (Swift / C++-WinRT) | AVCaptureScreenInput / DXGI-DD | Only supported paths post-macOS 15 / Win 11 |
| Input (Linux) | **libei via RemoteDesktop portal** (pure Go, godbus) | `/dev/uinput` via udev rule | Wayland-correct; zero-root |
| Input (macOS / Windows) | **enigo sidecar (Rust)** | | One binary per OS; no CGO in host |
| A11y tree | **Unified `Node{Role,Name,Bounds}`** across UIA / AXUIElement / AT-SPI2 / UiAutomator2 / CDP | — | Deterministic action target |
| Fast change detection | **dHash (goimagehash)** → SSIM (gocv) → DreamSim | pixelmatch (Go port) | Tiered; < 5 ms CPU fast path |
| Stagnation | **BOCPD** (Bayesian online change-point, Go port) | CUSUM | Probabilistic "screen stuck" |
| GPU compute | **TensorRT sidecar** (Triton) + NPP + FFmpeg NVDEC | Vulkan compute (cross-vendor) | Sidecar boundary; no CUDA in Go binary |
| Perceptual diff (deep) | **DreamSim** (Triton-hosted) | LPIPS | 96 % human agreement for "did the screen meaningfully change" |
| Observability | **Frida sidecar** (all OSes) + **cilium/ebpf** (Linux) + LD_PRELOAD hooks | Detours (Windows) | Passive evidence only; never for driving |
| Fuzzing | **pgregory.net/rapid** (stateful, Go) + Android Monkey | gopter | Satisfies Constitution Article V cat 5 (stress) |
| Benchmarks | **ScreenSpot-V2, ScreenSpot-Pro, OSWorld-Verified, AndroidWorld** via **ScreenSuite** | AndroidLab, SPA-Bench, WebArena | Satisfies Article V cat 10 (HelixQA) |

**Design invariants** (bind every recommendation):

1. `CGO_ENABLED=0` on the HelixQA Go host binary. All platform-specific native code lives in **sidecars** invoked over stdin/stdout framing or Unix-domain gRPC. Frame payloads travel over `memfd` with file-descriptor passing; metadata travels over the control channel.
2. **No sudo, no root, no elevated privileges**, anywhere, ever. Capability-granted helpers (e.g. `kmsgrab` with `cap_sys_admin`) ship as separately-installed binaries owned by the operator; HelixQA's runtime never invokes `sudo` or `su`.
3. **Universal solution**: all new capability lives in HelixQA; never in the app under test.
4. **Vision-only contract is preserved.** A grounding model that returns pixel coordinates is allowed *only* if every coordinate is verifiable against the accessibility tree within the same frame. No hard-coded coordinates in banks. Actions remain executable (`adb_shell:`, `browser.click(ref)`, `libei.click(x,y)`), never prose.
5. **llama.cpp RPC is the primary local inference backend.** Any new model (UI-TARS, OmniParser, etc.) must have a GGUF / llama.cpp path for the primary deployment. Triton / vLLM are secondary, used when a richer GPU is available.
6. Every capture source normalizes to `pkg/capture/frames.Frame{PTS, Width, Height, Format, Data | FD}`; every action normalizes through `pkg/navigator.Action{Kind, Target (ax_node|rect|text), Payload}`.

### 1.3 Phase roadmap (realistic)

| Phase | Weeks | Outcome |
|---|---|---|
| **0** — Constitutional hardening & doc corrections | 1 | §11 corrections merged; OpenClawing2/3 fabricated paths annotated; SUDO paragraphs retracted; audit appendix signed off. |
| **1** — Linux Wayland capture + scrcpy protocol bridge + libei input | 3–4 | Single biggest reliability win. Replaces `ffmpeg x11grab` and `xdotool` with Wayland-correct paths. Removes the dependency on the `scrcpy` desktop binary. |
| **2** — Unified accessibility-tree layer + perception tiers (dHash → SSIM → DreamSim) + BOCPD stagnation | 4 | Deterministic action targeting; real stagnation detection. |
| **3** — Grounding + parser integration (UI-TARS-1.5-7B on llama.cpp RPC, OmniParser v2 sidecar) + LangGraph phase graph | 4–6 | Upgrades NavigationStrategy, PlanningStrategy, AnalysisStrategy from generic LLM to GUI-specialist stack. |
| **4** — GPU compute sidecars (TensorRT + NPP + FFmpeg NVDEC) + Vulkan compute PoC (cross-vendor future-proofing) | 4 | Moves heavy perception off the host CPU; preserves sidecar boundary. |
| **5** — Observability (Frida sidecar, cilium/ebpf probes, LD_PRELOAD hook index) | 3 | Passive evidence pipeline; no actuation. |
| **6** — macOS + Windows capture sidecars (SCKit, WGC), iOS idb path, cross-platform polish | 4–6 | Everything else. |
| **Total** | **~24 weeks** | Feature-complete, constitution-compliant, bench-grade HelixQA. |

Contrast with OpenClawing3's 16-week plan for 47 technologies. This plan covers a comparable technology surface in ~24 weeks because every phase is independently testable against Article V (§9) and every component has a real place in the Go `pkg/...` tree (§7).

---

## 2. Audit of prior documents

Full forensic detail is in `OpenClawing4-Audit.md`. This section is the summary a reviewer needs.

### 2.1 `Starting_Point.md` — unsourced landscape, mostly fictional

| Concern | Evidence |
|---|---|
| ~37 % of repo URLs 404 | 9 of 24 primary URLs return HTTP 404 (NanoClaw, ZeroClaw, NullClaw, Hermes, Moltworker, DumbClaw, PycoClaw, BabyClaw, Clawlet — verified via `gh api repos/...`). |
| 10 projects have **contradictory URLs** across sections | The "List" section cites one path; the per-project write-up cites another. Picking either at random is a 50/50 shot at a 404. |
| Foundational OpenClaw baseline ("~420k LoC TypeScript", "port 18789", "Lane Queue") is **unsourced** | No citation; not traceable to any public repository. |

**Action:** `Starting_Point.md` is downgraded to "historical context" in this document. It is not cited as authority anywhere downstream. §11.1 lists the dead links.

### 2.2 `OpenClawing2.md` — prose-grade, fabricated internals

Against Brief-1 ("detailed comparison of each project source code vs OpenClaw in area of full control of application's UI / UX and whole flows! Exact codebase references … Focus is on codebase, exact source code files and classes and methods references"):

| Issue | Detail | Source in doc |
|---|---|---|
| Zero GitHub URLs in body | Brief explicitly asks "+ links to the GitHub repos". Body contains none; only the appendix lists a few. | §3–§9 throughout |
| Fabricated internal paths | `browser_use/browser/custom_browser.py` (§4.3.2) — not in `browser-use/browser-use`; `skyvern/agent/prompts.py` (§5.1.2) — Skyvern has no `agent/` directory; `PlanEvaluate` class (§10.1.1, §8.1.2) — not a real browser-use class. | §4.3.2 (L189), §5.1.2 (L241), §10.1.1 |
| Unquantified "winning" | "Stagehand wins by 44 %" — unsourced, no benchmark cited. | §7.x |
| Port-target framing wrong | Describes "OpenClaw" in TypeScript at `src/agents/pi-embedded-runner.ts`. HelixQA is Go; a source-to-source port of TypeScript into `pkg/` would be a non-trivial architectural change that the doc never acknowledges. | §2, §10 |
| Coverage gaps vs brief | Brief demands "any type of applications: Web, Mobile, Desktop, even APIs or TUI" and "no glitches in UI/UX/whole flows". Mobile = token mentions only; TUI/API = absent; glitch-detection = absent. | throughout |

**Action:** the three fabricated paths are retracted in §11.2. Port-target framing is re-anchored in §3. Anything §2.2 marks wrong is **never** citable downstream.

### 2.3 `OpenClawing3.md` — real tech, wrong plumbing, constitution-breaking

Against Brief-2 ("bringing in OpenCV heavy use … Vulkan and OpenGL … CUDA technology … extend capabilities of recording … every single source code file is referenced … in-depth full step by step guides to the smallest atoms level of precision"):

| Issue | Detail | Source in doc |
|---|---|---|
| All 19 cited third-party repos are **real** ✓ | OpenCV, OBS, scrcpy, vkCompViz, plthook, NVIDIA Maxine, Skyvern, … verified. | throughout |
| Invented `src/...` tree | HelixQA does not have `src/`. It has `pkg/`. 25 fabricated file paths including `src/capture/dxgi.cpp`, `src/vision/trt_engine.cpp`, `src/agent/pi-runner.ts`. | §6, §9, §11 |
| **SUDO violations** | `sudo dpkg -i`, `sudo apt-get install tensorrt`, `sudo usermod -aG input`, `sudo make install` in §14 setup steps. Directly violates the "No Sudo / No Root" mandatory constraint in `CLAUDE.md`. | §14 |
| Compile-blocking code | `namespace openclaw{}` inside a `.c` file (§6.2 L1091–1248 — C has no namespaces); undefined `BuilderFlag::kCUDA_GRAPH` (line 727 — not a TensorRT flag); `enqueueV3` misuse with `bindings_` array pattern (copy of TensorRT v2 API, invalid on v3+); `{key:'Down'}.repeat(n)` (line 2561 — a TS type error); obsolete `destroy()` methods; `namespace` references to never-declared types. | §6.2, §6.4, §7.1 |
| Incorrect scrcpy v1.x wire format | Missing `action_button` / `buttons` `uint32` fields; scrcpy's protocol has evolved past v2 with these fields present. | §6.5 |
| "Zero-copy" DXGI claim is wrong | `GetSharedHandle` returns a legacy NT handle unusable by Vulkan external-memory extensions — described path is GPU↔GPU copy, not zero-copy. | §6.1 (L1036-1037) |
| Benchmarks 3–7× optimistic | "< 2 ms template match on 1080p" on an RTX 3060: real measured values are 6–14 ms for `cv::cuda::matchTemplate` at 1080p depending on template size; "< 16 ms full pipeline end-to-end" ignores actuation and a11y verification cost. | §6.8 |
| Doc misses HelixQA invariants | Never cites llama.cpp RPC; proposes TensorRT as primary local inference (contrary to CLAUDE.md); proposes root-requiring `video`/`input` group membership; no threat model, no concurrency model across the five declared languages, no observability. | throughout |
| API / TUI coverage | Brief demanded "APIs or TUI" — appears only as an enum entry. | §4 |
| Plan sized 5–10× too small | 47 technologies in 16 weeks with no acceptance criteria per Article V category. | §12 |

**Action:** every `src/...` path cited is replaced with a real `pkg/...` path in §7. Every `sudo` line is rewritten as a user-space equivalent in §6.3. Compile-blockers are fixed in §12 game-changer additions / §7 file sketches. Benchmarks are re-anchored in §9 acceptance.

### 2.4 What to keep, fix, or discard

| Category | From OpenClawing2 | From OpenClawing3 |
|---|---|---|
| **Keep (cite downstream)** | The identification of **browser-use, Skyvern, Stagehand, UI-TARS** as relevant projects. | The list of 19 real tech repos (OpenCV, scrcpy, plthook, NVIDIA Maxine, Vulkan samples, etc.). |
| **Fix before cite** | Concrete "how it wins" claims (need benchmark URLs). | Code snippets (compile-block), sudo removal, path re-anchoring to `pkg/`. |
| **Discard** | The three fabricated internal paths; the TypeScript "OpenClaw" port-target framing. | The 16-week / 47-tech plan; the "zero-copy" DXGI claim; TensorRT as primary local path. |

---

## 3. Ground truth — HelixQA's real codebase anchor

This section replaces OpenClawing3's invented `src/...` tree with the actual HelixQA Go layout. Every future integration MUST land inside one of these directories — never in a new top-level tree.

### 3.1 Top-level layout

```
HelixQA/
├── cmd/                       # CLI entry points (helixqa, …)
├── pkg/                       # Core packages (50+ modules, all Go)
├── internal/                  # Private infrastructure (visionserver)
├── banks/                     # YAML / JSON test bank definitions
├── scripts/                   # Deployment & infrastructure helpers
├── tools/                     # Third-party tool wrappers (Appium, docker-android)
├── docs/                      # Documentation & design docs (includes openclawing/)
├── data/                      # SQLite memory.db, fixtures
├── docker/                    # Container recipes
├── .env.example               # Environment template
├── CLAUDE.md                  # Architecture & non-negotiable rules
├── ARCHITECTURE.md            # System design
├── go.mod / go.sum            # Module file (authoritative dep list)
└── Makefile / Dockerfile      # Build
```

### 3.2 Pipeline packages (Learn → Plan → Execute → Curiosity → Analyze)

- `pkg/autonomous/pipeline.go` defines `PipelineConfig`, `SessionPipeline`, `Run(ctx)`, `WithVisionProvider`, `WithChatProvider`, `WithPhaseSelector`, `WriteReport`.
- `pkg/learning/` — `KnowledgeBase`, `Screen`, `APIEndpoint`, `PlatformFeature`, `DocEntry`, `ChangeEntry`. Used in the Learn phase.
- `pkg/planning/` — `PlannedTest`, `TestPlan`, `PlanStats`, `GenerateTestPlan`, `ReconcileWithBank`, `Ranker`.
- `pkg/autonomous/coordinator.go` — `SessionCoordinator.Explore(ctx)` drives the Curiosity phase via an LLM loop.
- `pkg/analysis/` — `FindingCategory`, `FindingSeverity`, `AnalysisFinding`, `AnalyzeScreenshot`.
- `pkg/autonomous/stagnation.go` — current `StagnationDetector`. **This is the extension point for BOCPD (§5.8).**

### 3.3 LLM + vision abstractions

- `pkg/llm/provider.go` — canonical `Provider` interface: `Chat`, `Vision`, `Name`, `SupportsVision`.
- `pkg/llm/adaptive.go` — `AdaptiveProvider` fallback chain.
- `pkg/llm/phase_selector.go` — `PhaseModelSelector.SelectForPhase(phase)` with `PlanningStrategy`, `NavigationStrategy`, `AnalysisStrategy`.
- `pkg/llm/vision_ranking.go` — `rankVisionProviders` sources from `digital.vasic.llmsverifier/pkg/helixqa.VisionModelRegistry()`. **Every new grounding model (UI-TARS, OmniParser, ShowUI) must register here.**
- `pkg/llm/cost_tracker.go` — `CostTracker.RecordCall(record)`. Every new provider/ sidecar must feed this.

Concrete provider files: `pkg/llm/anthropic.go`, `openai.go`, `google.go`, `ollama.go`, `astica.go`, `bridge_provider.go`.

### 3.4 Platform executors (current)

`pkg/navigator/` hosts `ActionExecutor` and concrete implementations:

- `executor.go` — `ActionExecutor` interface (`Click`, `Type`, `Clear`, `Scroll`, `LongPress`, `Swipe`, `KeyPress`, `Back`, `Home`, `Screenshot`).
- `playwright_executor.go` — Playwright/chromedp path for web.
- `x11_executor.go` — xdotool wrapper. **To be retired in favour of libei (§5.2).**
- `api_executor.go`, `cli_executor.go`, `tvkeyboard.go`, `dual_screen.go`, `state.go`.

### 3.5 Capture & recording

- `pkg/capture/android_capture.go` — scrcpy-wrapper. **To be replaced with `pkg/bridge/scrcpy` direct protocol client (§5.1).**
- `pkg/capture/linux_capture.go` — `Xvfb`, `xwd`, `gnome-screenshot`. **To be replaced with PipeWire portal + pipewiresrc (§5.1).**
- `pkg/capture/macos_capture.go`, `windows_capture.go`.
- `pkg/session/recorder.go` — multi-platform session recorder with `Timeline`, `VideoManager`, `Screenshot`.
- `pkg/video/ffmpeg_recorder.go`, `frames.go`, `scrcpy.go`.
- `pkg/gst/pipeline.go`, `frame_extractor.go` — existing GStreamer plumbing to extend.

### 3.6 Advanced Nexus driver framework

`pkg/nexus/` already has skeletons for the very things OpenClawing3 invented. **Use these, not new trees.**

- `pkg/nexus/native/contracts/capture.go` — `CaptureSource`, `FrameData`. **Extension point for PipeWire, SCKit, WGC, scrcpy-server.**
- `pkg/nexus/native/contracts/interact.go` — `Interactor`. **Extension point for libei, enigo sidecars.**
- `pkg/nexus/native/contracts/observe.go` — `Observer`. **Extension point for Frida, eBPF, LD_PRELOAD, CDP-AX.**
- `pkg/nexus/native/contracts/vision.go` — `VisionPipeline`. **Extension point for OmniParser, UI-TARS, OpenCV.**
- `pkg/nexus/capture/{android,linux,web}/source.go` — platform capture sources.
- `pkg/nexus/interact/{android,linux,web}/interactor.go` — platform actuators.
- `pkg/nexus/observe/{cdp,ax_tree,dbus,ld_preload,plthook}/observer.go` — observation layer.
- `pkg/nexus/ai/` — `navigator.go`, `healer.go`, `predictor.go`, `generator.go`.
- `pkg/nexus/record/encoder/{nvenc,vaapi,x264}` — hardware/software encoders.
- `pkg/nexus/perf/` — `k6.go`, `run.go`, `metrics.go`.
- `pkg/nexus/observability/` — `otel_tracer.go`, `prometheus.go`.
- `pkg/nexus/orchestrator/` — `sso.go`, `s3_evidence.go`, `persistence.go`, `rbac.go`.

### 3.7 Detection, validation, reporting

- `pkg/detector/` — `Detector.Check(ctx)` with Android (ADB logcat), web (browser process), desktop (process monitor), dual-display routing detector, LLM crash analyser.
- `pkg/validator/validator.go` — `ValidateStep(step, screenshot)`.
- `pkg/reporter/reporter.go` — `WriteReport(report, dir)` with Markdown / HTML / JSON.
- `pkg/ticket/` — `Ticket`, `LLMSuggestedFix`, `VideoReference`, `Markdown()`.

### 3.8 Bridges registry

- `pkg/bridges/registry.go` — `DiscoverTools(runner)` scans PATH. **Every new sidecar binary (helixqa-capture-linux, helixqa-input, helixqa-frida) must be registered here.**

### 3.9 Memory store

- `pkg/memory/store.go` — SQLite-backed persistence (sessions, tests, findings, screenshots, metrics, knowledge, coverage).

### 3.10 Constitutional invariants that bind every new component

These are verbatim rules from `CLAUDE.md` Articles V–VII that every new component MUST satisfy. Any design that conflicts loses.

1. **Zero unfinished work** — no TODOs, FIXMEs, empty implementations, silent error swallows, fake data, panic-prone `unwrap()`, or empty catch blocks.
2. **No sudo / no root** — all operations at local-user level. Capability granted to sidecar binaries by the *operator*, not by HelixQA's runtime.
3. **`.devignore` devices are forbidden** — every ADB operation must check the device model and abort on match.
4. **Container-first for QA** — bare-metal only for rapid iteration.
5. **HTTP/3 + Brotli** — for all Catalogizer-facing traffic.
6. **100 % test coverage across 10 categories** — unit, integration, E2E, full automation, stress, security, DDoS, benchmarking, challenges, HelixQA. Shipping prohibited until all green.
7. **Universal solution** — HelixQA never modifies the app under test.
8. **Real-time log monitoring** — every session streams logs.
9. **Host resource limits** — 30–40 % max; challenges sequential, not parallel.
10. **Vision-driven contract** — screenshot → LLM analysis → action decision. If vision providers are unavailable, the phase **skips**, never fakes results.
11. **Executable actions in banks** — no prose descriptions.
12. **llama.cpp RPC is the primary local backend** — cloud providers complement, not replace.
13. **Every fix = four artefacts** — unit/integration test + `fixes-validation` entry + HelixQA bank entry + challenge.

---

## 4. Re-framed problem statement

Autonomous UI driving of any application, smoothly and flawlessly, reduces to four pillars. Every subsystem in §5 maps to one or more of them.

### 4.1 The four pillars

1. **Perceive** — obtain a faithful, current, low-latency view of the screen. Spans capture (§5.1), perception/vision (§5.4–§5.6).
2. **Decide** — given the view, pick an action. Spans grounding models, reasoning, planning (§5.6, §5.7).
3. **Act** — execute the action on the platform. Spans input/actuation (§5.2), browser control (§5.2.4).
4. **Verify** — confirm the action had its intended effect. Spans accessibility tree (§5.3), stagnation detection (§5.8), visual regression (§5.9).

No single model, library, or service covers all four. HelixQA's value is the *glue* — the phase graph, the evidence store, the bank-driven regression loop — and that glue already exists in `pkg/autonomous/`, `pkg/session/`, `pkg/memory/`. **OpenClawing3's principal mistake was proposing replacement components without respecting the glue.** §6 defines the glue explicitly.

### 4.2 Coverage surface

HelixQA must operate on **Web, Android, Android TV, Desktop (Linux + macOS + Windows), iOS, APIs, and TUI**. The following table states *which pillar* each surface needs from *which component*. Sections in the right column show where the design is defined.

| Surface | Perceive | Act | Verify | Sections |
|---|---|---|---|---|
| Web | CDP `Page.captureScreenshot` + CDP `Accessibility.getFullAXTree` | go-rod / Playwright / CDP input | CDP AX tree ↔ pixelmatch | §5.1.4, §5.2.4, §5.3.5 |
| Android / Android TV | scrcpy-server H.264 stream | scrcpy control protocol + UiAutomator2 | UiAutomator2 AX tree + Perfetto ANR | §5.1.3, §5.2.2, §5.3.4 |
| iOS | idb describe-ui + ReplayKit (real device) | Appium XCUITest / idb tap | idb describe-ui | §5.1.5, §5.2.5, §5.3.6 |
| Desktop Linux | xdg-desktop-portal ScreenCast + pipewiresrc | libei via RemoteDesktop portal + /dev/uinput fallback | AT-SPI2 tree over godbus | §5.1.1, §5.2.1, §5.3.3 |
| Desktop macOS | SCKit sidecar | enigo sidecar / CGEventPost | AXUIElement sidecar | §5.1.2 (mac), §5.2.3, §5.3.2 |
| Desktop Windows | WGC sidecar (C++/WinRT) + DXGI-DD fallback | SendInput (enigo sidecar) | UI Automation via go-ole | §5.1.2 (win), §5.2.3, §5.3.1 |
| APIs | n/a (no UI) | HTTP client + contract tests | Response schema + Frida/eBPF correlation | §5.11, §6.4 |
| TUI | ANSI escape parser (new: `pkg/capture/tui/`) | pty stdin injection | screen buffer diff (SSIM over character grid) | §5.1.6, §5.2.6, §5.4.4 |

---

## 5. Technology deep dive

Each subsection follows the same shape: **what / why / top 3 / Go integration / pitfalls / HelixQA landing point**. For every recommendation, the canonical URL is given inline and the landing-point `pkg/` directory is named.

### 5.1 Capture layer

#### 5.1.1 Linux desktop (Wayland-first)

Today HelixQA uses `ffmpeg -f x11grab -i :0.0` (see `pkg/capture/linux_capture.go`). On modern distributions (Fedora 41+, Ubuntu 25.04+, KDE Plasma 6, GNOME 46+) Wayland is the default; `x11grab` only works when rooted in XWayland, and **NVFBC is effectively dead on Wayland** (see [NVIDIA Capture SDK](https://developer.nvidia.com/capture-sdk)).

**Primary path — xdg-desktop-portal ScreenCast + GStreamer `pipewiresrc`.** Portal documentation: [ScreenCast portal spec](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html). GStreamer source: [pipewiresrc](https://gstreamer.freedesktop.org/documentation/pipewire/pipewiresrc.html). Works on GNOME, KDE, wlroots (Sway, Hyprland). Typical latency 2–8 ms/frame with DMA-BUF; 3–8 % CPU at 1080p60.

**Zero-copy tier (QA hosts only).** A `kmsgrab + scale_vaapi` sidecar: latency 1–4 ms, 2–6 % CPU. See [FFmpeg kmsgrab](https://github.com/FFmpeg/FFmpeg/blob/master/libavdevice/kmsgrab.c). **Requires `cap_sys_admin`** — granted by the operator once, not at runtime.

**Fallback tier (X11 / XWayland).** xcb-shm + XDamage. Only when the session is X11-rooted.

**Go integration recipe.**

- `pkg/capture/linux/portal.go` — pure Go. Uses `github.com/godbus/dbus/v5` to speak `org.freedesktop.portal.ScreenCast`: `CreateSession → SelectSources → Start`. The portal returns a PipeWire file descriptor + stream node id.
- `pkg/capture/linux/pipewire.go` — `exec.Cmd` launches `helixqa-capture-linux` (GStreamer pipeline: `pipewiresrc fd=N path=NODE ! videoconvert ! x264enc tune=zerolatency ! h264parse ! appsink`). FD is passed via `SysProcAttr.ExtraFiles`.
- `pkg/capture/linux/kmsgrab.go` — capability-granted sidecar; invoked only if `HELIX_LINUX_KMSGRAB=1` *and* the binary exists.

**Sources worth knowing.** [xdg-desktop-portal-wlr compatibility](https://github.com/emersion/xdg-desktop-portal-wlr/wiki/Screencast-Compatibility); [Phoronix Wayland-Share-HowTo](https://www.phoronix.com/news/Wayland-Share-HowTo-Pipe-XDG).

**Pitfalls.** Do not statically link GStreamer into the Go binary. Do not run the portal handshake on every frame — cache the session. Do not depend on NVFBC on Wayland.

**Landing point.** `pkg/capture/linux/`, `pkg/nexus/capture/linux/source.go`, `cmd/helixqa-capture-linux/`.

#### 5.1.2 macOS and Windows desktop

**macOS — ScreenCaptureKit only** ([SCKit docs](https://developer.apple.com/documentation/screencapturekit/)). `CGDisplayStream` is deprecated since macOS 14. `CGPreflightScreenCaptureAccess` is deprecated since macOS 15.1 ([xcap issue #160](https://github.com/nashaofu/xcap/issues/160)). Production path: a Swift sidecar `helixqa-capture-darwin` using `SCStreamOutput` + `AsyncStream<CMSampleBuffer>`, emitting NV12 or H.264 on stdout. Only the sidecar needs the `com.apple.security.screen-recording` entitlement — keeps HelixQA's Go host free of TCC grants.

**Windows — Windows.Graphics.Capture (WGC) primary, DXGI-DD fallback.** WGC supports multi-GPU + HDR + Game Bar ([MS Learn](https://learn.microsoft.com/en-us/windows/uwp/audio-video-camera/screen-capture)); DXGI-DD is lower overhead when the display and GPU match but fails on Optimus ([OBS: WGC vs DXGI](https://obsproject.com/forum/threads/windows-graphics-capture-vs-dxgi-desktop-duplication.149320/)). Production path: a C++/WinRT sidecar `helixqa-capture-win.exe` using `Direct3D11CaptureFramePool`, writing frames over a named pipe.

**Go integration recipe.**

- `pkg/capture/darwin/sckit.go` — `exec.Cmd` to the Swift sidecar; reads length-prefixed frames from stdout.
- `pkg/capture/windows/wgc.go` — `exec.Cmd` to the WinRT sidecar; named-pipe reader.
- `pkg/capture/windows/dxgi.go` — pure Go via `github.com/go-ole/go-ole` for stable same-GPU setups (fallback).

**Landing point.** `pkg/capture/{darwin,windows}/`, `pkg/nexus/capture/{darwin,windows}/source.go`, `cmd/helixqa-capture-darwin/`, `cmd/helixqa-capture-win/`.

#### 5.1.3 Android and Android TV — scrcpy-server direct protocol

scrcpy ([Genymobile/scrcpy](https://github.com/Genymobile/scrcpy)) is the de-facto ADB video path. Today HelixQA uses the **scrcpy binary** and parses its output (`pkg/capture/android_capture.go`). A better answer, used by modern QA frameworks: **run `scrcpy-server.jar` directly via ADB forward** and speak its documented binary protocol ([scrcpy/doc/develop.md](https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md)).

The protocol as of v3 (verified against the repo):

- Server is `app_process`-hosted Java running as shell UID 2000 on the device.
- Connection direction is **inverted**: the desktop client listens; the server connects out.
- Three sockets per session: `video`, `audio`, `control`.
- Video: raw H.264 NAL units, per-packet header with PTS + config flag.
- Control: keyboard/touch/text/clipboard events, documented binary frame format.

**Why HelixQA cares.** Removes the dependency on the `scrcpy` binary, unlocks audio (for media-playback verification), gains clipboard capture, and lets the Go client multiplex multiple device streams cleanly. The control socket doubles as the input path (§5.2.2).

**Go integration recipe.**

- `pkg/bridge/scrcpy/protocol.go` — wire-format decoder for v3 (video, audio, control).
- `pkg/bridge/scrcpy/server.go` — ADB forward + `app_process` launch + socket accept.
- `pkg/bridge/scrcpy/devguard.go` — `.devignore` enforcement (`getprop ro.product.model` before opening the control socket; abort on match).
- `pkg/capture/android/scrcpy.go` — normalises v3 stream into `pkg/capture/frames.Frame`.

**Fallback.** `adb exec-out screencap -p` for low-FPS sampling (unchanged).

**Pitfalls.** The scrcpy protocol *does* evolve — pin a specific server JAR version (shipped inside HelixQA's container) and test upgrades in `fixes-validation`. Do not rely on host `scrcpy` being installed.

**Landing point.** `pkg/capture/android/`, `pkg/bridge/scrcpy/`.

#### 5.1.4 Web — CDP `Page.captureScreenshot` and `Page.startScreencast`

For web, the existing chromedp / rod / Playwright paths are fine for screenshots. For continuous capture, use `Page.startScreencast` (part of CDP) — it streams JPEG frames at the requested FPS and is the path [browser-use migrated to](https://browser-use.com/posts/playwright-to-cdp) for low-latency vision.

**Go integration.** `pkg/nexus/capture/web/source.go` extends to use `Page.startScreencast`. The existing `pkg/navigator/playwright_executor.go` remains for interop with teams that prefer Playwright.

#### 5.1.5 iOS — idb (Simulator) and ReplayKit (device)

**Simulator** — [facebook/idb](https://github.com/facebook/idb) gRPC API. Supports `record-video`, `tap`, `swipe`, `install`, `describe-ui` (a11y tree).

**Real device** — ReplayKit broadcast-upload extension ([ReplayKit docs](https://developer.apple.com/documentation/replaykit)). Requires the app team to embed a signed `HelixQABroadcastExtension.framework` that forwards `CMSampleBuffer` to a local TCP sink. **This is the only unattended real-device path.** Apple does not expose an unsigned off-process screen-capture API.

**Go integration.** `pkg/bridge/idb/` — gRPC client. `pkg/capture/ios/replaykit_ext.md` — operator handoff doc (SDK integration, not runtime code).

#### 5.1.6 TUI — ANSI escape parser

Missing from every prior OpenClawing document. TUI apps (`htop`, `docker stats`, `k9s`, `helix`, etc.) are a first-class surface.

**Design.** A new `pkg/capture/tui/` reads from a `pty` and parses ANSI escape sequences into a character-grid buffer. Each cell carries `{char, fg, bg, attrs}`. Perception diffs compare cell grids, not pixels; dHash/SSIM are unnecessary. Libraries worth evaluating: [ConradIrwin/go-pty](https://github.com/ConradIrwin/go-pty), [gdamore/tcell](https://github.com/gdamore/tcell) (grid model reference). A sub-package `pkg/capture/tui/ansi.go` does the parsing.

**Pitfalls.** TUI apps frequently use raw mode + alternate screen buffer; the pty must be opened with the right flags. Terminal resize events must be honoured (SIGWINCH).

### 5.2 Input / actuation layer

#### 5.2.1 Linux — libei (Wayland-correct)

**Current state:** `pkg/navigator/x11_executor.go` uses `xdotool`, which is X11-only and will be unavailable on pure Wayland sessions.

**Primary path — libei via RemoteDesktop portal.** libei is Red Hat's Wayland-correct input injection library ([libei repo](https://gitlab.freedesktop.org/libinput/libei); [Who-T's announcement](http://who-t.blogspot.com/2022/12/libei-opening-portal-doors.html)). Flow:

1. `org.freedesktop.portal.RemoteDesktop.CreateSession` → portal prompt.
2. `SelectDevices(["keyboard","pointer"])` → `Start`.
3. The portal returns a UNIX-socket FD; the process writes EI binary-protocol frames to send key and pointer events.

`wtype` is *blocked on GNOME* because GNOME does not implement `virtual-keyboard-unstable-v1` ([KDE discuss](https://discuss.kde.org/t/xdotool-replacement-on-wayland/7242)) — so libei is the only portable answer.

**Fallback — `/dev/uinput`.** Kernel-level, pure Go. Requires a udev rule `SUBSYSTEM=="misc", KERNEL=="uinput", GROUP="helixqa", MODE="0660"` granted once by the operator at install time. **No runtime sudo.** Code: `pkg/navigator/linux/uinput.go` using `syscall.Syscall(SYS_IOCTL, …)`.

**Deprecated — xdotool.** `pkg/navigator/x11_executor.go` is retired, retained only for X11-only sessions as a `-tags x11legacy` build.

**Pitfalls.** libei support is present on GNOME 46+, KDE Plasma 6+, Hyprland. Older distributions will not have the portal — fall back to `/dev/uinput`. Never use `wtype` on GNOME.

**Landing point.** `pkg/navigator/linux/libei.go`, `pkg/navigator/linux/uinput.go`, `pkg/nexus/interact/linux/interactor.go`.

#### 5.2.2 Android and Android TV — scrcpy control socket + UiAutomator2

**Primary (coordinate / gesture path) — scrcpy control socket.** Shares the socket multiplex from §5.1.3. Control frames documented in the scrcpy protocol; support touch (down/up/move), key events, text, clipboard set/get.

**Primary (semantic / verified path) — UiAutomator2.** [Appium UiAutomator2 driver](https://github.com/appium/appium-uiautomator2-driver) already ships with HelixQA. Every click is followed by an a11y-tree query to verify the intended element is now focused/enabled/selected. This satisfies the "executable actions, verified outcomes" rule in `CLAUDE.md`.

**Input driver (root-only, deliberately NOT used).** `/dev/input/eventN` via `getevent`/`sendevent` requires root on most OEMs. HelixQA does not need it and will not use it.

**Landing point.** `pkg/navigator/android/scrcpy_control.go`, `pkg/navigator/android/uia2_http.go`.

#### 5.2.3 macOS and Windows — enigo sidecar (Rust)

[enigo-rs/enigo](https://github.com/enigo-rs/enigo) is the most-maintained cross-platform input crate; handles TCC consent on macOS and SendInput on Windows ([enigo CHANGES](https://github.com/enigo-rs/enigo/blob/main/CHANGES.md)). Ship one `helixqa-input` binary per OS; control over stdin JSON commands (`{"type":"click","x":100,"y":200}`). HelixQA host stays `CGO_ENABLED=0`.

**Landing point.** `pkg/navigator/darwin/enigo_sidecar.go`, `pkg/navigator/windows/enigo_sidecar.go`, `cmd/helixqa-input/`.

#### 5.2.4 Web — go-rod as default, Playwright for non-Chromium

Today HelixQA uses Playwright via `pkg/navigator/playwright_executor.go`. For Chromium this is overkill: [go-rod](https://github.com/go-rod/go-rod) is pure Go, ships a pinned Chromium, is faster (on-demand JSON decode vs chromedp's eager decode), and exposes the full CDP surface. Playwright remains first-class for Firefox and WebKit.

**Landing point.** `pkg/navigator/web/rod.go` (new, default), `pkg/navigator/web/playwright.go` (existing, Firefox/WebKit only), `pkg/navigator/web/cdp_ax.go` (Accessibility domain queries).

#### 5.2.5 iOS — WDA (device) or idb (simulator)

`pkg/navigator/ios/wda.go` wraps `appium/WebDriverAgent`. `pkg/navigator/ios/idb.go` wraps Facebook idb. The `.devignore` + device-guard pattern from Android applies equally.

#### 5.2.6 TUI — pty stdin injection

Paired with §5.1.6. Actions are `"type: abc"`, `"key: ctrl+c"`, `"resize: 80x24"`, `"paste: ..."`. Implemented in `pkg/navigator/tui/pty.go`.

### 5.3 Accessibility-tree layer (deterministic target resolution)

**Why it matters.** A button's AX role/name does not jitter with anti-aliasing; it survives theme changes and screen scaling. When both the vision model and the AX tree agree on a target, the test is deterministic. When they disagree, HelixQA pauses and records evidence. This pattern eliminates a large class of false-pass / false-fail noise.

**Unified tree type.** `pkg/nexus/observe/axtree/node.go`:

```go
type Node struct {
    Role     string
    Name     string
    Value    string
    Bounds   Rect
    Enabled  bool
    Focused  bool
    Selected bool
    Children []*Node
    Platform string // "windows" | "darwin" | "linux" | "android" | "ios" | "web"
    RawID    string // platform-native id for action targeting
}

type Snapshotter interface {
    Snapshot(ctx context.Context) (*Node, error)
}
```

#### 5.3.1 Windows — UI Automation via go-ole

COM-based API ([MS Learn overview](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-uiautomationoverview)). Go path: [go-ole/go-ole](https://github.com/go-ole/go-ole) + hand-written `IUIAutomation` vtable wrappers. Pragmatic escape hatch: a C# sidecar using [FlaUI](https://github.com/FlaUI/FlaUI).

**Landing point.** `pkg/nexus/observe/axtree/windows.go`.

#### 5.3.2 macOS — AXUIElement via Swift sidecar

`AXUIElement` is Swift/Objective-C. A small Swift CLI `helixqa-axtree-darwin` walks the tree from `AXUIElementCreateApplication` and emits JSON. Go reads stdout.

**Landing point.** `pkg/nexus/observe/axtree/darwin.go`, `cmd/helixqa-axtree-darwin/`.

#### 5.3.3 Linux — AT-SPI2 over a11y bus

AT-SPI2 runs on a **separate D-Bus** (the "a11y bus") to avoid flooding the session bus ([at-spi2-core README](https://github.com/GNOME/at-spi2-core/blob/main/bus/README.md)). Bootstrap:

1. `org.a11y.Bus.GetAddress` on the session bus.
2. Reconnect to the returned address.
3. Walk from `/org/a11y/atspi/accessible/root` via `Accessible.GetChildren`, `GetRole`, `GetName`, etc.

Pure Go using [godbus/dbus](https://github.com/godbus/dbus). **Landing point.** `pkg/nexus/observe/axtree/linux.go`.

#### 5.3.4 Android — UiAutomator2 via HTTP

Already integrated. `UiAutomation.getRootInActiveWindow()` → `AccessibilityNodeInfo` tree, served by the UiAutomator2 Appium driver.

**Landing point.** `pkg/nexus/observe/axtree/android.go` (wraps existing `pkg/navigator/android/uia2_http.go`).

#### 5.3.5 Web — CDP Accessibility domain

`Accessibility.getFullAXTree` / `Accessibility.queryAXTree` return a deterministic tree keyed by backend node id. Both go-rod and chromedp expose the domain.

**Landing point.** `pkg/nexus/observe/axtree/web.go`.

#### 5.3.6 iOS — idb describe-ui

`idb describe-ui` returns a JSON tree. `pkg/nexus/observe/axtree/ios.go`.

### 5.4 Classical vision (OpenCV spine)

Brief-2 explicitly asked for "heavy use" of OpenCV. The audit found OpenClawing3 *mentioned* OpenCV but wired it into an invented C++ tree. The real landing point is `pkg/opencv/` (already present via `gocv.io/x/gocv`) and `pkg/vision/detection/` (ORB already implemented).

#### 5.4.1 Template matching

`cv::matchTemplate` with `TM_CCOEFF_NORMED` for button/icon presence confirmation. gocv: `gocv.MatchTemplate(image, templ, &result, gocv.TmCcoeffNormed, mask)`. Use for "is the logo still on screen" regression checks; never for finding clickable targets (too brittle under scaling/AA).

#### 5.4.2 Feature matching — ORB / AKAZE

Already partially in `pkg/vision/detection/orb.go`. ORB is rotation-invariant, scale-robust, fast. Use for cross-device "same screen" matching (phone-vs-TV layout) in conjunction with DreamSim.

#### 5.4.3 Optical flow — DIS on CPU, NVOF 2.0 on GPU

Two paths:

- **CPU fast path — `cv::DISOpticalFlow`** ([OpenCV contrib sample](https://github.com/opencv/opencv_contrib/blob/master/modules/optflow/samples/optical_flow_evaluation.cpp)). 4–8 ms on 720p on a modern core. Exposed via gocv. Use: "is the list actually scrolling or is the app frozen".
- **GPU high-fidelity path — `cv::cuda::NvidiaOpticalFlow_2_0`** ([OpenCV docs](https://docs.opencv.org/4.x/db/d70/classcv_1_1cuda_1_1NvidiaOpticalFlow__2__0.html)). Hardware-accelerated on Turing+ OFA. **Not exposed in gocv** — wrap in the `qa-vision-infer` C++ sidecar (§5.5.3) and hand results back over SHM.

#### 5.4.4 Text localization — EAST

`cv::dnn::TextDetectionModel_EAST` ([OpenCV docs](https://docs.opencv.org/4.x/d8/ddc/classcv_1_1dnn_1_1TextDetectionModel__EAST.html)). Locate candidate clickable text regions *before* sending to the VLM — the LLM prompt then carries bounding boxes, saving tokens and sharpening accuracy. Also usable for MSER+SWT on well-structured UI chrome.

**Landing point.** `pkg/vision/text/east.go`, `pkg/vision/detection/orb.go` (existing), `pkg/vision/flow/dis.go`, `pkg/vision/template/match.go`.

#### 5.4.5 TUI differential

For the TUI surface (§5.1.6), the comparator is cell-grid diff, not pixel diff. `pkg/vision/tui/grid_diff.go` — cheap and deterministic.

### 5.5 GPU compute (CUDA / Vulkan / OpenGL)

Brief-2 asked explicitly for heavy use of CUDA, Vulkan, OpenGL on the RTX card. OpenClawing3 proposed this, but in-tree in an invented C++ layout, using `sudo` to install drivers, and with compile-blocking code snippets. Here is the correct design.

#### 5.5.1 Architecture — sidecar boundary

Two dedicated sidecars, each owning its GPU context:

- `qa-vision-infer` (C++) — owns CUDA + TensorRT + NPP + OpenCV-CUDA. Hosts UI-TARS (if compiled to TRT) or OmniParser (YOLO-v8 + Florence-2) engines.
- `qa-video-decode` (C) — FFmpeg + NVDEC; pushes decoded NV12 frames via ring-buffered SHM.

Go talks to both over UDS gRPC. Frame metadata goes over gRPC; pixel payloads travel on file-descriptor-passed `memfd`. **HelixQA Go binary never links CUDA.**

#### 5.5.2 CUDA 12.x + TensorRT-RTX + CUDA Graphs

[TensorRT for RTX docs](https://docs.nvidia.com/deeplearning/tensorrt-rtx/latest/index.html). CUDA Graphs guide: [TensorRT-RTX CUDA Graphs](https://docs.nvidia.com/deeplearning/tensorrt-rtx/1.2/inference-library/work-with-cuda-graphs.html). **TensorRT-RTX 1.2 supports Parallel CUDA Graph Capture**, which is how we overlap analyze-frame-N with capture-frame-N+1 cheaply.

**Deployment.** Container base `nvcr.io/nvidia/cuda:12.x-runtime-ubuntu24.04`. Never install a CUDA toolkit inside the container; depend on the host driver + NVIDIA Container Toolkit + Podman CDI (`nvidia.com/gpu=all`). Zero sudo.

**Engine versioning.** TensorRT engines are driver-version-pinned. Rebuild when the host driver's major version changes. Store the engine + driver version in the QA-session archive for reproducibility.

#### 5.5.3 NPP + OpenCV-CUDA in the sidecar

NPP ([NVIDIA NPP docs](https://docs.nvidia.com/cuda/npp/index.html)) ships with CUDA. Use for on-device preprocessing (BGR→grayscale, resize, histogram) in the same CUDA stream that feeds TensorRT — zero host round-trip. Pair with OpenCV-CUDA for cv::cuda::NvidiaOpticalFlow_2_0, cv::cuda::matchTemplate.

**RTX 3060 8 GB budget.** Reserve 2 GB for decode, 3 GB for one TRT engine + workspace, 2 GB headroom, 1 GB for graphics / Xvfb. Do not load two large engines simultaneously.

#### 5.5.4 NVDEC via FFmpeg SDK 13

[FFmpeg with NVIDIA GPU (SDK 13)](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.0/ffmpeg-with-nvidia-gpu/index.html). RTX 3060: 5th-gen NVDEC (HEVC/AV1 decode, H.264/HEVC encode; **no AV1 encode** — that starts at Ada). Command-line path from the `qa-video-decode` sidecar: `ffmpeg -hwaccel cuda -hwaccel_output_format cuda -i <src> -f rawvideo -pix_fmt nv12 pipe:1`. Go reads NV12 from stdout or from a `memfd` ring.

#### 5.5.5 Vulkan compute — cross-vendor future-proofing

**Not a day-one win on NVIDIA** — published benchmarks show naive Vulkan compute is ~30× slower than CUDA, well-tuned code ~3× ([NVIDIA forum comparison](https://forums.developer.nvidia.com/t/cuda-vs-vulkan-performance-difference/238633)). For HelixQA's workload (pHash, SSIM, small diff kernels at 60 fps), Vulkan is *sufficient* on NVIDIA but *optimal* when HelixQA eventually runs on AMD / Intel Arc QA hosts.

**Build blocks.** [charles-lunarg/vk-bootstrap](https://github.com/charles-lunarg/vk-bootstrap) (instance/device/queue boilerplate, MIT). [VulkanMemoryAllocator](https://github.com/GPUOpen-LibrariesAndSDKs/VulkanMemoryAllocator) (VMA, MIT) — use `vmaImportVulkanFunctionsFromVolk` for volk interop. Synchronisation: **VK_KHR_synchronization2 + timeline semaphores** — signal N++ when a frame's compute completes; Go waits on the timeline value over a channel.

**Compile-time layer.** Validation layers cost 20–50 % throughput. Gate behind `-tags vkvalidate`.

**Implementation home.** A second C++ sidecar `qa-vulkan-compute`, only enabled under `HELIX_VULKAN=1`. Not required for Phase 4; target Phase 5 PoC.

#### 5.5.6 Vulkan Video — optional cross-vendor decode

`VK_KHR_video_decode_queue` / `VK_KHR_video_encode_queue` supported since NVIDIA driver 535.43.22 ([wccftech](https://wccftech.com/nvidia-first-to-offer-driver-support-new-vulkan-h-265-h-264-video-encode-extensions/)). NVIDIA's in-depth [Vulkan Video blog](https://developer.nvidia.com/blog/gpu-accelerated-video-processing-with-nvidia-in-depth-support-for-vulkan-video/). **Decision rule.** NVIDIA production path is FFmpeg + NVDEC; Vulkan Video is only a win on non-NVIDIA or when unifying decode + compute + present in one pipeline.

#### 5.5.7 OpenGL / EGL headless

Many containers have no display server, yet the agent still needs to render diff overlays or grab frames. **EGL pbuffer / surfaceless context** ([NVIDIA blog: EGL Eye](https://developer.nvidia.com/blog/egl-eye-opengl-visualization-without-x-server/)). PBO async readback ([song ho's PBO tutorial](https://www.songho.ca/opengl/gl_pbo.html), [NVIDIA forum: avoiding glFinish](https://forums.developer.nvidia.com/t/glreadpixels-to-pbo-avoiding-implicit-glfinish/54211)). CUDA↔OpenGL interop via `cudaGraphicsGLRegisterImage` gives the "screen capture → analyze" path *zero host copies* ([3dgep tutorial](https://www.3dgep.com/opengl-interoperability-with-cuda/)). Container ENV: `NVIDIA_DRIVER_CAPABILITIES=graphics,compute,utility,video,display`.

### 5.6 GUI-grounding foundation models (2024–2026)

OpenClawing2 mentioned UI-TARS. OpenClawing3 proposed generic VLMs. Neither identified the correct 2026 stack for HelixQA.

#### 5.6.1 The 2026 landscape (ranked for HelixQA)

| Model | Weights | Params | Key number | License | URL |
|---|---|---|---|---|---|
| **UI-TARS-1.5-7B** | [HF ByteDance-Seed/UI-TARS-1.5-7B](https://huggingface.co/ByteDance-Seed/UI-TARS-1.5-7B) | 7B | **94.2 % ScreenSpot-V2, 61.6 % ScreenSpot-Pro** | Apache-2.0 | [bytedance/UI-TARS](https://github.com/bytedance/UI-TARS) |
| **OmniParser v2** | [HF microsoft/OmniParser-v2.0](https://huggingface.co/microsoft/OmniParser-v2.0) | YOLO-v8 + Florence-2 | **39.5 % ScreenSpot-Pro** (parser, feeds any LLM) | MIT (code) | [microsoft/OmniParser](https://github.com/microsoft/OmniParser) |
| **ShowUI-2B** | [HF showlab/ShowUI-2B](https://huggingface.co/showlab/ShowUI-2B) | 2B | **75.1 % zero-shot ScreenSpot** | MIT | [showlab/ShowUI](https://github.com/showlab/ShowUI) |
| **OS-Atlas (4B / 7B)** | [OS-Copilot on HF](https://huggingface.co/OS-Copilot) | 4B / 7B | SOTA ScreenSpot-V2 at release (ICLR'25) | Apache-2.0 | [OS-Copilot/OS-Atlas](https://github.com/OS-Copilot/OS-Atlas) |
| **UGround-V1 (2B / 7B)** | [osunlp on HF](https://huggingface.co/osunlp) | 2B / 7B | +20 % vs prior SOTA on ScreenSpot avg | Apache-2.0 | [OSU-NLP-Group/UGround](https://github.com/OSU-NLP-Group/UGround) |
| **Claude Computer Use (Sonnet 5)** | closed | API | **88.3 % OSWorld-Verified (self-reported)**; Mythos Preview 79.6 % on leaderboard | commercial | [Anthropic news](https://www.anthropic.com/news/3-5-models-and-computer-use) |

#### 5.6.2 HelixQA's choice — UI-TARS-1.5-7B primary, OmniParser v2 secondary, ShowUI-2B fallback

**Primary — UI-TARS-1.5-7B on llama.cpp RPC.** Fits a 24 GB consumer GPU as GGUF Q4 / Q5; parses screenshots and directly emits `{"action":"click","coords":[x,y]}` JSON or higher-level semantic actions. Drops into `PhaseModelSelector` under `NavigationStrategy`. **Coordinate outputs must be cross-validated against the AX tree** (§5.3) before execution — this preserves the "no hardcoded coordinates" rule: the model generates coordinates online from vision, HelixQA verifies them against a11y before acting. Coordinates are never committed to banks.

**Secondary — OmniParser v2 sidecar.** YOLO-v8 detects all interactables; Florence-2 captions them; the parser emits a JSON "set-of-mark" list. *Any* VLM (Claude, Qwen2.5-VL, local Llava) can then consume the marks. This is the right path when the navigator model changes per-test or per-phase and we want a consistent element index.

**Fallback — ShowUI-2B.** 2B params, <6 GB VRAM, 75 % ScreenSpot zero-shot. Ideal when the llama.cpp RPC cluster is saturated or a low-resource worker node is all that's available.

**Cloud — Claude Computer Use / GPT-5.x CUA.** Comparator / golden. Gated behind API-key env vars; never a hard dependency.

#### 5.6.3 Integration surface

- All HF / Apache-2.0 models serve through **llama.cpp server** (OpenAI-compatible REST) in GGUF form, or **vLLM** / **SGLang** for heavier hosts. HelixQA's existing `AdaptiveProvider` already speaks OpenAI-compatible REST.
- OmniParser v2 runs as a Python sidecar (`helixqa-omniparser`). Go calls REST; response is set-of-mark JSON.
- Coordinates from any model flow through `pkg/navigator.Action{Target: ax_node|rect|text}` → AX-tree reconciliation → actuation.

#### 5.6.4 Research-only items to avoid

- **Ferret-UI 2 / Ferret-UI Lite** — Apple has not released weights. Do not cite as available.
- **UI-TARS-2 72B** — reference only; 2×A100 required.
- **MAI-UI 235B** — multi-GPU fp8 only.

#### 5.6.5 Registration into HelixQA

Every new grounding model must:

1. Add a `ProviderConfig` entry to `pkg/llm/providers_registry.go`.
2. Register scoring metadata in `digital.vasic.llmsverifier/pkg/helixqa.VisionModelRegistry()` — so `vision_ranking.go` picks it correctly per phase.
3. Add a bank entry exercising it (`banks/grounding-verification.yaml`).
4. Add a `fixes-validation` regression entry if the model has a known pitfall.

### 5.7 Agentic orchestration & planning

#### 5.7.1 LangGraph phase graph

[langchain-ai/langgraph](https://github.com/langchain-ai/langgraph). Models HelixQA's Learn → Plan → Execute → Curiosity → Analyze as a *stateful graph with explicit nodes and edges*. Supports deterministic replay and checkpointing — matches the "pause on ANR/crash" rule. Runs as a Python sidecar; HelixQA Go speaks to it over gRPC. Phase events are stored in `pkg/memory/store.go` alongside session metadata.

**Why not an in-Go graph?** Existing `pkg/autonomous/pipeline.go` is linear; converting it to a graph is a large Go refactor. LangGraph's Python ecosystem (checkpointers, tracers) gives 90 % of the value for 10 % of the cost, and the sidecar boundary keeps the Go host clean.

**Landing point.** `cmd/helixqa-langgraph/`, `pkg/bridge/langgraph/`.

#### 5.7.2 browser-use for the web surface slice

[browser-use/browser-use](https://github.com/browser-use/browser-use) + [browser-use/browser-harness](https://github.com/browser-use/browser-harness). Self-healing CDP-level harness; thousands of production users. Integrates as a sidecar under `cmd/helixqa-browser-use/` for the web platform worker in `pkg/autonomous/worker.go`. Prompts are driven by HelixQA's own `NavigationStrategy`.

#### 5.7.3 SmolAgents code-agent for the Curiosity phase

[huggingface/smolagents](https://github.com/huggingface/smolagents). When Curiosity needs ad-hoc logic ("fill this form then diff DB state"), a code-agent is cleaner than a JSON-tool schema. Runs sandboxed inside a dedicated container; exposes results over HTTP. Only enabled under `HELIX_CURIOSITY_CODE_AGENT=1`.

### 5.8 Stagnation / change-point detection

OpenClawing3 proposed simple frame-diff thresholds. The right tool is **Bayesian Online Change-Point Detection (BOCPD)**.

#### 5.8.1 The pipeline

```
Frame ──► dHash-64 (goimagehash)         # 1 ms CPU
         │
         ├─► Hamming distance to t-1  ──► s_t ──► BOCPD           ──► "stuck" posterior
         └─► Hamming distance to t-N  ──► drift signal (slow)
                                       │
                                       └─► if posterior(runlength ≥ 10 s) > 0.9:
                                           └─► SSIM on 480p luma  # 3 ms CPU
                                               └─► if SSIM > 0.99: emit STUCK event
                                                   └─► DreamSim (GPU) confirms
```

- **Tier 1** — [corona10/goimagehash](https://github.com/corona10/goimagehash) dHash-64, <1 ms CPU, native Go.
- **Tier 2** — SSIM via gocv on 480p grayscale, 3–5 ms CPU.
- **Tier 3** — [DreamSim](https://github.com/ssundaram21/dreamsim) on GPU (Triton-hosted), 30–100 ms. Runs only on suspected-stagnation segments. 96 % agreement with human judges — this is the tiebreaker the current detector lacks.

**BOCPD implementations.** Go port [dtolpin/bocd](https://github.com/dtolpin/bocd) and [y-bar/bocd](https://github.com/y-bar/bocd). Run in-process — zero IPC. Hazard `1/300` (~5 s at 60 fps), Gaussian likelihood on Hamming-distance stream.

**Post-session segmentation.** [deepcharles/ruptures](https://github.com/deepcharles/ruptures) PELT — offline, Python sidecar, for the FINAL-REPORT analysis directory.

**Reference-frame strategy.** Keep a rolling "last confirmed different frame" baseline, not just `t-1`. This prevents slow-drift blindness (1-pixel-per-frame spinner that fools `t-1` diff but fails against a 2-s-old baseline).

**False-positive suppression.** Cursor blink, loading spinners, video players all trip naive detectors. Banks carry per-step `ignore_regions: [{x,y,w,h}, ...]` to mask known-animated ROIs. Animated loaders specifically are paired with a *progress* signal (network traffic via Frida hook, ANR watchdog) before the "stuck" event fires.

#### 5.8.2 Landing points

- `pkg/autonomous/stagnation.go` — refactor to `Detector` interface; `BOCPDDetector`, `WindowDetector` impls.
- `pkg/vision/hash/dhash.go` — wrapper around goimagehash.
- `pkg/vision/perceptual/dreamsim.go` — Triton client.
- `pkg/analysis/pelt/` — post-session Python-bridge.

### 5.9 Visual regression & pixel-diff

Brief-2 asked for real-time recording + in-depth analysis. The right pixel-diff answer exists and is simple.

- **[mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)** — 150 LoC, MIT, AA-aware, YIQ colour diff. Go port [orisano/pixelmatch](https://github.com/orisano/pixelmatch) — in-process, zero deps.
- **Delta-E (CIEDE2000)** — only on changed tiles, for colour-critical paths (brand compliance, dark-mode verification).
- **[reg-viz/reg-cli](https://github.com/reg-viz/reg-cli)** — turnkey HTML reporter; output committed to `docs/reports/qa-sessions/.../analysis/`.

**Landing point.** `pkg/regression/pixelmatch.go`, `pkg/regression/deltae.go`, `pkg/regression/reporter.go`.

### 5.10 Model serving — llama.cpp RPC primary

Per `CLAUDE.md`: "llama.cpp RPC distributed inference is the primary local backend."

#### 5.10.1 Current reality

- [ggml-org/llama.cpp](https://github.com/ggml-org/llama.cpp) supports multimodal via `llama-mtmd-cli` / `llama-server` with CLIP projectors (LLaVA, MiniCPM-V, Qwen-VL, Qwen2-VL).
- Video input in llama.cpp is **not yet supported** ([#17660](https://github.com/ggml-org/llama.cpp/issues/17660)) — HelixQA continues to extract frames itself (§5.1).
- Multi-RPC works. Benchmarks on consumer GPUs: Qwen2.5-VL-7B Q4_K_M ≈ 20–30 tok/s on 3060 12 GB, ~1.5 s/image at 1024². UI-TARS-1.5-7B (similar architecture) fits the same envelope.

#### 5.10.2 When to use vLLM / SGLang / TensorRT-LLM instead

- **vLLM V1** ([Red Hat blog](https://developers.redhat.com/articles/2025/02/27/vllm-v1-accelerating-multimodal-inference-large-language-models)) — when a bigger GPU (A100, H100) is available for batched throughput across parallel autonomous tests. Qwen2.5-VL-32B is only feasible here.
- **SGLang** — when structured JSON output matters (NavigationStrategy): `guided_json` / regex constraints reduce parser retries in `pkg/autonomous/structured_executor.go`.
- **TensorRT-LLM** — best raw throughput on NVIDIA but heavy compile; not for dev laptops.

#### 5.10.3 HelixQA registration

- `pkg/llm/phase_selector.go` already routes per-phase. New serving backends register as `ProviderConfig` entries and declare phase preferences.
- `pkg/llm/vision_ranking.go` sources from `digital.vasic.llmsverifier` — update the registry when a new model is added.
- `pkg/llm/cost_tracker.go` — local models incur zero USD cost but non-zero time cost; track GPU-seconds per phase in the session report.

### 5.11 Low-level observation & hooks

Use for *passive* evidence only. Never for driving.

| Tool | Platforms | Role in HelixQA |
|---|---|---|
| [Frida](https://frida.re) (incl. [v17.4.0 "simmy" iOS Simulator backend](https://frida.re/news/2025/10/12/frida-17-4-0-released/)) | Win/mac/Linux/iOS/Android | Userspace API hooks (network calls, TLS keys, IPC) |
| [cilium/ebpf](https://github.com/cilium/ebpf) | Linux | Uprobes on libcurl/TLS/sqlite; `bpf2go`-generated programs embedded in Go binary |
| [LD_PRELOAD hook library](https://github.com/gaul/awesome-ld-preload) | Linux/macOS | GTK/Qt render hooks, libc intercept |
| [Microsoft Detours](https://github.com/microsoft/Detours) | Windows | User-mode function hooking |
| Perfetto | Android | System trace alongside video |

**Architecture.** All except cilium/ebpf run as sidecars (`helixqa-frida`). Go ships JS snippets over gRPC; Frida streams JSON events back. eBPF runs in-process — pure Go, CGO_ENABLED=0-compatible, uprobes attach by symbol name without kernel modifications.

**Landing points.** `pkg/nexus/observe/frida/`, `pkg/nexus/observe/ebpf/`, `pkg/nexus/observe/ldpreload/`, `pkg/nexus/observe/detours/` (Windows only), `pkg/nexus/observe/perfetto/`.

### 5.12 Fuzzing & adversarial exploration

Satisfies Article V category 5 (stress) and complements the Curiosity phase.

- **[pgregory.net/rapid](https://github.com/flyingmutant/rapid)** — Go stateful property-based testing. Model Catalogizer's UI as a state machine; rapid synthesises action sequences and shrinks counterexamples. Becomes the backbone of `banks/fixes-validation.yaml`.
- **Android Monkey** (`adb shell monkey`) — free crash discovery; overnight run under the builder container.
- **VLM-guided DFS (HelixQA-internal)** — implemented inside Curiosity. Maintain a "visited screens" set keyed by DreamSim; the agent prefers unvisited screens. No external dep; uses HelixQA's own VLM.

**Landing points.** `pkg/stress/rapid_driver.go`, `scripts/monkey-overnight.sh`, Curiosity extension in `pkg/autonomous/coordinator.go`.

---

## 6. Integration architecture

### 6.1 The sidecar boundary

Every native-code dependency lives in a sidecar process. The HelixQA Go binary:

- Is `CGO_ENABLED=0`.
- Speaks to sidecars over stdin/stdout framing (length-prefixed JSON) **or** UDS gRPC.
- Passes large binary payloads (frame buffers) over `memfd_create` + `sendmsg(SCM_RIGHTS)` file-descriptor passing. Metadata travels on the control channel.
- Knows how to detect missing sidecars (`pkg/bridges/registry.go.DiscoverTools`) and degrade gracefully — per `CLAUDE.md` the pipeline **skips** the affected phase rather than faking results.

Sidecar inventory:

| Binary | Language | Platforms | Purpose |
|---|---|---|---|
| `helixqa-capture-linux` | C (+ GStreamer) | Linux | PipeWire/kmsgrab capture |
| `helixqa-capture-darwin` | Swift | macOS | SCKit capture |
| `helixqa-capture-win` | C++/WinRT | Windows | WGC capture |
| `helixqa-input` | Rust (enigo) | mac, Windows | Input injection |
| `helixqa-axtree-darwin` | Swift | macOS | AXUIElement → JSON |
| `helixqa-frida` | Rust (frida-core) | all | Dynamic instrumentation |
| `helixqa-omniparser` | Python | Linux GPU | OmniParser v2 |
| `helixqa-langgraph` | Python | all | Phase graph |
| `helixqa-browser-use` | Python | all | Web agent |
| `qa-vision-infer` | C++ | Linux GPU | TensorRT + NPP + OpenCV-CUDA |
| `qa-video-decode` | C | Linux GPU | FFmpeg + NVDEC |
| `qa-vulkan-compute` | C++ | any GPU | Vulkan compute (PoC) |

Every sidecar:

1. Has its own container image, versioned independently.
2. Registers itself via `pkg/bridges/registry.go`.
3. Ships a minimal health-check subcommand (`--health`) returning 0/1.
4. Logs in `structlog`-compatible JSON to stderr; Go tees to the session archive.

### 6.2 Frame & action contracts

**Frame contract.**

```go
// pkg/capture/frames/frame.go
type Format int
const (
    FormatNV12 Format = iota
    FormatRGBA
    FormatBGRA
    FormatH264AnnexB
)

type Frame struct {
    PTS     time.Duration // since session start
    Width   int
    Height  int
    Format  Format
    Source  string   // "pipewire", "scrcpy", "sckit", "wgc", "idb", ...
    DataFD  int      // memfd (zero if Data is inline)
    DataLen int      // bytes in DataFD
    Data    []byte   // inline payload (fallback)
    AXTree  *axtree.Node // optional, if Snapshotter ran in sync
}
```

**Action contract.**

```go
// pkg/navigator/action.go
type Kind int
const (
    KindClick Kind = iota
    KindType
    KindSwipe
    KindKey
    KindScroll
    KindBack
    KindHome
)

type Target struct {
    AXNodeRawID string  // preferred
    Rect        Rect    // fallback 1
    Text        string  // fallback 2 (e.g. "the Login button")
    Coords      *Point  // last resort, must be AX-validated before use
}

type Action struct {
    Kind    Kind
    Target  Target
    Text    string        // for KindType
    Timeout time.Duration
}
```

**Resolution order.** `AXNodeRawID` > `Rect` > `Text` > `Coords`. Coordinates are only executed if the same frame's AX tree confirms an interactable element at that coordinate.

### 6.3 No-sudo, no-root enforcement

OpenClawing3 proposed multiple sudo invocations. All of them have user-space replacements:

| OpenClawing3 path | User-space replacement |
|---|---|
| `sudo dpkg -i ...tensorrt.deb` | Install TensorRT inside a container image built by `podman build`; container base already has GPU via NVIDIA Container Toolkit. |
| `sudo apt-get install tensorrt` | Same as above. |
| `sudo usermod -aG input ...` | `/dev/uinput` access granted by a udev rule installed once by the operator; HelixQA runs as a member of the existing `helixqa` group. |
| `sudo make install` | Install into `$HOME/.local/bin`; the container image handles system-wide when required. |

Pre-commit hook: `scripts/hooks/no-sudo.sh` greps the repository for `\bsudo\b`, fails commit (documentation allowed only as `~~sudo~~` strike-through in design docs).

### 6.4 HTTP/3 and Brotli

Internal HelixQA→Catalogizer traffic uses HTTP/3 + Brotli per `CLAUDE.md`. Sidecar→Sidecar traffic uses UDS gRPC (no protocol choice) or stdio.

### 6.5 Observability

All sidecars emit OTel spans via `pkg/nexus/observability/otel_exporter.go`. Metrics go to Prometheus (`pkg/nexus/observability/prometheus.go`). Traces are joined by session id (`SessionPipeline.sessionID`). The operator console, already mandated by Article VII, consumes the Prometheus scrape.

### 6.6 Threat model

OpenClawing3 omitted a threat model. Below the minimum:

| Threat | Surface | Mitigation |
|---|---|---|
| Malicious sidecar binary | `helixqa-*` in PATH | Pin binary paths in `pkg/bridges/registry.go`; verify SHA-256 at startup; ship binaries inside the HelixQA container image. |
| Evidence tampering | Session archive | Archive directories are append-only during a session; sealed SHA-256 manifest written on session end. |
| LLM prompt injection via screenshot text | VLM prompt | OCR output is *not* concatenated into the system prompt; it enters as structured `candidates` list. |
| API key leakage | `.env` | `.gitignore` covers `.env`; pre-commit hook scans for suspicious patterns. |
| GPU memory exhaustion | TRT engine | Budget check in `qa-vision-infer` on startup; abort with clear error if < 7 GB free. |
| Device-under-test model spoof | `.devignore` | Every ADB path calls `getprop ro.product.model` before actuation. |

---

## 7. Package-level mapping

This section is the *transcription* of every recommended technology into the real `pkg/...` layout. Every entry says: what exists, what changes, what is new.

### 7.1 New packages

| Package | Purpose | Phase |
|---|---|---|
| `pkg/capture/linux/` | PipeWire portal + kmsgrab | 1 |
| `pkg/capture/darwin/` | SCKit wrapper | 6 |
| `pkg/capture/windows/` | WGC + DXGI-DD | 6 |
| `pkg/capture/tui/` | ANSI escape parser | 6 |
| `pkg/capture/frames/` | Normalised `Frame` type | 1 |
| `pkg/bridge/scrcpy/` | scrcpy-server v3 direct protocol | 1 |
| `pkg/bridge/langgraph/` | LangGraph gRPC client | 3 |
| `pkg/bridge/omniparser/` | OmniParser v2 REST client | 3 |
| `pkg/bridge/browser_use/` | browser-use sidecar wrapper | 3 |
| `pkg/bridge/sidecarutil/` | stdio framing, FD passing | 1 |
| `pkg/navigator/linux/libei.go` | Wayland input via RemoteDesktop portal | 1 |
| `pkg/navigator/linux/uinput.go` | `/dev/uinput` fallback | 1 |
| `pkg/navigator/darwin/enigo_sidecar.go` | macOS input via enigo | 6 |
| `pkg/navigator/windows/enigo_sidecar.go` | Windows input via enigo | 6 |
| `pkg/navigator/ios/idb.go` | Facebook idb client | 6 |
| `pkg/navigator/tui/pty.go` | TUI injection | 6 |
| `pkg/nexus/observe/axtree/` | Unified AX tree | 2 |
| `pkg/nexus/observe/ebpf/` | cilium/ebpf uprobes | 5 |
| `pkg/nexus/observe/frida/` | Frida sidecar client | 5 |
| `pkg/nexus/observe/ldpreload/` | LD_PRELOAD hook catalogue | 5 |
| `pkg/vision/hash/` | dHash/pHash/wHash via goimagehash | 2 |
| `pkg/vision/perceptual/` | DreamSim, LPIPS clients | 2 |
| `pkg/vision/flow/` | DIS and NVOF wrappers | 2 |
| `pkg/vision/template/` | matchTemplate wrapper | 2 |
| `pkg/vision/text/` | EAST text detection | 2 |
| `pkg/vision/tui/` | Cell-grid diff | 6 |
| `pkg/analysis/pelt/` | Ruptures PELT post-session | 2 |
| `pkg/regression/` | pixelmatch, Delta-E, reg-cli | 2 |
| `pkg/stress/rapid_driver.go` | rapid stateful fuzz | 5 |

### 7.2 Modified packages

| File | Change | Phase |
|---|---|---|
| `pkg/autonomous/stagnation.go` | Add `BOCPDDetector`; refactor to `Detector` interface | 2 |
| `pkg/autonomous/coordinator.go` | VLM-guided DFS with DreamSim visited-set | 5 |
| `pkg/autonomous/pipeline.go` | Optional LangGraph sidecar hook (feature-flagged) | 3 |
| `pkg/autonomous/structured_executor.go` | SGLang `guided_json` awareness | 3 |
| `pkg/llm/phase_selector.go` | Add UI-TARS / OmniParser strategies | 3 |
| `pkg/llm/vision_ranking.go` | Register new grounding models | 3 |
| `pkg/llm/providers_registry.go` | New env var keys (`HELIX_OMNIPARSER_URL`, `HELIX_UITARS_URL`) | 3 |
| `pkg/navigator/x11_executor.go` | Build-tag `x11legacy`; default is libei | 1 |
| `pkg/navigator/executor.go` | Extend `ActionExecutor` with `Verify(a Action) error` | 2 |
| `pkg/capture/android_capture.go` | Delegate to `pkg/bridge/scrcpy` when `HELIX_SCRCPY_DIRECT=1` | 1 |
| `pkg/capture/linux_capture.go` | Prefer `pkg/capture/linux/portal.go` when available | 1 |
| `pkg/bridges/registry.go` | New sidecar probes (all of §6.1) | per-phase |
| `pkg/memory/store.go` | New tables: `axtree_snapshots`, `stagnation_events`, `costs_gpu_seconds` | 2 |

### 7.3 Worked example — wiring UI-TARS-1.5-7B

Goal: UI-TARS becomes an available NavigationStrategy provider on llama.cpp RPC.

1. **Model bring-up (operator-side).** Convert UI-TARS-1.5-7B to GGUF (Q4_K_M). Drop into `~/models/`. Start `llama-server --host 0.0.0.0 --port 18100 --model ~/models/ui-tars-1.5-7b-q4_k_m.gguf --mmproj ~/models/ui-tars-mmproj.gguf`.
2. **Register in HelixQA.**
   - `pkg/llm/providers_registry.go`: add `"uitars15"` with `HELIX_UITARS_URL` default `http://localhost:18100/v1`.
   - `pkg/llm/vision_ranking.go`: the registry from `digital.vasic.llmsverifier` is updated to carry UI-TARS 1.5 scores.
3. **Phase routing.**
   - `pkg/llm/phase_selector.go`: `NavigationStrategy.Weights["gui_grounding"] += 0.25` for UI-TARS-family.
4. **Action resolution.**
   - UI-TARS emits `{"action":"click","coords":[412,738]}`.
   - `pkg/navigator.resolveTarget(Action, frame)` calls `axtree.NodeAt(frame.AXTree, 412, 738)`. If a node exists with `Enabled=true`, the `Target.AXNodeRawID` is set. Execution uses the AX raw id, not the raw coordinate.
   - If no AX node exists at that coordinate, the action is rejected and logged — the VLM has hallucinated. A `stagnation_event` is recorded; the Curiosity phase picks a different strategy.
5. **Bank.** `banks/grounding-verification.yaml` adds a test case per known-good screen where the coordinate-and-axnode pair is asserted. Any drift triggers a `fixes-validation` entry.
6. **Cost.** llama-server response includes token counts; `pkg/llm/cost_tracker.go` records a new `CostRate{FreeTier: true}` row. Wall-clock + GPU-seconds are tracked.

### 7.4 Worked example — replacing `x11grab` with portal + pipewiresrc on Linux

1. **Detect environment.** `pkg/capture/linux_capture.go` probes `XDG_SESSION_TYPE`. If `wayland`, route to `portal.go`. If `x11`, route to `x11grab` (unchanged) under build tag `x11legacy` for compatibility.
2. **Portal handshake.** `portal.go` uses `godbus` to call `CreateSession → SelectSources(types=MONITOR | WINDOW) → Start`. Returned: `streams`, `restore_token`.
3. **Launch pipeline.** `pipewire.go` forks `helixqa-capture-linux` sidecar with the PipeWire FD in `SysProcAttr.ExtraFiles[0]`. The sidecar runs `pipewiresrc fd=3 path=<node> ! videoconvert ! x264enc tune=zerolatency ! appsink`. stdout yields Annex-B; Go parses NAL units.
4. **Normalise.** Emit `Frame{Format: FormatH264AnnexB, ...}`. Downstream `pkg/gst/frame_extractor.go` can continue to extract PNG frames on demand.
5. **Graceful degradation.** If the portal prompt fails (e.g. over SSH without D-Bus forwarded), `pkg/capture/linux/` falls back to xcb-shm (also new) and then to the x11grab path.
6. **Acceptance.** `banks/capture-linux.yaml` runs on GNOME, KDE, Hyprland, and a legacy X11 session; at least p95 < 10 ms/frame capture latency at 1080p60.

### 7.5 Worked example — scrcpy-server direct protocol

1. **Pin server jar version.** Embed `scrcpy-server-v3.x.jar` as a resource in `pkg/bridge/scrcpy/`. Hash verified at startup.
2. **Connect.** `server.go` runs `adb forward tcp:27183 localabstract:scrcpy`, then `adb shell CLASSPATH=/data/local/tmp/scrcpy-server.jar app_process / com.genymotion.scrcpy.Server <args>`. Accepts three inbound connections (video, audio, control) on port 27183.
3. **Device guard.** Before accepting, `devguard.go` runs `adb -s <serial> shell getprop ro.product.model`. Match against `.devignore` → abort.
4. **Decode video.** `protocol.go` reads the H.264 NAL stream, strips scrcpy's per-packet header into `(pts, config_flag)`, and emits `Frame` objects. The control socket is kept open for input injection (§5.2.2).
5. **Acceptance.** `banks/capture-android.yaml` exercises the path against every listed `.devconnect` device at 1080p60; p95 < 20 ms end-to-end latency.

---

## 8. Phased delivery plan

Recap of §1.3 with acceptance cross-links.

| Phase | Scope | Article V categories exercised | New banks |
|---|---|---|---|
| 0 | Documentation corrections (§11); pre-commit `no-sudo.sh`; audit sign-off | 6 (security) | `banks/docs-audit.yaml` |
| 1 | Linux PipeWire + kmsgrab; scrcpy-server protocol; libei + uinput | 1,2,3,4,5,8 | `banks/capture-linux.yaml`, `banks/capture-android.yaml`, `banks/input-linux.yaml` |
| 2 | Unified AX tree; dHash + SSIM + DreamSim; BOCPD | 1,2,3,4,8,10 | `banks/axtree-*.yaml`, `banks/stagnation-*.yaml` |
| 3 | UI-TARS + OmniParser + ShowUI; LangGraph; SGLang | 1,2,3,4,8,10 | `banks/grounding-verification.yaml`, `banks/phase-graph.yaml` |
| 4 | qa-vision-infer (TRT+NPP+OpenCV-CUDA); qa-video-decode (FFmpeg+NVDEC); qa-vulkan-compute PoC | 5,8,9 | `banks/gpu-compute-*.yaml` |
| 5 | Frida sidecar; cilium/ebpf uprobes; LD_PRELOAD catalogue; rapid fuzzing; VLM-guided DFS | 5,6,7,9 | `banks/observability-*.yaml`, `banks/stress-rapid.yaml` |
| 6 | SCKit/WGC/TUI capture; enigo sidecars; idb; polishing | all | `banks/capture-{darwin,windows,tui}.yaml`, `banks/ios-*.yaml` |

Each phase ends with a clean Full-QA Master Cycle pass (Article VII) — no phase claims completion until the cycle ends GREEN with the new banks included and a regression entry added to `banks/fixes-validation.yaml` for every bug found along the way.

---

## 9. Acceptance criteria (per Article V category)

Every new component is subject to the ten categories. Below is the minimum bar.

1. **Unit** — ≥ 95 % branch coverage in the package; all public funcs have table-driven tests; error paths return typed errors.
2. **Integration** — end-to-end test that invokes the sidecar under `podman-compose --profile integration`.
3. **E2E** — at least one full Learn→Plan→Execute→Curiosity→Analyze run that exercises the component; archived in `qa-results/session-*/`.
4. **Full automation** — unattended, reproducible under `scripts/helixqa-orchestrator.sh`; no manual inputs.
5. **Stress** — 24 h soak where applicable; rapid fuzz sequence ≥ 10 k actions; memory bounded (`pprof` evidence).
6. **Security** — `govulncheck` clean; no `sudo` / `su` in repo; secret scan clean; threat-model entry added (§6.6).
7. **DDoS / rate-limit** — sidecars must survive `k6` saturation (see `pkg/nexus/perf/k6.go`).
8. **Benchmarking** — stated latency budget (§1.2) met on the reference RTX 3060 host at p95; regression alerts on > 10 % drift.
9. **Challenges** — a `digital.vasic.challenges` entry registered; runs as part of the regular challenge suite.
10. **HelixQA** — every screen/flow under the component's surface has a bank entry in `banks/<component>.yaml`; adversarial variants (malformed A11y, flaky sidecar, partial frame) covered.

Acceptance artefact inventory per component:

```
docs/reports/qa-sessions/<YYYY-MM-DD-THH-MM>/
├── FINAL-REPORT.md              # 10-category pass/fail table
├── logs/                        # per-phase logs
├── challenges/                  # challenge-suite results
├── helixqa/                     # bank results + autonomous runs
├── videos/                      # session recordings (scrcpy / pipewire / sckit / wgc)
├── screenshots/                 # decisive frames
├── tickets/                     # per-defect with VideoReference
└── analysis/                    # pixelmatch/reg-cli reports + PELT segmentation
```

---

## 10. Risk register

| ID | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R-01 | libei support missing on a customer's Linux distro | med | med | Fallback to `/dev/uinput` via udev rule. |
| R-02 | scrcpy protocol break in future Android | med | high | Pin server JAR in HelixQA image; regression bank on upgrade. |
| R-03 | NVIDIA driver bump breaks TRT engines | med | high | Engine rebuild script run on driver upgrade; archive engine + driver version per session. |
| R-04 | macOS TCC prompt defeats unattended runs | high | high | SCKit sidecar with pre-granted entitlement; never run full GUI login flows during peak hours. |
| R-05 | Vulkan compute is slower than expected on NVIDIA | high | low | Keep Vulkan gated behind `HELIX_VULKAN=1`; NVIDIA path uses CUDA. |
| R-06 | LLM hallucinated coordinates escape AX verification | low | high | Execution layer refuses actions with unresolved `Target.AXNodeRawID`. |
| R-07 | Sidecar supply chain (frida-core, enigo) | low | high | SHA-256 pin + audited upstream tags; build in-house images. |
| R-08 | OSWorld / AndroidWorld benchmark drift | high | low | Pin benchmark commit hashes; rerun only on explicit bank update. |
| R-09 | Cost tracking misses local GPU seconds | low | low | `pkg/llm/cost_tracker.go` adds GPU-seconds column; emits in `pipeline-report.json`. |
| R-10 | Unplanned sudo reintroduction by contributor | med | critical | Pre-commit hook `scripts/hooks/no-sudo.sh`; blocks. |
| R-11 | Documentation diverges from code | high | med | This document is versioned; sections have implementation owners; every merge that changes an owner area must touch §7. |
| R-12 | Evidence tampering | low | high | Per-session SHA-256 manifest written + signed on close (§6.6). |

---

## 11. Corrections appendix (retractions of prior docs)

### 11.1 Starting_Point.md — dead / inconsistent URLs

The following repository URLs cited in `Starting_Point.md` return HTTP 404 as of 2026-04-19 and must be treated as invalid:

- NanoClaw — cited URLs inconsistent between list and write-up; neither resolves.
- ZeroClaw — same.
- NullClaw — same.
- Hermes — cited URL 404.
- Moltworker — 404.
- DumbClaw — 404.
- PycoClaw — 404.
- BabyClaw — 404.
- Clawlet — 404.

Additionally, the "~420k LoC TypeScript", "port 18789", and "Lane Queue" claims describing the baseline "OpenClaw" are unsourced. This document does not treat them as authoritative. Downstream readers must not cite them.

### 11.2 OpenClawing2.md — fabricated internal paths

- `browser_use/browser/custom_browser.py` (cited at §4.3.2, approx L189) — does not exist in `browser-use/browser-use`. Retracted.
- `skyvern/agent/prompts.py` (cited at §5.1.2, L241) — Skyvern has no `agent/` directory. Retracted.
- `PlanEvaluate` class in browser-use (cited at §10.1.1 and §8.1.2) — not present in the browser-use repo. Retracted.

Port-target framing. OpenClawing2 describes "OpenClaw" as TypeScript at `src/agents/pi-embedded-runner.ts` and frames porting as TS→TS. HelixQA is Go. Any "port" is in fact a Go reimplementation that lands in `pkg/navigator/` or `pkg/autonomous/`. All "port" language in OpenClawing2 should be read as "reimplement in Go in HelixQA's `pkg/...`".

### 11.3 OpenClawing3.md — compile-blockers, sudo, and ghost trees

**Retracted paths.** Every file path that begins `src/...` in OpenClawing3 does not exist. Use the mapping in §7.1 / §7.2 to locate the real `pkg/...` home of each proposed change. Specifically (non-exhaustive): `src/capture/dxgi.cpp`, `src/capture/pipewire.cpp`, `src/vision/trt_engine.cpp`, `src/vision/opencv_pipeline.cpp`, `src/agent/pi-runner.ts`, `src/driver/input_linux.cpp`, `src/driver/input_mac.m`, `src/observe/plthook_hooks.cpp`, `src/net/quic_tunnel.rs`, plus 16 siblings. None of these are in HelixQA.

**Retracted code snippets (compile errors):**

1. `namespace openclaw { ... }` inside a `.c` file (§6.2 L1091–1248). C has no `namespace`. Re-write in C++ if needed, or drop.
2. `BuilderFlag::kCUDA_GRAPH` (line 727) — not a valid TensorRT `BuilderFlag`. CUDA Graphs are captured via runtime API, not a builder flag. See [TensorRT-RTX CUDA Graphs](https://docs.nvidia.com/deeplearning/tensorrt-rtx/1.2/inference-library/work-with-cuda-graphs.html).
3. `context->enqueueV3(bindings_, stream, nullptr)` using a `bindings_` array (§6.4) — this is the v2 API. TensorRT v3 uses `setTensorAddress(name, ptr)` then `enqueueV3(stream)`.
4. Use of `destroy()` method on TensorRT objects — deprecated; `delete` instead.
5. `{key:"Down"}.repeat(n)` (line 2561) — invalid TypeScript.
6. scrcpy v1.x control packet layout — use current v3 format with `action_button` and `buttons` uint32 fields.

**Retracted sudo lines (§14):** `sudo dpkg -i tensorrt.deb`, `sudo apt-get install tensorrt`, `sudo usermod -aG input helixqa`, `sudo make install`. Replaced per §6.3.

**Retracted "zero-copy" DXGI claim (§6.1 L1036-1037):** `GetSharedHandle` returns a legacy NT handle unusable by Vulkan external-memory extensions; the proposed flow is GPU↔GPU copy, not zero-copy. Windows zero-copy from DXGI into Vulkan requires `IDXGIResource1::CreateSharedHandle` + `VK_KHR_external_memory_win32` with a matched handle type — non-trivial and out of scope for Phase 4.

**Retracted benchmarks.** "< 2 ms template match on 1080p" and "< 16 ms full pipeline" on RTX 3060 are off by 3–7×. The realistic end-to-end budget is ~20 ms p95 (§5.5 Latency Budget).

**Plan reset.** The 16-week / 47-technology schedule is replaced by the ~24-week / 7-phase plan in §8.

---

## 12. Game-changer additions (net-new)

These are additions HelixQA needs that neither OpenClawing2 nor OpenClawing3 proposed.

1. **On-device perception tier in pure Go.** dHash via `corona10/goimagehash` inside the HelixQA Go process — sub-millisecond per frame, CGO_ENABLED=0. Huge win for stagnation detection. (Prior docs skipped to GPU inference.)
2. **BOCPD-based stagnation.** Probabilistic, online, interpretable. Replaces hand-tuned thresholds. (§5.8)
3. **Accessibility-tree-first action resolution.** Deterministic target pinning; coordinates only survive if the AX tree confirms them. (§5.3, §6.2)
4. **Unified `Node{Role,Name,Bounds}` across 6 platforms.** One tree type; one `Snapshotter` interface. (§5.3)
5. **scrcpy-server direct protocol (no desktop binary dependency).** Pure-Go client speaks scrcpy v3 wire format. (§5.1.3, §7.5)
6. **libei + RemoteDesktop portal for Linux input.** Kills xdotool's Wayland problem. (§5.2.1)
7. **Frida 17.4.0 "simmy" backend for iOS Simulator.** New in 2025-10; lets HelixQA instrument simulator processes. (§5.11)
8. **LangGraph phase graph as a Python sidecar.** Adds deterministic replay and checkpoints without refactoring Go. (§5.7.1)
9. **OmniParser v2 as universal set-of-mark upstream.** Any downstream VLM (Claude, local Qwen, UI-TARS) shares the same element index. (§5.6)
10. **Timeline semaphores + CUDA graphs for pipeline overlap.** Analyze-N + Capture-N+1 execute concurrently. (§5.5)
11. **pixelmatch Go port for VisualRegression submodule.** 150 LoC, MIT, AA-aware. (§5.9)
12. **ScreenSuite wire-up for Article V category 10.** External bench-as-a-service proving HelixQA quality. (§5.6)
13. **DreamSim as the tiebreaker.** 96 % human agreement for "is this screen meaningfully different?". Resolves ambiguous stagnation events. (§5.8)
14. **VLM-guided DFS in the Curiosity phase.** Maintain a visited-screens set keyed by DreamSim embeddings; prefer unvisited. (§5.12)
15. **A realistic phase plan.** 24 weeks across 7 phases with Article V acceptance per phase. (§8, §9)
16. **Signed per-session SHA-256 manifest** for evidence-tamper resistance. (§6.6, §9)
17. **Cost tracker extension to GPU-seconds.** Local inference is "free" in USD but not in time. (§5.10.3)
18. **TUI as a first-class surface.** ANSI-parser + cell-grid diff. (§4.2, §5.1.6, §5.2.6, §5.4.5)

---

## 13. Glossary and canonical references

### 13.1 Acronyms

- **AX** — Accessibility (tree / node).
- **BOCPD** — Bayesian Online Change-Point Detection.
- **CDP** — Chrome DevTools Protocol.
- **DIS** — Dense Inverse Search (optical flow).
- **DXGI-DD** — DXGI Desktop Duplication (Windows).
- **OMP** — OmniParser.
- **PELT** — Pruned Exact Linear Time (segmentation).
- **SCKit** — ScreenCaptureKit (macOS).
- **SoM** — Set-of-Mark (UI element overlay).
- **UIA** — UI Automation (Windows).
- **UTA** — UI-TARS.
- **VLM** — Vision-Language Model.
- **WGC** — Windows.Graphics.Capture.

### 13.2 Canonical URLs (all verified during 2026-04-19 research pass)

**HelixQA-internal.**
- `CLAUDE.md` — HelixQA mandatory constraints.
- `CONSTITUTION.md` (Catalogizer root) — Article V, VI, VII.
- `docs/OPEN_POINTS_CLOSURE.md` — operator-action source of truth.
- `HelixQA/docs/openclawing/OpenClawing4-Audit.md` — forensic audit feeding this document.

**Capture.**
- [xdg-desktop-portal ScreenCast](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html)
- [pipewiresrc GStreamer element](https://gstreamer.freedesktop.org/documentation/pipewire/pipewiresrc.html)
- [FFmpeg kmsgrab.c](https://github.com/FFmpeg/FFmpeg/blob/master/libavdevice/kmsgrab.c)
- [NVIDIA Capture SDK](https://developer.nvidia.com/capture-sdk)
- [ScreenCaptureKit](https://developer.apple.com/documentation/screencapturekit/)
- [Windows.Graphics.Capture](https://learn.microsoft.com/en-us/windows/uwp/audio-video-camera/screen-capture)
- [OBS WGC vs DXGI forum](https://obsproject.com/forum/threads/windows-graphics-capture-vs-dxgi-desktop-duplication.149320/)
- [Genymobile/scrcpy](https://github.com/Genymobile/scrcpy)
- [scrcpy protocol develop.md](https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md)
- [facebook/idb](https://github.com/facebook/idb)
- [ReplayKit](https://developer.apple.com/documentation/replaykit)

**Input.**
- [libei](https://gitlab.freedesktop.org/libinput/libei)
- [Who-T: libei opening the portal doors](http://who-t.blogspot.com/2022/12/libei-opening-portal-doors.html)
- [ReimuNotMoe/ydotool](https://github.com/ReimuNotMoe/ydotool)
- [enigo-rs/enigo](https://github.com/enigo-rs/enigo)

**Accessibility.**
- [UI Automation Overview (MS)](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-uiautomationoverview)
- [godbus/dbus](https://github.com/godbus/dbus)
- [at-spi2-core a11y bus README](https://github.com/GNOME/at-spi2-core/blob/main/bus/README.md)
- [CDP Accessibility domain](https://chromedevtools.github.io/devtools-protocol/tot/Accessibility/)

**Vision / GPU.**
- [hybridgroup/gocv](https://github.com/hybridgroup/gocv)
- [OpenCV TextDetectionModel_EAST](https://docs.opencv.org/4.x/d8/ddc/classcv_1_1dnn_1_1TextDetectionModel__EAST.html)
- [TensorRT for RTX](https://docs.nvidia.com/deeplearning/tensorrt-rtx/latest/index.html)
- [TensorRT-RTX CUDA Graphs](https://docs.nvidia.com/deeplearning/tensorrt-rtx/1.2/inference-library/work-with-cuda-graphs.html)
- [NVIDIA NPP](https://docs.nvidia.com/cuda/npp/index.html)
- [NVIDIA Video Codec SDK](https://developer.nvidia.com/video-codec-sdk)
- [FFmpeg with NVIDIA GPU](https://docs.nvidia.com/video-technologies/video-codec-sdk/13.0/ffmpeg-with-nvidia-gpu/index.html)
- [Triton Inference Server](https://github.com/triton-inference-server/server)
- [vk-bootstrap](https://github.com/charles-lunarg/vk-bootstrap)
- [VulkanMemoryAllocator](https://github.com/GPUOpen-LibrariesAndSDKs/VulkanMemoryAllocator)
- [NVIDIA Vulkan Video](https://developer.nvidia.com/blog/gpu-accelerated-video-processing-with-nvidia-in-depth-support-for-vulkan-video/)
- [EGL Eye (OpenGL without X)](https://developer.nvidia.com/blog/egl-eye-opengl-visualization-without-x-server/)

**Grounding models.**
- [bytedance/UI-TARS](https://github.com/bytedance/UI-TARS) / [HF UI-TARS-1.5-7B](https://huggingface.co/ByteDance-Seed/UI-TARS-1.5-7B)
- [microsoft/OmniParser](https://github.com/microsoft/OmniParser) / [HF OmniParser-v2.0](https://huggingface.co/microsoft/OmniParser-v2.0)
- [showlab/ShowUI](https://github.com/showlab/ShowUI) / [HF ShowUI-2B](https://huggingface.co/showlab/ShowUI-2B)
- [OS-Copilot/OS-Atlas](https://github.com/OS-Copilot/OS-Atlas)
- [OSU-NLP-Group/UGround](https://github.com/OSU-NLP-Group/UGround)
- [Anthropic computer use](https://www.anthropic.com/news/3-5-models-and-computer-use)

**Benchmarks.**
- [ScreenSpot-Pro leaderboard](https://gui-agent.github.io/grounding-leaderboard/)
- [OSWorld-Verified leaderboard](https://llm-stats.com/benchmarks/osworld-verified)
- [OSWorld repo](https://github.com/xlang-ai/OSWorld)
- [AndroidWorld](https://github.com/google-research/android_world)
- [ScreenSuite (HF blog)](https://huggingface.co/blog/screensuite)

**Agent frameworks.**
- [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph)
- [browser-use/browser-use](https://github.com/browser-use/browser-use)
- [huggingface/smolagents](https://github.com/huggingface/smolagents)

**Perception.**
- [corona10/goimagehash](https://github.com/corona10/goimagehash)
- [mapbox/pixelmatch](https://github.com/mapbox/pixelmatch)
- [ssundaram21/dreamsim](https://github.com/ssundaram21/dreamsim) / [arXiv 2306.09344](https://arxiv.org/html/2306.09344v3)
- [dtolpin/bocd](https://github.com/dtolpin/bocd)
- [y-bar/bocd](https://github.com/y-bar/bocd)
- [deepcharles/ruptures](https://github.com/deepcharles/ruptures)

**Model serving.**
- [ggml-org/llama.cpp](https://github.com/ggml-org/llama.cpp)
- [vLLM V1 multimodal (Red Hat)](https://developers.redhat.com/articles/2025/02/27/vllm-v1-accelerating-multimodal-inference-large-language-models)

**Observation / hooks.**
- [Frida](https://frida.re/) / [Frida 17.4.0 release notes](https://frida.re/news/2025/10/12/frida-17-4-0-released/)
- [cilium/ebpf](https://github.com/cilium/ebpf)
- [microsoft/Detours](https://github.com/microsoft/Detours)
- [gaul/awesome-ld-preload](https://github.com/gaul/awesome-ld-preload)

**Fuzzing.**
- [pgregory.net/rapid](https://github.com/flyingmutant/rapid)
- [leanovate/gopter](https://github.com/leanovate/gopter)

### 13.3 Change log (for this document)

| Version | Date | Author | Change |
|---|---|---|---|
| 1.0 | 2026-04-19 | HelixQA platform team | Initial release. Supersedes OpenClawing2 / OpenClawing3 per §11. |

---

## 14. Appendix A — proposed new YAML banks

Skeletons follow the schema in `pkg/testbank/schema.go`. Every bank added here is a *declaration* — the maintainer creating Phase X is responsible for filling in concrete test cases before the phase can close.

### A.1 `banks/capture-linux.yaml`

```yaml
version: "1.0"
name: "Linux Wayland & X11 capture"
metadata:
  author: helixqa-platform
  app: helixqa
test_cases:
  - id: CAP-LIN-PORTAL-001
    name: "xdg-desktop-portal ScreenCast happy path (GNOME 46)"
    category: capture
    priority: critical
    platforms: [desktop]
    steps:
      - name: "Negotiate ScreenCast session"
        action: "sidecar: helixqa-capture-linux --mode portal --probe"
        expected: "Portal returns PipeWire FD"
        timeout: 15
      - name: "Sustain 10s capture at 1080p60"
        action: "sidecar: helixqa-capture-linux --duration 10 --rate 60"
        expected: "p95 frame latency < 10 ms; zero frame drops"
  - id: CAP-LIN-KMSGRAB-001
    # zero-copy path (capability-granted sidecar)
    ...
  - id: CAP-LIN-X11-001
    # legacy X11 fallback
    ...
```

### A.2 `banks/capture-android.yaml`

```yaml
version: "1.0"
name: "Android scrcpy-server direct protocol"
metadata:
  author: helixqa-platform
test_cases:
  - id: CAP-AND-SCRCPY-001
    name: "scrcpy-server v3 multi-socket bring-up"
    category: capture
    priority: critical
    platforms: [android]
    steps:
      - name: "devguard check"
        action: "adb_shell: getprop ro.product.model"
        expected: "Device not in .devignore"
      - name: "Forward localabstract socket"
        action: "adb_forward: tcp:27183 localabstract:scrcpy"
        expected: "Forward succeeded"
      - name: "Launch server"
        action: "adb_shell: CLASSPATH=/data/local/tmp/scrcpy-server.jar app_process / com.genymotion.scrcpy.Server ..."
        expected: "Three sockets accepted (video|audio|control)"
      - name: "Sustain 10s H.264 stream"
        action: "helixqa: assert stream PTS monotonic, bitrate>=6Mbps"
        expected: "Pass"
```

### A.3 `banks/grounding-verification.yaml`

```yaml
version: "1.0"
name: "UI-TARS + OmniParser grounding verification"
metadata:
  author: helixqa-platform
test_cases:
  - id: GRND-UITARS-001
    name: "Login button grounding (Android, Catalogizer)"
    category: grounding
    priority: critical
    platforms: [android]
    steps:
      - name: "Capture screenshot at launch"
        action: "screenshot"
      - name: "Run UI-TARS-1.5-7B via llama-server"
        action: "llm.vision: HELIX_UITARS_URL prompt='Find the Login button'"
        expected: "Coords within bounds of AX node Role=Button Name=Login"
      - name: "AX-tree reconciliation"
        action: "helixqa: resolveTarget(action, frame.AXTree)"
        expected: "AXNodeRawID set"
```

### A.4 `banks/stagnation.yaml`

```yaml
version: "1.0"
name: "Stagnation detection (dHash + BOCPD + SSIM + DreamSim)"
test_cases:
  - id: STAG-BOCPD-001
    name: "Stuck screen for 10s triggers STAGNATION event"
    priority: critical
    steps:
      - name: "Drive app into known-stuck state"
        action: "helixqa: ..."
      - name: "Run stagnation detector"
        action: "pkg.autonomous.stagnation.BOCPDDetector.Run(duration=15s)"
        expected: "event fired between 10s and 11s"
```

### A.5 `banks/observability.yaml`

Covers Frida / eBPF / LD_PRELOAD observation — entries added in Phase 5.

### A.6 `banks/stress-rapid.yaml`

Covers `pgregory.net/rapid` stateful fuzz sequences — entries added in Phase 5.

### A.7 `banks/fixes-validation.yaml`

Per Article VII, every bug found during OpenClawing2/3 adoption **must** land a regression entry here. Initial entries seeded from §11 corrections (e.g. `FIX-OC2-001` through `FIX-OC3-011`). Growing over time.

---

## 15. Appendix B — migration checklist (operator-facing)

1. Read `CLAUDE.md` sections Mandatory Constraints and Host Resource Limits.
2. Install the HelixQA container image that ships all sidecars for your OS (the image bundles `helixqa-capture-*`, `helixqa-input`, `helixqa-frida`, etc.).
3. Provision GPU: NVIDIA Container Toolkit + Podman CDI; verify with `podman run --gpus all --rm nvidia/cuda:12.x-runtime-ubuntu24.04 nvidia-smi`.
4. Grant udev rule for `/dev/uinput` group access: operator-side one-liner; **not** via `sudo` at HelixQA runtime.
5. Drop UI-TARS-1.5-7B Q4_K_M and OmniParser v2 weights under `~/models/`.
6. Start `llama-server` + RPC workers per `.env`.
7. Start `helixqa-omniparser` (Python sidecar, container).
8. Run `scripts/helixqa-orchestrator.sh androidtv` — should pass bank `CAP-AND-SCRCPY-001`.
9. Read `docs/reports/qa-sessions/<latest>/FINAL-REPORT.md`.
10. Subscribe to `banks/fixes-validation.yaml` — every bug closure lands here.

---

## 16. Closing

OpenClawing 2 and 3 named many of the right ingredients. They misnamed the kitchen, borrowed someone else's recipe written in a dead language, and quietly invited sudo into the pantry. OpenClawing 4 is what a production engineering team would build to replace that: every recommendation traces to a real HelixQA package, every sudo has been stripped, every benchmark is anchored to a real measurement path, every external model carries a verified URL, and every phase ends on an Article V acceptance pass.

Review this document with §11 (retractions) open; anyone who cites OpenClawing2 or OpenClawing3 after this point does so against §11's warnings.

— end of OpenClawing4.md
