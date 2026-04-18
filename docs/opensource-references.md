# Open-Source Reference Codebases

This document tracks every OSS codebase HelixQA vendors as a git
submodule under `tools/opensource/`. The purpose is **reference
study** — HelixQA ports *patterns* from these frameworks, not
compiled code. Pinning via submodules lets the team inspect the
exact source the OpenClawing2.md analysis draws from, without
reintroducing any project-specific names or external runtime
dependencies into HelixQA itself.

**Last refreshed:** 2026-04-18

---

## OpenClawing2 reference set

These five frameworks back the v3.0 "magical" integration plan in
`docs/plans/2026-04-18-openclawing2-integration-plan.md`.

| Path | Framework | Source | Licence | Pinned SHA | Why vendored |
|---|---|---|---|---|---|
| `tools/opensource/browser-use` | browser-use | github.com/browser-use/browser-use | **MIT** | `702b7352c3dc1d39ab8daef2f076dfe7adcfa1c3` | Agent.step() loop, PlanEvaluate cycle, message compaction, exp-backoff + loop detection, DOM processing service |
| `tools/opensource/skyvern` | Skyvern | github.com/Skyvern-AI/skyvern | **AGPL-3.0 (reference only)** | `e13f34b996685a0b65a5c629cfcc09ee1cd726da` | Observe → Plan → Act state machine, bounding-box element overlays, task-history persistence |
| `tools/opensource/stagehand` | Stagehand | github.com/browserbase/stagehand | **MIT** | `20b601dc8779a1bacf66abb68ebdf276a238e5db` | act/extract/observe/agent hybrid primitives, CDP-native performance, prompt caching, self-healing re-inference |
| `tools/opensource/ui-tars` | UI-TARS (model) | github.com/bytedance/UI-TARS | **Apache-2.0** | `582f3a7ea5d285ee8ed9e2e84048d1ab01453c49` | Native multimodal agent model, coordinate-grounded action space |
| `tools/opensource/ui-tars-desktop` | UI-TARS-desktop (app) | github.com/bytedance/UI-TARS-desktop | **Apache-2.0** | `7986f5aea500c4535c0e55dc5c5d0cda73767c45` | `pyautogui`-style local control, screenshot + OCR multi-modal input processing |
| `tools/opensource/anthropic-quickstarts` | Anthropic computer-use-demo | github.com/anthropics/anthropic-quickstarts | **MIT** | `4b2549e8093a6dee1c394bdd8fcf83cb914a271a` | System-prompt engineering for vision-grounded actions, ToolCollection dispatch idiom, coordinate-scaling algorithm |

---

## Pre-existing vendored references (inherited from prior sessions)

These submodules were vendored by earlier HelixQA work (visual
regression, mobile automation research, doc processing, etc.) and
are tracked here so the Phase 1 audit gate passes. Each row will
be revisited during the quarterly refresh so the matrix stays
accurate.

| Path | Purpose |
|---|---|
| `tools/opensource/allure2` | Allure 2 test report format reference. |
| `tools/opensource/appcrawler` | Android UI crawler reference. |
| `tools/opensource/appium` | Appium mobile automation reference. |
| `tools/opensource/chroma` | Chroma vector DB reference for AI memory. |
| `tools/opensource/docker-android` | Android-in-Docker reference for emulator automation. |
| `tools/opensource/docling` | Docling document processing reference. |
| `tools/opensource/kiwi-tcms` | Kiwi TCMS test-case management reference. |
| `tools/opensource/leakcanary` | LeakCanary Android memory-leak reference. |
| `tools/opensource/llama-index` | LlamaIndex RAG reference. |
| `tools/opensource/marker` | PDF → markdown converter reference. |
| `tools/opensource/mem0` | Mem0 memory reference. |
| `tools/opensource/midscene` | Midscene visual testing reference. |
| `tools/opensource/moondream` | Moondream vision model reference. |
| `tools/opensource/perfetto` | Perfetto trace collection reference. |
| `tools/opensource/redroid` | Redroid container Android reference. |
| `tools/opensource/scrcpy` | scrcpy Android screen mirror reference. |
| `tools/opensource/shortest` | Shortest AI testing framework reference. |
| `tools/opensource/signoz` | Signoz observability reference. |
| `tools/opensource/testdriverai` | TestdriverAI agent reference. |
| `tools/opensource/unstructured` | Unstructured document parsing reference. |

---

## Files HelixQA mirrors patterns from (exact source paths)

HelixQA never copies code wholesale — it studies the patterns and
re-implements in Go. The table below maps each porting target to the
exact upstream file(s) the implementation was inspired by, so a
future reviewer can compare behaviour.

### Phase 2 — Agent-step state machine

- `browser-use/browser_use/agent/service.py::Agent.step()` — canonical
  iterative loop with Evaluate / Memory / NextGoal / Actions output.
- `browser-use/browser_use/agent/views.py::AgentOutput` — the typed
  output struct HelixQA's `AgentStep` mirrors.
- `skyvern/skyvern/agent/agent.py` — Observe/Plan/Act state-machine
  reference for the explicit phase-separation idea.

### Phase 3 — Message manager + compaction

- `browser-use/browser_use/agent/message_manager/service.py` —
  `MessageManager` with `prepare_step_state` + `create_state_messages`.
- `anthropic-quickstarts/computer-use-demo/computer_use_demo/loop.py::_make_api_tool_result` —
  reference for structured tool-result formatting.

### Phase 4 — Self-healing + exp-backoff + loop detection

- `browser-use/browser_use/agent/service.py::_execute_actions` —
  retry + backoff pattern.
- `browser-use/browser_use/agent/service.py::_check_for_loops` —
  rolling history pattern for 2/3-cycle detection.
- `stagehand/lib/StagehandPage.ts` — self-healing re-inference on
  stale selector.

### Phase 5 — Stagehand primitives

- `stagehand/lib/v3/act.ts` — natural-language → Playwright command.
- `stagehand/lib/v3/extract.ts` — schema-driven structured extraction.
- `stagehand/lib/v3/observe.ts` — element-descriptor → selector.
- `stagehand/lib/v3/agent.ts` — scoped autonomous mode.

### Phase 6 — UI-TARS coordinate-grounded adapter

- `ui-tars-desktop/apps/ui-tars/src/main/services/runner.ts` —
  pyautogui command dispatch + coordinate handling.
- `ui-tars/docs/action_space.md` — canonical `click(x, y)` /
  `type(text)` / `scroll(dx, dy)` action grammar.
- `anthropic-quickstarts/computer-use-demo/computer_use_demo/tools/computer.py::scale_coordinates` —
  resolution-independent coordinate scaling algorithm (MAX_SCALING_TARGETS).

### Phase 7 — Rich ticketing

- `anthropic-quickstarts/computer-use-demo/computer_use_demo/loop.py::_make_api_tool_result` —
  base64-encoded screenshot embedding pattern for closed-loop
  feedback.
- `browser-use/browser_use/agent/history_tree_processor/` — event
  history persistence reference.

---

## Sync script

Run `scripts/sync-opensource-references.sh` to fetch + pull every
OSS submodule to the tip of its tracked branch. The script also
prints the pinned-vs-current SHA diff so operators can decide
whether to bump a submodule SHA in this doc.

---

## Licence gate (CI + local check)

`banks/fixes-validation-decoupling.yaml::FIX-DECOUPLE-006` + a
future `OC-REF-001` challenge assert that every `tools/opensource/*`
directory has a matching row in this document. The CI grep gate is:

```sh
for dir in tools/opensource/*/; do
  name=$(basename "$dir")
  grep -q "tools/opensource/$name" docs/opensource-references.md ||
    echo "MISSING: $name not documented"
done
```

Every new OSS submodule added to `tools/opensource/` must include
an entry here + a licence-inventory row in the same PR.

---

## Refresh policy

- **Quarterly.** A human reviewer bumps each pinned SHA to the
  latest stable tag + re-runs the OpenClawing2 comparative analysis.
- **On breaking API change.** When the ported pattern in HelixQA
  diverges from the upstream, either update HelixQA to the new
  pattern or add a note in the relevant phase section here.
- **On licence change.** Any upstream licence change triggers an
  immediate re-review. AGPL / GPL adoption by a previously
  permissive upstream forces re-classification to "reference only"
  (see Skyvern row above).
