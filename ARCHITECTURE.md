# HelixQA Architecture

## Overview

HelixQA is a QA orchestration engine built on Go 1.24+ that drives cross-platform testing with real-time crash/ANR detection, step validation, and evidence-based reporting. It composes existing vasic-digital modules (Challenges, Containers) rather than reimplementing their functionality.

## Module Dependency Graph

```
HelixQA (Orchestration Layer)
├── pkg/orchestrator  ── Main QA brain
├── pkg/testbank      ── YAML test bank management
├── pkg/detector      ── Real-time crash/ANR detection
├── pkg/validator     ── Step-by-step validation
├── pkg/evidence      ── Evidence collection (screenshots, logs, video)
├── pkg/ticket        ── Markdown ticket generation
├── pkg/reporter      ── QA report generation (MD, HTML, JSON)
├── pkg/config        ── Configuration types
└── cmd/helixqa       ── CLI entry point
    ↓ imports ↓
Challenges (Test Execution)
├── pkg/runner        ── Challenge execution engine
├── pkg/bank          ── Challenge bank loading (JSON)
├── pkg/challenge     ── Core types (Challenge, Definition, Result)
├── pkg/report        ── Report generation (MD, HTML, JSON)
├── pkg/logging       ── Structured logging
└── pkg/userflow      ── Multi-platform automation adapters
    ↓ imports ↓
Containers (Infrastructure)
├── pkg/compose       ── Container orchestration
├── pkg/runtime       ── Container runtime
└── pkg/lifecycle     ── Container lifecycle
```

## Data Flow

```
                    ┌──────────────────┐
                    │   CLI / Caller   │
                    └────────┬─────────┘
                             │ config + bank paths
                    ┌────────▼─────────┐
                    │   Orchestrator   │ ← Main brain
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼────┐  ┌─────▼──────┐ ┌────▼──────┐
     │  TestBank   │  │  Detector  │ │ Validator  │
     │  Manager    │  │ (per plat) │ │ (per step) │
     └────────┬────┘  └─────┬──────┘ └────┬──────┘
              │              │              │
              │         ┌────▼──────┐      │
              │         │ Evidence  │◄─────┘
              │         │ Collector │
              │         └────┬──────┘
              │              │
     ┌────────▼──────────────▼──────┐
     │         Reporter             │
     │  (Markdown / HTML / JSON)    │
     └──────────────┬───────────────┘
                    │
              ┌─────▼──────┐
              │   Ticket   │
              │  Generator │
              └────────────┘
```

## Package Responsibilities

### pkg/orchestrator
The central coordinator. Loads test banks, iterates over platforms, runs challenges via the Challenges runner, invokes validation between steps, and produces the final report. Supports functional options for dependency injection.

### pkg/testbank
Manages QA-specific YAML test banks. Extends the Challenges JSON bank format with: platform targeting, priority levels (critical/high/medium/low), documentation references for consistency checking, and step definitions. Converts to `challenge.Definition` for execution.

### pkg/detector
Real-time crash and ANR detection per platform:
- **Android**: ADB logcat parsing, pidof process checks, screencap
- **Web**: Browser process monitoring, console error collection
- **Desktop**: Process alive checks, stderr monitoring

Uses the `CommandRunner` interface for testability.

### pkg/validator
Wraps the detector to perform pre/post-step validation. Takes screenshots before and after each step, runs crash detection, and produces `StepResult` with evidence. Prevents false positives by correlating detection with step state.

### pkg/evidence
Centralized evidence collection: screenshots (ADB screencap, Playwright, X11 import), logcat capture, video recording lifecycle, and console logs. All items are tracked with metadata (type, platform, timestamp, file size).

### pkg/ticket
Generates detailed Markdown issue tickets from failed steps or raw detections. Each ticket includes: severity, platform, reproduction steps, expected/actual behavior, stack traces, logs, and screenshot evidence. Designed to feed into AI fix pipelines.

### pkg/reporter
Produces QA reports in Markdown, HTML, or JSON. Reuses `digital.vasic.challenges/pkg/report` for individual challenge formatting. Adds QA-specific sections: platform breakdown, crash/ANR counts, step validation tables, and evidence references.

### pkg/config
Configuration types: platform selection, speed modes (slow/normal/fast), report formats, device targeting, and validation toggles. Supports YAML/JSON serialization.

## Design Decisions

1. **Composition over reimplementation**: HelixQA imports Challenges types directly. No wrapper types around `challenge.Definition`, `bank.Bank`, or `report.Reporter`.

2. **Functional options pattern**: All constructors use `WithX()` options for clean dependency injection and testing.

3. **CommandRunner interface**: Abstracts command execution (`adb`, `npx`, etc.) behind an interface, enabling full test coverage without real devices.

4. **Platform-agnostic orchestration**: The orchestrator runs the same pipeline for all platforms. Platform-specific behavior is encapsulated in detector and evidence packages.

5. **Evidence-first reporting**: Every failure includes evidence (screenshots, logs, traces). Tickets are self-contained for AI pipeline consumption.

## Test Coverage

| Package | Tests | Focus |
|---------|-------|-------|
| config | Unit + edge | Validation, parsing, defaults |
| detector | Unit + platform | Android/Web/Desktop detection |
| validator | Unit + concurrent | Step validation, evidence |
| reporter | Unit + format | Markdown, HTML, JSON output |
| orchestrator | Unit + edge + integration + stress | Full pipeline, cancellation |
| testbank | Unit + stress + benchmark | YAML loading, filtering |
| ticket | Unit + stress + benchmark | Markdown generation |
| evidence | Unit + stress + benchmark | Concurrent capture |

Total: **235 tests**, all passing with `-race` flag.

## CLI

```
helixqa run      --banks <paths> [--platform all] [--speed fast]
helixqa list     --banks <paths> [--platform android] [--json]
helixqa report   --input <dir>   [--format html]
helixqa version
```
