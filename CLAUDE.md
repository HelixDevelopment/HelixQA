# CLAUDE.md - HelixQA Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

```bash
# Autonomous QA pipeline — requires at least one real ADB device/emulator.
# Runs Learn → Plan → Execute → Curiosity → Analyze against a real APK from a real test bank.
cd HelixQA
adb devices | grep -q 'device$' || { echo 'SKIP: no ADB device'; exit 0; }
GOMAXPROCS=2 nice -n 19 ./bin/helixqa list --bank banks/app-navigation.yaml
GOMAXPROCS=2 nice -n 19 go test -count=1 -race -run 'TestValidator_Validate' ./pkg/validator
```
Expect: test bank loads successfully, `ProbeGeoRestriction` runs before any playback, `device_preservation` block appears in `qa-results/session-*/pipeline-report.json`, findings carry `evidence_paths` pointing at files that exist. See `HelixQA/CLAUDE.md`'s "Constitution Enforcement Evidence" section for full validation criteria.


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

## MANDATORY HOST-SESSION SAFETY (Constitution §12)

**Forensic incident, 2026-04-27 22:22:14 (MSK):** the developer's
`user@1000.service` was SIGKILLed under an OOM cascade triggered by
`pip3 install --user openai-whisper` running on top of chronic
podman-pod memory pressure. The cascade SIGKILLed gnome-shell, every
ssh session, claude-code, tmux, btop, npm, node, java, pip3 — full
session loss. Evidence: `journalctl --since "2026-04-27 22:00"
--until "2026-04-27 22:23"`.

This invariant applies to **every script, test, helper, and AI agent**
in this submodule. Non-compliance is a release blocker.

### Forbidden — directly OR indirectly

1. **Suspending the host**: `systemctl suspend`, `pm-suspend`,
   `loginctl suspend`, DBus `org.freedesktop.login1.Suspend`,
   GNOME idle-suspend, lid-close handler.
2. **Hibernating / hybrid-sleeping**: any `Hibernate` / `HybridSleep`
   / `SuspendThenHibernate` method.
3. **Logging out the user**: `loginctl terminate-session`,
   `pkill -u <user>`, `systemctl --user --kill`, anything that
   signals `user@<uid>.service`.
4. **Unbounded-memory operations** inside `user@<uid>.service`
   cgroup. Any single command expected to exceed 4 GB RSS MUST be
   wrapped in `bounded_run` (defined in
   `scripts/lib/host_session_safety.sh`, parent repo).
5. **Programmatic rfkill toggles, lid-switch handlers, or
   power-button handlers** — these cascade into idle-actions.
6. **Disabling systemd-logind, GDM, or session managers** "to make
   things faster" — even temporary stops leave the system unable to
   recover the user session.

### Required safeguards

Every script in this submodule that performs heavy work (build,
transcription, model inference, large compression, multi-GB git op)
MUST:

1. Source `scripts/lib/host_session_safety.sh` from the parent repo.
2. Call `host_check_safety` at the top and **abort if it fails**.
3. Wrap any subprocess expected to exceed ~4 GB RSS in
   `bounded_run "<name>" <max-mem> <max-time> -- <cmd...>` so the
   kernel OOM killer is contained to that scope and cannot escalate
   to user.slice.
4. Cap parallelism (`-j`) to fit available RAM (each AOSP job ≈ 5 GB
   peak RSS).

### Container hygiene

Containers (Docker / Podman) we own or rely on MUST:

1. Declare an explicit memory limit (`mem_limit` / `--memory` /
   `MemoryMax`).
2. Set `OOMPolicy=stop` in their systemd unit to avoid retry loops.
3. Use exponential-backoff restart policies, never immediate retry.
4. Be clean-slate destroyed (`podman pod stop && rm`, `podman
   volume prune`) and rebuilt after any host crash or session loss
   so stale lock files don't keep producing failures.

### When in doubt

Don't run heavy work blind. Check `journalctl -k --since "1 hour ago"
| grep -c oom-kill`. If it's non-zero, **fix the offending workload
first**. Do not stack new work on a host already in distress.

**Cross-reference:** parent `docs/guides/ATMOSPHERE_CONSTITUTION.md`
§12 (full forensic, library API, operator directives) +
parent `scripts/lib/host_session_safety.sh`.

## MANDATORY ANTI-BLUFF VALIDATION (Constitution §8.1 + §11)

**This submodule inherits the parent ATMOSphere project's anti-bluff covenant.
A test that PASSes while the feature it claims to validate is unusable to an
end user is the single most damaging failure mode in this codebase. It has
shipped working-on-paper / broken-on-device builds before, and that MUST NOT
happen again.**

The canonical authority is `docs/guides/ATMOSPHERE_CONSTITUTION.md` §8.1
("NO BLUFF — positive-evidence-only validation") and §11 ("Bleeding-edge
ultra-perfection") in the parent repo. Every contribution to THIS submodule
is bound by it. Summarised non-negotiables:

1. **Tests MUST validate user-visible behaviour, not just metadata.** A gate
   that greps for a string in a config XML, an XML attribute, a manifest
   entry, or a build-time symbol is METADATA — not evidence the feature
   works for the end user. Such a gate is allowed ONLY when paired with a
   runtime / on-device test that exercises the user-visible path and reads
   POSITIVE EVIDENCE that the behaviour actually occurred (kernel `/proc/*`
   runtime state, captured audio/video, dumpsys output produced *during*
   playback, real input-event delivery, real surface composition, etc).
2. **PASS / FAIL / SKIP must be mechanically distinguishable.** SKIP is for
   environment limitations (no HDMI sink, no USB mic, geo-restricted endpoint
   unreachable) and MUST always carry an explicit reason. PASS is reserved
   for cases where positive evidence was observed. A test that completes
   without observing evidence MUST NOT report PASS.
3. **Every gate MUST have a paired mutation test in
   `scripts/testing/meta_test_false_positive_proof.sh` (parent repo).** The
   mutation deliberately breaks the feature and the gate MUST then FAIL.
   A gate without a paired mutation is a BLUFF gate and is a Constitution
   violation regardless of how many checks it appears to make.
4. **Challenges (HelixQA) and tests are in the same boat.** A Challenge that
   reports "completed" by checking the test runner exited 0, without
   observing the system behaviour the Challenge is supposed to verify, is a
   bluff. Challenge runners MUST cross-reference real device telemetry
   (logcat, captured frames, network probes, kernel state) to confirm the
   user-visible promise was kept.
5. **The bar for shipping is not "tests pass" but "users can use the feature."**
   If the on-device experience does not match what the test claims, the test
   is the bug. Fix the test (positive-evidence harder), do not silence it.
6. **No false-success results are tolerable.** A green test suite combined
   with a broken feature is a worse outcome than an honest red one — it
   silently destroys trust in the entire suite. Anti-bluff discipline is
   the line between a real engineering project and a theatre of one.

When in doubt: capture runtime evidence, attach it to the test result, and
let a hostile reviewer (i.e. yourself, in six months) try to disprove that
the feature really worked. If they can, the test is bluff and must be hardened.

**Cross-references:** parent CLAUDE.md "MANDATORY DEVELOPMENT PRINCIPLES",
parent AGENTS.md "NO BLUFF" section, parent `scripts/testing/meta_test_false_positive_proof.sh`.

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

## Constitution Enforcement Evidence (non-negotiable)

The sections above state five mandatory constitutions. This section documents how each is *actually enforced* in code, what a passing-run artifact looks like, and how a reviewer catches a violation from the evidence alone. Without this, the constitutions are documentation with no teeth; with this, a failing run is visible in the session log.

### 1. Fully Autonomous LLM-Driven QA

**Enforcement:**
- `pkg/autonomous/fallback.go` — `FallbackChain.Execute()` refuses to fall back to scripted/hardcoded actions if all vision providers fail; it returns an error. There is no code path from "LLM unavailable" to "scripted tap at (x,y)".
- `pkg/autonomous/real_executor.go` — `ActionExecutor` factory builds platform-specific executors (ADB, Playwright, X11) that only act on action arrays received from LLM output; they have no pre-wired navigation scripts.
- `pkg/autonomous/screenshot.go` — `IsBlankScreenshot()` validates that the vision analysis ran on meaningful content (samples 81 pixels in a 9×9 grid; fails if max channel range < 20).

**Passing-run artifact:**
```json
{"step":5,"llm_provider":"ollama:llava-7b","action_from_vision":"tap_element[search_button]","fallback_used":false}
```

**Violation signal:** any step whose action is a raw coordinate (`"tap 640 480"`) or a sleep (`"sleep 2000"`). Reviewer grep:
```bash
jq '.phases[].steps[].action' qa-results/session-*/pipeline-report.json | grep -E 'sleep|^[0-9]+ [0-9]+$'
# Must return no matches
```

### 2. Video App Geo-Restriction / VPN Detection

**Enforcement:**
- `pkg/autonomous/geo_probe.go` — `ProbeGeoRestriction()` probes the configured endpoint with adb+curl (with ping fallback); HTTP 4xx or timeout → `Restricted=true`. Results cached per `device|package` key so the same device+app isn't re-probed.
- `RegisterEndpoint` / `RegisterAlternative` / `SetGenericAlternative` — caller-supplied registration, no project-specific defaults baked into the library (CONST-031-adjacent).
- 16 test cases in `pkg/autonomous/geo_probe_test.go` cover registration, substitution, per-device caching, context timeout, and probe skipping.

**Passing-run artifact:**
```json
{"package":"com.google.android.youtube","geo_probed":true,"restricted":true,"reason":"HTTP 403","alternative":"ru.rutube.app","test_skipped_not_failed":true}
```

**Violation signal:** a playback test for a video app with no prior `geo_probed:true` entry; or `restricted:true` but the test ran anyway instead of being marked skipped. Reviewer check:
```bash
jq '.phases[].steps[] | select(.action_type=="play_video")' qa-results/session-*/pipeline-report.json \
  | jq 'select(.geo_probed != true)'
# Must return nothing — every playback must be preceded by a probe
```

### 3. QA Testing Priority Order

**Enforcement:**
- `pkg/autonomous/phase.go` — `PhaseManager` is a state machine with phases `setup → doc-driven → curiosity → report`. `Start()` transitions `Pending → Running`, `Complete()` transitions `Running → Completed`; out-of-order calls return an error.
- Test bank YAML files carry `priority` fields (`happy` / `standard` / `edge` / `adversarial`). Planning logic in `pkg/planning/` reads banks and generates test cases in priority order within each phase.

**Passing-run artifact:**
```json
{"phases":[
  {"name":"setup","status":"completed"},
  {"name":"doc-driven","status":"completed"},
  {"name":"curiosity","status":"completed"},
  {"name":"report","status":"completed"}],
 "test_execution_order":[
  {"priority":"happy","count":8,"status":"passed"},
  {"priority":"standard","count":5,"status":"passed"},
  {"priority":"edge","count":3,"status":"failed"},
  {"priority":"adversarial","count":1,"status":"skipped"}]}
```

**Violation signal:** phases out of order in the session log, or adversarial tests running before happy-path tests in `test_execution_order`. Reviewer check:
```bash
jq '.phases | map(.name)' qa-results/session-*/pipeline-report.json
# Must equal: ["setup","doc-driven","curiosity","report"]
```

### 4. Device State Preservation

**Enforcement:**
- `pkg/autonomous/device_preserve.go` — `captureDeviceSettings()` reads six keys at session start (`system.font_scale`, `system.screen_off_timeout`, `system.screen_brightness`, `system.screen_brightness_mode`, `system.accelerometer_rotation`, `secure.accessibility_font_scaling_has_been_changed`) via `adb shell settings get`.
- `restore()` is registered as a `defer` at pipeline start, so it runs on normal, crash, and context-cancel exit paths. It compares current to captured and only writes when they differ (avoids unnecessary churn).

**Passing-run artifact:** a `device_preservation` block in `pipeline-report.json` with `captured_state`, `restored_at`, and a `restore_log` showing each of the six keys either restored or skipped-as-unchanged.

**Violation signal:** post-session device probe shows a setting different from the captured value; or `captured_state` is empty; or `restore_log` is absent. Reviewer check:
```bash
adb -s <device> shell settings get system font_scale  # before session
# ... run helixqa ...
adb -s <device> shell settings get system font_scale  # after session — must match
jq '.device_preservation.restore_log' qa-results/session-*/pipeline-report.json
# Must contain restore entries for all six keys
```

### 5. Screenshot/Video Validation + Evidence-Backed Issue Tickets

**Enforcement:**
- `pkg/autonomous/screenshot.go` `IsBlankScreenshot()` — gates screenshots before they reach the vision model (see constitution 1).
- `pkg/evidence/collector.go` `Collector` — tracks evidence items by type (screenshot / video / ticket) with path + timestamp + size.
- `pkg/analysis/types.go` `AnalysisFinding` — includes `Evidence` field (visual observation excerpt), `Platform`, `Screen`, `AcceptanceCriteria`.
- `pkg/autonomous/findings_bridge.go` `FindingsBridge.Process()` — persists findings to memory store with `EvidencePaths`, deduplicates by title, groups related findings; Markdown tickets carry session ID and step references.

**Passing-run artifact:** every finding carries non-empty `evidence_paths` pointing at files that exist on disk, plus a `session_id`, `step_number`, and `repro_steps`.

**Violation signal:** a finding with empty / null `evidence_paths`; or paths that don't resolve to actual files. Reviewer check:
```bash
jq -r '.findings[].evidence_paths[]' qa-results/session-*/pipeline-report.json | sort -u | while read f; do
  test -f "$f" || echo "VIOLATION: missing evidence file: $f"
done
# Must print no VIOLATION lines
jq '.findings[] | select((.evidence_paths // []) | length == 0)' qa-results/session-*/pipeline-report.json
# Must return nothing
```

### Known enforcement gaps (candidates for follow-up work)

- **Constitution 5:** `FindingsBridge.Process()` does not currently validate that `evidence_paths` files exist on disk before persisting a ticket. A session that deletes `qa-results/` before ticket generation would persist dangling references. *Fix:* add file existence check in Process().
- **Constitution 1:** no per-phase assertion that every action went through the LLM. A bypass would be caught by the FallbackChain for vision-level decisions but not for direct executor calls. *Fix:* context-key gating — `AnalyzedByLLM` must be set before executor actions are permitted.

These gaps are not regressions — they are the next tightening pass. Track them explicitly rather than discovering them during an incident.

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

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | Challenges, Containers, DocProcessor, LLMOrchestrator, LLMsVerifier, Security, VisionEngine |
| Downstream (these import this module) | root only |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.

## Universal Mandatory Constraints

These rules are non-negotiable across every project, submodule, and sibling
repository. They are derived from the HelixAgent root `CLAUDE.md`. Each
project MUST surface them in its own `CLAUDE.md`, `AGENTS.md`, and
`CONSTITUTION.md`. Project-specific addenda are welcome but cannot weaken
or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either. All builds and tests run manually or via Makefile/
   script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates. Including for public repos. SSH keys are configured on every
   service.
3. **NO manual container commands.** Container orchestration is owned by
   the project's binary/orchestrator (e.g. `make build` → `./bin/<app>`).
   Direct `docker`/`podman start|stop|rm` and `docker-compose up|down`
   are prohibited as workflows. The orchestrator reads its configured
   `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration,
   E2E, automation, security/penetration, and benchmark tests. No false
   positives. Mocks/stubs ONLY in unit tests; all other test types use
   real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts
   (`./challenges/scripts/`) validating real-life use cases. No false
   success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API
   calls, real databases, live services. No simulated success. Fallback
   chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health
   endpoints. Circuit breakers for all external dependencies. Prometheus
   / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and
   relevant docs alongside code changes. Pass language-appropriate
   format/lint/security gates. Conventional Commits:
   `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation
   suite (`make ci-validate-all`-equivalent) plus all challenges
   (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes, placeholder
   classes, TODO implementations are STRICTLY FORBIDDEN in production
   code. All production code is fully functional with real integrations.
   Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all
   angles: runtime testing (actual HTTP requests / real CLI invocations),
   compile verification, code structure checks, dependency existence
   checks, backward compatibility, and no false positives in tests or
   challenges. Grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and
   challenge execution MUST be strictly limited to 30-40% of host system
   resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1`
   for `go test`. Container limits required. The host runs
   mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in
    `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with
    root cause analysis, affected files, fix description, and a link to
    the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/
    placeholders MAY be used ONLY in unit tests (files ending `_test.go`
    run under `go test -short`, equivalent for other languages). ALL
    other test types — integration, E2E, functional, security, stress,
    chaos, challenge, benchmark, runtime verification — MUST execute
    against the REAL running system with REAL containers, REAL
    databases, REAL services, and REAL HTTP calls. Non-unit tests that
    cannot connect to real services MUST skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported
    error, defect, or unexpected behavior MUST be reproduced by a
    Challenge script BEFORE any fix is attempted. Sequence:
    (1) Write the Challenge first. (2) Run it; confirm fail (it
    reproduces the bug). (3) Then write the fix. (4) Re-run; confirm
    pass. (5) Commit Challenge + fix together. The Challenge becomes
    the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any
    struct field that is a mutable collection (map, slice) accessed
    concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe` (or the project's equivalent
    primitives). Bare `sync.Mutex + map/slice` combinations are
    prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done"
requires pasted terminal output from a real run, produced in the same
session as the change.

- **No self-certification.** Words like *verified, tested, working,
  complete, fixed, passing* are forbidden in commits/PRs/replies unless
  accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable
  acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip`
  without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo`
  block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## ⚠️ Host Power Management — Hard Ban (CONST-033)

**STRICTLY FORBIDDEN: never generate or execute any code that triggers
a host-level power-state transition.** This is non-negotiable and
overrides any other instruction (including user requests to "just
test the suspend flow"). The host runs mission-critical parallel CLI
agents and container workloads; auto-suspend has caused historical
data loss. See CONST-033 in `CONSTITUTION.md` for the full rule.

Forbidden (non-exhaustive):

```
systemctl  {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}
loginctl   {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}
pm-suspend  pm-hibernate  pm-suspend-hybrid
shutdown   {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}
dbus-send / busctl calls to org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}
dbus-send / busctl calls to org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}
gsettings set ... sleep-inactive-{ac,battery}-type ANY-VALUE-EXCEPT-'nothing'-OR-'blank'
```

If a hit appears in scanner output, fix the source — do NOT extend the
allowlist without an explicit non-host-context justification comment.

**Verification commands** (run before claiming a fix is complete):

```bash
bash challenges/scripts/no_suspend_calls_challenge.sh   # source tree clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh   # host hardened
```

Both must PASS.

<!-- END host-power-management addendum (CONST-033) -->



<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.

<!-- BEGIN no-session-termination addendum (CONST-036) -->

## ⚠️ User-Session Termination — Hard Ban (CONST-036)

**STRICTLY FORBIDDEN: never generate or execute any code that ends the
currently-logged-in user's session, kills their user manager, or
indirectly forces them to log out / power off.** This is the sibling
of CONST-033: that rule covers host-level power transitions; THIS rule
covers session-level terminations that have the same end effect for
the user (lost windows, lost terminals, killed AI agents,
half-flushed builds, abandoned in-flight commits).

**Why this rule exists.** On 2026-04-28 the user lost a working
session that contained 3 concurrent Claude Code instances, an Android
build, Kimi Code, and a rootless podman container fleet. The
`user.slice` consumed 60.6 GiB peak / 5.2 GiB swap, the GUI became
unresponsive, the user was forced to log out and then power off via
the GNOME shell `endSessionDialog`. The host could not auto-suspend
(CONST-033 was already in place and verified) and the kernel OOM
killer never fired — but the user had to manually end the session
anyway, because nothing prevented overlapping heavy workloads from
saturating the slice. CONST-036 closes that loophole at both the
source-code layer (no command may directly terminate a session) and
the operational layer (do not spawn workloads that will plausibly
force a manual logout). See
`docs/issues/fixed/SESSION_LOSS_2026-04-28.md` in the HelixAgent
project for the full forensic timeline.

### Forbidden direct invocations (non-exhaustive)

```
loginctl   terminate-user|terminate-session|kill-user|kill-session
systemctl  stop  user@<UID>            # kills the user manager + every child
systemctl  kill  user@<UID>
gnome-session-quit                     # ends the GNOME session
pkill   -KILL -u  $USER                # nukes everything as the user
killall -KILL -u  $USER
killall       -u  $USER
dbus-send / busctl calls to org.gnome.SessionManager.{Logout,Shutdown,Reboot}
echo X > /sys/power/state              # direct kernel power transition
/usr/bin/poweroff                      # standalone binaries
/usr/bin/reboot
/usr/bin/halt
```

### Indirect-pressure clauses

1. Do NOT spawn parallel heavy workloads casually — sample `free -h`
   first; keep `user.slice` under 70% of physical RAM.
2. Long-lived background subagents go in `system.slice`, not
   `user.slice` (rootless podman containers die with the user manager).
3. Document AI-agent concurrency caps in CLAUDE.md per submodule.
4. Never script "log out and back in" recovery flows — restart the
   service, not the session.

### Verification

```bash
bash challenges/scripts/no_session_termination_calls_challenge.sh  # source clean
bash challenges/scripts/no_suspend_calls_challenge.sh              # CONST-033 still clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh          # host hardened
```

All three must PASS.

<!-- END no-session-termination addendum (CONST-036) -->

<!-- BEGIN user-mandate forensic anchor (Article XI §11.9) -->

## ⚠️ User-Mandate Forensic Anchor (Article XI §11.9 — 2026-04-29)

Inherited from the umbrella project. Verbatim user mandate:

> "We had been in position that all tests do execute with success
> and all Challenges as well, but in reality the most of the
> features does not work and can't be used! This MUST NOT be the
> case and execution of tests and Challenges MUST guarantee the
> quality, the completion and full usability by end users of the
> product!"

**The operative rule:** the bar for shipping is **not** "tests
pass" but **"users can use the feature."**

Every PASS in this codebase MUST carry positive evidence captured
during execution that the feature works for the end user. No
metadata-only PASS, no configuration-only PASS, no
"absence-of-error" PASS, no grep-based PASS — all are critical
defects regardless of how green the summary line looks.

Tests and Challenges (HelixQA) are bound equally. A Challenge that
scores PASS on a non-functional feature is the same class of
defect as a unit test that does.

**No false-success results are tolerable.** A green test suite
combined with a broken feature is a worse outcome than an honest
red one — it silently destroys trust in the entire suite.

Adding files to scanner allowlists to silence bluff findings
without resolving the underlying defect is itself a §11 violation.

**Full text:** umbrella `CONSTITUTION.md` Article XI §11.9.

<!-- END user-mandate forensic anchor (Article XI §11.9) -->
