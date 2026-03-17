# HelixQA

QA orchestration framework for cross-platform testing with real-time crash detection, step validation, and evidence-based reporting.

Built on [digital.vasic.challenges](../Challenges) and [digital.vasic.containers](../Containers).

## Features

- **Cross-platform testing**: Android, Web, and Desktop
- **Real-time crash detection**: ADB-based Android crash/ANR detection, browser and JVM process monitoring
- **Step-by-step validation**: Evidence collection at each test step to prevent false positives
- **Evidence-based reporting**: Screenshots, logs, and stack traces attached to failures
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
# Run all platforms against a test bank
helixqa --banks tests/banks/format-tests.json --platform all

# Android-specific with device
helixqa --banks tests/ --platform android \
  --device emulator-5554 \
  --package com.example.app

# Web platform with fast mode
helixqa --banks tests/web-bank.json --platform web --speed fast

# Desktop with JSON report
helixqa --banks tests/ --platform desktop --report json

# All options
helixqa \
  --banks tests/bank1.json,tests/bank2.json \
  --platform android,web,desktop \
  --device emulator-5554 \
  --package com.example.app \
  --output qa-results \
  --speed normal \
  --report markdown \
  --validate \
  --record \
  --verbose \
  --timeout 30m
```

## Architecture

```
cmd/helixqa/          CLI entry point
pkg/
  config/             Configuration types and validation
  detector/           Platform-specific crash/ANR detection
    android.go        ADB-based detection (pidof, logcat, screencap)
    web.go            Browser process monitoring (pgrep)
    desktop.go        JVM/process monitoring (pgrep, kill)
  validator/          Step-by-step validation with evidence
  reporter/           QA report generation (reuses challenges/pkg/report)
  orchestrator/       Main QA pipeline coordinator
```

## Testing

```bash
make test       # Run all tests
make test-race  # With race detection
make test-cover # With coverage report
make vet        # Static analysis
```

## License

Apache-2.0
