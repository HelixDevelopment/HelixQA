# AGENTS.md - HelixQA Development Guide

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
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Modify commands to work within user permissions

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
