# OpenClawing2 Integration Plan — HelixQA Autonomous UI/UX Control Upgrade

**Date:** 2026-04-18
**Status:** Draft for approval before implementation begins.
**Source research:** `docs/OpenClawing2.md` (623 lines, 2026-04-18).
**Target release:** HelixQA v3.0 — "Magical" production gate.

---

## 0. Non-negotiable ground rules

Before any code change lands, these constraints are absolute:

- **Project-agnostic.** HelixQA + every submodule carries zero project-specific names. Every feature ported must integrate via a pluggable registry / manifest / config, never a Catalogizer-branded default.
- **LLM-vision only.** The new primitives must extend the existing constitution — no hardcoded tap coordinates, sleep timers, or scripted fallbacks. When a vision backend is unavailable, the phase skips.
- **100% test coverage across all ten categories.** Every new file, every new primitive, every new loop state must ship with unit + integration + E2E + full-automation + stress + security + DDoS + benchmark + challenge + HelixQA-bank coverage BEFORE PR merge.
- **Evidence-backed tickets.** Every defect the new engine surfaces must produce a ticket with video timestamp, screenshot filename, session id, reproduction path — the rich-ticketing the user asked for explicitly.
- **Host resource budget.** Max 4 CPU / 8 GB RAM across the whole campaign. Model selection stays in the LLMsVerifier strategy pool; no heavyweight provider lock-in.

If any phase violates one of these, the phase fails — revert and rework before moving on.

---

## 1. Framework landscape — what OpenClawing2.md brings to HelixQA

| Framework | Source | What HelixQA ports in | What stays out |
|---|---|---|---|
| **browser-use** | github.com/browser-use/browser-use | `Agent.step()` loop (Plan → Act → Evaluate → Replan), `MessageManager` context compaction, exponential-backoff + jitter retry, loop detection via history analysis, DOM processing service with index-based references | The 2captcha integration (legal/ethical concerns + not generic) |
| **Skyvern** | github.com/Skyvern-AI/skyvern | Explicit Observe / Plan / Act state machine, bounding-box element labels on screenshots, task-history persistence | Cloud-only scheduling — HelixQA stays local-first |
| **Stagehand** | github.com/browserbase/stagehand (already vendored under `tools/opensource/stagehand`) | `act` / `extract` / `observe` / `agent` primitives, prompt caching, self-healing re-inference on selector failure | Browserbase cloud service — optional adapter only |
| **UI-TARS** | github.com/bytedance/UI-TARS-desktop | Native multimodal agent model path, coordinate-grounded `click(x,y)` action space, screenshot-as-feedback, `pyautogui`-style desktop control via the existing `pkg/nexus/desktop` engine | The desktop Electron shell; HelixQA keeps its own CLI runner |
| **Anthropic computer-use-demo** | github.com/anthropics/anthropic-quickstarts | System-prompt engineering patterns for vision-grounded actions, `ToolCollection` dispatch idiom, coordinate-scaling algorithm for resolution-independent control | None (reference implementation — no code imported, patterns only) |

---

## 2. Gap analysis vs. current HelixQA

| Capability | Current state | Target state | OpenClawing2 reference |
|---|---|---|---|
| Agent loop | Linear Execute → Curiosity → Analyze. No Plan/Evaluate/Replan cycle. | State machine with explicit phases + typed `AgentStep` outputs. Each step carries `evaluation`, `memory`, `next_goal`, `actions[]`. | browser-use `Agent.step()` + Skyvern state machine |
| DOM processing | `pkg/nexus/browser/snapshot.go` extracts interactive elements. | Add element indexing + bounding-box overlay + filter-by-visibility pass. Screenshot includes numbered badges so vision model can refer by index. | browser-use DOM service |
| Self-healing | Navigator retries with same prompt on failure. | On selector/element miss, re-invoke planner with fresh snapshot + explicit "the previous attempt failed because …" context. | Stagehand self-healing |
| Retry + backoff | Linear retry loop. | Exponential backoff (base 1s, factor 2, jitter ±25%), capped at 30s per step. | browser-use |
| Loop detection | None. | Track last N actions in a rolling buffer; if a 2-cycle or 3-cycle repeats 3+ times without snapshot change, pause + escalate. | browser-use |
| Context compaction | None. | `MessageManager` style — pin system prompt, keep last 4 turns verbatim, summarise older turns into a "memory digest" before re-sending to the LLM. | browser-use |
| Hybrid primitives | No `act` / `extract` / `observe` surface. | Add `pkg/nexus/primitives/` with four entry points: act(natural-lang), extract(schema), observe(descriptor), agent(goal). Each delegates to Navigator under the hood but gives integrators deterministic hooks. | Stagehand |
| Coordinate-grounded action path | Nexus actions target `ElementRef` only. | Add `Action{Kind:"coord_click", X, Y}` + a `UITarsAdapter` that issues it when the caller selects a direct-vision model. | UI-TARS |
| Prompt caching | None. | Wrap the shared LLM client with a `PromptCache` layer keyed on (system prompt + N most recent turns hash) with TTL. | Anthropic + Stagehand |
| Rich ticketing | Existing ticketing lacks video timestamp + action trace binding. | Extend `pkg/ticket` so every ticket auto-captures: (a) video filename + mm:ss timestamp, (b) before + after screenshots, (c) LLM reasoning transcript, (d) stack trace, (e) reproduction bank entry auto-generated. | OpenClawing2 §10.3 visual-feedback |

---

## 3. Phased plan — 8 phases, 57 fine-grained tasks

Each phase is a single PR. Each PR carries (a) code, (b) tests in all 10 categories, (c) fixes-validation bank entry, (d) challenge entry, (e) docs update. A phase CANNOT merge until every test category passes locally AND the section-9 campaign is green.

### Phase 1 — OSS submodule vendoring + sandbox (est. 2 days)

Purpose: bring every reference codebase into `HelixQA/tools/opensource/` so the team can inspect the source without leaving the monorepo. No behaviour change yet.

Tasks:
- **P1.T1** — Add `browser-use` as submodule at `tools/opensource/browser-use` (SSH URL). Tracks `main`.
- **P1.T2** — Add `Skyvern` as submodule at `tools/opensource/skyvern`. Tracks `main`.
- **P1.T3** — Confirm `tools/opensource/stagehand` already vendored; freeze at latest tested tag.
- **P1.T4** — Add `UI-TARS-desktop` as submodule at `tools/opensource/ui-tars-desktop`.
- **P1.T5** — Add `anthropic-quickstarts` as submodule at `tools/opensource/anthropic-quickstarts`.
- **P1.T6** — Update `.gitmodules` with SSH URLs + branch pins; add `scripts/sync-opensource-references.sh` that fetches + pulls each.
- **P1.T7** — Add `docs/opensource-references.md` — for each submodule: source URL, version tag, licence, which files HelixQA mirrors patterns from (exact file paths), why the full codebase isn't imported wholesale.
- **P1.T8** — Licence audit: verify each is Apache-2.0 / MIT / BSD compatible with HelixQA's Apache-2.0 headers. Record the matrix in `docs/licences-inventory.md`.
- **P1.T9** — Unit tests: `pkg/opensource/registry_test.go` asserts every `tools/opensource/*` directory has a matching entry in `opensource-references.md`.
- **P1.T10** — Challenge: `OC-REF-001` verifies every vendored submodule has a non-empty `.git` + a `README` + a compatible licence file.

Exit gate: `git submodule status` shows every new entry clean at a pinned SHA; licence matrix signed off; `OC-REF-001` passes.

### Phase 2 — Agent-step state machine foundation (est. 4 days)

Purpose: port browser-use's `Agent.step()` into `pkg/nexus/agent/step.go` as a new state-machine runtime that existing adapters can opt into.

Tasks:
- **P2.T1** — Define `AgentStep` struct: `Evaluation string`, `Memory string`, `NextGoal string`, `Actions []nexus.Action`, `Phase string` (one of `prepare|plan|execute|postprocess`).
- **P2.T2** — Define `AgentState` struct: `TaskGoal`, `History []AgentStep`, `CurrentPhase`, `SessionID`, `Iteration int`.
- **P2.T3** — Implement `Phase1_PrepareContext(ctx, state, adapter)` — mirror of `browser-use._prepare_context`: calls `adapter.Snapshot`, `adapter.Screenshot`, resolves action manifest for the current page.
- **P2.T4** — Implement `Phase2_PlanActions(ctx, state, llm)` — single LLM call returning typed `AgentStep`. Use structured output / JSON schema to force the shape.
- **P2.T5** — Implement `Phase3_Execute(ctx, state, adapter)` — dispatch each action through the adapter, collect `ActionResult`s.
- **P2.T6** — Implement `Phase4_PostProcess(ctx, state)` — update History, run sanity checks (screen state changed? expected outcome matches?), emit telemetry.
- **P2.T7** — Wire all four phases into `Agent.Step()`; `Agent.Run(ctx, goal)` loops until `state.Done` or `state.Iteration >= MaxIterations`.
- **P2.T8** — Interface `LLMClient` extended with `PlanStep(ctx, messages) (AgentStep, error)` so the same call site works for OrchestratorClient + HTTPLLMClient.
- **P2.T9** — Unit tests: every phase returns deterministic outputs for a fixture state; error paths covered.
- **P2.T10** — Integration test: `TestAgent_Run_LoginFlowGreen` drives a fake browser adapter through a multi-step login.
- **P2.T11** — E2E test: against a local test server + Nexus browser engine, the agent navigates home → login → dashboard.
- **P2.T12** — Stress test: `TestAgent_Run_100Iterations` asserts memory stays bounded, goroutine count stable.
- **P2.T13** — Security test: `TestAgent_Run_MaliciousActionsRejected` asserts action manifest whitelist blocks `navigate file://` etc.
- **P2.T14** — DDoS test: concurrent 50 agents run the same flow; rate limiter keeps LLM calls under 200/min.
- **P2.T15** — Benchmark: `BenchmarkAgent_Step` tracks ns/op + allocations per iteration.
- **P2.T16** — Challenge: `OC-AGENT-001` runs the whole loop against a synthetic page bank.
- **P2.T17** — Bank: `banks/openclaw-agent-step.yaml` with 15 entries covering happy path + every error path.
- **P2.T18** — Fixes-validation bank: `banks/fixes-validation-agent-step.yaml` locks in every regression that arises during Phase 2.

Exit gate: all 10 test categories green for the agent package; `OC-AGENT-001` challenge passes on the CI bench.

### Phase 3 — Message manager + context compaction (est. 3 days)

Purpose: port browser-use's `MessageManager` so long-running sessions never blow the LLM context window.

Tasks:
- **P3.T1** — New file `pkg/nexus/agent/messages.go`: `type MessageManager struct{ SystemPrompt, History, Digest }`.
- **P3.T2** — `PrepareStepState(state AgentState)` — builds the full message list for the next LLM call.
- **P3.T3** — `CreateStateMessages(snapshot, screenshot)` — formats the current browser state as a user message.
- **P3.T4** — `Compact(tokenBudget int)` — when the history exceeds 70% of budget, summarise the oldest 80% into `Digest` via a small-model summariser call, keep most recent 20% verbatim.
- **P3.T5** — Tokenizer abstraction via `pkg/nexus/tokens` — pluggable counter (tiktoken-compatible).
- **P3.T6** — Unit tests for compaction threshold, overflow handling, digest invariants (summarised output must reference the AgentState.TaskGoal).
- **P3.T7** — Fuzz target on token counting to guard against regex catastrophic-backtracking issues.
- **P3.T8** — Benchmark: `BenchmarkMessageManager_Compact` tracks ns/kb + alloc count.
- **P3.T9** — Challenge: `OC-AGENT-002` exercises a 500-step session and asserts prompt size stays under 80% of budget.
- **P3.T10** — Bank + fixes-validation entry locking in compaction semantics.

Exit gate: compaction fires within budget in all fixtures; no `ExceedsContext` error across 1000-iteration stress run.

### Phase 4 — Self-healing + exponential backoff + loop detection (est. 3 days)

Purpose: port browser-use's error-recovery stack so the agent never spirals.

Tasks:
- **P4.T1** — `pkg/nexus/agent/retry.go`: `ExpBackoffWithJitter(base, factor, max, jitterPct)`.
- **P4.T2** — `pkg/nexus/agent/loop_detector.go`: rolling N-action buffer + hash-based cycle detection.
- **P4.T3** — `pkg/nexus/agent/self_healer.go`: on action failure, re-invoke planner with `{"previous_attempt_failed_because": "<reason>"}` in the message stream.
- **P4.T4** — Integrate all three into `Agent.Step()` — retry wraps action execution, healer wraps retry-exhausted failure, loop-detector runs before Phase2.
- **P4.T5** — Config: `AgentConfig{MaxRetries, BackoffBase, CycleBufferSize, HealingEnabled}`.
- **P4.T6** — Unit tests per component: retry respects jitter, loop detector catches 2-cycle + 3-cycle, healer escalates after N failed heals.
- **P4.T7** — Integration test: synthetic adapter that fails the first K attempts proves retry+healer recover.
- **P4.T8** — Adversarial test: adapter returns "always stuck on the same screen" — loop detector must trigger within 9 iterations.
- **P4.T9** — Stress test: 1000 sequential sessions with random failure injection — no memory leak, no zombie goroutines.
- **P4.T10** — Security test: confirm retry does not leak secrets into backoff-jitter randomness seed.
- **P4.T11** — Benchmark: retry overhead per action under 200μs.
- **P4.T12** — Challenge: `OC-AGENT-003` guarantees at least three healer invocations trigger in a 20-step flaky fixture.
- **P4.T13** — Bank + fixes-validation entries.

Exit gate: all three mechanisms green; no agent session ever exceeds its configured MaxIterations without raising a clear error.

### Phase 5 — Stagehand primitives (act / extract / observe / agent) (est. 5 days)

Purpose: add the deterministic hybrid primitives so integrators can write scripted flows with AI spots, not all-or-nothing autonomy.

Tasks:
- **P5.T1** — `pkg/nexus/primitives/act.go`: `Act(ctx, session, naturalLang)` — single LLM call maps natural-language → `nexus.Action`.
- **P5.T2** — `pkg/nexus/primitives/extract.go`: `Extract[T any](ctx, session, schema, prompt)` — returns typed struct via JSON schema prompt.
- **P5.T3** — `pkg/nexus/primitives/observe.go`: `Observe(ctx, session, descriptor)` — returns element selector(s).
- **P5.T4** — `pkg/nexus/primitives/agent.go`: wrapper that re-enters the Phase 2 state machine with a scoped goal.
- **P5.T5** — Prompt caching: `pkg/nexus/llm/cache.go` with TTL (5 min default) + hash key.
- **P5.T6** — Self-healing re-inference: when `Act` fails because the selector is stale, silently call `Observe` + retry with the fresh selector.
- **P5.T7** — Unit tests for each primitive (16 tests minimum).
- **P5.T8** — Integration tests against real browser engine.
- **P5.T9** — E2E: scripted `login → navigate → Act("click Save") → Extract(productList)` flow.
- **P5.T10** — Fuzz the prompt cache key function for collision resistance.
- **P5.T11** — Security test: extract schema cannot escape its container — prompt-injection via natural-language input must not pull arbitrary model output into the returned struct.
- **P5.T12** — Stress: 10k primitive calls in rapid succession; cache hit ratio > 40%.
- **P5.T13** — Benchmark: Act < 50ms cached, Extract < 200ms cached.
- **P5.T14** — Challenge: `OC-PRIM-001` .. `OC-PRIM-004` cover each primitive.
- **P5.T15** — Bank + fixes-validation entries.

Exit gate: primitives usable from an external Go script against a real browser; cache measurably reduces token cost.

### Phase 6 — Coordinate-grounded UI-TARS adapter (est. 4 days)

Purpose: add a second action-grounding path so vision-only tasks (including the desktop campaigns OpenClaw's current Nexus stack can't do smoothly) work end-to-end.

Tasks:
- **P6.T1** — Extend `nexus.Action` with `Kind="coord_click"|"coord_type"|"coord_drag"`, `X int`, `Y int`, `ResolutionWidth int`, `ResolutionHeight int`.
- **P6.T2** — `pkg/nexus/coordinate/scale.go`: port Anthropic coordinate-scaling algorithm (`MAX_SCALING_TARGETS`, aspect-ratio match).
- **P6.T3** — `pkg/nexus/browser/coord_driver.go`: execute coord actions via chromedp `Input.dispatchMouseEvent`.
- **P6.T4** — `pkg/nexus/desktop/linux_coord.go`: execute coord actions via existing xdotool path (with B9 guard still in play — X/Y required).
- **P6.T5** — `pkg/nexus/mobile/coord_adb.go`: execute coord actions via `adb shell input tap x y`.
- **P6.T6** — Adapter selector: `AgentConfig.ActionMode = "element"|"coord"|"auto"`. Auto picks element-mode when snapshot has >0 interactive elements, coord-mode otherwise.
- **P6.T7** — LLM prompt templates for coord mode: instruct the model to output `{x, y}` normalised 0..1 floats; adapter scales to screen resolution.
- **P6.T8** — Screenshot overlay helper: `pkg/nexus/visual/grid.go` draws an optional reference grid on screenshots so models that need it can see coordinates.
- **P6.T9** — Unit tests for scaler (resolution matrix), coord driver (event dispatch), ADB tap command shape.
- **P6.T10** — Integration tests on each platform (web, desktop-linux, android).
- **P6.T11** — E2E test: drive a desktop calculator app end-to-end through coord mode only (no element snapshot).
- **P6.T12** — Stress: 1000 random coord clicks — no driver leak, process stable.
- **P6.T13** — Security test: coord values out of screen bounds rejected with clear error.
- **P6.T14** — Benchmark: coord click < 10ms end-to-end on web, < 50ms on desktop.
- **P6.T15** — Challenge: `OC-COORD-001` + `OC-COORD-002` (web + desktop).
- **P6.T16** — Bank + fixes-validation entries.

Exit gate: coord mode closes every demo the DOM mode can't handle; resolution scaling identical to Anthropic reference; no regressions in element mode.

### Phase 7 — Rich ticketing + evidence capture upgrade (est. 3 days)

Purpose: deliver the "better documented tickets with as much material as possible" the user called out explicitly.

Tasks:
- **P7.T1** — Extend `pkg/ticket.Ticket` struct: `VideoTimestamp string` (mm:ss), `BeforeScreenshotPath`, `AfterScreenshotPath`, `LLMReasoningTranscript []string`, `StackTrace string`, `ReproductionBank string`, `SessionID string`, `StepNumber int`.
- **P7.T2** — `pkg/ticket/capture.go`: helper that wraps any `AgentStep` execution; on failure, auto-captures every field above.
- **P7.T3** — Auto-generate a regression bank stub for every new ticket — the ticket carries a YAML template that, once filled + moved into `banks/fixes-validation-*.yaml`, permanently guards the fix.
- **P7.T4** — Markdown ticket renderer: every field rendered with clickable relative paths (file://evidence/frame-00123.png, etc.).
- **P7.T5** — Video frame extraction helper: `pkg/video/extract.go` already exists — extend so every ticket can request `ExtractFrameAt(session, ts)`.
- **P7.T6** — Unit tests for capture + renderer.
- **P7.T7** — E2E: inject a bug in a fixture flow and verify the ticket includes all evidence fields.
- **P7.T8** — Security test: ticket body sanitised via bluemonday-strict before writing to disk (prevents stored-XSS if tickets are ever served over HTTP).
- **P7.T9** — Challenge: `OC-TICKET-001` validates every mandatory field is populated on a known-bad flow.
- **P7.T10** — Bank + fixes-validation entries.

Exit gate: a single failed step produces a ticket that a human reviewer can act on without re-reading any logs.

### Phase 8 — Production-readiness campaign (est. 2 days)

Purpose: prove the full stack works under the Article V "100% coverage, ten categories" rule + two consecutive green section-9 runs.

Tasks:
- **P8.T1** — Update `docs/nexus/helixqa-production-readiness-plan.md` to include the Phase 1–7 deliverables in the acceptance matrix.
- **P8.T2** — `scripts/openclaw-full-campaign.sh`: orchestrates every relevant bank + challenge in one command (agent-step, message-manager, retry/healer/loop-detect, primitives, coord-mode, ticketing).
- **P8.T3** — Benchmark harness: `tests/benchmarks/openclaw/` collecting ns/op + alloc + p95 latency for every new entry point.
- **P8.T4** — Grafana dashboard: `monitoring/openclaw-dashboard.json` with panels for agent-step duration, compaction trigger rate, retry count, loop detection hits, primitive cache hit ratio, coord-mode pixel-distance error.
- **P8.T5** — Run the campaign; capture the FINAL-REPORT + Grafana screenshots + video samples.
- **P8.T6** — Run the campaign a second time 30 min later; the outputs must be green + within noise floor of the first run.
- **P8.T7** — Publish release notes under `docs/releases/v3.0-openclaw-integration.md` with every ported technique + its source attribution + per-phase metrics.
- **P8.T8** — Cut the `v3.0.0` tag; push to every HelixQA remote + bump the Catalogizer superproject.
- **P8.T9** — Update `memory/MEMORY.md` pointing at the full session note so next-session context is warm.

Exit gate: two consecutive green section-9 runs + all ten test categories at 100% on every new package.

---

## 4. Cross-cutting test matrix (enforced per phase)

For every new file / feature:

| Category | Where | Owner |
|---|---|---|
| Unit | `*_test.go` beside source | feature author |
| Integration | `tests/integration/openclaw/` | feature author |
| E2E | `tests/e2e/openclaw/` | feature author |
| Full automation | `scripts/openclaw-full-campaign.sh` | Phase 8 |
| Stress | `tests/stress/openclaw/` | feature author |
| Security | `tests/security/openclaw/` + Semgrep rules | feature author |
| DDoS / rate-limit | `tests/ddos/openclaw/` + k6 scenarios | Phase 6/8 |
| Benchmarking | `tests/benchmarks/openclaw/` | feature author |
| Challenges | `catalog-api/challenges/openclaw_*.go` + `challenges/config/*.json` | feature author |
| HelixQA banks | `banks/openclaw-*.yaml` + `banks/fixes-validation-*.yaml` | feature author |

Each row is a merge blocker. No phase exits until every column is green for the files it touched.

---

## 5. Risks + mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Vision model JSON compliance drifts (LLM returns malformed plans) | Medium | High | LLMsVerifier already scores JSON-compliance; Phase 2.T4 uses structured-output JSON schema; fallback is a single retry before loop-detection fires |
| Upstream OSS repos rename API surfaces between pins | Medium | Medium | Phase 1 pins submodule SHAs + licence matrix; quarterly refresh tracked in `docs/opensource-references.md` |
| UI-TARS local model size exceeds dev-machine RAM | High | Medium | Always default to cloud vision path; make UI-TARS opt-in via `HELIX_UITARS_ENABLED=true`; document recommended hardware (≥24 GB VRAM + 64 GB RAM) |
| Self-healing loops eat the LLM budget | Medium | Medium | Cost tracker in `pkg/llm/cost_tracker.go` already tracks per-phase spend; healer enforces `MaxHealingAttempts=3` per step |
| Coordinate drift across HiDPI / Retina displays | Medium | High | Port Anthropic's `MAX_SCALING_TARGETS` + aspect-ratio match exactly; Phase 6 benchmark logs pixel-distance error on every coord click |
| Prompt cache leakage (stored PII) | Low | Critical | Cache keys are SHA-256 of normalised prompt; cache never persists to disk; TTL capped at 5 minutes; opt-out via `HELIX_PROMPT_CACHE=off` |

---

## 6. Open decisions needing user sign-off

Before Phase 1 starts, confirm:

1. **Submodule URLs.** Do we clone the official upstreams (github.com/browser-use/browser-use, etc.) as read-only references, or fork each into `github.com/vasic-digital/...` first for long-term pin safety?
2. **UI-TARS model hosting.** Default to HuggingFace download on first run, or require operators to pre-fetch into a local cache?
3. **Stagehand dependency.** The existing `tools/opensource/stagehand` is vendored but not wired. Is the plan to re-use its TypeScript primitives via a subprocess shim, or to re-implement the four primitives natively in Go (the plan above assumes native Go)?
4. **Scope cap.** If Phase 8's two-consecutive-green gate fails, do we ship a partial release (v3.0-beta) and iterate, or hold the tag entirely?
5. **Ticketing destination.** Tickets currently land in `qa-results/session-*/tickets/`. Should v3.0 also push tickets into the Linear/Jira integration the orchestrator already knows about, or keep them local?

---

## 7. Timeline summary

| Phase | Duration | Cumulative | Gate |
|---|---|---|---|
| 1. OSS vendoring | 2 d | 2 d | Licence matrix signed |
| 2. Agent step | 4 d | 6 d | All 10 test cats green on pkg/nexus/agent |
| 3. Message manager | 3 d | 9 d | 500-step session stays inside budget |
| 4. Retry + healer + loop | 3 d | 12 d | No runaway loops across 1000 sessions |
| 5. Primitives | 5 d | 17 d | Cache hit >40%, all primitives E2E green |
| 6. Coord mode | 4 d | 21 d | Desktop calculator E2E green in coord mode |
| 7. Rich ticketing | 3 d | 24 d | Ticket-per-failure has all mandatory fields |
| 8. Production campaign | 2 d | 26 d | Two consecutive green section-9 runs |

**Total:** 26 working days of focused engineering (≈5 calendar weeks at a steady pace).

---

## 8. Definition of done

HelixQA v3.0 ships only when:

- Every phase above is committed + pushed to all 4 HelixQA remotes + the 6 Catalogizer superproject remotes.
- `docs/nexus/remaining-work.md` has zero open W / B / P entries.
- Article V "100% coverage, ten categories" column matrix is green for every new package.
- The section-9 campaign runs twice in a row green with non-zero Grafana panels + zero critical alerts.
- `docs/releases/v3.0-openclaw-integration.md` credits every ported technique to its source framework.
- A pretend "day-in-the-life" smoke: an integrator clones a blank project, points HelixQA at it with a 10-line manifest, runs `helixqa autonomous --nexus`, and gets a green report with evidence-backed tickets for every seeded bug — without writing a single project-specific line of HelixQA code.

When all of the above is true, HelixQA is **magical** per the user's brief — production-ready, self-sufficient, and truly generic.
