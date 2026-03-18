# CLAUDE.md - HelixQA Module

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
