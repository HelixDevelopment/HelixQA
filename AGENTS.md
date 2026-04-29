# AGENTS.md - HelixQA Development Guide

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

## MANDATORY: Project-Agnostic / 100% Decoupled

**This module MUST remain 100% decoupled from any consuming project. It is designed for generic use with ANY project, not one specific consumer.**

- NEVER hardcode project-specific package names, endpoints, device serials, or region-specific data
- NEVER import anything from a consuming project
- NEVER add project-specific defaults, presets, or fixtures into source code
- All project-specific data MUST be registered by the caller via public APIs — never baked into the library
- Default values MUST be empty or generic

Violations void the release. Refactor to restore generic behaviour before any commit.

## MANDATORY: No CI/CD Pipelines

**NO GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository!**

- No `.github/workflows/` directory
- No `.gitlab-ci.yml` file
- No Jenkinsfile, .travis.yml, .circleci, or any other CI configuration
- All builds and tests are run manually or via Makefile targets
- This rule is permanent and non-negotiable

## MANDATORY: Everything Runs Inside Containers

**ALL execution MUST happen inside Docker/Podman containers. No exceptions.**

- All builds, tests, dev servers, QA campaigns, and any process execution MUST run inside containers
- Client apps (admin, web, desktop) MUST be served from containers
- Mobile testing MUST use Android emulators running inside containers (e.g., `budtmo/docker-android`)
- HelixQA campaigns MUST execute inside containers with Playwright for browser automation
- Video recording MUST happen inside containers using Playwright video capture or CDP screencast
- Never run `go build`, `npm run dev`, or any tooling directly on the host machine
- Resource limits MUST be enforced: max 35% of host CPU and RAM per CLAUDE.md constraints

## Architecture

HelixQA is a thin orchestration layer over two foundational modules:

1. **digital.vasic.challenges** -- Test execution engine with bank loading, challenge running, assertion evaluation, and report generation.
2. **digital.vasic.containers** -- Container lifecycle, health checking, and service discovery.

HelixQA adds:
- **Crash/ANR detection** via platform-specific system commands (ADB, pgrep, kill)
- **Step validation** correlating crash detection with test execution phases
- **Evidence collection** (screenshots, logs, stack traces)
- **QA-specific reporting** wrapping the Challenges report infrastructure

## Module Dependency Graph

```
digital.vasic.helixqa
  |-- digital.vasic.challenges
  |     |-- digital.vasic.containers
  |-- digital.vasic.containers (direct)
```

## Development Workflow

```bash
# Build
go build ./...

# Test with race detection
go test ./... -race -count=1

# Vet
go vet ./...

# Run CLI
go run ./cmd/helixqa --help
```

## Adding a New Platform

1. Add platform constant in `pkg/config/config.go`
2. Add detection logic in `pkg/detector/`
3. Update `Detector.Check()` dispatch in `pkg/detector/detector.go`
4. Add tests in `pkg/detector/<platform>_test.go`
5. Update `Orchestrator.getDetector()` for platform-specific options
6. Update `Config.ExpandedPlatforms()` if included in "all"

## Adding a New Report Format

1. Add format constant in `pkg/config/config.go`
2. Add writer in `pkg/reporter/reporter.go`
3. Update `WriteReport()` dispatch
4. Add tests

## Vision Provider Architecture

HelixQA uses a dual-model architecture for autonomous QA:

### Vision Models (screenshot analysis — Execute and Curiosity phases)
- **Astica.AI** — Specialized computer vision API (`ASTICA_API_KEY`)
- **Gemini 2.0 Flash** — Primary cloud vision for autonomous navigation
- **OpenAI GPT-4o** — Alternate cloud vision provider
- **Ollama** (local) — Free inference via `minicpm-v:8b` or similar (`HELIX_OLLAMA_URL`)
- **llama.cpp RPC** — Distributed inference across multiple hosts

### Chat Models (reasoning — Learn, Plan, Analyze phases)
- Any text-capable provider (OpenAI, Anthropic, Gemini, Groq, Mistral, etc.)
- Selected by LLMsVerifier using dynamic scoring (no hardcoded preferences)

### Dynamic Model Selection
Model selection is fully dynamic via LLMsVerifier's Strategy pattern:
- All configured providers are probed at session start
- Scored on quality, speed, cost, reliability dimensions
- Best available model selected per-phase requirements
- No hardcoded model preferences — scores determine selection

### Local Model Probing
- Ollama instances on configured hosts are auto-discovered
- Local models compete alongside cloud providers on scoring dimensions
- Local models get cost=1.0 (free), competing on quality/speed/reliability
- Distributed hosts (`HELIX_VISION_HOSTS`) are each probed independently

## Testing Strategy

- All packages use `testify` for assertions
- Detector tests use `CommandRunner` interface with mock implementations
- Orchestrator tests use mock runners and temporary bank files
- No external dependencies required for test execution


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


## API Keys & Secrets
- **NEVER commit `.env` files** — real API keys
- **NEVER add keys to source code** — use `.env` only
- `.env.example` (templates) OK to commit
- Before commit: `git ls-files --cached | grep ".env"` must show NO `.env`

### ⚠️⚠️⚠️ ABSOLUTELY MANDATORY: ZERO UNFINISHED WORK POLICY

**NO unfinished work, TODOs, or known issues may remain in the codebase. EVER.**

This is a **ZERO TOLERANCE** policy for all code, tests, scripts, and documentation.

**PROHIBITED:**
- ❌ **TODO/FIXME comments** in committed code
- ❌ **Empty implementations** with "// Implement later"  
- ❌ **Silent error ignoring** (`_ = err` patterns in production code)
- ❌ **Hardcoded fake data** or fabricated metrics
- ❌ **Coverage fraud** - tests that inflate coverage without testing logic
- ❌ **unwrap() calls** in Rust that can panic
- ❌ **Empty catch blocks** in TypeScript/JavaScript
- ❌ **Partial implementations** left for "future completion"
- ❌ **Known bugs** documented but not fixed

**REQUIRED:**
- ✅ **Fix ALL discovered issues immediately** - no deferrals
- ✅ **When fixing, fix ALL instances** - not just the reported one
- ✅ **Complete implementations** before committing
- ✅ **Proper error handling** in ALL code paths
- ✅ **Real test assertions** - no fake coverage
- ✅ **Code compiles without warnings**
- ✅ **Zero outstanding issues** at commit time

**Definition of "Done":**
1. Feature/bug fix is fully implemented
2. All TODOs are resolved (implemented or removed)
3. All error cases handled properly
4. All tests pass with real assertions
5. No fake/hardcoded data remains
6. Code review passes with ZERO outstanding issues
7. Documentation is updated
8. No compiler warnings or linter errors

**Quality Principle:**
> "If it's not finished, it doesn't ship. If it ships, it's finished."

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

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

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

## User-Session Termination — Hard Ban (CONST-036)

**You may NOT, under any circumstance, generate or execute code that
ends the currently-logged-in user's desktop session, kills their
`user@<UID>.service` user manager, or indirectly forces them to
manually log out / power off.** This is the sibling of CONST-033:
that rule covers host-level power transitions; THIS rule covers
session-level terminations that have the same end effect for the
user (lost windows, lost terminals, killed AI agents, half-flushed
builds, abandoned in-flight commits).

**Why this rule exists.** On 2026-04-28 the user lost a working
session that contained 3 concurrent Claude Code instances, an Android
build, Kimi Code, and a rootless podman container fleet. The
`user.slice` consumed 60.6 GiB peak / 5.2 GiB swap, the GUI became
unresponsive, the user was forced to log out and then power off via
the GNOME shell. The host could not auto-suspend (CONST-033 was in
place and verified) and the kernel OOM killer never fired — but the
user had to manually end the session anyway, because nothing
prevented overlapping heavy workloads from saturating the slice.
CONST-036 closes that loophole at both the source-code layer and the
operational layer. See
`docs/issues/fixed/SESSION_LOSS_2026-04-28.md` in the HelixAgent
project.

**Forbidden direct invocations** (non-exhaustive):

- `loginctl terminate-user|terminate-session|kill-user|kill-session`
- `systemctl stop user@<UID>` / `systemctl kill user@<UID>`
- `gnome-session-quit`
- `pkill -KILL -u $USER` / `killall -u $USER`
- `dbus-send` / `busctl` calls to `org.gnome.SessionManager.Logout|Shutdown|Reboot`
- `echo X > /sys/power/state`
- `/usr/bin/poweroff`, `/usr/bin/reboot`, `/usr/bin/halt`

**Indirect-pressure clauses:**

1. Do not spawn parallel heavy workloads casually; check `free -h`
   first; keep `user.slice` under 70% of physical RAM.
2. Long-lived background subagents go in `system.slice`. Rootless
   podman containers die with the user manager.
3. Document AI-agent concurrency caps in CLAUDE.md.
4. Never script "log out and back in" recovery flows.

**Defence:** every project ships
`scripts/host-power-management/check-no-session-termination-calls.sh`
(static scanner) and
`challenges/scripts/no_session_termination_calls_challenge.sh`
(challenge wrapper). Both MUST be wired into the project's CI /
`run_all_challenges.sh`.

<!-- END no-session-termination addendum (CONST-036) -->
