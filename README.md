# HelixQA

QA orchestration framework for cross-platform testing with real-time crash detection, step validation, evidence collection, and automated ticket generation.

Built on [digital.vasic.challenges](../Challenges) and [digital.vasic.containers](../Containers).

## Features

- **Cross-platform testing**: Android, Web, and Desktop
- **Real-time crash detection**: ADB-based Android crash/ANR detection, browser and JVM process monitoring
- **Step-by-step validation**: Evidence collection at each test step to prevent false positives
- **YAML test banks**: QA-specific test case definitions with platform targeting, priority, and documentation references
- **Evidence collection**: Screenshots, logcat, video recording, stack traces — all centralized
- **Markdown ticket generation**: Auto-generated issue tickets with full evidence for AI fix pipelines
- **Multiple report formats**: Markdown, HTML, JSON
- **Speed modes**: Slow (debugging), Normal, Fast (CI)
- **Composable architecture**: Reuses Challenges framework for test execution and reporting

## Prerequisites

- Go 1.24+
- Sibling directories:
  - `../Challenges` (digital.vasic.challenges)
  - `../Containers` (digital.vasic.containers)

## Installation

```bash
go install digital.vasic.helixqa/cmd/helixqa@latest
```

Or build from source:

```bash
make build
# Binary at bin/helixqa
```

## Usage

```bash
# Run QA pipeline
helixqa run --banks tests/banks/ --platform all

# Android-specific with device
helixqa run --banks tests/ --platform android \
  --device emulator-5554 \
  --package com.example.app

# List test cases from banks
helixqa list --banks tests/banks/ --platform android

# Generate report from existing results
helixqa report --input qa-results --format html

# Version info
helixqa version
```

## Test Bank Format (YAML)

```yaml
version: "1.0"
name: "Yole Core Tests"
test_cases:
  - id: TC-001
    name: "Create new document"
    category: functional
    priority: critical
    platforms: [android, web, desktop]
    steps:
      - name: "Open app"
        action: "Launch application"
        expected: "Main editor screen visible"
    tags: [core, smoke]
    documentation_refs:
      - type: user_guide
        section: "3.1"
        path: "docs/USER_MANUAL.md"
```

## Architecture

```
cmd/helixqa/          CLI entry point (subcommands: run, list, report, version)
pkg/
  config/             Configuration types and validation
  testbank/           YAML test bank management with platform/priority filtering
  detector/           Platform-specific crash/ANR detection
    android.go        ADB-based detection (pidof, logcat, screencap)
    web.go            Browser process monitoring (pgrep)
    desktop.go        JVM/process monitoring (pgrep, kill)
  validator/          Step-by-step validation with evidence
  evidence/           Centralized evidence collection (screenshots, video, logs)
  ticket/             Markdown ticket generation for AI fix pipelines
  reporter/           QA report generation (reuses challenges/pkg/report)
  orchestrator/       Main QA pipeline coordinator
```

See [ARCHITECTURE.md](ARCHITECTURE.md) and [API_REFERENCE.md](API_REFERENCE.md) for details.

## Testing

```bash
make test       # Run all tests (235 tests)
make test-race  # With race detection
make test-cover # With coverage report
make vet        # Static analysis
```

## License

Apache-2.0
