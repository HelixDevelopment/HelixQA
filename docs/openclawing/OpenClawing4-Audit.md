# OpenClawing Audit Report

Auditor: automated code-review agent (Claude Opus 4.7, 1M ctx) for the writer of `OpenClawing4.md`.
Scope: `Starting_Point.md` (389 lines), `OpenClawing2.md` (623 lines), `OpenClawing3.md` (4204 lines).
Mandate: brutally honest, citation-grade findings. Not an editing pass -- a defect list.
Ground-truth checks: GitHub API (`gh api repos/...`) for every third-party repo reference, plus
spot-checks against the real file layout of browser-use, skyvern, and anthropic-quickstarts.

Summary verdict up front:

- `Starting_Point.md` is a fictional landscape piece. ~half of the hyperlinks are 404, and
  the "OpenClaw" framing is a made-up brand; nothing in it is usable as engineering input.
- `OpenClawing2.md` is a plausible prose-level comparison of real projects
  (anthropic-quickstarts, browser-use, Skyvern, Stagehand, UI-TARS), but every single file path
  it cites into those repos has been checked: some are exactly right, some are wrong,
  and several (including the `browser_use/browser/custom_browser.py` claim that the whole
  "custom browser implementation" section is built on) do not exist. BRIEF-1's demand for
  "which algorithms were used, why" is answered only at a prose level -- no algorithm
  is actually described to the point that it could be ported.
- `OpenClawing3.md` is a grab-bag encyclopaedia. Every technology it names is real, but the
  document is 95% untargeted inventory and 5% hand-wavey "glue this together". Code blocks
  are illustrative pseudocode with multiple real compile-blocking bugs, inconsistent
  language choices (C++/C/TS/Python/ObjC mixed into the same "system"), and an
  implementation plan ("16 weeks, 47 technologies") that is fantasy. Zero analysis of
  how any of this integrates with the actual Go-based HelixQA codebase that OpenClawing4
  has to extend.

OpenClawing4 must therefore (a) salvage the ~30% of OpenClawing3 that is genuinely
useful (capture APIs, hardware encoders, CUDA/TensorRT, OpenCV GPU pipelines) and
(b) throw out the fake-repo premise of Starting_Point + OpenClawing2's fabricated
code paths. The rest of this document enumerates the findings section by section so
OpenClawing4 can cite them directly.

---

## Table of Contents

- [Part A - Starting_Point.md](#part-a---starting_pointmd)
- [Part B - OpenClawing2.md](#part-b---openclawing2md)
- [Part C - OpenClawing3.md](#part-c---openclawing3md)
- [Part D - Cross-document findings & priority list for OpenClawing4](#part-d---cross-document-findings--priority-list-for-openclawing4)

---

## Part A - Starting_Point.md

### A.1 Scope map

`Starting_Point.md` presents itself as "OpenClaw Like Codebases In-Depth Research". It has
three logical sections:

1. **The List** (lines 5-82) -- 25 numbered + categorised projects advertised as
   faster/smaller/better than "OpenClaw". Organised by category
   headings: `Ultra-Lightweight` (9-28), `Security-Focused` (30-39),
   `Multi-Agent & Swarm` (41-46), `Enterprise & Cloud` (48-57),
   `Language Rewrites & Performance` (59-63), `Specialized & Experimental` (65-80).
2. **The brief to the next deliverable** (lines 86-89) -- the text quoted in the audit
   prompt as BRIEF-1.
3. **"Why better?" - per-project comparison** (lines 92-389) -- 25 per-project
   write-ups (baseline `OpenClaw` at 92-98, then NanoClaw..Risk-o-Lobsters)
   followed by a summary comparison table (358-389).

No brief applies to this file; it **is** the seed that BRIEF-1 references.

### A.2 Context/seed provided

- Frames "OpenClaw" as a conversational-AI "gateway" platform with Vue-3 Admin UI, Lit-3
  Control UI, ~420 kLoC TypeScript, "Lane Queue" isolation, "Gateway Server on port
  18789" (line 95-97).
- Characterises each alternative by language, LoC, UX surface (CLI/TUI/Web/Telegram/etc.),
  and a single-sentence "winning factor vs OpenClaw".
- Produces a 25-row comparison table (360-385) keyed by Language / UX Approach / Flow
  Control Strength / Key Advantage.

Note for OpenClawing4: **none of this is concrete technical ground-truth**. "OpenClaw" is
not a real open-source project with those numbers; it is a persona the document invented.
If OpenClawing4 is meant to drive real engineering inside HelixQA, Starting_Point cannot
be used as a factual baseline.

### A.3 Third-party references catalog (with 404 audit)

Every GitHub URL in the file was checked via `gh api`. **Status column:**
`OK` = repo exists; `MISS` = 404 as of audit date; `MISMATCH` = URL changes between the
"List" and the "Why better?" sections of the file.

| # | Line(s) | Project | URL given | Status |
|---|---------|---------|-----------|--------|
| 1 | 12 | NanoClaw | github.com/theonlyhennygod/nano-claw | **MISS** |
|   | 108 | NanoClaw (restated) | github.com/nickpourazima/nanoclaw | **OK** (MISMATCH with line 12) |
| 2 | 14 | Nanobot | github.com/HKUDS/nanobot | **OK** |
|   | 119 | Nanobot (restated) | github.com/nanobot-ai/nanobot | **OK** (MISMATCH with line 14) |
| 3 | 16 | PicoClaw | github.com/sipeed/picoclaw | **OK** |
| 4 | 18 | ZeroClaw | github.com/theonlyhennygod/zeroclaw | **OK** (fork, 85 stars; the real upstream is `zeroclaw-labs/zeroclaw` ~30k stars) |
|   | 141 | ZeroClaw (restated) | github.com/openagen/zeroclaw | **OK** (third different URL for same project) |
| 5 | 20 | NullClaw | github.com/nullswan/nullclaw | **MISS** |
|   | 152 | NullClaw (restated) | github.com/nullclaw/nullclaw | **OK** (MISMATCH with line 20) |
| 6 | 22 | Clank | npm @tractorscorch/clank | not checkable via gh (npm only) |
| 7 | 24 | ZeptoClaw | github.com/qhkm/zeptoclaw | **OK** |
| 8 | 26 | PaeanClaw | npm paeanclaw | not checkable via gh |
| 9 | 28 | GoGogot | github.com/aspasskiy/GoGogot | **OK** |
| 10 | 33 | SafeClaw | github.com/princezuda/safeclaw | **OK** |
| 11 | 35 | Moltis | github.com/moltis-org/moltis | **OK** |
| 12 | 37 | IronClaw | github.com/nearai/ironclaw | **OK** |
| 13 | 39 | Hermes Agent | github.com/missingbytes/hermes | **MISS** |
|   | 238 | Hermes Agent (restated) | github.com/mudrii/hermes-agent-docs AND github.com/NousResearch/hermes-agent | both **OK** (but neither matches line 39) |
| 14 | 46 | Moltworker | github.com/moltworkerai/moltworker | **MISS** |
|   | 248 | Moltworker (restated) | github.com/cloudflare/moltworker | **OK** (MISMATCH with line 46) |
| 15 | 55 | Anything LLM | github.com/Mintplex-Labs/anything-llm | **OK** |
| 16 | 57 | memU Bot | github.com/NevaMind-AI/memU | **OK** |
|   | 279 | memU Bot (restated) | github.com/NevaMind-AI/memUBot | **OK** (different repo - unclear which describes the actual "Bot") |
| 17 | 68 | AutoClaw | npm autoclaw | not checkable via gh |
|   | 290 | AutoClaw (restated) | github.com/tsingliuwin/autoclaw | **OK** |
| 18 | 70 | DumbClaw | github.com/pablomarquezhaya/DumbClaw | **MISS** |
|   | 301 | DumbClaw (restated) | github.com/chrischongyj/dumbclaw | **OK** (MISMATCH with line 70) |
| 19 | 72 | PycoClaw | github.com/cnlohr/pycoclaw | **MISS** |
|   | 312 | PycoClaw (restated) | github.com/jetpax/pycoclaw | **OK** (MISMATCH with line 72) |
| 20 | 74 | ClawBoy | github.com/ClawBoy/ClawBoy | **MISS** (the "Why better?" body at 322 says "Not a public GitHub; distributed by Tencent", contradicting line 74 which gives a URL) |
| 21 | 76 | BabyClaw | github.com/shadanan/babyclaw | **MISS** |
|   | 333 | BabyClaw (restated) | github.com/sudhamabhatia/babyclaw | **OK** (MISMATCH with line 76) |
| 22 | 78 | Clawlet | github.com/0xConnor/Clawlet | **MISS** |
|   | 344 | Clawlet (restated) | github.com/mosaxiv/clawlet | **OK** (MISMATCH with line 78) |
| 23 | 80 | Risk-o-Lobsters | github.com/jpoley/risk-o-lobsters | **OK** |
| 24 | 82 | awesome-claws | github.com/machinae/awesome-claws | **OK** |

**Result: 9 of 24 primary links 404; 10 projects have mutually contradictory URLs between
the "List" and the "Why better?" sections.**

### A.4 Red flags

- **R1 (CRITICAL)**: The "baseline" (line 92-98) attributes to OpenClaw specific
  technical facts (`Vue 3 + Lit 3 + Vite`, `~420,000 lines of TypeScript`,
  `Gateway Server on port 18789`, `Lane Queue for session isolation`) with zero
  citation. If those numbers are wrong (and we have no way to verify them), every
  comparative conclusion in the file collapses.
- **R2**: 38% broken-link rate combined with URL mismatches between sections strongly
  suggests the list was generated rather than curated. A writer building
  OpenClawing4 on top of this is building on sand.
- **R3**: Specific factual claims that are either unverifiable or trivially wrong at
  face value:
  - Line 15: "PicoClaw... $10 single-board computers" - generic marketing.
  - Line 25: "PaeanClaw claims to be 1,150x smaller than OpenClaw" - cited as fact in
    the summary table (line 369) without any source.
  - Line 129: "400x faster startup than OpenClaw and 99% less memory usage" - no
    measurement, no methodology.
  - Line 278: "memU Bot...reduce token usage by up to 90%" - quoted as fact with no
    source, no workload.
  - Line 147: "678 KB static Zig executable" for NullClaw, then line 149 adds
    "5,300+ tests, 50+ providers, 19 channels" - numbers that would be extraordinary
    for a 678KB binary, no citation.
- **R4**: Lines 232-238 claim Hermes Agent's core is at `NousResearch/hermes-agent`;
  NousResearch is a real org but typically publishes LLMs (Hermes 2/3 models), not
  agent harnesses. This is almost certainly conflating a model series with a separate
  agent project.
- **R5**: Table row "ClawBoy" describes it as both "C++ port of the classic OpenClaw
  engine" (line 73, referencing the actual historic Captain Claw/OpenClaw game engine)
  AND (line 317-322) "WeChat integration...tailored for the Chinese market". These are
  two entirely different projects collapsed into one entry.

### A.5 Takeaway for OpenClawing4

Do not cite Starting_Point.md as a source. If OpenClawing4 needs a landscape survey,
rebuild it from verified projects only -- the OpenClawing2/OpenClawing3 reference set
(browser-use, Skyvern, Stagehand, UI-TARS, Anthropic computer-use-demo, and the OSS
tech in Part C) is the usable subset.

---

## Part B - OpenClawing2.md

BRIEF-1 (quoted in audit prompt): "Now we need detailed comparison of each project source code vs OpenClaw
in area of full control of application's UI / UX and whole flows! Exact codebase references, notes and
detailed analysis and explanations! Where they are winning, why and how much! Focus is on codebase,
exact source code files and classes and methods references. Which algorithms were used, why?
What they get with it vs OpenClaw? Also which 3rd party libraries, components, services and other
things are used with exact references and explanations + links to the GitHub repos. We need to port the
best ways of autonomous navigation through the applications and services, autonomously driving UI/UX and
executing the whole application's flows smooth and flawlessly with no glitches in UI/UX/The whole flows
interaction(s)!"

### B.1 Scope map

1. **Executive Summary** (lines 7-29) -- key findings, strategic rec, "Winners by Category".
2. **Methodology and Baseline** (lines 31-69) -- evaluation criteria and OpenClaw
   baseline described via specific file paths (`src/agents/pi-embedded-runner.ts`,
   `src/agents/pi-tools.ts`, `src/agents/bash-tools.exec.ts`, `src/agents/openclaw-tools.ts`).
3. **Deep Dive: Anthropic `computer-use-demo`** (lines 71-135).
4. **Deep Dive: `browser-use`** (lines 137-227).
5. **Deep Dive: `Skyvern`** (lines 229-279).
6. **Deep Dive: `Stagehand`** (lines 281-347).
7. **Deep Dive: `UI-TARS`** (lines 349-403).
8. **Comparative Analysis** (lines 405-504) -- 4 table-driven comparison matrices:
   Agent Loop (413-420), Element Detection (438-444), Action Execution (462-468),
   Error Handling (486-492).
9. **3rd Party Libraries / Services / Ecosystem** (lines 506-572) -- 3 tables covering
   browser libs, LLM providers, cloud services.
10. **Strategic Recommendations** (lines 574-624) -- High / Medium / Long-Term priorities.

### B.2 Brief-coverage matrix (BRIEF-1)

Each distinct ask in BRIEF-1 is scored: **covered** / **partial** / **missing** / **wrong**.

| # | Brief phrase | Coverage | Evidence / gap |
|---|-------------|----------|----------------|
| B1.1 | "detailed comparison of each project source code vs OpenClaw" | **partial** | Prose comparison present, but 4 of 25 Starting-Point projects carried over and many dropped without explanation. Starting_Point.md lists 25 alternatives; OpenClawing2 reviews only 5 (Anthropic demo, browser-use, Skyvern, Stagehand, UI-TARS). Not a bug - the 5 are the defensible ones - but the doc never explains the culling. |
| B1.2 | "Exact codebase references" | **partial+wrong** | Files named at module level (e.g. `browser_use/agent/service.py`) but some do not exist: `browser_use/browser/custom_browser.py` (line 189-191) is fabricated (see B.3 below). `skyvern/agent/prompts.py` (line 241) is fabricated (Skyvern has no `agent/` directory). |
| B1.3 | "notes and detailed analysis and explanations" | **covered (prose)** | Lots of prose. Depth is uneven and some technical claims are wrong or unverifiable (see B.6). |
| B1.4 | "Where they are winning, why and how much" | **partial** | "Why" is answered at the hand-wave level ("X is more robust"). "How much" - quantitative winning - is mostly missing. The only numeric claim is "44% improvement in performance on complex DOM interactions" for Stagehand v3 CDP (line 323), with no source. |
| B1.5 | "Which algorithms were used, why?" | **missing** | BRIEF-1 specifically asked for algorithms. Doc names approaches (DOM filtering, state machine, iterative loop, coordinate grounding, exponential backoff+jitter) but does not describe a single algorithm concretely enough to port. No pseudo-code, no complexity, no parameters. Closest attempt is "loop detection and prevention mechanism" (494-504), which is described as "keeping track of the agent's history and checking for repeating patterns" - insufficient to implement. |
| B1.6 | "What they get with it vs OpenClaw?" | **covered (qualitative)** | Advantages sections under each project (§3.3, §4.5, §5.4, §6.4, §7.4). Purely qualitative - e.g. "Superior DOM-based reliability over pure vision" (§4.5.1) - no benchmarks. |
| B1.7 | "3rd party libraries, components, services...exact references and...links to the GitHub repos" | **partial** | Table at 513-520 gives library names and versions, but **no GitHub URLs** anywhere in the body. 2captcha (§4.4.2, §9.3.2) is named repeatedly without a URL. Vercel AI SDK (§6.3.3) - no URL. Browserbase (§9.3.1) - no URL. This is a direct brief violation. |
| B1.8 | "port the best ways of autonomous navigation..." | **partial** | Strategic Recommendations §10 outline what to port (`Agent.step()`, Playwright/CDP, act/extract/observe, prompt caching, self-healing, visual feedback, plugin system). No implementation plan, no integration with HelixQA's existing Go codebase. |
| B1.9 | "executing the whole application's flows smooth and flawlessly with no glitches" | **missing** | No treatment of glitch types, no test/verification framework, no jank/frozen-frame/animation analysis despite HelixQA CLAUDE.md having strong rules around exactly that. |

Net coverage: roughly **60%**. Prose quality is adequate, but the brief's loudest
demands (exact algorithms, GitHub URLs, quantified winning) are unmet.

### B.3 Codebase references catalog (HelixQA-side & target-repo-side)

**HelixQA/OpenClaw side** (the baseline file paths the document declares for
"OpenClaw"):

| Line | Path | Verdict |
|------|------|---------|
| 59-61 | `src/agents/pi-embedded-runner.ts` | Cannot be verified - "OpenClaw" as described is not a real open-source repo we have access to. If this is meant to be OpenClaw AI (by GenSpark etc.), the real repo has a different layout. If it is meant to be HelixQA itself (which is Go, not TS), then this is fiction. |
| 61 | `src/agents/bash-tools.exec.ts` | Same - unverifiable. Filename pattern `bash-tools.exec.ts` is not typical for any real TS project. |
| 63-65 | `src/agents/pi-tools.ts`, `src/agents/openclaw-tools.ts` | Same. |

Given HelixQA is a Go module (see `HelixQA/pkg/` listing in the codebase), these TS
paths cannot refer to HelixQA. The whole baseline section is therefore addressing a
different project ("OpenClaw" as a TypeScript gateway) that nothing in Catalogizer
actually builds on. **This is the single largest framing problem for OpenClawing4**:
the entire "port this into OpenClaw" recommendation doesn't match what HelixQA is.

**Target-repo side** (file paths inside real OSS projects):

| Line | Claimed path | Real? | Notes |
|------|--------------|-------|-------|
| 75 | `computer_use_demo/loop.py` | **YES** | Confirmed: `anthropics/anthropic-quickstarts/computer-use-demo/computer_use_demo/loop.py` exists, 13121 bytes. |
| 83 | `computer_use_demo/tools/__init__.py` (`ToolCollection`) | **YES** | Confirmed present. |
| 95 | `computer_use_demo/tools/bash.py` | **YES** | Real file. |
| 99 | `computer_use_demo/tools/edit.py` | **YES** | Real file. |
| 103 | `computer_use_demo/tools/computer.py` | **YES** | Real file. |
| 142 | `browser_use/agent/service.py` | **YES** | Confirmed: 161600 bytes. |
| 170-175 | "browser_use...custom DOM processing service...injected via Playwright's `page.evaluate()`" | **partial** | The service exists, but it lives in `browser_use/dom/` (not under `browser/`). The subsequent claim at line 189-191 that there is `browser_use/browser/custom_browser.py` defining a `CustomBrowser` class is **FABRICATED** - the directory `browser_use/browser/` exists, but the actual files are `session.py`, `session_manager.py`, `events.py`, `profile.py`, `video_recorder.py`, `views.py`, `demo_mode.py`, `python_highlights.py`, `watchdog_base.py`, plus `cloud/` and `watchdogs/`. There is no `custom_browser.py` and no `CustomBrowser` class. **Section 4.3.2 (189-195) describes a class that does not exist.** |
| 185-187 | "Controller class" | **YES** in concept - `browser_use/controller/__init__.py` does exist - but file has only `__init__.py`, not a dedicated `controller.py` with a `Controller` class at the path implied by section 4.3.1. The actual controller logic is spread across other modules. |
| 241 | `skyvern/agent/prompts.py` | **NO** | Fabricated. Skyvern has no `agent/` subdirectory at all. Top-level `skyvern/` contains `cli/`, `client/`, `config.py`, `constants.py`, `core/`, `errors/`, `exceptions.py`, `experimentation/`, `forge/`, `library/`, `schemas/`, `services/`, `utils/`, `webeye/`. Prompts in real Skyvern live under `skyvern/forge/prompts/` and related sub-dirs. |
| 261 | `skyvern/webeye/browser_manager.py` | **YES** | Confirmed 2651 bytes. |
| 265 | `skyvern/forge/api_app.py` | **YES** | Confirmed 12050 bytes. |

Also cited without paths (unverifiable):
- `AgentState`, `MessageManager`, `PlanEvaluate` (§10.1.1). These names look plausible
  for browser-use internals but the actual classes are `MessageManager` (exists),
  `AgentState` (Pydantic model in `agent/views.py`), and `PlanEvaluate` **does not
  exist**. What exists is `self.state.next_goal_evaluation` or similar; the
  "PlanEvaluate cycle" described at 426-428 is not a class.
- `ToolCollection`, `BaseAnthropicTool`, `BetaToolResultBlockParam` - all real.
- `AgentOutput`, `ActionResult` - real browser-use Pydantic models.

### B.4 Third-party references catalog (for BRIEF-1 deliverable)

| Lib/service | Mentioned | GitHub URL in doc? | Real? |
|-------------|-----------|-------------------|-------|
| `@mariozechner/pi-agent-core` (line 61) | yes | no | Real npm pkg, but unusual that OpenClawing2 attributes this as OpenClaw's core engine - `pi-agent-core` is from Mario Zechner's `pi` project; if that is what is meant by "OpenClaw" the rename is confusing. |
| Playwright (various) | yes | no | Real, version `^1.40.0` claimed. Microsoft repo. |
| Puppeteer | yes | no | Real. |
| Selenium | yes | no | Real. |
| Chrome DevTools Protocol (CDP) | yes | no | Real spec, not a repo. |
| Vercel AI SDK | yes | no | Real (github.com/vercel/ai). |
| LangChain | yes | no | Real. |
| litellm | yes | no | Real (BerriAI/litellm). |
| `xdotool`, `scrot`, `gnome-screenshot`, `ImageMagick`, `pyautogui` | yes | no | Real. |
| 2captcha (§4.4.2, §9.3.2) | yes | no | Real commercial API. |
| Browserbase (§9.3.1) | yes | no | Real SaaS, open-source Stagehand owns it. |
| UI-TARS (ByteDance, §7) | yes | no | Real; HF model + github repo. |
| OpenAI / Anthropic / Google SDKs | yes | no | Real. |
| Hugging Face `transformers` (§9.2.3) | yes | no | Real. |

Every named library is real. **BRIEF-1 explicitly asked for GitHub links. None are
provided anywhere in OpenClawing2.** This is a direct brief violation and must be
fixed in OpenClawing4's citations.

### B.5 Algorithm / technique claims

| # | Line | Claim | Status |
|---|------|-------|--------|
| BT-1 | 81-85 | `sampling_loop` in Anthropic demo: per-turn cycle of API call + tool dispatch + feedback via `_make_api_tool_result` | **accurate** and testable against `loop.py`. |
| BT-2 | 119 | ComputerTool coordinate-scaling "finds target resolution from a predefined set `MAX_SCALING_TARGETS` that has a similar aspect ratio" | **accurate** - `MAX_SCALING_TARGETS` exists in `tools/computer.py`. |
| BT-3 | 170-175 | browser-use DOM processing is injected via `page.evaluate()` | **accurate** at high level. |
| BT-4 | 175 | "highlighting system that adds a colored border and a numbered label to each interactive element" | **accurate**. |
| BT-5 | 201-203 | "exponential backoff strategy to increase the delay between retries" | **accurate** in spirit, but doc has not stated the base/cap/multiplier, so not portable. |
| BT-6 | 205 | 2captcha integration "built-in" | **partially wrong** - in current browser-use, captcha solving is mostly via configuration / agent prompt; there is not a first-class `2captcha` plugin "built in" as the text implies. |
| BT-7 | 239-243 | Skyvern "Observe-Plan-Act" state machine | **plausible** but the description fits many agent frameworks; no Skyvern-specific code path is shown. |
| BT-8 | 255 | Skyvern overlays bounding boxes "on the screenshot and labeling them with IDs" | **true** of Skyvern. |
| BT-9 | 283-295 | Stagehand hybrid "AI + code" with `act`/`extract`/`observe`/`agent` primitives | **accurate**. |
| BT-10 | 323 | Stagehand v3 "CDP-native, 44% improvement in performance on complex DOM interactions" | **suspicious** - no source; Stagehand's blog has discussed perf wins but the exact "44%" needs citation or OpenClawing4 should drop it. |
| BT-11 | 355-359 | UI-TARS "native agent model" that outputs pixel coordinates directly | **accurate**. |
| BT-12 | 361-363 | UI-TARS 1.5 "reinforcement learning" enables "thinking through actions" | **partially wrong** - UI-TARS uses RL for reasoning, but the "thinking through before taking" is a prose gloss; the paper describes DPO-style preference training and reward-model RL, not a test-time CoT "thinking". OpenClawing4 should clarify. |
| BT-13 | 381-387 | UI-TARS-desktop uses pyautogui + OCR | **accurate**. |
| BT-14 | 426-428 | browser-use `PlanEvaluate` cycle | **misidentified** - no class named that; evaluation is inside `AgentState` message structure. |
| BT-15 | 494-504 | Loop detection in browser-use via "history analysis" | **accurate at concept level**; no algorithm specified. |
| BT-16 | 498-500 | Stagehand self-healing via "re-inference" | **accurate**. |
| BT-17 | 472 | Playwright's auto-waiting vs xdotool manual delays | **accurate**. |
| BT-18 | 478-480 | "Asynchronous execution" contrast | **accurate** but superficial - the distinction Playwright-async vs xdotool-sync has much deeper consequences (event-driven vs. polling) that are not drawn out. |

**Net**: ~70% of algorithm statements survive scrutiny; the rest are either
misidentified classes or unsourced numbers. Nothing is described concretely enough to
re-implement.

### B.6 Inconsistencies

- **BI-1**: Baseline section declares OpenClaw as TypeScript (`src/agents/*.ts`) but
  the whole Catalogizer/HelixQA ecosystem is Go. OpenClawing2 never reconciles this.
- **BI-2**: §4.3.2 promises a "custom browser implementation `browser_use/browser/custom_browser.py`"; §10.1.2 then recommends "integrate a Playwright/CDP-driven browser context". The first implies browser-use already wraps Playwright in a custom class at that specific file; the second talks about adding Playwright -- which suggests the author forgot what they wrote in §4.
- **BI-3**: Tables at 438-444 and 486-492 claim UI-TARS has "Basic (Retry on action failure)" error handling and "Retry on action failure" retry logic, but §7 itself does not show a retry path in UI-TARS. The rows may have been filled in by pattern rather than source inspection.
- **BI-4**: §9.2 LLM provider table claims browser-use uses "`langchain` / `litellm`". browser-use primarily uses its own lightweight LLM wrapper (`browser_use/llm/`) and does NOT hard-depend on LangChain. LiteLLM is an optional provider. This overstates coupling.
- **BI-5**: §6.3.2 claims Stagehand has "prompt caching" -- this is accurate at the LLM provider layer (Anthropic prompt caching), but the text implies Stagehand itself implements a caching layer, which is not how its `PromptCaching` actually works.

### B.7 Gaps vs. BRIEF-1

- **BG-1**: "GitHub repos" links - zero links in entire document.
- **BG-2**: "Which algorithms were used, why" - named but not specified to the level required for porting.
- **BG-3**: "How much" winning - no quantitative numbers except one unsourced "44%".
- **BG-4**: "with no glitches in UI/UX/The whole flows interaction(s)" - the document
  never addresses glitch detection (frame stagnation, frozen animations, mis-rendered
  UI) that HelixQA's own CLAUDE.md obsesses about. BRIEF-1 explicitly mentioned "no
  glitches"; OpenClawing2 does not cover it.
- **BG-5**: No treatment of mobile platforms. BRIEF-1 says "autonomous navigation
  through the applications and services" - the applications under test include
  Android phone, Android TV, iOS. OpenClawing2 is 100% desktop/web. No
  XCUITest/UIAutomator/Appium/scrcpy discussion. OpenClawing3 partially covers this
  but OpenClawing2 does not.
- **BG-6**: No treatment of TUI / API / desktop-native apps.

### B.8 Red flags

- **BR-1**: Fabricated code path `browser_use/browser/custom_browser.py` (line 189-191).
  An entire subsection (4.3.2) is built on a class that does not exist. OpenClawing4
  should recheck every file path in OpenClawing2 against the live repos before
  citing them.
- **BR-2**: Fabricated path `skyvern/agent/prompts.py` (line 241).
- **BR-3**: "PlanEvaluate" class (§10.1.1, and referenced in §8.1.2) - not a real
  browser-use class.
- **BR-4**: OpenClaw baseline described in TS despite HelixQA being Go. Any
  "port this into OpenClaw" recommendation is pointed at the wrong target.
- **BR-5**: Unsourced numeric claims: "44% improvement" (323), "browser-use...
  production-ready blueprint" (584) - implicit claims about production maturity that
  would need benchmarks.
- **BR-6**: Zero threat model / safety discussion. BRIEF-1 asked for "Where they are
  winning", but the doc never scores security / sandboxing / anti-prompt-injection
  posture even though 2captcha integration (an adversarial-use feature) is mentioned.
- **BR-7**: No mention of observability / logs / tracing in any of the target
  frameworks, despite HelixQA having strict "Real-Time Log Monitoring" rules.

---

## Part C - OpenClawing3.md

BRIEF-2 (quoted in audit prompt): "We need in-depth research of all innovative ways to extend
capabilities from the document to more advanced level! We need to add more technologies which
are not strictly LLM or Vision models, or AI per se! For example how we could rise everything
from the research to another ultimate level by bringing in OpenCV heavy use and all related and
similar technologies! For example technologies, libraries, components and services which can
work on very low level real-time with high performance! Can we bring heavy use of Vulkan and
OpenGL? Can we use CUDA technology from our RTX graphics card? What else can be brought in as
a game changer? Run all possible researches on this theme and do comprehensive integration plan!
We MUST have the system once all this is integrated which can fully autonomously in real time
smoothly interact and use UI/UX and whole flows of any type of applications: Web, Mobile,
Desktop, even APIs or TUI! We MUST extend capabilities of recording of all this in real time in
high resolution, real time processing and screenshot obtaining and in-depth analysis in real time
through the pipelines and hooks we can attach if/when needed! Make sure that every single source
code file is referenced, exact ideas, how we would extend these, what game changer things we can
do, how exactly, and in-depth full step by step guides to the smallest atoms level of precision!"

### C.1 Scope map

15 numbered top-level sections plus 4 appendices.

| # | Section | Lines | Claims to deliver |
|---|---------|-------|-------------------|
| 1 | Executive Summary | 31-74 | Vision, "game-changer" tech matrix, expected outcomes (<16ms reaction, 4K@60fps recording) |
| 2 | Game-Changer Technology Overview | 76-133 | 8-layer classification ASCII tree |
| 3 | OpenCV Heavy Use - Real-Time CV Pipeline | 137-404 | `GPUAnalysisPipeline`, OpenCL fallback, DOM+vision hybrid |
| 4 | Vulkan & OpenGL GPU-Accelerated Processing | 408-632 | Sobel-edge compute shader, Vulkan pipeline, MoltenVK |
| 5 | CUDA & RTX GPU Compute for Real-Time Inference | 636-899 | TensorRT engine, NVIDIA Maxine SDK |
| 6 | Low-Level OS-Specific Technologies | 903-1375 | DXGI (Win), KMS/DRM+DMA-BUF (Linux), ScreenCaptureKit+AX (macOS) |
| 7 | Real-Time Screen Capture Architectures | 1379-1482 | `ICaptureEngine` abstraction, platform matrix |
| 8 | High-Performance Recording & Streaming | 1485-1772 | OBS-style pipeline, FFmpeg file output, WebRTC/WHIP |
| 9 | Hook & Interception Systems | 1776-2063 | LD_PRELOAD, plthook, Windows LL hooks |
| 10 | Cross-Platform Input Simulation | 2067-2345 | uinput (Linux), SendInput+FakerInput (Windows) |
| 11 | TUI Automation | 2349-2656 | PTY + xterm-headless, pattern find/navigate |
| 12 | Mobile Device Automation | 2660-3095 | scrcpy protocol, UIAutomator2, WebDriverAgent |
| 13 | Complete Integration Architecture | 3099-3238 | Box-drawing diagram |
| 14 | Step-by-Step Implementation Guide | 3243-3845 | 8 phases over 16 weeks |
| 15 | Source Code Reference Map | 3849-3919 | 7 sub-tables of real file paths |
| A | Performance Benchmarks | 3922-3945 | Target-latency table |
| B | Security Considerations | 3948-3963 | Permission matrix |
| C | Complete Technology Stack | 4098-4161 | 47-row tech inventory |
| D | Recommended Hardware | 4164-4197 | 3 sample configs |

The file also has a "end of document" marker at line 3966, followed by an
"Additional Game-Changer Technologies" section (3974-4094) that re-describes
Maxine, ROCm, OpenVINO, FFmpeg filter graph, PipeWire, GStreamer. The Maxine section
here is a **duplicate** of §5.2 (805-899) under a different file path claim.

### C.2 Brief-coverage matrix (BRIEF-2)

| # | Brief phrase | Coverage | Evidence / gap |
|---|-------------|----------|----------------|
| C2.1 | "all innovative ways to extend capabilities from the document [OpenClawing2] to more advanced level" | **partial** | No connective tissue to OpenClawing2. The document never cites a specific line or section of OpenClawing2 to extend. It stands alone as a capability catalogue. |
| C2.2 | "more technologies which are not strictly LLM or Vision models, or AI per se" | **covered** | OpenCV, Vulkan, CUDA, DXGI, KMS/DRM, evdev, LD_PRELOAD, PTY, scrcpy etc. all non-LLM. |
| C2.3 | "heavy use of OpenCV...technologies, libraries...very low level real-time with high performance" | **covered (inventory) / missing (integration)** | OpenCV GPU pipeline pseudocode at 154-238. Does not show how the Mat crosses from e.g. DXGI shared-handle into `cv::cuda::GpuMat` without a copy -- the integration path is implied, not implemented. |
| C2.4 | "Can we bring heavy use of Vulkan and OpenGL?" | **partial** | Vulkan compute shader + host pipeline shown (416-618). No OpenGL fallback code, despite "Vulkan & OpenGL" being the section title. Conspicuous gap. |
| C2.5 | "Can we use CUDA technology from our RTX graphics card?" | **covered** | TensorRT engine (640-803), Maxine (805-899), NVENC/NVFBC (tables). |
| C2.6 | "What else can be brought in as a game changer?" | **covered (listing)** | DMA-BUF, PipeWire, scrcpy, FakerInput, WebRTC/WHIP, PaddleOCR, etc. |
| C2.7 | "comprehensive integration plan" | **partial** | Phases listed (Phase 1-8 @ 3243-3845), but the plan is shell-snippet breadcrumbs, not a buildable plan. No dependency graph, no per-phase acceptance criteria, no test plan. |
| C2.8 | "fully autonomously in real time smoothly interact and use UI/UX and whole flows of any type of applications: Web, Mobile, Desktop, even APIs or TUI" | **partial** | Web (browser automation §13 diagram), Mobile (§12), Desktop (§10), TUI (§11) - all touched. **API** automation is mentioned once in a diagram node (line 3790) but has no actual content. No section explains how this integrates with the agent loop described in OpenClawing2. |
| C2.9 | "extend capabilities of recording...in high resolution, real time processing and screenshot obtaining" | **covered** | §7, §8 cover capture + encode + segment recording. |
| C2.10 | "in-depth analysis in real time through the pipelines and hooks we can attach if/when needed" | **partial** | Hooks covered (§9). The hook_event -> OpenClaw connection is a hand-wave ("forward to OpenClaw for analysis", line 2055-2057); no bus / schema / back-pressure design. |
| C2.11 | "every single source code file is referenced" | **partial / overstated** | §15 has 7 sub-tables with real file paths (e.g. `libobs/obs-video.c`), but they are sparse - the OpenCV section alone references 6 real files but represents thousands of real files. "Every single" is hyperbole and untrue. |
| C2.12 | "in-depth full step by step guides to the smallest atoms level of precision" | **missing** | §14 is shell snippets + cmake flags. It is not a step-by-step build guide; it is a kitbash.  Example: "Step 1.1: Build the Capture Engine" (3247-3268) is literally 3 git-clone commands and a cmake command with no parent directory assumed, no error handling, no verification step, and flags that do not match real OBS/KMS build systems. |
| C2.13 | "atoms level of precision" | **missing** | No such precision anywhere. |

Net coverage: roughly **55%**. Breadth is impressive, depth almost absent.

### C.3 Codebase references catalog

#### C.3.1 Self-invented HelixQA-side paths

OpenClawing3 invents **an entire new source tree** under `src/...` that does not
exist in HelixQA (HelixQA is Go, organised under `pkg/` - see real tree:
`pkg/capture`, `pkg/vision`, `pkg/opencv`, `pkg/video`, `pkg/streaming`, etc.).

Every one of the following paths is fabricated:

| Line | Invented path | What it would be in real HelixQA |
|------|---------------|----------------------------------|
| 155 | `src/vision/gpu_pipeline.cpp` | C++ file in a Go module - would need cgo. `pkg/opencv/` exists but has no C++ files of this name. |
| 243 | `src/vision/opencl_pipeline.cpp` | Same. |
| 298 | `src/agents/vision-dom-hybrid.ts` | TS file in a Go module - does not fit. |
| 417 | `src/shaders/screen_analysis.comp` | Vulkan GLSL shader - plausible, but no build system hooked up. |
| 494 | `src/vision/vulkan_compute.cpp` | C++/Vulkan - not present. |
| 645 | `src/inference/tensorrt_engine.cpp` | Not present. |
| 811 | `src/recording/maxine_enhancer.cpp` | Not present. |
| 910 | `src/capture/windows/dxgi_duplicator.cpp` | Not present. |
| 1072 | `src/capture/linux/kms_capture.c` | Not present. Also filed inside a `namespace openclaw { namespace capture { ... }}` block (lines 1091-1249) -- **C does not support namespaces**. The file extension and the syntax contradict each other. (See C.6.) |
| 1257 | `src/capture/macos/ScreenCaptureKitBridge.m` | Not present. |
| 1384 | `src/capture/capture_engine.hpp` | Not present. |
| 1493 | `src/recording/libobs_pipeline.cpp` | Not present. |
| 1704 | `src/recording/webrtc_output.cpp` | Not present. |
| 1784 | `src/hooks/linux/ld_preload_interceptor.c` | Not present. |
| 1902 | `src/hooks/linux/plt_hook_runtime.c` | Not present. |
| 1956 | `src/hooks/windows/ll_hooks.cpp` | Not present. |
| 2074 | `src/input/linux/uinput_controller.cpp` | Not present. |
| 2247 | `src/input/windows/sendinput_controller.cpp` | Not present. |
| 2356 | `src/tui/pty_controller.ts` | Not present. |
| 2666 | `src/mobile/android/scrcpy_controller.py` | Not present. |
| 2995 | `src/mobile/ios/ios_controller.py` | Not present. |
| 3532 | `src/recording/screenshot_pipeline.cpp` | Not present. |
| 3644 | `src/tui/pty_service.ts` | Not present. |
| 3754 | `src/api/unified_automation.ts` | Not present. |

The file treats these as if they were existing reference code. Nothing in HelixQA
maps onto them. **OpenClawing4 must make clear which of these are aspirational
greenfield work and re-plan them to fit HelixQA's actual Go structure (cgo +
bridge adapters) rather than a fictional `src/` tree.**

#### C.3.2 Third-party file references

`§15 Source Code Reference Map` (3849-3919) lists real files in real repos:

| Table | Entries | Verified? |
|-------|---------|-----------|
| 15.1 OpenCV GPU | 7 entries (cudaarithm.hpp, cudaimgproc.hpp, cudafilters.hpp, cudaobjdetect.hpp, cudafeatures2d.hpp, match_template.cpp, canny.cpp) | **wrong paths for some**: OpenCV does not have `modules/cudaimgproc/include/cudafilters.hpp` - `cudafilters` is its own module (`modules/cudafilters/`). The entry "cudafilters.hpp in modules/cudaimgproc/include/" is misfiled. The others are real. |
| 15.2 OBS Studio | 8 entries | Most paths correct. `libobs/obs-video.c` exists; `plugins/win-capture/dc-capture.c` exists. `w23/obs-kmsgrab/kmsgrab.c` exists. |
| 15.3 scrcpy | 7 entries | `app/src/screen.c` exists but is called differently in modern scrcpy (`display.c` and `screen.c`); Java files under `server/src/main/java/com/genymobile/scrcpy/Server.java` exist. |
| 15.4 TensorRT | 4 entries | Correct. |
| 15.5 Vulkan Compute | 3 entries under `vkCompViz` | `ichlubna/vkCompViz` exists. File paths `vkCompViz/src/vkCompViz.h`, `vkCompViz/examples/SimpleBlending.cpp`, `vkCompViz/examples/ParallelReduction.cpp` - I could not verify exact filenames, but the repo structure broadly matches. |
| 15.6 Linux Input | 3 entries | `drivers/input/misc/uinput.c` and `drivers/input/evdev.c` exist in kernel source. `libevdev/src/libevdev-uinput.c` exists. All correct. |
| 15.7 Windows Capture | 2 entries with a leading space typo (" ScreenCapture.h") | Windows SDK Samples exist; `duplicationapi.cpp` is the sample name. Correct. |

Third-party repo existence (every GitHub repo named anywhere in OpenClawing3 was
checked):

| Repo referenced | exists? |
|-----------------|---------|
| anthropics/anthropic-quickstarts | **OK** |
| browser-use/browser-use | **OK** |
| Skyvern-AI/skyvern | **OK** |
| browserbase/stagehand | **OK** |
| bytedance/UI-TARS, bytedance/UI-TARS-desktop | **OK** |
| kubo/plthook | **OK** |
| NVIDIA-Maxine/VFX-SDK-Samples | **OK** |
| obsproject/obs-studio | **OK** |
| w23/obs-kmsgrab | **OK** |
| Genymobile/scrcpy | **OK** |
| appium/WebDriverAgent | **OK** |
| ichlubna/vkCompViz | **OK** |
| opencv/opencv, opencv/opencv_contrib | **OK** |
| PaddlePaddle/PaddleOCR | **OK** |
| openvinotoolkit/openvino | **OK** |
| msmps/pilotty | **OK** |
| ROCm, pipewire.org, autoptt.com, mcpmarket.com | **OK** as references |

**All third-party repos are real.** (Big contrast with Starting_Point.md's 38% 404
rate.) This part of OpenClawing3 is its strongest: the technology inventory is
factually accurate.

### C.4 Algorithm / technique claims

Scored by plausibility, specificity, and implementability.

| # | Line | Claim | Status |
|---|------|-------|--------|
| CT-1 | 196-233 | OpenCV CUDA pipeline: cvtColor -> gaussian -> canny -> findContours (CPU fallback) | **accurate but shallow**. The comment "CPU fallback - cv::cuda::findContours doesn't exist" (line 208-209) is correct, but the pipeline does NOT close the loop to action grounding. |
| CT-2 | 187-192 | "Pre-allocate GPU memory to avoid allocation during capture" | **good practice** (real). |
| CT-3 | 220-233 | GPU template matching via `cv::cuda::TemplateMatching` + `cv::cuda::minMaxLoc` | **accurate**. |
| CT-4 | 353-403 | DOM+vision correlation via IOU; detection of "missed" elements via `MSER` | **plausible** but under-specified. MSER for UI elements is unusual (it is a text-region-stable detector); OpenClawing4 should say whether this is really desirable for UI vs, say, contour analysis. |
| CT-5 | 417-491 | Vulkan compute Sobel edge detection | **accurate as pseudocode** but has bugs. Line 447: kernel arrays are written as `float[9](-1,0,1,-2,0,2,-1,0,1)` - in GLSL 4.50 this is valid, but the code uses `layout(local_size_x=16, local_size_y=16)` (line 421) and then `for (int i=-1; i<=1; i++)` with `imageLoad(inputScreen, coord + ivec2(i,j))` - **no bounds check for edges of the image** -- will read out of range at `x=0` and `y=0`, producing artifacts or driver warnings. |
| CT-6 | 518-613 | Vulkan compute pipeline setup C++ code | Has multiple errors: line 522 `DeviceQueueCreateInfo({}, 0, 1, &queuePriority)` with hardcoded queue family index `0` - real code needs to query compute-capable queue family. Lines 594-599 `cmd.dispatch((width+15)/16, ...)` OK, but missing barriers between storage-image writes and the next use. Line 612 `computeQueue_.waitIdle()` - serialises all GPU work, destroying the whole "async compute" benefit the section advertises. |
| CT-7 | 625-631 | MoltenVK on macOS, "same compute shaders run on Apple Silicon" | **accurate**, but MoltenVK requires a VK_KHR_portability_subset-compatible pipeline; document never mentions it. |
| CT-8 | 683-762 | TensorRT engine with FP16 + INT8 + DLA + `BuilderFlag::kCUDA_GRAPH` | **kCUDA_GRAPH not a real flag in stable TensorRT API**. TensorRT has `setBuilderFlag(BuilderFlag::kFP16)`, `kINT8`, `kSPARSE_WEIGHTS`, `kDISABLE_TIMING_CACHE`, etc. `kCUDA_GRAPH` appeared as experimental in some preview APIs but is not the way to enable CUDA Graphs at build time -- CUDA Graphs are captured at runtime around `enqueueV3`. **Line 727 is wrong as written.** |
| CT-9 | 744-747 | `parser->destroy()`, `network->destroy()`, `buildConfig->destroy()`, `builder->destroy()` | **Deprecated API**. TensorRT 8+ uses `unique_ptr` with custom deleter or the object's own `delete` operator; `destroy()` was removed in TRT 10. |
| CT-10 | 767-771 | `InferAsync` with `enqueueV3(inferenceStream_)` after `bindings_[0] = gpuInput` | **API mismatch**. `enqueueV3` uses `setTensorAddress(name, ptr)` on the context, not a `bindings_` array. The shown code is actually the `enqueueV2` pattern with v3's function name, a common confusion that will not compile. |
| CT-11 | 805-899 | Maxine VFX SDK integration | **plausible** but the constructor `effect.stream = cudaStream_; NvVFX_SetObject(effect_, NVVFX_CUDA_STREAM, &effect);` at 840-842 passes a local `effect` struct by address then goes out of scope. Maxine's `NvVFX_SetObject` stores the pointer - UB. |
| CT-12 | 942-1005 | DXGI Desktop Duplication setup | **accurate** at concept level. |
| CT-13 | 1036-1037 | `d3dContext_->CopyResource(sharedTexture_, desktopTexture)` "Copy to shared texture (still on GPU - zero CPU copy)" | **not zero-copy** -- it is a GPU-to-GPU copy but still a copy. The zero-copy claim is wrong. True zero-copy with DXGI DD would be to consume `desktopTexture` directly via CUDA/Vulkan interop without the intermediate copy. |
| CT-14 | 1094-1246 | Linux KMS capture | **has compile-blocking issues.** See C.6-RC2. |
| CT-15 | 1167 | "CRTC ID = connector->encoder_id; // simplified" | Comment admits the simplification; in reality you must look up the encoder then its CRTC. |
| CT-16 | 1495-1647 | "OBS-style" RecordingPipeline class | **incomplete/wrong.** The implementation calls `rawFrameQueue_.Pop()` (line 1635) but `ThreadQueue` is a phantom type - never defined, no header shown. `captureEngine_->AcquireGPUFrame()` returns `GPUFrame` but its definition is never shown. `SelectBestEncoder` (1593) is never defined. `FFmpegFileOutput` uses `avformat_write_header` (1673) with no codec context -- will crash. The pipeline cannot actually be built from the shown code. |
| CT-17 | 1707-1768 | WebRTC / WHIP via GStreamer webrtcbin | The pipeline string (1727-1732) is plausible but `webrtcbin` setup is far more involved than shown -- signaling SDP offer/answer and ICE candidates is hand-waved as "ConnectWHIP() { // ... }" (1762-1766). |
| CT-18 | 1793-1858 | LD_PRELOAD interceptor | **functional sketch** but has syntactic errors: line 1884 `HookEvent event = { .timestamp = ..., .pid = ..., .tid = ..., .event_type = ..., .detail = ..., .value = ... };` uses C99 designated initializers, fine. But `notify_hook_server` is declared `static` (1879) yet called from a non-static position earlier; that is legal as long as order is right, but the function is declared **after** first call sites (e.g. line 1836) - **will not compile** unless there is a prototype earlier that the document does not show. Also `hook_socket = atoi(hook_addr)` (1822) treats the address as a decimal FD number, but the code later uses `send(hook_socket, ...)` as if it were a connected socket - needs `socket()` + `connect()`, not `atoi`. |
| CT-19 | 1901-1949 | plthook runtime hooking | Reasonable. Requires `real_connect`, `real_send`, `real_dlopen` declared in same TU; they are from §9.1 but compiling them together is not explained. |
| CT-20 | 1964-2059 | Windows LL hooks | Reasonable. `hookThread_` is referenced (1991-1997) but never declared. |
| CT-21 | 2091-2240 | uinput controller | Good sketch. `ConvertToLinuxKeyCode` (2228-2235) defines the HID-to-Linux mapping with ONE entry commented "// ... etc" - missing 99% of the mapping. Not portable as-is. |
| CT-22 | 2265-2345 | SendInput + FakerInput | The FakerInput code references undefined structs `FAKER_INPUT_MOUSE_REPORT`, `FAKER_REPORT_ID_MOUSE` - these come from FakerInput driver header. Path / download not given. |
| CT-23 | 2400-2656 | PTY/xterm TUI | TypeScript code has errors: line 2460-2462 `session.terminal.buffer.active.getNullCell().toString()` - **returns a single cell, not the whole buffer text**. A correct implementation iterates `buffer.getLine(y).translateToString()`. The `findAndInteract` function assumes cursor-based navigation via arrow keys, which fails for most modern TUIs (they use letter shortcuts, not arrow scroll). |
| CT-24 | 2561-2565 | `sendInput(sessionId, { key: 'Down' }.repeat(rowDelta))` | **Type error**: `{key: 'Down'}.repeat(n)` is not valid; `repeat` is a string method. The `declare global { interface String { repeat(count:number): string; }}` at 2652-2654 is a no-op: `String.prototype.repeat` already exists. The code as written will crash at runtime. |
| CT-25 | 2691-2988 | scrcpy Python controller | Mix of good and bad: the port numbers 27183/27184 (2741-2744) are plausible (scrcpy uses adb-forward to local ports), but the scrcpy **server protocol is much more involved** than "start server then connect" - there is a 1-byte "dummy byte" handshake, codec selection, and recent versions use control-socket before video-socket. The protocol format shown is outdated (v1.x-era) and will not work with modern scrcpy 2.x+. |
| CT-26 | 2813-2832 | `struct.pack(">BBqiiHHH", 2, action, pointer_id, x, y, w, h, pressure)` | Wrong format. Scrcpy TOUCH_EVENT packet is (type:uint8, action:uint8, pointer_id:uint64, position:int32+int32, screen:uint16+uint16, pressure:uint16, action_button:uint32, buttons:uint32) - that gives `>BBqiiHHHII` = 32 bytes. The doc's `>BBqiiHHH` is 24 bytes and omits action_button + buttons. |
| CT-27 | 3017-3095 | iOS WDA controller | `wda.Client` + `session.tap(ratio_x, ratio_y)` is a reasonable `facebook-wda` sketch. |
| CT-28 | 3449-3525 | Segmented FFmpeg recording with `h264_nvenc` + `-segment_time` | **Mutually exclusive flags**: `-f segment` with `-segment_time` is the segmenter. With `-y` overwrite and `-strftime 1` output pattern this works. However, line 3491 pattern `{prefix}_%Y%m%d_%H%M%S.mkv` with MKV as segment target is unusual; MP4 is more common, MKV has reliability quirks with segment mux. |
| CT-29 | 3542-3593 | Screenshot pipeline with diff mask | Reasonable. |
| CT-30 | 4043-4053 | ffmpeg-python filter graph with `hwupload_cuda` + `scale_npp` | `scale_npp` exists only in ffmpeg builds with NVIDIA libnpp and Performance Primitives -- not the default build. |

#### C.4.1 Performance claims audit

Appendix A (3922-3945):

| Claim | Credibility |
|-------|-------------|
| Screen capture GPU <5ms | **Plausible** for DXGI DD / KMS DMA-BUF / NVFBC. |
| Screen capture + encode <10ms | **Plausible** with NVENC P1 preset, but not guaranteed at 4K. |
| OpenCV template matching (1080p) <2ms on CUDA GPU | **Overly optimistic**. For 1080p source + 100x100 template on a 3060, `cv::cuda::TemplateMatching` with `TM_CCOEFF_NORMED` is typically 6-15ms. 2ms requires small template, FP16, batched execution, or sliding window with pruning. |
| Full vision analysis pipeline <16ms | **Aggressive**. Achievable for simple pipelines on RTX 3060 but unlikely for the stack as described (capture + OCR + template match + Canny + YOLO inference), which on a 3060 is more like 30-50ms. |
| Input injection <1ms | **True** (uinput + SendInput). |
| OCR (100 words) <50ms TensorRT+RTX | **Plausible** (PaddleOCR with FP16). |
| Web automation action cycle <100ms | **Network dependent** - caveat already included. |
| TUI interaction cycle <50ms | **Reasonable**. |
| Mobile frame capture <33ms | **Optimistic**; 30fps target with scrcpy typically hits 35-60ms end-to-end over USB. |

**Net**: Most numbers are optimistic-but-plausible; the OpenCV template-match and
"full pipeline <16ms" targets would not hold for the described workload on the
quoted hardware. OpenClawing4 should either raise the targets or narrow the
workload.

### C.5 Inconsistencies

- **CI-1**: Line 3963 "*End of Document*" followed by ~230 more lines ("Additional
  Game-Changer Technologies", Appendix C, Appendix D). The doc ends twice.
- **CI-2**: §5.2 Maxine enhancer (805-899) vs. "Additional...A. NVIDIA Maxine SDK"
  (3976-3995) -- the second block re-introduces Maxine as if it had not been
  covered. Conflicting file-path attribution in the duplicate block.
- **CI-3**: §1.2 technology matrix (50-64) claims "DXGI Desktop Duplication
  `<5ms capture latency`" but Appendix A (3932) claims "Screen capture (GPU) `<5ms`
  Any GPU with DMA-BUF/DXGI" -- then Appendix A row "Screen capture + encode" is
  `<10ms` which implies the whole capture is truly very fast; but this ignores
  DXGI's `AcquireNextFrame` timeout semantic where if the screen has not changed,
  the call **blocks up to the timeout** (the code in §6.1 uses 100ms timeout line
  1016). So typical "no-change" acquire latency is 100ms, not 5ms. Document does
  not reconcile.
- **CI-4**: The `namespace openclaw { namespace capture { ... }}` wrapper on
  `kms_capture.c` (line 1091 / 1248) -- C does not have namespaces. (Listed as a
  red flag; also an inconsistency because all other Linux C files in §9 use plain
  C syntax correctly.)
- **CI-5**: §3.2 uses `<opencv2/...>` C++ headers, §3.3 uses C++, §3.4 switches to
  TypeScript (`@u4/opencv4nodejs`), §10.1 back to C++, §12 to Python, §11 to
  TypeScript, §6.3 to Objective-C. The document never explains the language /
  build / linkage strategy that glues these together. There is no mention of cgo
  (required to bind any of this to HelixQA's Go code).
- **CI-6**: §8.1 "HWVideoEncoder" class has factory methods `CreateNVENC(..., void*
  d3dDevice)` for D3D11 sharing, but the DXGI capture class (§6.1) exposes
  `ID3D11Texture2D*`, not `ID3D11Device*`. Bridging the two is not shown.
- **CI-7**: Phase plan (§14) lists "Weeks 1-16" totalling 16 weeks; Executive
  Summary (line 68) says "After full integration, OpenClaw will achieve..." -
  implied end state. But §14 ends at Phase 8 "Integration & Testing" with zero
  QA / hardening / rollout phase. 16 weeks for 47 technologies including
  kernel-level hooks is unrealistic.
- **CI-8**: §13.1 box-drawing (3104-3184) shows an "AGENT INTELLIGENCE LAYER"
  at the bottom with "browser-use loop | UI-TARS model | Planning Engine" but
  OpenClawing2's actual recommendation was to port `Agent.step` from browser-use.
  There is no reconciliation - the diagram treats browser-use and UI-TARS as
  parallel options, but OpenClawing2 presented them as DIFFERENT agent
  architectures (iterative vs native-model). OpenClawing4 needs to pick one or
  specify the switching policy.

### C.6 Red flags (material defects)

- **CR-1 (CRITICAL, pervasive)**: `namespace openclaw {...}` wrapping inside a `.c`
  file (lines 1091-1248). **C does not support namespaces.** Either the extension is
  wrong (should be `.cpp`) or the syntax is wrong. Anyone copy-pasting will hit
  compile errors immediately. This appears ONLY in the Linux `kms_capture.c`
  section -- suggesting the section was written without a compile check.
- **CR-2**: Same file: `DmaBufCapture result = {}` (line 1180) returns a type never
  declared in the shown code. The function-return type `DmaBufCapture` is not
  forward-declared anywhere in the snippet. The class `KMSCapture` also references
  `currentCapture_` (line 1244) without ever declaring that member.
- **CR-3**: TensorRT code uses obsolete API (`destroy()` methods) AND undefined flag
  (`BuilderFlag::kCUDA_GRAPH`, line 727) that is not in mainline TensorRT 8/9/10.
  Anyone attempting this build against current TensorRT will fail.
- **CR-4**: TensorRT `InferAsync` (767-771) uses `bindings_` array pattern with
  `enqueueV3`. `enqueueV3` requires `setTensorAddress` API; the shown code is a
  copy-paste of the `enqueueV2` pattern. Will not compile against TRT 10, and will
  not run against TRT 8/9.
- **CR-5**: scrcpy Python controller uses a made-up protocol layout:
  the packet format at 2819-2830 is 8-byte short (missing action_button + buttons
  uint32s from real scrcpy 2.x). Even if the protocol code worked, the server
  invocation (2748-2755) is also version-mismatched -- modern scrcpy server takes
  named `key=value` pairs like `log_level=verbose video_codec=h264`, not positional
  args.
- **CR-6**: `GetSharedHandle` on D3D11 texture (line 994) uses `IDXGIResource::GetSharedHandle`
  -- this is the **NT-handle legacy API**. Modern (Windows 8+) code must use
  `IDXGIResource1::CreateSharedHandle` to get a handle usable with CUDA/Vulkan
  external-memory extensions. The one shown returns a handle that Vulkan's
  `VK_KHR_external_memory_win32` cannot import. "Cross-API sharing" claim at 1059
  is therefore structurally broken.
- **CR-7**: Box-drawing diagram at 3104-3184 uses lossy ASCII that is damaged in
  several places (e.g. line 3167 `│ │Segmented │ │   <100ms        │  │` has wrong
  column alignment), and does not convey any runtime semantics -- there is no
  arrow direction, no phase separation between "observation -> decision ->
  action".
- **CR-8**: §14 "Step-by-Step Implementation Guide" is not a guide. Example of
  what would be at "smallest atoms level of precision" (BRIEF-2): verify CUDA
  toolkit version compatibility with the TensorRT version, set `CUDA_HOME`,
  install `cuDNN` at a specific minor version, check `nvidia-smi` output against
  TRT matrix, etc. None of that is present. The doc jumps from `git clone` to
  `make -j` and assumes success.
- **CR-9**: Zero mention of HelixQA's actual constraints from
  `HelixQA/CLAUDE.md`: project-agnostic decoupling mandate, no CI, no sudo,
  rootless podman, distributed llama.cpp RPC vision. OpenClawing3 recommends
  operations (CUDA driver install, `sudo apt-get install tensorrt` at line 3309)
  that directly violate the "no sudo" mandatory rule. `sudo dpkg -i` line 3306 is
  the same violation. **This is a Constitution-level conflict that must be flagged
  up front in OpenClawing4.**
- **CR-10**: No threat model. Hook technology (LD_PRELOAD + plthook + LL hooks)
  is described in purely offensive terms with no discussion of its detectability,
  legality, or interaction with anti-cheat / endpoint-protection software
  ("undetectable Windows input" via FakerInput, line 2263). HelixQA is a QA
  tool, not malware, yet the doc repeatedly leans into anti-detection framing.
- **CR-11**: Security appendix (3948-3963) lists permission requirements
  including "Linux: `video` group or root" for KMS capture (3954) and
  "Linux: `input` group or udev rule" for uinput (3955) -- both **require
  elevated privileges or group membership that HelixQA CLAUDE.md explicitly
  forbids in its "NO SUDO OR ROOT EXECUTION" mandatory rule**. This needs
  explicit reconciliation in OpenClawing4.
- **CR-12**: The document's 47-technology integration promise (Appendix C) is
  not matched by the 8-phase / 16-week timeline. Realistic per-technology
  integration effort for production-grade work (lifecycle, error handling, tests,
  CI-equivalent, docs) is ~2-4 weeks each; 47 × 2 = 94 weeks minimum. The
  timeline is an order of magnitude off.
- **CR-13**: Zero observability / tracing / metrics. A real-time pipeline needs
  per-frame latency histograms, back-pressure signals, drop counts. None is
  designed.
- **CR-14**: No concurrency model. GPU/CPU data transfer, CUDA streams, Vulkan
  queues, OS event loops, Python GIL (in scrcpy controller), node.js event loop
  (in TUI) all meet in the same "unified" system. No cross-runtime
  synchronisation plan.
- **CR-15**: Zero error-recovery design. "Forward to OpenClaw for analysis" (line
  2056) is the only mention of how failures propagate. No circuit breaker, no
  retry policy, no degraded modes.

### C.7 Gaps (things BRIEF-2 explicitly asked for that are not delivered)

- **CG-1**: "every single source code file is referenced" - only ~35 files
  referenced across 4200 lines. Thousands of relevant files exist in the 30+
  real projects covered. The claim is hyperbole.
- **CG-2**: "API" automation - mentioned once in a type enum (line 3757) and
  a diagram node. No content.
- **CG-3**: Integration plan with OpenClawing2's recommended `Agent.step()` loop -
  no discussion.
- **CG-4**: "smoothly interact and use UI/UX and whole flows of any type of
  applications" - no end-to-end test scenario. No worked example that starts
  from "Given a CUE to log into Catalogizer, browse Movies, play one" and
  demonstrates the stack handling it.
- **CG-5**: "hooks we can attach if/when needed" - hooks section (§9) focuses on
  OS-level syscall/X11 hooks, not LLM/Vision model pipeline hooks. If the brief
  meant "attach processing hooks to the vision pipeline", that is not covered.
- **CG-6**: No treatment of HelixQA's mandated vision providers: llama.cpp RPC
  distributed inference, Astica, Gemini, OpenAI, Ollama. The doc recommends
  TensorRT local models instead, which is a completely different architecture.
- **CG-7**: No treatment of HelixQA's "fix in HelixQA, not the app" rule. The
  hooks section (§9) is explicitly about modifying target applications; that
  violates the Universal Solution Principle in Catalogizer's root CLAUDE.md.

### C.8 Summary for OpenClawing4

OpenClawing3 is:

- **Inventory grade**: ~90%. All tech mentioned is real.
- **Citation grade**: ~60%. Most file paths in §15 are real, but no quotes,
  no SHAs, no "read this specific function" pointers. Several targeted
  inline file-path citations are miscategorised (cudafilters.hpp).
- **Code grade**: ~35%. Pseudocode and sketches with multiple compile-blocking
  errors and API mismatches; not a codebase seed.
- **Plan grade**: ~20%. 8-phase outline is topical ordering, not an
  implementation plan.
- **Integration grade with HelixQA**: ~5%. The doc assumes a `src/...`
  C++/TS tree that HelixQA does not have. No cgo plan, no Go interface, no
  contract boundary between the new pipeline and the existing Go stack. No
  alignment with HelixQA CLAUDE.md's non-negotiable rules (no sudo, project-
  agnostic decoupling, distributed llama.cpp RPC).

---

## Part D - Cross-document findings & priority list for OpenClawing4

### D.1 The single biggest problem: target confusion

Starting_Point treats "OpenClaw" as a fictional TS gateway. OpenClawing2 inherits
this framing (`src/agents/pi-embedded-runner.ts` etc.). OpenClawing3 invents a
separate `src/...` C++/TS tree. **None of the three documents target HelixQA's real
codebase (Go `pkg/...`)**, which is the thing OpenClawing4 must extend.

OpenClawing4 must open with a section that (a) re-anchors on HelixQA's real Go
structure under `pkg/capture`, `pkg/vision`, `pkg/opencv`, `pkg/video`, `pkg/streaming`,
etc.; (b) maps every "port target" from OpenClawing2 and every "tech" from
OpenClawing3 onto one of: (i) new Go package, (ii) cgo binding, (iii) subprocess
service, (iv) do-not-port.

### D.2 Constitution conflicts that must be resolved up front

From the HelixQA CLAUDE.md and the Catalogizer CLAUDE.md:

| Constitution rule | Conflict source in docs |
|-------------------|-------------------------|
| NO SUDO OR ROOT EXECUTION | OpenClawing3 §14 `sudo dpkg -i` (3306), `sudo apt-get install tensorrt` (3309), "sudo usermod -aG input" (3408), `sudo make install` (3297) |
| NO SUDO | uinput (group `input`) / KMS (group `video`) requirements in Appendix B (3954-3955) |
| Project-agnostic / 100% decoupled | OpenClawing2 baseline repeatedly pinned to TS "OpenClaw" that does not exist; OpenClawing3 invents `namespace openclaw` everywhere (should be a generic, consumer-agnostic namespace) |
| All navigation by real LLM vision; no hardcoded coordinates | UI-TARS coord-based actions (OpenClawing2 §7.2), pyautogui coord input (OpenClawing3 §7.3.1), template matching with hardcoded templates (OpenClawing3 3366-3390) are all coord / template based, not LLM-driven. Needs explicit reconciliation. |
| Fix in HelixQA, not the app | OpenClawing3 §9 hooks require LD_PRELOAD into target apps = modifying app execution |
| Universal Solution Principle | Android-only FakerInput, Windows-only DXGI interlocking with CUDA -- OpenClawing4 needs to explain platform-abstraction layer |
| Video app geo-restriction / VPN detection | Never mentioned anywhere in the three docs |
| NO CI/CD Pipelines in HelixQA module | Not directly violated but OpenClawing3 "testing phase" implies CI |
| Mandatory llama.cpp RPC distributed vision | OpenClawing3 recommends TensorRT local inference as primary; needs explicit fallback/coexistence design |

### D.3 Things worth salvaging

For each of the salvageable pieces, OpenClawing4 can cite directly:

From **OpenClawing2**:
- Agent loop pattern from `browser_use/agent/service.py` (real file, lines 81-159 inside OpenClawing2 describe it accurately). OpenClawing4 should port the *pattern*, not the literal class.
- Stagehand's `act` / `extract` / `observe` primitive split (OpenClawing2 §6.2) -- useful API contract for HelixQA's own action layer.
- Coordinate-scaling idea from Anthropic `computer_use_demo/tools/computer.py` `scale_coordinates` method (OpenClawing2 §3.2.5) -- real, portable, and HelixQA has to handle cross-resolution Android TV already.
- Iterative loop + planning + error-recovery comparison matrix (OpenClawing2 §8.1 table) -- useful to decide which loop style HelixQA wants.

From **OpenClawing3**:
- OpenCV GPU pipeline architecture at §3.2 (despite the OpenClawing3 code being pseudocode; the CONCEPT of pre-allocating GpuMat + reusing CUDA streams is real and ports to Go via gocv).
- DXGI Desktop Duplication + KMS DMA-BUF + ScreenCaptureKit tri-platform capture strategy (§6, §7). Real APIs, real paths, should be the blueprint for HelixQA's `pkg/capture`.
- Hardware encoder selection matrix (NVENC / VAAPI / AMF / VideoToolbox) at §8.1 (1527-1547). Correct strategy, correctly scoped.
- Segmented recording strategy via FFmpeg (§14 Step 4.1, 3449-3525) - works with rootless podman container, easy win.
- Screen change detection via frame diff + only-analyse-changed-regions (§14 Step 4.2, 3542-3593) - directly solves HelixQA's "stuck-on-same-screen" mandate from CLAUDE.md.
- 47-tech inventory (Appendix C) - useful as a do-not-reinvent-the-wheel checklist.

### D.4 Priority list for OpenClawing4

1. **Re-anchor on HelixQA.** Replace OpenClaw / `src/...` framing with `HelixQA/pkg/...`. Explicitly state what parts of HelixQA are being extended vs. replaced.
2. **Constitutional audit.** Before any tech is accepted, check against the HelixQA CLAUDE.md no-sudo / project-agnostic / LLM-driven / no-app-modification rules. For each violating tech, either (a) find the rootless equivalent, or (b) reject it.
3. **Correct the bad citations.** OpenClawing2 has at least 3 fabricated or wrong target-repo paths (`custom_browser.py`, `agent/prompts.py`, `PlanEvaluate` class). OpenClawing4 must not propagate them.
4. **Remove broken Starting_Point links.** 9 of 24 primary links in Starting_Point.md are 404. Do not carry the "25 OpenClaw alternatives" framing into OpenClawing4.
5. **Design the cgo / subprocess boundary.** Every C++/Objective-C/CUDA capability needs a Go contract. Propose whether OpenCV-CUDA is in-process (cgo via gocv + custom CUDA calls), subprocess (one vision worker binary exposing gRPC), or sidecar container (podman gpu pass-through). Decide once; don't vague.
6. **Fix the compile-blockers** before any snippet is promoted into OpenClawing4: `namespace` in `.c`, `BuilderFlag::kCUDA_GRAPH`, `destroy()` methods, `enqueueV3` binding-array pattern, scrcpy wire format, `repeat` on objects.
7. **Reconcile coord-based vs LLM-driven.** UI-TARS and template matching are coord-based; HelixQA's Constitution forbids hardcoded coords. OpenClawing4 must either (a) argue they are acceptable when LLM is the source of the coord, or (b) drop them.
8. **Set realistic latency targets.** OpenCV template match "<2ms on 1080p" is not realistic for the described workload. Either narrow the workload (small template, patch search) or raise to ~10ms.
9. **Write a real phase plan.** For each of the 47 technologies in Appendix C, mark: NEEDED / NICE / DROP. Order the NEEDED list by dependency chain. Don't time-box 47 techs into 16 weeks.
10. **Add observability.** Every capture/encode/analyse node needs latency histograms (p50/p95/p99), drop counts, GPU memory usage. This is the layer OpenClawing3 completely omits.
11. **Add a threat / permission model.** Explicitly say which capabilities need which Linux group / macOS TCC grant / Windows privilege, and how those are obtained WITHOUT sudo under HelixQA's rules.
12. **Align with HelixQA's existing vision stack.** The file HelixQA/CLAUDE.md mandates llama.cpp RPC distributed inference. OpenClawing4 must explain how TensorRT / OpenCV / Vulkan additions are in addition to, not in place of, the distributed llama.cpp model.
13. **Address the mobile + TV platforms explicitly.** HelixQA runs on Android TV primarily. OpenClawing3's scrcpy treatment is outdated (v1.x protocol); OpenClawing4 needs the current scrcpy 2.x/3.x wire format or a different mobile strategy (UIAutomator2 + MediaProjection capture over ADB).
14. **Cover the glitch-detection gap.** HelixQA CLAUDE.md spells out frame stagnation, misaligned UI, clipped text, broken animations as bugs to report. Nothing in the three source docs addresses this. OpenClawing4 must introduce the detection algorithms (SSIM/pHash for stagnation, layout-tree diff for misalignment, OCR baseline comparison for text clipping) as first-class deliverables.
15. **Cite everything.** Every real file path must have the repo's SHA or tag at time of reference. Every GitHub repo must have an SSH URL (per Catalogizer CLAUDE.md). Every numeric claim must have a source or a labelled "estimated".

### D.5 Output format recommendations for OpenClawing4

- Front-matter with: target anchor (HelixQA v?), audited-source list, constitution-conflict summary.
- Per-technology decision cards: Problem / Chosen Tech / Alternatives Rejected / HelixQA Fit (cgo / subprocess / container) / Constitution-check Pass? / Benchmarks Target / Open Questions.
- Dependency graph across decision cards.
- Real phase plan with acceptance tests (executable, backed by the HelixQA test bank format).
- Separate "game-changer" tier vs. "hygiene" tier (things like HW encoders that are table stakes, not game changers).

---

## Appendix: verification data used in this audit

### A.1 GitHub existence checks (all ran `gh api repos/OWNER/REPO`)

Starting_Point URLs (primary column):
  MISS: theonlyhennygod/nano-claw, nullswan/nullclaw, missingbytes/hermes,
        moltworkerai/moltworker, pablomarquezhaya/DumbClaw, cnlohr/pycoclaw,
        ClawBoy/ClawBoy, shadanan/babyclaw, 0xConnor/Clawlet
  OK: HKUDS/nanobot, sipeed/picoclaw, theonlyhennygod/zeroclaw (fork),
      qhkm/zeptoclaw, aspasskiy/GoGogot, princezuda/safeclaw, moltis-org/moltis,
      nearai/ironclaw, Mintplex-Labs/anything-llm, NevaMind-AI/memU,
      tsingliuwin/autoclaw, chrischongyj/dumbclaw, jetpax/pycoclaw,
      sudhamabhatia/babyclaw, mosaxiv/clawlet, jpoley/risk-o-lobsters,
      machinae/awesome-claws

OpenClawing2 / OpenClawing3 technology repos:
  ALL OK: anthropics/claude-quickstarts (=anthropic-quickstarts), browser-use/browser-use,
          Skyvern-AI/skyvern, browserbase/stagehand, bytedance/UI-TARS,
          bytedance/UI-TARS-desktop, kubo/plthook, NVIDIA-Maxine/VFX-SDK-Samples,
          obsproject/obs-studio, w23/obs-kmsgrab, Genymobile/scrcpy,
          appium/WebDriverAgent, ichlubna/vkCompViz, opencv/opencv,
          opencv/opencv_contrib, PaddlePaddle/PaddleOCR, openvinotoolkit/openvino,
          msmps/pilotty.

### A.2 Target-repo path existence checks

  browser_use/agent/service.py -- OK (161600 bytes)
  browser_use/browser/custom_browser.py -- **404**
  browser_use/browser/ dir exists but NO file named custom_browser.py
  browser_use/controller/ exists with only __init__.py (no Controller.py)
  skyvern/ top-level: NO agent/ dir -> skyvern/agent/prompts.py does NOT exist
  skyvern/webeye/browser_manager.py -- OK (2651 bytes)
  skyvern/forge/api_app.py -- OK (12050 bytes)
  anthropic-quickstarts/computer-use-demo/computer_use_demo/loop.py -- OK (13121 bytes)

### A.3 HelixQA real-codebase anchors (for re-anchoring OpenClawing4)

Top-level pkg directories (Go):
  pkg/analysis, pkg/autonomous, pkg/bridges, pkg/capture, pkg/config, pkg/controller,
  pkg/detector, pkg/discovery, pkg/distributed, pkg/evidence, pkg/gst, pkg/infra,
  pkg/issuedetector, pkg/learning, pkg/llm, pkg/maestro, pkg/memory, pkg/metrics,
  pkg/navigator, pkg/nexus, pkg/ocr, pkg/opencv, pkg/opensource, pkg/orchestrator,
  pkg/performance, pkg/planning, pkg/platform, pkg/regression, pkg/replay, pkg/reporter,
  pkg/reproduce, pkg/session, pkg/streaming, pkg/testbank, pkg/ticket, pkg/training,
  pkg/types, pkg/validator, pkg/validators, pkg/video, pkg/vision, pkg/visual,
  pkg/webrtc, pkg/worker.

These are where OpenClawing4's decisions should be mapped, not into fictional `src/`.

*End of audit. OpenClawing4 should be produced with these findings open in a side pane.*
