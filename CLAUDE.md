# CLAUDE.md - HelixQA Module

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

## MANDATORY: No Hardcoded QA Flows

**ALL QA testing MUST be driven by LLM vision — NEVER by hardcoded scripts. This is NON-NEGOTIABLE.**

- **NEVER** write fixed tap coordinates, sleep timers, or keystroke sequences. These break on different devices and produce false positives
- The `helixqa autonomous` command handles everything: device detection, screenshot→LLM→action loop, validation, reporting
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
| `cmd/helixqa` | CLI entry point (subcommands: run, list, report, version) |

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
