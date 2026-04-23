# CLAUDE.md - HelixQA Module

## CONSTITUTION: Project-Agnostic / 100% Decoupled (MANDATORY)

**HelixQA and ALL its submodule dependencies (Challenges, Containers, DocProcessor, LLMOrchestrator, LLMProvider, VisionEngine) MUST be 100% decoupled and ready for generic use with ANY project — not just ATMOSphere, not just any single consumer.**

- **NEVER** hardcode ATMOSphere package names, RU-region endpoints, device serials, or any project-specific data inside HelixQA or its dependencies.
- **NEVER** add project-specific test banks, fixtures, or default configurations into HelixQA source code.
- **NEVER** import anything from the consuming project (e.g., `device/rockchip/...`, ATMOSphere-specific Go modules).
- All project-specific data (known endpoints, alternative apps, geo-restriction lists, device-specific behaviours) MUST be registered by the caller via public APIs (e.g., `RegisterEndpoint`, `RegisterAlternative`, `SetGenericAlternative`), NEVER baked into the library.
- Default values in HelixQA libraries MUST be empty or generic — no RU/US/EU-specific preset lists.
- Package documentation MUST call out the generic contract and show how ANY project (a TV vendor, an Android farm, a museum kiosk) can register its own data.
- Test banks (`banks/*.yaml`) ARE allowed to contain project-specific test cases, because YAML is consumer-owned data, not library code — but the bank-loading MACHINERY in `pkg/testbank` must remain fully generic.

**A HelixQA release that only works with ATMOSphere is a critical infrastructure failure.** Violations of this constitution void the entire release — the module must be immediately refactored to restore generic behaviour before any commit is accepted.

## CONSTITUTION: Fully Autonomous LLM-Driven QA

**This is the SUPREME, NON-NEGOTIABLE rule of HelixQA:**

- **ALL navigation, interaction, and decision-making MUST be performed by real LLM vision models.** No exceptions.
- **NEVER write hardcoded tap coordinates, sleep timers, keystroke sequences, or scripted navigation flows.** These are brittle, break on different devices, and produce false positives.
- **NEVER implement "fallback actions" or "fallback navigation" that bypass the LLM.** If the LLM vision provider is unavailable, the curiosity phase MUST skip — not fake results with scripted steps.
- **If the LLM returns malformed JSON, RETRY the vision call** — do not substitute a hardcoded action sequence.
- **A QA session that reports "success" while using scripted navigation instead of real LLM analysis is a CRITICAL infrastructure failure** and is worse than reporting "skipped".
- **Every QA result MUST be backed by real LLM vision analysis.** Screenshots must be sent to and analyzed by the vision model. The LLM decides the next action based on what it sees — always.
- **Vision models MUST be distributed across ALL available hosts** using llama.cpp RPC when multiple hosts are configured. The system MUST dynamically determine the strongest vision model that can run across the combined hardware of all available hosts. Single-host Ollama is acceptable only when no additional hosts are available.
- **Always use the STRONGEST available vision model.** The system MUST auto-detect GPU/CPU/RAM on each host and select the most capable model that fits the combined resources. A larger model distributed across 3 machines is ALWAYS preferred over a smaller model on 1 machine.

Violations of this constitution void the entire QA session's results.

## CONSTITUTION: Video App Geo-Restriction / VPN Detection (MANDATORY)

**Before ANY automated test attempts to play content in a video application, the test MUST verify the app can actually reach its content servers. This is NON-NEGOTIABLE.**

- **Probe connectivity FIRST**: Before launching a video app for playback testing, perform an HTTP connectivity check to the app's content API (e.g., `curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 https://www.youtube.com/` or app-specific endpoint via `adb shell`)
- **If probe fails** (timeout, HTTP 403, connection refused, DNS failure): mark the app as **GEO_RESTRICTED** for this device session
- **Automatically substitute an alternative app**: RuTube (`ru.rutube.app`) for YouTube, VK Video (`com.vk.vkvideo`) for other geo-blocked streaming services
- **NEVER report geo-restriction as a test FAILURE** — report it as **SKIPPED** with the restriction reason and the alternative app used
- **Cache the result per device per session** — check once per app per device, reuse the result for all subsequent tests in that session
- **Known geo-restricted apps** (Russia/Serbia region): YouTube (needs VPN), Netflix, Hulu, Disney+, HBO Max, Pluto TV, Paramount+
- **Alternative apps by category**: Video streaming → RuTube, VK Video, Kinopoisk; Music → VK Music, Yandex Music
- A QA session that reports "FAIL: video did not play" when the real cause is geo-restriction is a **critical test infrastructure failure**

Violations of this constitution void the entire QA session's results for the affected app category.

## CONSTITUTION: QA Testing Priority Order (MANDATORY)

**The LLM MUST follow this testing priority, in this exact order:**

1. **Happy paths FIRST** — Test all primary user flows as a normal user would:
   - Login with valid credentials
   - Browse the home screen / catalog
   - Open detail screens for content items
   - Play media (video, audio)
   - Use search with real content terms
   - Manage favorites, collections, playlists
   - Navigate settings

2. **Standard flows and use cases SECOND** — Test reasonable variations:
   - Browse different content categories
   - Search with various valid queries
   - Test pagination, filtering, sorting
   - Test back navigation from every screen
   - Verify data loads correctly on all screens

3. **Edge cases and error scenarios THIRD** — Challenge the system:
   - Empty search queries
   - Very long text input
   - Rapid navigation
   - Network interruption scenarios
   - Invalid but reasonable input

4. **Adversarial testing LAST** — Only after all above are covered:
   - Invalid credentials
   - Unexpected input formats
   - Stress testing UI elements

**CRITICAL RULES:**
- **NEVER type login credentials into search fields.** The LLM MUST understand which screen it is on and use context-appropriate input.
- **NEVER repeat the same action pattern for more than 3 consecutive steps.** If stuck, navigate to a DIFFERENT screen.
- **Search queries MUST be content-related** (e.g., movie titles, genres, artists) — NOT usernames, passwords, or test strings.
- **After login, IMMEDIATELY explore the app** — do not return to the login screen.
- **Every screen transition MUST be intentional** — the LLM must state WHY it's navigating there.

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## MANDATORY: Screenshot and Video Validation

**Every autonomous QA session MUST validate its own evidence. This is NON-NEGOTIABLE.**

- After every login attempt, verify via UI dump that "Sign In" text is ABSENT. If present, login FAILED — do NOT proceed
- After every phase transition, analyze the latest screenshot to confirm expected screen state
- Compare screen content against API/database data — empty screens when data exists = BUG to report
- Review video recordings for visual glitches, frozen frames, unexpected app exits
- A session that reports "success" while the app never left the login screen is a **critical test infrastructure failure**
- False positives are UNACCEPTABLE — every "PASS" must be backed by visual evidence
- API keys and secrets MUST NEVER be committed to git

## MANDATORY: Evidence-Backed Issue Tickets

**Every issue ticket created during or after a QA session MUST include concrete evidence references. This is NON-NEGOTIABLE.**

- **Video reference**: Exact filename of the video recording AND exact timestamp (MM:SS) where the issue is visible
- **Screenshot reference**: Exact filename(s) of screenshot(s) showing the issue (both before and after states when applicable)
- **Session reference**: HelixQA session ID and step number where the issue was observed
- **Reproduction path**: Sequence of actions that led to the issue (derived from the session log)
- Tickets WITHOUT video/screenshot evidence are INVALID and must be rejected
- Post-QA analysis of all video recordings and screenshots is MANDATORY — every frame must be examined for UI/UX imperfections
- ALL issues discovered during video/screenshot analysis (visual glitches, misaligned elements, truncated text, missing images, broken animations, wrong colors, empty screens with data, oversized/undersized elements) MUST result in tickets with evidence
- Video recordings MUST be valid (non-trivial file size, playable, correct duration) — a 20KB recording for a 30-minute session is a CRITICAL infrastructure failure
- Screenshots MUST show visual changes between steps — consecutive identical screenshots indicate a navigation or capture failure

## MANDATORY: Device State Preservation

**A QA session MUST return every test device to the exact state it was in at session start. This is NON-NEGOTIABLE.** (Catalogizer Constitution Article VIII.)

- The pipeline captures sensitive `settings get system|secure` keys at Phase 0b (right after ADB reverse proxy) and registers a `defer` to restore them on any exit path (normal, crash, ctrl-C, context timeout). Implementation: `pkg/autonomous/device_preserve.go`.
- Preserved keys include: `system.font_scale`, `system.screen_off_timeout`, `system.screen_brightness`, `system.screen_brightness_mode`, `system.accelerometer_rotation`, `secure.accessibility_font_scaling_has_been_changed`. Add new keys as operator reports surface them.
- The LLM-driven curiosity phase MUST NOT navigate into device Settings → Accessibility / Display / Font areas. If it does, that's a curiosity-policy bug, not justification for relaxing the preservation hook (which is defence in depth).
- A session that leaves a device with a different `font_scale`, `wm density`, brightness, or rotation than it started with is a Constitution violation.

## MANDATORY: No Manual Tooling Workarounds

**HelixQA is testing infrastructure. If it produces broken output, fix the Go code — don't paper over it with a bash script.** (Constitution Article IX.)

- No manually-invoked `adb shell screenrecord` loops pulled from the operator side. The recorder (`pkg/video/scrcpy.go`) handles the 180-second `screenrecord` cap by looping segments and concatenating them via `ffmpeg -f concat -c copy`. A 2-hour session produces a continuous 2-hour MP4.
- No `tee`-style exit-code laundering in the orchestrator. The shell script captures the exit code of the HelixQA binary directly (`PIPESTATUS[0]`) and refuses to print "✓ completed successfully" unless the real exit code is zero AND the `pipeline-report.json` doesn't contain `"Session failed"`.
- No log line that says "✓ PASSED" or "✓ completed successfully" may be reachable without its gating assertion passing (FIX-QA-2026-04-20-001/002).

## MANDATORY: Flawless Session Documentation

**Every QA session MUST produce complete, valid, and analyzable documentation. This is NON-NEGOTIABLE.**

- Video recordings must be properly finalized (remote screenrecord process killed via `killall -INT` before pull)
- Screenshots must capture BOTH pre-action and post-action states for every curiosity step
- All platforms and apps tested must have their own screenshots and video recordings
- Session logs must include per-step timing, actions taken, and LLM reasoning
- Pipeline report must accurately reflect tests run, coverage, and issues found
- All evidence must be archived in the session directory under `qa-results/session-*/`

## MANDATORY: Prepared Test Plans + LLM-Driven Execution

**Test plans with steps and data MUST be prepared in advance. The LLM vision drives the EXECUTION — how to interact with the UI. This is NON-NEGOTIABLE.**

### What MUST be prepared (test banks):
- Test case ID, name, priority, platform
- **What to do**: "Navigate to Movies category", "Search for 'Matrix'", "Open entity detail for a movie"
- **What data to use**: Specific titles, usernames, search terms, boundary values
- **What to verify**: "Movies grid displays", "Search results contain 'Matrix'", "Cover art is visible"
- Test cases organized by priority: happy paths → standard flows → edge cases → adversarial

### What the LLM decides at runtime:
- **How to click**: Which UI element to tap/focus, exact coordinates determined by vision
- **How to navigate**: Which DPAD directions, how many presses, which menu items
- **How to type**: When to press DPAD_CENTER first (Android TV keyboard), when to use TAB
- **How to verify**: Analyzing screenshot content against expected outcomes

### Hard rules:
- **NEVER** write fixed tap coordinates, sleep timers, or keystroke sequences
- The `helixqa autonomous` command handles device detection, screenshot→LLM→action loop, validation, reporting
- If the autonomous pipeline doesn't work, **fix the Go code** — do NOT write bash workarounds
- The LLM vision analyzes each screenshot, decides the next action (tap, type, swipe, DPAD), and validates the result
- On Android TV: the LLM must know that DPAD_CENTER opens the keyboard before `input text` works
- `uiautomator dump` failures ("null root node") are real bugs to fix, not to ignore
- Every connected ADB device MUST be tested. Skipping devices = failure
- **Stay in the fix-test loop** until the pipeline completes with verified screenshots showing ALL screens navigated with real data

## Overview

`digital.vasic.helixqa` is a QA orchestration framework built on top of the `digital.vasic.challenges` and `digital.vasic.containers` Go modules. It provides real-time crash/ANR detection, step-by-step validation, and evidence-based reporting for cross-platform testing.

**Module**: `digital.vasic.helixqa` (Go 1.24+)
**Depends on**: `digital.vasic.challenges`, `digital.vasic.containers`

## Vision Provider Architecture

HelixQA uses a **phase-specific model selection architecture** for autonomous QA sessions. Each pipeline phase uses a different LLMsVerifier strategy optimised for that phase's requirements:

### Phase-Specific Strategy Selection

| Phase | Strategy | Model Type | Optimised For |
|-------|----------|-----------|---------------|
| **Learn** | `PlanningStrategy` | Chat | Large context window, fast text processing |
| **Plan** | `PlanningStrategy` | Chat | Strong reasoning, structured test case output |
| **Execute** | `NavigationStrategy` | Vision | JSON action arrays, GUI understanding, speed |
| **Curiosity** | `NavigationStrategy` | Vision | JSON action arrays, exploration, speed |
| **Analyze** | `AnalysisStrategy` | Vision | Rich descriptions, OCR, object detection |

The `PhaseModelSelector` wires each phase to its optimal strategy via LLMsVerifier presets:
- `recipe.NavigationPreset()` for Execute/Curiosity -- prioritises JSON compliance (40%), GUI understanding (25%), speed (20%), cost (15%)
- `recipe.AnalysisPreset()` for Analyze -- prioritises description quality (35%), OCR (20%), object detection (20%), comprehensiveness (15%), cost (10%)
- `recipe.PlanningPreset()` for Learn/Plan -- prioritises reasoning (35%), context window (25%), structured output (20%), speed (10%), cost (10%)

### Vision Models (Execute/Curiosity/Analyze phases)
Used for screenshot analysis and UI navigation.

**MANDATORY: llama.cpp RPC distributed inference** is the primary local vision backend. It distributes model layers across ALL configured host machines (thinker.local GPU + amber.local CPU + localhost). This is SUPERIOR to Ollama and is the required approach when multiple hosts are available. Ollama is only a fallback when llama.cpp RPC is unavailable.

- **llama.cpp RPC** (distributed, MANDATORY) -- Split vision models across multiple hosts via RPC workers. Master runs on GPU host, workers contribute RAM. Configured via `HELIX_LLAMACPP=true`, `HELIX_LLAMACPP_RPC_WORKERS`, `HELIX_LLAMACPP_MODEL`.
- **Astica.AI** (cloud, Analyze only) -- Specialized computer vision API for rich analysis. Cannot produce JSON actions, so NOT used for Execute/Curiosity. Configured via `ASTICA_API_KEY`.
- **Gemini/OpenAI/Kimi** (cloud) -- Cloud vision providers for navigation (JSON-capable).
- **Ollama** (local fallback) -- Only when llama.cpp RPC is unavailable. Inferior to llama.cpp for performance.

### Chat Models (Learn/Plan phases)
Used for knowledge base processing, test generation, and report writing.
- Any provider supporting text chat (OpenAI, Anthropic, Gemini, Groq, Mistral, etc.)
- Selected dynamically by LLMsVerifier `PlanningStrategy` based on reasoning quality and context window.

### Bridged CLI Models
Models available via CLI coding assistants (Claude Code, Qwen Coder, OpenCode) are discovered by `pkg/bridge/` and included in the scoring pool. They have zero token cost (CLI handles billing) and compete alongside cloud and local models. Only Claude Code currently supports vision input.

### Dynamic Model Selection (no hardcoded preferences)
Model selection is handled by LLMsVerifier using phase-specific strategies (`NavigationStrategy` for Execute/Curiosity, `AnalysisStrategy` for Analyze, `PlanningStrategy` for Learn/Plan). There are no hardcoded model preferences. All available providers are probed, scored by the appropriate strategy, and the best is selected at runtime.

### Vision Provider Scoring (pkg/llm/vision_ranking.go)
The `rankVisionProviders()` function dynamically scores and sorts providers:
- **Score formula**: `(0.6 * quality + 0.4 * reliability) * availability * costBonus`
- **Availability**: providers with configured API keys get 2x multiplier; Ollama is always available
- **Cost bonus**: free providers get 1.10x; cheap (<$0.002/1k) get 1.05x
- **Local Ollama models** (thinker.local, amber.local) are scored alongside cloud providers
- Provider scores are derived from `visionModelRegistry` which mirrors LLMsVerifier's `VisionModelRegistry()`
- Both registries MUST stay in sync -- see `LLMsVerifier/pkg/helixqa/models.go`

### Local Ollama Configuration
Local Ollama models participate in vision scoring via `HELIX_OLLAMA_URL`:
```bash
HELIX_OLLAMA_URL=http://thinker.local:11434  # Ollama API endpoint
HELIX_OLLAMA_MODEL=minicpm-v:8b              # Vision model name
```
When configured, Ollama appears in both the vision and chat provider pools. The `ollamaProvider.Vision()` method sends base64-encoded screenshots to `/api/chat` with the images array.

### MANDATORY: llama.cpp RPC Distributed Inference
- **This is NON-NEGOTIABLE when multiple hosts are configured.**
- llama.cpp RPC MUST be used instead of Ollama when hosts are available.
- Each host runs `rpc-server` binary built with `-DGGML_RPC=ON`.
- The master node (GPU host) runs `llama-server` with `--rpc worker1:port,worker2:port`.
- Model files (GGUF) are stored on the master; layers distributed to workers.
- `HELIX_LLAMACPP_FREE_GPU=true` stops Ollama to reclaim GPU VRAM for llama.cpp.

### Host Machine Configuration
Distributed vision runs across multiple machines:
- **thinker.local** -- GPU host (RTX 3060 6GB), master node, GGUF models in `~/models/`
- **amber.local** -- CPU host, RPC worker contributing RAM
- **localhost** -- Orchestrator, optional RPC worker
- SSH keys configured for passwordless access
- `HELIX_VISION_HOSTS=thinker.local,amber.local` in .env
- `HELIX_LLAMACPP_RPC_WORKERS=thinker.local:50052,amber.local:50052` in .env

## Critical Constraint

HelixQA IMPORTS from Challenges and Containers -- it NEVER reimplements their functionality. Use `digital.vasic.challenges` and `digital.vasic.containers` packages directly.

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go vet ./...
make help                # Show all targets
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party (challenges/containers), internal (blank line separated)
- Line length target 80 chars (100 max)
- Naming: `camelCase` private, `PascalCase` exported
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven where appropriate, `testify`, naming `Test<Struct>_<Method>_<Scenario>`
- SPDX headers on every .go file

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/config` | Configuration types (platforms, speed, report format) |
| `pkg/testbank` | YAML test bank management with platform/priority filtering |
| `pkg/detector` | Real-time crash/ANR detection (Android ADB, Web, Desktop) |
| `pkg/validator` | Step-by-step validation with evidence collection |
| `pkg/evidence` | Centralized evidence collection (screenshots, video, logs) |
| `pkg/ticket` | Markdown ticket generation for AI fix pipelines |
| `pkg/reporter` | QA report generation (reuses `challenges/pkg/report`) |
| `pkg/orchestrator` | Main QA brain tying everything together |
| `pkg/llm` | LLM provider abstraction, adaptive fallback, cost tracking |
| `cmd/helixqa` | CLI entry point (subcommands: run, list, report, version) |

## Autonomous QA Pipeline

The `helixqa autonomous` command runs a 5-phase pipeline:

| Phase | Description |
|-------|-------------|
| 0. Deploy | Auto-ensures Ollama + vision model on remote host via SSH (`HELIX_VISION_HOST`) |
| 1. Learn | Scans project docs, code, git for knowledge base |
| 2. Plan | LLM generates test cases from knowledge |
| 3. Execute | Screenshots + video recording per test |
| 3.5 Curiosity | LLM vision drives exploration (login, browse, favorites, play) |
| 4. Analyze | LLM vision analyzes screenshots, creates deduplicated issue tickets |

### Remote Vision (Auto-Deploy)

HelixQA auto-deploys Ollama on a remote GPU host before each session:
```bash
HELIX_VISION_HOST=thinker.local   # Remote host with GPU
HELIX_VISION_USER=milosvasic      # SSH user
HELIX_VISION_MODEL=llava:7b       # Vision model to use
HELIX_OLLAMA_URL=http://thinker.local:11434  # Ollama API endpoint
```

The deployer (from `digital.vasic.visionengine/pkg/remote`) checks: Ollama installed → API running → model pulled. All automatic, no manual setup needed.

### Output Structure

```
qa-results/
├── latest -> session-NNNN   # Symlink to most recent session (gitignored)
├── session-1774785711/
│   ├── screenshots/          # PNG screenshots (execute + curiosity phases)
│   ├── videos/               # MP4 recordings (pulled from Android device)
│   ├── evidence/             # Logcat dumps, crash traces
│   ├── frames/               # Video frame extracts
│   └── pipeline-report.json  # Session results (tests, coverage, issues)
```

### Issue Deduplication

`FindingsBridge.Process()` prevents duplicate tickets:
- Same-title findings are skipped (cross-session dedup via `FindDuplicateByTitle`)
- Intra-batch duplicates tracked in memory
- Related findings in same category+platform are grouped with "Related Issues" section

## LLM Cost Tracking

Every autonomous QA session tracks the cost of all LLM API calls. The cost tracker is created automatically by `NewSessionPipeline` and attached to all `AdaptiveProvider` instances.

### Architecture

- `pkg/llm/cost_tracker.go` -- `CostTracker` accumulates per-call cost records (thread-safe via `sync.RWMutex`)
- `pkg/llm/adaptive.go` -- `AdaptiveProvider.recordCost()` auto-records after every successful `Chat()` and `Vision()` call
- `pkg/autonomous/pipeline.go` -- `SessionPipeline` creates the tracker, sets the phase label before each phase, and attaches cost summary to `PipelineResult`

### Cost Rates

Rates are sourced from `visionModelRegistry` (vision_ranking.go) and LLMsVerifier's `VisionModelRegistry()` (models.go):

| Provider | Input $/1K tokens | Output $/1K tokens |
|----------|-------------------|--------------------|
| OpenAI | 0.005 | 0.015 |
| Anthropic | 0.003 | 0.015 |
| Google | 0.0001 | 0.0004 |
| Kimi | 0.0003 | 0.0006 |
| Astica | 0.0005 | 0.0005 |
| Qwen | 0.001 | 0.002 |
| xAI | 0.0025 | 0.0025 |
| Ollama | 0.0 | 0.0 (free/local) |
| StepFun | 0.0 | 0.0 (free tier) |
| NVIDIA | 0.0 | 0.0 (free tier) |

Custom rates can be set via `CostTracker.SetRate(provider, CostRate{...})`.

### Report Output

Cost data is included in `pipeline-report.json` under the `cost` field:

```json
{
  "cost": {
    "total_cost_usd": 0.042,
    "total_calls": 15,
    "total_input_tokens": 25000,
    "total_output_tokens": 8000,
    "by_provider": { "google": { "calls": 10, "total_cost_usd": 0.004 } },
    "by_phase": { "plan": 0.02, "execute": 0.01, "curiosity": 0.008, "analyze": 0.004 },
    "by_call_type": { "chat": 0.03, "vision": 0.012 }
  }
}
```

### API

- `SessionPipeline.CurrentCost()` -- returns a `CostSummary` snapshot (safe to call during a running session)
- `CostTracker.Summary()` -- full summary with individual records
- `CostTracker.SummaryCompact()` -- summary without records (for progress reporting)
- `CostTracker.TotalCost()` / `CostByProvider()` / `CostByModel()` / `CostByPhase()` -- individual breakdowns

### Token Estimation

When a provider does not report token counts (InputTokens/OutputTokens are both 0), the system estimates output tokens as `len(content) / 4` (roughly 1 token per 4 characters). Providers that do report tokens (OpenAI, Anthropic, Google, Ollama) use exact counts.

## Key Interfaces

- `detector.CommandRunner` -- abstraction for command execution (testable)
- `report.Reporter` (from challenges) -- report generation
- `runner.Runner` (from challenges) -- challenge execution
- `bank.Bank` (from challenges) -- test bank loading

## Design Patterns

- **Functional Options**: All constructors use `WithX()` options
- **Dependency Injection**: `CommandRunner` interface for detector testing
- **Composition**: Orchestrator composes detector + validator + reporter + runner
- **Evidence-Based**: All failures include screenshots, logs, stack traces

## Commit Style

Conventional Commits: `feat(detector): add iOS crash detection`


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**


## MANDATORY API KEY & SECRETS CONSTRAINTS

- **NEVER commit `.env` files** — they contain real API keys for LLM providers
- **NEVER add API keys to source code** — use environment variables or `.env` files only
- **ALWAYS verify `.gitignore` protects `.env`** before committing
- `.env.example` files (templates with placeholder keys) are OK to commit
- `.env` file permissions MUST be `chmod 600` (owner read/write only)
- Before every commit: verify `git ls-files --cached | grep ".env"` shows NO `.env` files
