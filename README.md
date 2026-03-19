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
cmd/helixqa/          CLI entry point (subcommands: run, list, report, autonomous, version)
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
  autonomous/         SessionCoordinator, PlatformWorker, PhaseManager
  navigator/          NavigationEngine, ActionExecutor (ADB, Playwright, X11)
  issuedetector/      LLM-powered bug detection (visual, UX, accessibility, functional)
  session/            SessionRecorder, Timeline, VideoManager
```

See [ARCHITECTURE.md](ARCHITECTURE.md) and [API_REFERENCE.md](API_REFERENCE.md) for details.

## Autonomous QA Session

HelixQA includes an **Autonomous QA Session** mode that uses LLM-powered agents and computer vision to autonomously navigate applications, verify documented features, discover bugs, and generate comprehensive QA reports with video evidence.

### What It Does

The autonomous session runs in 4 phases:

1. **Setup** -- Select LLMs via LLMsVerifier, build a feature map from project docs via DocProcessor, spawn CLI agents via LLMOrchestrator, and initialize VisionEngine.
2. **Doc-Driven Verification** -- Platform workers verify every documented feature against the running app, capturing screenshots and video evidence at each step.
3. **Curiosity-Driven Exploration** -- Workers explore undiscovered areas of the app, testing edge cases, empty inputs, rapid interactions, and undocumented behaviors.
4. **Report & Cleanup** -- Aggregate coverage, tickets, and navigation maps into a QA report (Markdown, HTML, JSON) with linked video timestamps.

### New Packages

| Package | Purpose |
|---------|---------|
| `pkg/autonomous` | SessionCoordinator, PlatformWorker, PhaseManager |
| `pkg/navigator` | NavigationEngine with platform-specific ActionExecutors (ADB, Playwright, X11) |
| `pkg/issuedetector` | LLM-powered bug detection across visual, UX, accessibility, and functional categories |
| `pkg/session` | SessionRecorder with video management and timeline event tracking |

### External Modules

The autonomous session integrates 4 external Go modules (consumed as Git submodules):

| Module | Purpose |
|--------|---------|
| LLMsVerifier | Strategy-based LLM selection and scoring |
| LLMOrchestrator | Headless CLI agent management (opencode, claude-code, gemini, junie, qwen-code) |
| VisionEngine | GoCV mechanical vision + LLM Vision API analysis |
| DocProcessor | Documentation loading, feature map building, coverage tracking |

### CLI Subcommand

```bash
helixqa autonomous --project /path/to/Yole \
  --platforms android,desktop,web \
  --env .env \
  --timeout 2h \
  --coverage-target 0.9 \
  --output qa-results/ \
  --report markdown,html,json
```

### Configuration

All settings are managed via a `.env` file. Copy `.env.example` to `.env` and fill in your API keys and platform-specific paths. Key configuration groups:

- **Master switch**: Enable/disable, platform selection, timeout, coverage target
- **LLMsVerifier**: Strategy, score thresholds, caching
- **API keys**: OpenAI, Anthropic, Google, Groq, Mistral, DeepSeek, xAI, Together, Qwen, Junie
- **CLI agents**: Enabled agents, binary paths, pool size, retry config
- **Vision**: Provider selection, OpenCV toggle, SSIM threshold
- **Recording**: Video/screenshot capture, ffmpeg path, quality
- **Platforms**: Android device, web URL/browser, desktop process/display

### Quick Start

```bash
# 1. Copy and edit the configuration
cp .env.example .env
# Edit .env — set at least one API key and platform settings

# 2. Run an autonomous session against a project
helixqa autonomous --project /path/to/Yole \
  --platforms desktop \
  --env .env \
  --timeout 30m \
  --output qa-results/

# 3. View the results
cat qa-results/qa-report.md
ls qa-results/tickets/
ls qa-results/videos/
```

See [USER_GUIDE_AUTONOMOUS.md](USER_GUIDE_AUTONOMOUS.md) and [VIDEO_COURSE_AUTONOMOUS.md](VIDEO_COURSE_AUTONOMOUS.md) for detailed tutorials.

## Testing

```bash
make test       # Run all tests (235 tests)
make test-race  # With race detection
make test-cover # With coverage report
make vet        # Static analysis
```

## License

Apache-2.0
