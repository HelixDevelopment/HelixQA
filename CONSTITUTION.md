# HelixCode Constitution

> **Inherits from:** `HelixConstitution/Constitution.md` — all universal
> clauses in that document apply unconditionally. Rules below extend or
> tighten them; they MUST NOT weaken any inherited clause.

## HelixCode Project Constitution

**Version**: 1.0.0
**Effective Date**: 2026-04-30
**Scope**: This Constitution applies to HelixCode and ALL its submodules
**Authority**: Cascaded from HelixAgent root governance with HelixCode-specific addenda

---

## Preamble

HelixCode is an enterprise-grade distributed AI development platform. This Constitution establishes the non-negotiable rules that govern all development, testing, deployment, and maintenance activities within the project. Every contributor, agent, and automated process MUST adhere to these rules. No exceptions.

---

## CONST-001: No CI/CD Pipelines (Permanent)

No `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline. No Git hooks. All builds and tests run manually or via Makefile/script targets.

**Rationale**: Manual execution ensures human oversight and prevents automated propagation of bluffs.

---

## CONST-002: No Mocks in Production (Permanent)

### CONST-002a: Production Code
Mocks, stubs, fakes, placeholder classes, TODO implementations are STRICTLY FORBIDDEN in production code. All production code is fully functional with real integrations.

### CONST-002b: Test Code
Mocks/stubs/fakes MAY be used ONLY in unit tests (files ending `_test.go` run under `go test -short`).

**Rationale**: Production bluffs have repeatedly been discovered where features appeared implemented but were non-functional.

---

## CONST-003: No HTTPS for Git (Permanent)

SSH URLs only (`git@github.com:…`, `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule updates. SSH keys are configured on every service.

---

## CONST-004: No Manual Container Commands (Permanent)

Container orchestration is owned by the project's binary/orchestrator (e.g., `make build` → `./bin/<app>`). Direct `docker`/`podman start|stop|rm` and `docker-compose up|down` are prohibited as workflows.

---

## CONST-005: 100% Real Data for Non-Unit Tests

Beyond unit tests, all components MUST use actual API calls, real databases, live services. No simulated success. Fallback chains tested with actual failures.

**Verification**: Every integration/E2E test MUST connect to real services or skip (not fail) if unavailable.

---

## CONST-006: Challenge Coverage (Permanent)

Every component MUST have Challenge scripts (`./challenges/scripts/`) validating real-life use cases. No false success — validate actual behavior, not return codes.

---

## CONST-007: Health & Observability

Every service MUST expose health endpoints. Circuit breakers for all external dependencies. Prometheus / OpenTelemetry integration where applicable.

---

## CONST-008: Documentation & Quality

Update `CLAUDE.md`, `AGENTS.md`, and relevant docs alongside code changes. Pass language-appropriate format/lint/security gates. Conventional Commits: `<type>(<scope>): <description>`.

---

## CONST-009: Validation Before Release

Pass the project's full validation suite (`make ci-validate-all`-equivalent) plus all challenges (`./challenges/scripts/run_all_challenges.sh`).

---

## CONST-010: Comprehensive Verification

Every fix MUST be verified from all angles: runtime testing (actual HTTP requests / real CLI invocations), compile verification, code structure checks, dependency existence checks, backward compatibility, and no false positives. Grep-only validation is NEVER sufficient.

---

## CONST-011: Resource Limits for Tests & Challenges

ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1` for `go test`. Container limits required.

---

## CONST-012: Bugfix Documentation

All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` with root cause analysis, affected files, fix description, and a link to the verification test/challenge.

---

## CONST-013: Real Infrastructure for All Non-Unit Tests

Mocks/fakes/stubs/placeholders MAY be used ONLY in unit tests. ALL other test types — integration, E2E, functional, security, stress, chaos, challenge, benchmark, runtime verification — MUST execute against REAL running systems with REAL containers, REAL databases, REAL services, and REAL HTTP calls.

---

## CONST-014: Reproduction-Before-Fix (Mandatory)

Every reported error, defect, or unexpected behavior MUST be reproduced by a Challenge script BEFORE any fix is attempted. Sequence:
1. Write the Challenge first
2. Run it; confirm fail (it reproduces the bug)
3. Then write the fix
4. Re-run; confirm pass
5. Commit Challenge + fix together

The Challenge becomes the regression guard for that bug forever.

---

## CONST-015: Concurrent-Safe Containers

Any struct field that is a mutable collection (map, slice) accessed concurrently MUST use thread-safe primitives. Bare `sync.Mutex + map/slice` combinations are prohibited for new code.

---

## CONST-016: Definition of Done (Universal)

A change is NOT done because code compiles and tests pass. "Done" requires pasted terminal output from a real run.

- **No self-certification**: Words like *verified, tested, working, complete, fixed, passing* are forbidden in commits/PRs/replies unless accompanied by pasted output from a command that ran in that session.
- **Demo before code**: Every task begins by writing the runnable acceptance demo
- **Real system, every time**: Demos run against real artifacts
- **Skips are loud**: `t.Skip` without a trailing `SKIP-OK: #<ticket>` comment breaks validation

---

## CONST-035 — Anti-Bluff Tests & Challenges (User-Mandate Forensic Anchor)

**§11.9 User-Mandate Forensic Anchor (2026-04-29)**

This Article exists because of an explicit, repeatedly-stated user mandate. The verbatim text:

> "We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

This anchor is the primary authority for the entire Article. The operative rule is:

**The bar for shipping is not "tests pass" but "users can use the feature."**

Every PASS in this codebase MUST carry positive evidence captured during execution that the feature works for the end user. Metadata-only PASS, configuration-only PASS, "absence-of-error" PASS, and grep-based PASS without runtime evidence are all critical defects regardless of how green the summary line looks.

Tests and Challenges (HelixQA) are bound equally — a Challenge that scores PASS on a non-functional feature is the same class of defect as a unit test that does. Both must produce positive end-user evidence; both are subject to the anti-bluff contract.

No false-success results are tolerable. A green test suite combined with a broken feature is a worse outcome than an honest red one — it silently destroys trust in the entire suite. Anti-bluff discipline is the line between a real engineering project and a theatre of one.

**Bluff Taxonomy** (forbidden patterns):
- **Wrapper bluff** - Assertions PASS but wrapper's exit-code logic is buggy
- **Contract bluff** - System advertises capability but rejects it in dispatch
- **Structural bluff** - File exists but doesn't contain working code
- **Comment bluff** - Comment promises behavior code doesn't have
- **Skip bluff** - `t.Skip("not running yet")` without `SKIP-OK` marker

**Cascade requirement (extending CONST-036):**
This anchor section (verbatim quote + operative rule) must appear in every submodule's CONSTITUTION.md / CLAUDE.md / AGENTS.md. Non-compliance is a release blocker regardless of context. Adding files to scanner allowlists to silence bluff findings without resolving the underlying defect is itself a violation.

---

## CONST-018: Host Power Management Hard Ban

**Host Power Management is Forbidden.**

You may NOT generate or execute code that sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt, reboot, or any other power-state transition.

Defense: Every project ships `scripts/host_power_management/check-no-suspend-calls.sh` and `challenges/scripts/no_suspend_calls_challenge.sh`.

---

## CONST-019: Container Up ≠ Healthy

Container `Up` status does NOT mean the application is healthy. Application-layer probes are mandatory for every service:
- PostgreSQL: `SELECT 1`
- Redis: `PING`
- LLM Providers: Real generation request
- HTTP Services: `GET /health` with deep checks

---

## CONST-020: Provider Fallback Chain Reality

Every LLM provider fallback chain MUST be tested with actual failures. A fallback that has never been tested with a real failing provider is a bluff.

---

## CONST-021: No Mocks Above Unit Build Target

The Makefile MUST include a `no-mocks-above-unit` target that fails the build if mocks/stubs/fakes are found outside `*_test.go` files.

---

## CONST-022: Submodule Governance Propagation

Every submodule MUST either:
1. Have its own Constitution.md, CLAUDE.md, and AGENTS.md, OR
2. Have a symlink to the parent repository's governance files, OR
3. Have a reference comment in its README pointing to parent governance

No submodule is exempt from these rules.

---

## CONST-023: Docker Health Checks Mandatory

Every Dockerfile MUST include:
```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1
```

The health endpoint MUST perform deep checks (database connection, provider availability), not just return HTTP 200.

---

## CONST-024: Version Pinning

All dependencies MUST be pinned to specific versions in `go.mod`. No `latest`, no floating tags. Renovate or Dependabot (manual review only — see CONST-001) may propose updates.

---

## CONST-025: Secret Management

NO secrets in code. EVER. Secrets via:
- Environment variables (production)
- `.env` files (development, in `.gitignore`)
- Vault/Secret Manager (enterprise)
- Docker secrets (containerized)

`go mod tidy` MUST NOT add secret-scanning bypasses.

---

## CONST-026: Minimal Privilege Containers

Containers run as non-root. Every Dockerfile:
```dockerfile
RUN adduser -D -u 1001 helixcode
USER helixcode
```

---

## CONST-027: Network Isolation

Container orchestration MUST use internal networks. Services communicate via named hosts, not exposed ports where possible.

---

## CONST-028: Backup Before Destructive Operations

Every file editing tool MUST create backups before modification. The backup MUST be restorable.

---

## CONST-029: Input Validation at All Boundaries

Every public function MUST validate inputs. No trust of caller-provided data. SQL injection, path traversal, command injection MUST be impossible by design.

---

## CONST-030: Graceful Degradation

When external services are unavailable, the system MUST degrade gracefully:
- Return partial results where possible
- Queue operations for retry
- Inform user of degraded state
- NEVER crash or hang indefinitely

---

## CONST-031: Audit Trail

Every significant operation MUST be logged with:
- Timestamp
- User identity
- Operation type
- Success/failure status
- Resource affected

Log retention: 90 days minimum.

---

## CONST-032: Emergency Stop

Every long-running or distributed operation MUST support cancellation via `context.Context`. Users MUST be able to interrupt any operation.

---

## CONST-033: Data Integrity

Database writes MUST be transactional. Partial writes MUST be rolled back. Consistency checks MUST run periodically.

---

## CONST-034: API Stability

Public APIs maintain backward compatibility within major versions. Deprecation requires:
- 6-month notice
- Migration guide
- Compatibility shim

---

## CONST-035: End-User Usability Mandate (2026-04-29 Strengthening)

A test or Challenge that PASSES is a CLAIM that the tested behavior **works for the end user of the product**.

The HelixAgent project has repeatedly hit the failure mode where every test ran green AND every Challenge reported PASS, yet most product features did not actually work. This MUST NOT recur in HelixCode.

Every PASS result MUST guarantee:
a. **Quality** - correct behavior under real inputs, edge cases, concurrency
b. **Completion** - wired end-to-end with no stub/placeholder gaps
c. **Full usability** - a user following documented request shapes SUCCEEDS

A passing test that doesn't certify all three is a **bluff** and MUST be tightened.

**Bluff taxonomy** (each pattern observed and now forbidden):
- **Wrapper bluff** - assertions PASS but wrapper's exit-code logic is buggy
- **Contract bluff** - system advertises capability but rejects it in dispatch
- **Structural bluff** - `check_file_exists` passes but doesn't run the test
- **Comment bluff** - comment promises behavior code doesn't actually have
- **Skip bluff** - `t.Skip("not running yet")` without `SKIP-OK: #<ticket>` marker

**Full background**: `docs/HOST_POWER_MANAGEMENT.md` and this Constitution (CONST-035).

---

## CONST-036: Propagation to Submodules

This Constitution, along with CLAUDE.md and AGENTS.md, MUST be propagated to ALL submodules. Each submodule's governance MUST reference this parent Constitution. Changes to this Constitution MUST trigger review of all submodule governance files.

---

## CONST-037: LLMsVerifier Single Source of Truth Mandate

**Rule**: LLMsVerifier SHALL BE the sole authoritative source for:
1. All model metadata (names, IDs, context windows, capabilities)
2. All provider metadata (endpoints, auth types, supported models)
3. All verification status (verified, partial, failed, pending)
4. All scoring data (overall scores, capability scores, tier rankings)
5. All rate-limit and cooldown state

**Prohibition**: NO hardcoded model lists, NO hardcoded provider lists, NO simulated model discovery. Any code path that presents a model or provider listing to a user MUST fetch that listing from the LLMsVerifier subsystem or its cached replica.

**Anti-Bluff Verification**:
- The challenge script `challenges/scripts/verifier_hardcode_check.sh` MUST scan all Go source files for hardcoded model arrays.
- Any `[]string{"gpt-4", "claude-3"}` or equivalent literal in production code is a constitutional violation.
- The only permitted hardcoded data is the LLMsVerifier service endpoint URL and the list of verification test types.

**Enforcement**: `make test-complete` MUST include a test that asserts `ModelManager.GetAvailableModels()` returns at least as many models as the verifier's database contains for configured providers. A test that passes while the CLI shows a hardcoded list is a TEST BLUFF and violates CONST-035.

---

## CONST-038: Model Provider Anti-Bluff Guarantee

**Rule**: Every model displayed to an end user MUST have been verified by LLMsVerifier within the last `verification_timeout` period (default: 24h). Models older than this MUST display a "stale" indicator and be deprioritized.

**Prohibition Against Test Bluffing**:
- A unit test that mocks the verifier client and asserts `GetAvailableModels()` returns 3 models DOES NOT satisfy this rule.
- An integration test that starts the verifier server, performs real provider discovery, and confirms the model count matches the actual provider API response DOES satisfy this rule.
- The Makefile target `make test-verifier-integration` MUST exist and MUST run without mocks.

**The "Tests Pass But Features Don't Work" Guarantee**:
```
NO TEST MAY PASS UNLESS THE FEATURE IT TESTS IS DEMONSTRABLY USABLE
BY AN END USER IN THE SAME BUILD.
```
- If `TestModelList` passes but `helixcode --list-models` shows hardcoded data, the test is a BLUFF.
- If `TestProviderHealth` passes but the health endpoint returns `200 OK` for a provider that is actually down, the test is a BLUFF.
- If `TestLLMGeneration` passes but `--prompt "hello"` returns a simulated string, the test is a BLUFF.
- Bluff tests MUST be rewritten or deleted. There is no "grandfather" exception.

**Evidence Standard**: Every test that claims to verify model/provider functionality MUST:
1. Call a real API endpoint or a real verifier database
2. Assert on response content that could only come from that real source
3. Include a test that runs the CLI binary with `--list-models` and checks output against verifier data

---

## CONST-039: Real-Time Model Status Accuracy

**Rule**: Model status (available, rate-limited, cooldown, offline, deprecated) displayed to users MUST reflect the actual state as known by LLMsVerifier within `max_staleness` seconds (default: 60s).

**Polling vs. Push**:
- If WebSocket/SSE push is unavailable, the system MUST poll LLMsVerifier at most every `status_poll_interval` (default: 30s).
- The TUI MUST display a "last updated" timestamp with every model listing.
- Models in "cooldown" or "rate-limited" state MUST show the estimated recovery time if known.

**Accuracy Verification**:
- Challenge script `challenges/scripts/model_status_accuracy_challenge.sh` MUST:
  1. Artificially rate-limit a provider by exhausting its quota
  2. Wait for the status to propagate to the verifier
  3. Check that `helixcode --list-models` shows the rate-limited status within 60s
  4. Check that `SelectOptimalModel()` no longer selects the rate-limited model

**Prohibition**: Status indicators that are "always green" or that lag >60s behind reality violate this rule.

---

## CONST-040: All Providers and Models Integration Mandate

**Rule**: HelixCode MUST integrate with ALL providers and models that LLMsVerifier supports, subject only to:
1. The provider being explicitly disabled in configuration (`enabled: false`)
2. The API key being absent and the provider requiring one
3. The provider being marked `deprecated` in the verifier database

**Minimum Provider Set** (SHALL NOT be reduced without constitutional amendment):
| Provider | Auth Type | Required Env Var |
|----------|-----------|-----------------|
| OpenAI | API Key | `OPENAI_API_KEY` |
| Anthropic | API Key / OAuth | `ANTHROPIC_API_KEY` |
| Gemini | API Key | `GEMINI_API_KEY` |
| DeepSeek | API Key | `DEEPSEEK_API_KEY` |
| Groq | API Key | `GROQ_API_KEY` |
| Mistral | API Key | `MISTRAL_API_KEY` |
| xAI | API Key | `XAI_API_KEY` |
| OpenRouter | API Key | `OPENROUTER_API_KEY` |
| Ollama | Local | None (auto-detect) |
| Llama.cpp | Local | None (auto-detect) |

**Integration Requirement**: For every provider in the minimum set:
- There MUST be a provider adapter file in `internal/llm/` or `internal/verifier/adapters/`
- There MUST be a `*_test.go` file with real API tests (skipped only if `HELIX_SKIP_LIVE_PROVIDER_TESTS` is set)
- There MUST be a challenge script in `challenges/scripts/`
- The model listing MUST include models from this provider when the provider is enabled

---

## CONST-041: MCP / LSP / ACP / Embedding / RAG / Skills / Plugins Integration Mandate

**Rule**: LLMsVerifier integration SHALL extend beyond basic model listing to cover ALL capability dimensions:

1. **MCP (Model Context Protocol)**: The verifier MUST report which models support MCP tool calling. HelixCode's MCP subsystem MUST consult verifier capability flags before selecting a model for tool-use tasks.

2. **LSP (Language Server Protocol)**: The verifier MUST report code-analysis capabilities. Models without `code_analysis` capability MUST NOT be selected for refactoring or debugging tasks.

3. **ACP (Agent Capability Protocol)**: The verifier MUST report multi-agent coordination support. Models with `supports_parallel_tool_use` MUST be preferred for ACP workflows.

4. **Embedding**: The verifier MUST report `supports_embeddings` for each model. The `CogneeConfig` embedding model selection MUST be verifier-aware.

5. **RAG (Retrieval-Augmented Generation)**: The verifier MUST report context-window sizes. RAG chunking strategies MUST adapt to the selected model's `context_window_tokens` as reported by the verifier.

6. **Skills / Plugins**: The verifier MUST track plugin compatibility. Models flagged `plugin_compatible` MUST be used when skill/plugin execution is required.

**Capability Checklist** (MUST be verified by challenge):
- [ ] MCP tool calling verified for at least 3 providers
- [ ] LSP code-analysis verified for at least 3 providers
- [ ] ACP parallel tool use verified for at least 2 providers
- [ ] Embedding generation verified for at least 2 providers
- [ ] RAG context-window adaptation verified
- [ ] Skills/plugin execution verified for at least 2 providers

**Prohibition**: Capability flags MUST NOT be hardcoded. The `Provider.GetCapabilities()` method MUST return data sourced from the verifier's `VerificationResult` fields.

---

## Article XII — Repository Safety

### §12.1 (CONST-042) — No-Secret-Leak

No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital, transitively or otherwise. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak — to git, logs, build artefacts, screenshots, or external services — is a release blocker until rotated and post-mortemed.

**Operational requirements:**
- Every repo must have `.env`, `.env.local`, `.env.*` (with `!.env.example` exception), `*.pem`, `*.key`, `*.crt`, `id_rsa*` in `.gitignore`.
- `scripts/scan-secrets.sh` (or equivalent) must run before every push; failing it blocks the push.
- API keys for development are sourced from the canonical `../HelixAgent/.env` (mode 0600, never under git) and copied — never symlinked, never committed — into per-repo `.env` files.

**Cascade requirement:** This article must appear verbatim in every owned-by-us repository's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Owned-by-us repos are listed in `scripts/owned-repos.txt` (or, until that file exists, the meta-repo `propagate-governance.sh` script's submodule walk excluding third-party trees).

### §12.2 (CONST-043) — No-Force-Push

No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval given for that specific operation. Authorization for one push does not extend to subsequent pushes. Bypassing hooks (`--no-verify`), signature verification (`--no-gpg-sign`), or protected-branch rules also requires explicit approval. This applies to every repository in the HelixDevelopment / vasic-digital stack.

**Operational requirements:**
- Local pre-push hook at `scripts/git_hooks/pre-push` (installed by `scripts/install-git-hooks.sh`) must reject `--force` / `--force-with-lease` unless `HELIX_FORCE_PUSH_APPROVED=1` is set.
- The hook is a courtesy gate; this constitutional clause is the actual contract.
- Regular non-force pushes of new commits to existing branches on already-configured remotes are PERMITTED without per-push approval, scoped to a programme/conversation in which the user has authorised the cadence.

**Cascade requirement:** Same as §12.1 — verbatim, every owned-by-us repo's three governance files.

---

## Article XIII — Programme Continuity

### §13.1 (CONST-044) — Continuation Document Maintenance Mandate

The `docs/CONTINUATION.md` document at the meta-repo (`HelixCode/`) MUST be maintained in sync with the actual state of the CLI-Agent Fusion programme at all times. It is the authoritative resumption record for any CLI agent or LLM picking up the work from any session, at any time.

**Mandate:** Every commit that advances programme state — feature task completion, feature close-out, push to remotes, known-issue discovery, deferred-item resolution, phase transition, addition or removal of submodules / remotes — MUST update `docs/CONTINUATION.md` to reflect the new state in the same commit (or in an immediately-following commit if the state-changing commit is small and topical).

**Definition of "out-of-sync":**
- `Last updated` SHA in CONTINUATION ≠ `git rev-parse HEAD` on `main`.
- `Active feature in flight` in CONTINUATION ≠ feature in `docs/improvements/PROGRESS.md` "Current focus".
- Tasks marked done in CONTINUATION ≠ ticked tasks in `PROGRESS.md`.
- Known-issue list missing a documented failure that exists in evidence files (`docs/improvements/0[5-9]_phase_*_evidence.md`).
- Repository state table missing or stale-SHA for any submodule listed in `.gitmodules`.

**Severity:** Out-of-sync `CONTINUATION.md` is a **CRITICAL DEFECT**. Same severity as a false-success test result under CONST-035 / Article XI §11.9. A green build with an out-of-sync continuation document is unshippable.

**Verification:** `scripts/verify_continuation_sync.sh` (TBD; planned for Phase 3) compares CONTINUATION fields against `PROGRESS.md`, `git rev-parse HEAD`, evidence files, and `.gitmodules`. Non-zero exit = sync violation → blocking pre-push.

**Cascade requirement:** This article must appear verbatim — or be referenced by `CONST-044` ID with a pointer back to this anchor — in every owned-by-us repository's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Submodules without their own `docs/CONTINUATION.md` reference the meta-repo's CONTINUATION as the authoritative document; submodules that maintain their own CONTINUATION must pin its SHA and update on every state-changing commit identically.

---

## Amendment Process

Constitution amendments require:
1. Written proposal with rationale
2. Challenge demonstrating the need
3. 72-hour review period
4. Approval by project architect
5. Update to all submodule governance files

---

*This Constitution is the supreme law of the HelixCode project. No code, test, or process may contradict it.*
<!-- BEGIN submodule-decoupling-and-reusability (parent-mirror) -->

### Submodule Decoupling & Reusability — Mandatory

**Status:** Mandatory. Non-negotiable.

**Rule:** This repository is a **shared submodule** consumed by
multiple independent consumer projects. Its value depends on staying
**fully decoupled and reusable**. No change in this repository may
introduce coupling that breaks its standalone reusability for any
consumer.

**Prohibited inside this repository:**

1. Hardcoding any specific consumer project's name, paths, platform
   list, version strings, release-naming conventions, branding, or
   feature names.
2. `import` / dependency on any consumer-project namespace, package,
   or build coordinate.
3. Embedding consumer-project-specific governance, rule numbering, or
   release cadence into this repository's `CONSTITUTION.md` /
   `CLAUDE.md` / `AGENTS.md`.
4. Assuming this repository is consumed by a particular CLI, build
   system, language toolchain version, or target architecture beyond
   what its public interface documents.

**Required inside this repository:**

1. All public surfaces (APIs, CLIs, configuration files, environment
   variables, scripts) MUST be expressed in terms of THIS repository's
   own domain — not any consumer's.
2. Governance MUST describe responsibilities and contract from THIS
   repository's perspective. Consumer projects appear as illustrative
   examples at most, never as load-bearing requirements.
3. Cross-project rules adopted from a consumer (such as a
   cross-platform impact mandate) MUST be phrased generically —
   "every consuming project's full platform matrix" — and never
   hardcode any single consumer's matrix.

**Why:** Repositories like this one have shipped changes in the past
where one consumer's platform list, feature names, or rule numbering
leaked into shared-repo governance — and then collided at merge time
with another consumer's parallel work, leaving the repository
unmergeable until manual conflict resolution stripped the
consumer-specific text back out. Decoupling is the only mechanism
that preserves this repository's value as shared infrastructure.

**Recursive scope:** any submodule this repository consumes inherits
the same decoupling+reusability rule. Third-party upstream submodules
that this repository merely vendors (e.g. open-source tools under a
`tools/opensource/` tree, if present) are explicitly out of scope —
we are not their owners.

<!-- END submodule-decoupling-and-reusability (parent-mirror) -->

<!-- BEGIN helix-constitution-inheritance + anti-bluff escalation -->

## Anti-Bluff End-User Quality Guarantee (Escalated via HelixConstitution)

**Canonical authority:** `HelixConstitution/Constitution.md` §7.1 and §11.4.
This section is this submodule's binding pointer to those universal clauses.

**Forensic anchor — verbatim operator mandate (2026-04-28):**

> "We had been in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does not work
> and can't be used! This MUST NOT be the case and execution of tests and
> Challenges MUST guarantee the quality, the completition and full usability
> by end users of the product! This MUST BE part of Constitution of our
> project, its CLAUDE.MD and AGENTS.MD if it is not there already, and to be
> applied to all Submodules's Constitution, CLAUDE.MD and AGENTS.MD as well
> (if not there already)!"

**Operative rule:** the bar for shipping is **not** "tests pass" but
**"users of any consuming project can use the feature."** Every PASS MUST
carry positive runtime evidence captured during execution that the feature
actually works end-to-end. Metadata-only PASS, configuration-only PASS,
"absence-of-error" PASS, and source-grep-only PASS without runtime evidence
are critical defects.

**Required minimum evidence per test category:**

| Category | Minimum evidence |
|---|---|
| Unit tests | Real inputs + mutation-verified assertions (stub-swap must cause FAIL) |
| Integration tests | Real subsystems only; mocks only at honest external boundaries |
| E2E / Challenge tests | Per-test PASS/FAIL lines + log-file artefact path; RUNTIME layer mandatory |

**Decoupling constraint:** this rule applies to every consuming project's
full platform matrix — specific platform names are not hardcoded here.

<!-- END helix-constitution-inheritance + anti-bluff escalation -->

## CONST-062 — CONST-078: Round-191 cascade — anchors §11.4.42 through §11.4.58

The following anchors propagate from constitution submodule §11.4.42-58 (User mandates 2026-05-17 / 2026-05-18 / 2026-05-19) into this submodule's governance via short-form reference per CONST-049 step 6. Severity-equivalent to a §11.4 PASS-bluff at the respective enforcement layer. Cascade requirement: this block (verbatim or by ID reference) MUST appear in this submodule's CONSTITUTION.md, CLAUDE.md, AGENTS.md, and propagate to any nested owned-by-us submodule.

- **CONST-062 / §11.4.42 — Iteration-discipline mandate.** Each fix cycle MUST run pre-build gate + post-build/post-flash test + paired §1.1 mutation + post-validation §11.4.40 retest. Truncating cycle to ship faster = violation. Subagents default to this path.
- **CONST-063 / §11.4.43 — TDD-Fix-Discipline.** Every bug fix starts with a failing RED test reproducing the defect on the target topology. Fix lands only after RED → GREEN against real infrastructure. Skipping RED step = violation.
- **CONST-064 / §11.4.44 — Document Revision Header.** Every governance/status/plan doc MUST carry `**Revision:** N` + `**Last modified:** YYYY-MM-DD` + `**Maintainer:**` directly below H1; bumped per content edit. Missing/stale header = violation.
- **CONST-065 / §11.4.45 — Integration-Status-Doc Maintenance.** Every `Status.md` (integration, programme, sub-system) carries the §11.4.44 header and stays in sync with actual programme state at every commit advancing state. Out-of-sync = violation (CONST-044 severity).
- **CONST-066 / §11.4.46 — Validate-recent-work before post-flash sweep.** Each recent-work item since previous sweep MUST have its §11.4.43 RED test run against the live device and report GREEN before post-flash full-test sweeps. Skipping = violation.
- **CONST-067 / §11.4.47 — Firebase Data Review.** Every Firebase Crashlytics/Analytics finding triaged per §11.4.47 severity table, deduped against Issues.md; new stacktrace gets §11.4.43 RED test before fix. Untriaged past SLA = violation.
- **CONST-068 / §11.4.48 — UI-Driven Video Testing.** Every supported app/service ships UI-driven traversal tests covering surfaces A-E; each run captures screen-recording as anti-bluff evidence. Missing driver or missing video = violation.
- **CONST-069 / §11.4.49 — Dual-Approach Testing.** Every feature test ships in BOTH UI-driven (uiautomator) AND Intent-driven (programmatic API) variants. Both RED until fix lands, then both GREEN. Single-variant = violation.
- **CONST-070 / §11.4.50 — Deterministic Consistency.** Every test runs N iterations with consistent results; flaky tests forbidden; intermittent FAIL must be diagnosed not retried. First-PASS-only reporting = violation.
- **CONST-071 / §11.4.51 — Live-ADB-First Maximization.** Prefer live-ADB probes over post-flash sweeps wherever the fix surface allows (§11.4.43 step 2 maximised). Skipping live-probe when feasible = violation.
- **CONST-072 / §11.4.52 — Autonomous-Validation.** Every test/feature/flow MUST have an autonomous execution path (instrumentation APK / headless driver) so subagent flows run unattended. Operator-attended-only = violation.
- **CONST-073 / §11.4.53 — Fixed_Summary parity.** `Fixed_Summary.md` (+ HTML/PDF exports per §11.4.12/§11.4.44) regenerated on every Issues.md → Fixed.md migration. Stale exports = violation. HTML+PDF travel together with the `.md`.
- **CONST-074 / §11.4.54 — ATM-NNN ticket identifier.** Every Issue/Feature/Task entry carries an `ATM-NNN` id (zero-padded, monotonically increasing per project); cross-references in governance/plans/changelogs use the ATM-NNN id. Missing/duplicate = violation.
- **CONST-075 / §11.4.55 — Reopens-history + per-item Reopens.md.** Every Reopened item gets `docs/reopens/ATM-NNN.md` tracking each cycle (date, source AI/User, reason from CONST-058 vocabulary, evidence path). `Reopens_Summary.md` regenerated on every reopen. Missing per-item doc = violation.
- **CONST-076 / §11.4.56 — Status_Summary parity + two-audience format.** `Status_Summary.md` (+ HTML/PDF exports) ships in operator-side + AI-side sections and stays in parity with underlying status docs at every commit advancing state. Drift/missing audience section = violation.
- **CONST-077 / §11.4.57 — README.md doc-link section + revision metadata.** Every `README.md` carries (a) §11.4.44 revision header below H1, (b) Documentation link section listing canonical governance + status + plan docs. Missing section or stale links = violation.
- **CONST-078 / §11.4.58 — Parallel-development methodology (PWU).** Project work proceeds through the Parallel Work Unit pipeline: Stage 1 DEVELOP (parallel, isolated worktrees) → Stage 2 MERGE (serial flock + §11.4.41 4-step) → Stage 3 REBUILD+FLASH → Stage 4 VALIDATE (parallel) → Stage 5 SWEEP. Four-layer lock hierarchy: L1 flock / L2 git / L3 contention-path advisory / L4 per-PWU worktree. Disjoint-scope PWUs run fully parallel; cross-scope overlap rejected to rebase. Conductor REFUSES merge of any PWU lacking captured evidence per §11.4.5/§11.4.52.

For full mandate text + verbatim user quotes + gates + paired mutations + composition tables, see constitution submodule `Constitution.md` §11.4.42-58 (canonical-root inheritance per CONST-059).

## Round-207 cascade — §11.4.59 + §11.4.60 (README always-sync + Documentation always-sync composite covenant)

> Verbatim user mandate (2026-05-19): *"fully review and update our main README document. ... Make sure main README is among documents we MUST ALWAYS keep updated and in Sync with the projects and other documentation! Make sure we always export it (on every update) into PDF and HTML."* AND (2026-05-19 ~09:00Z): *"Double check if all documents are properly tied with our root Constitution, CLAUDE.MD and AGENTS.MD so they are always up to date, always in sync and exported into PDF and HTML! ... Issues, Issues_Summary, Fixed, Fixed_Summary, Continuation, Status and Status_Summary for all contexts (areas) — THEY ALL MUST BE REGULARLY UPDATED, IN SYNC AND CONSISTENT without giving at any moment false picture about the state of the project or particular area(s) of it!"*

- **§11.4.59 — README always-sync mandate.** `README.md` at the project root is a §11.4.12-class always-sync document: kept current with every doc/integration/Status.md change, lockstep with `docs/CONTINUATION.md`, exported to `.html` + `.pdf` on every update via `scripts/testing/sync_readme_export.sh` (auto-invoked by `sync_issues_docs.sh`), carrying §11.4.44 revision header + Documentation Map section linking every Status / Status_Summary / spec / plan / guide / script-companion / changelog + the constitution submodule + per-audience navigation. Pre-build gate `CM-README-EXPORT-SYNC` enforces mtime parity (README.html + README.pdf ≥ README.md). Paired mutation backdates HTML+PDF → gate FAILs. No escape hatch — no `--skip-readme-sync`, `--no-readme-export`, `--readme-stale-OK` flag.
- **§11.4.60 — Documentation always-sync composite covenant.** Eight doc classes (Issues, Issues_Summary, Fixed, Fixed_Summary, CONTINUATION, README, every Status.md, every Status_Summary.md) MUST be in sync at all times across `.md` + `.html` + `.pdf` artefacts. Per-class anchors §11.4.12 / §11.4.44 / §11.4.45 / §11.4.53 / §11.4.56 / §11.4.57 / §11.4.59 / §12.10 govern individually; §11.4.60 binds them via single composite gate `CM-DOCS-COMPOSITE-SYNC` that FAILs the build if ANY instance's `.html` or `.pdf` mtime is older than `.md` mtime. Walks `docs/` recursively for Status fleet. Paired mutation backdates `docs/Issues.html` → gate FAILs. No escape hatch — no `--skip-composite-doc-sync`, `--allow-stale-html`, `--summary-not-applicable` flag exists.

**Cascade requirement:** These anchors (verbatim or by `§11.4.59` / `§11.4.60` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.59 + §11.4.60 for the full mandates.

## CONST-068: Shell-script target-shell-parseability mandate (cascaded from constitution submodule §11.4.67)

> Verbatim user mandate (2026-05-19): *"any issue we spot must be fixed, bash scripts as well if they are broken!"* + *"Make sure that this is mandatory rule!"*

> Verbatim 2026-05-19 operator mandate: *"all existing tests and Challenges do work in anti-bluff manner - they MUST confirm that all tested codebase really works as expected! We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completition and full usability by end users of the product!"*

Every committed shell script MUST be parseable by its target interpreter (`sh -n` for `/bin/sh`, `bash -n` for `/bin/bash`, etc.) AND MUST declare a shebang matching its actual syntax usage. Bash-only constructs (`>(...)`, `<(...)`, `[[ ]]`, `<<<`, arrays, `${var^^}`, etc.) used in scripts that may be invoked via `sh script.sh` MUST be wrapped in `eval` so the parser sees only a string (target shells like mksh parse the entire script before executing — runtime guards cannot save a parse-time rejection). Honest shebangs only: `#!/bin/bash` only if bash actually expected; `#!/bin/sh` requires POSIX-clean body. Fix at source per §11.4.1, never at callsites. Composes with §11.4.1 / §11.4.4 / §11.4.6 / §11.4.50 / §11.4.51. Pre-build gate `CM-SCRIPT-TARGET-SHELL-PARSEABLE` runs `sh -n` on every in-scope script. No escape hatch — no `--skip-parseability-check`, `--bash-only-script`, `--runtime-guard-suffices` flag.

**Cascade requirement:** This anchor (verbatim or by `CONST-068` ID reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. See constitution submodule `Constitution.md` §11.4.67 for the full mandate.

## §11.4.68 — Positive Sink-Side / Downstream Evidence Mandate (cascaded from constitution submodule §11.4.68)

> Verbatim user mandate (2026-05-20): *"We still do not hear any audio played from D3 device! Arvus Web Dashboard when we play music from D3 shows nothing for Codec In Use! This MUST BE investigated and fixed! How come we passed the tests with Arvus validation? What were values for the Codec In Use field? Empty means nothing! This is not working! It MUST BE FIXED, TESTED AND VERIFIED WITH FULL AUTOMATION TESTING ASAP!!!"*

A test that asserts audio or video routing PASS MUST capture and verify **positive sink-side or downstream evidence** — never config-only, never metadata-only, never PCM-open-state-only. At least one of the closed enumeration MUST be captured for every audio/video routing PASS: (1) sink-side codec-state with non-empty Codec-In-Use matching the expected codec regex; (2) strictly-positive PCM frames-written delta from `/proc/asound/.../status hw_ptr`; (3) ALSA ELD/EDID-Like-Data showing negotiated channel count + format; (4) ffprobe-on-captured-mp4 with non-zero frame count + expected codec/resolution/fps; (5) recording-analyzer event match per §11.4.2/§11.4.5; (6) tinycap RMS amplitude above the line-level floor. Empty / `<unreachable>` / `<N.E.>` / `<None>` placeholders are NOT positive evidence; a missing-but-required sink is `OPERATOR-BLOCKED` (release-blocker), never SKIP, never PASS. No escape hatch — no `--skip-sink-evidence`, `--allow-empty-codec`, `--sink-unreachable-is-pass`, `--metadata-only-suffices` flag exists.

**Cascade requirement:** This anchor (verbatim or by `§11.4.68` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a §11.4 PASS-bluff at the sink-side-evidence layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.68 for the full mandate.


## §11.4.70 — Subagent-Driven Execution Is The Default (cascaded from constitution submodule §11.4.70)

> Verbatim user mandate (2026-05-20): *"Always do if possible Subagent-driven! Add this into our root (constitution Submodule) Constitution.md, CLAUDE.md and AGENTS.md. This should be the default choice ALWAYS!"*

When executing implementation plans (or any task-decomposed execution flow), the **default execution model is subagent-driven** per `superpowers:subagent-driven-development`. Inline execution is permitted ONLY when (a) the task is trivial AND fits a single sub-300-line edit, OR (b) the operator explicitly requests inline at brainstorm-handoff time. Subagent-driven is the default because it gives isolated context per task, naturally enforces two-stage review, is parallel-PWU compatible (§11.4.58), creates an anti-bluff seam (§11.4), and survives operator absence. No escape hatch — `--inline-execution-required`, `--no-subagents`, `--monolithic-execution` are NOT permitted flags. Skipping subagent-driven for non-trivial work without recorded operator authorisation is itself a §11.4 PASS-bluff.

**Cascade requirement:** This anchor (verbatim or by `§11.4.70` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a §11.4 PASS-bluff at the execution-model layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.70 for the full mandate.


## §11.4.71 — Pre-Push Fetch + Investigate + Integrate Mandate (cascaded from constitution submodule §11.4.71)

> Verbatim user mandate (2026-05-20): *"before pushing changes to any upstream for any repository - main repo or Submodule, we MUST fetch and pull all changes. Once these are obtained WE MUST investigate what is different compared to head position we were on last time before fetching and pulling new changes! We MUST understand what is done and for what purpose, easpecially how that does affect our project and our System in general! Any mandatory changes or improvements required by fresh changes we just have brough in MUST BE incorporated, covered with all supported types of the tests which will produce as a result of its success execution REAL PROOFS of working for all componetns and functionalities covered and work fully in anti-bluff manner!"*

The everyday-push variant of §11.4.41. EVERY push (every repository — main + every submodule) MUST follow the 5-step cycle: (1) fetch all remotes (`git fetch --all --prune --tags`, capture stdout); (2) pull all upstream branches whose tip differs, resolving conflicts per consumer judgment (never auto-`--ours`/`--theirs`); (3) investigate the diff vs OUR previous HEAD — read EVERY foreign commit's body, understand what/why/how-it-affects-our-system; (4) integrate mandatory changes with §11.4.4(b) four-layer coverage + §11.4.43 TDD-fix discipline, every PASS carrying §11.4.5 captured-evidence (REAL PROOFS, not metadata-only); (5) only then push, verifying with `git ls-remote` post-push. No escape hatch — no `--skip-fetch`, `--no-investigate`, `--fast-push`, `--trust-upstream` flag.

**Cascade requirement:** This anchor (verbatim or by `§11.4.71` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a §11.4 PASS-bluff at the push-discipline layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.71 for the full mandate.


## §11.4.72 — Audio Top-Priority Mandate (cascaded from constitution submodule §11.4.72)

> Verbatim user mandate (2026-05-20): *"Make sure all fixes for audio are always top priority in main working stream!"*

The conductor (main working stream — Claude Code session, AI agent, or human operator) MUST treat audio fixes as the highest-priority class on the serial dispatch queue. Any time the conductor faces a choice between dispatching an audio task vs a non-audio task on the SAME serial resource, the audio task wins. Parallel BACKGROUND subagents (research, refactors, infrastructure documentation) MAY run concurrently with audio work but do NOT preempt audio on the main-stream serial dispatch queue. No escape hatch — there is no "but this non-audio task is faster" or "but this research is more interesting" override; audio-stack regressions are user-perceptible and high-impact while research and refactors can wait.

**Cascade requirement:** This anchor (verbatim or by `§11.4.72` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a process violation at the dispatch-priority layer.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.72 for the full mandate.


## §11.4.73 — Main-Specification Document Versioning + Revision Discipline (cascaded from constitution submodule §11.4.73)

> Verbatim user mandate (2026-05-20): *"Make sure everything we add now in previous and upcoming requests IS ALWAYS applied to the main specification — if we have one. Since all these are not major changes we could increase Specification version per change for secondary version instead of the primary. Primary version MUST BE increased for much bigger levels of changes! Add this into root (constitution Submodule) Constitution.md, CLAUDE.md and AGENTS.md as mandatory rule / constraint applicable ONLY IF we have something like the main specification document or we do recognize something like the main specification document. Document MUST BE updated ALWAYS to follow the versioning rules we are appling here + revision and other properties we have!"*

Applies **only when a project recognises a main specification document**. When it does: (1) every additive operator requirement, refinement, or accepted recommendation MUST be applied to the spec before or as part of the implementing work; (2) spec versioning has two axes — *primary* (V1/V2/V3, bumped for major rewrites by explicit operator decision, old versions archived) and *secondary* (the §11.4.61 metadata-table `Revision` integer, bumped for every other change); (3) the metadata table MUST stay current (`Revision`, `Last modified`, `Status summary`, `Fixed`); (4) propagated copies of the rule MUST reference the active `specification.V<primary>.md`, not a stale archive; (5) on primary bump the old file moves to `<spec-dir>/archive/` with `Status: superseded`. Classification: universal, applicable conditionally per the scope condition.

**Cascade requirement:** This anchor (verbatim or by `§11.4.73` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a release blocker when a project has a main spec and lets it drift.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.73 for the full mandate.


## §11.4.74 — Submodule-Catalogue-First Discovery + Extend-Don't-Reimplement (cascaded from constitution submodule §11.4.74)

> Verbatim user mandate (2026-05-20): *"We MUST ALWAYS check which already developed features / functionalities do exist as a part of our comprehensive Submodules catalogue located in vasic-digital and HelixDevelopment organizations on GitHub and GitLab both! Project MUST BE aware of all its existence so we do not implement same things multiple times if they are already done as some of existing universal, reusable general development purpose Submodules! For any missing features that some Submodules we incorporate may be missing we MUST IMPLEMENT the properly and extend those Submodules furter! We do control all of the and we CAN and MUST maintain and extend the regularly! All development cycle rules we have MUST BE applied to them and fully respected!"*

Before scaffolding ANY new module, package, helper, or utility, the contributor (human or AI agent) MUST: (1) survey the canonical Submodule catalogue — `vasic-digital` and `HelixDevelopment` on both GitHub AND GitLab; (2) inventory existing Submodules; (3) reuse before reimplement — if a Submodule provides the functionality (or 80%+ of it), add it as a Git submodule rather than write fresh; (4) extend in-place when 80%+ matches but features are missing — add the missing features TO THAT SUBMODULE (PR upstream + bump pointer), never as a duplicating consuming-project helper; (5) apply all development-cycle rules to those Submodules; (6) document the survey result in the feature's tracker entry with a `Catalogue-Check:` field (`reuse <org/repo>@<sha>` / `extend <org/repo>@<sha>` / `no-match <date>`). Classification: universal.

**Cascade requirement:** This anchor (verbatim or by `§11.4.74` reference) MUST appear in every owned submodule's `CONSTITUTION.md`, `CLAUDE.md`, and `AGENTS.md`. Severity-equivalent to a process violation; duplicate implementations landed without catalogue check are release blockers.
**Canonical authority:** constitution submodule `Constitution.md` §11.4.74 for the full mandate.
