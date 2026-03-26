# AGENTS.md - HelixQA Development Guide

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

## Testing Strategy

- All packages use `testify` for assertions
- Detector tests use `CommandRunner` interface with mock implementations
- Orchestrator tests use mock runners and temporary bank files
- No external dependencies required for test execution
