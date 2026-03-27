# Architecture Reference

This page is the complete architecture reference for the HelixQA codebase. It documents every package, the data flow between them, and the extension points available for customisation.

## Module Identity

```
Module:     digital.vasic.helixqa
Language:   Go 1.24+
Depends on: digital.vasic.challenges, digital.vasic.containers
Entry:      cmd/helixqa/
```

HelixQA imports from `digital.vasic.challenges` and `digital.vasic.containers` -- it never reimplements their functionality. The challenges module provides the challenge execution framework. The containers module provides container discovery and service port detection.

---

## Package Structure Overview

HelixQA contains 24 packages across 5 architectural layers. Each package has a single responsibility and communicates with other packages through defined interfaces.

```
cmd/
  helixqa/                 CLI entry point (run, autonomous, list,
                           report, version subcommands)

pkg/
  # Core Engine
  orchestrator/            Main QA brain, ties everything together
  autonomous/              Autonomous pipeline (phases, coordinator,
                           executor factory, retry, findings bridge)
  maestro/                 Open-source tool runner (Maestro integration)

  # Detection and Validation
  detector/                Real-time crash/ANR detection
                           (Android, Web, Desktop)
  validator/               Step-by-step validation with evidence
  issuedetector/           Issue categorisation and LLM analysis
  visual/                  Visual comparator (SSIM-based duplicate
                           detection)

  # Navigation and Execution
  navigator/               Platform executors (ADB, Playwright,
                           X11, CLI, API)
  session/                 Session recording (recorder, timeline,
                           video)
  video/                   Video frame extraction (scrcpy, frames)
  evidence/                Centralised evidence collection
                           (screenshots, video, logs, audio)
  performance/             Performance metrics collection

  # Intelligence
  llm/                     LLM provider abstraction (40+ providers,
                           adaptive routing)
  learning/                Project knowledge ingestion (codebase,
                           git, docs, reader)
  planning/                LLM-driven test plan generation (planner,
                           ranker, reconciler)
  analysis/                Vision-based screenshot analysis

  # Data and Reporting
  memory/                  SQLite persistence (sessions, findings,
                           coverage, knowledge, cognitive)
  reporter/                QA report generation (markdown, HTML,
                           JSON)
  ticket/                  Issue ticket generation (markdown files
                           in docs/issues/)
  testbank/                YAML test bank loading, validation, and
                           generation

  # Infrastructure
  config/                  Configuration types, parsing, defaults
  types/                   Shared type definitions (issue types)
  bridges/                 External tool bridge registry (scrcpy,
                           appium, allure, perfetto, maestro,
                           ffmpeg, adb, xdotool)
```

---

## Layer Architecture

### Layer 1: Core Engine

The core engine orchestrates the overall QA session lifecycle.

#### `pkg/orchestrator/`

The main QA brain. It composes the detector, validator, reporter, and runner into a single execution loop. The orchestrator:

- Accepts a configuration and platform list
- Creates per-platform detectors and validators
- Runs test cases sequentially with crash detection between steps
- Collects results and delegates to the reporter

Key types:

| Type | Role |
|------|------|
| `Orchestrator` | Top-level coordinator |
| `OrchestratorConfig` | Configuration for timeouts, platforms, output |

#### `pkg/autonomous/`

The autonomous pipeline implements the 4-phase session (Learn, Plan, Execute, Analyse) plus curiosity exploration. This is the largest package, containing:

| File | Responsibility |
|------|---------------|
| `pipeline.go` | Phase sequencing and session lifecycle |
| `coordinator.go` | Coordinates platform executors and LLM providers |
| `phase.go` | Phase type definitions and transitions |
| `executor_factory.go` | Creates platform-specific executors from config |
| `real_executor.go` | Concrete executor that drives platform navigation |
| `findings_bridge.go` | Bridges analysis results to memory findings |
| `adapters.go` | Adapts between HelixQA types and Challenges types |
| `retry.go` | Exponential backoff retry for LLM calls |
| `fallback.go` | Provider fallback on failure |
| `sanitize.go` | Input/output sanitisation |
| `worker.go` | Background worker for async operations |
| `result.go` | Phase result types |

#### `pkg/maestro/`

Runs QA flows using the open-source Maestro tool. The `Runner` launches Maestro as a subprocess and parses its output.

---

### Layer 2: Detection and Validation

This layer monitors for crashes, ANRs, and visual issues during test execution.

#### `pkg/detector/`

Real-time crash and ANR detection. Supports three platform backends:

| File | Platform | Detection Method |
|------|----------|-----------------|
| `android.go` | Android (ADB) | `logcat` pattern matching: `FATAL EXCEPTION`, `ANR in` |
| `android_dual_display.go` | Android dual display | Multi-display crash detection |
| `web.go` | Web (browser) | Console error monitoring, process health |
| `desktop.go` | Desktop (X11) | Process exit monitoring, stderr scanning |
| `llm_analyzer.go` | All | LLM-based log analysis for subtle issues |

The `CommandRunner` interface abstracts command execution, making the detector fully testable without real devices.

#### `pkg/validator/`

Wraps the detector to provide step-by-step validation. For each test step, the validator:

1. Captures a pre-screenshot
2. Runs crash/ANR detection
3. Records the step result (passed, failed, skipped, error)
4. Captures a post-screenshot
5. Stores all evidence

Key types: `StepResult`, `StepStatus`, `Validator`.

#### `pkg/issuedetector/`

Analyses evidence to categorise detected issues. Uses predefined category definitions (`categories.go`) and LLM-powered analysis (`llm_analyzer.go`) with structured prompts (`prompts.go`).

#### `pkg/visual/`

SSIM-based visual comparator for detecting duplicate screenshots and measuring screen changes between test steps.

---

### Layer 3: Navigation and Execution

This layer drives platform-specific UI interactions and captures evidence.

#### `pkg/navigator/`

The navigation engine translates high-level LLM agent decisions into platform-specific UI actions.

| File | Role |
|------|------|
| `executor.go` | `ActionExecutor` interface definition |
| `engine.go` | Navigation engine coordinating executors |
| `llm_navigator.go` | LLM-driven navigation decisions |
| `state.go` | Screen state tracking |
| `playwright_executor.go` | Web platform (Playwright browser API) |
| `x11_executor.go` | Desktop platform (xdotool) |
| `api_executor.go` | REST API testing (HTTP client) |
| `cli_executor.go` | CLI process interaction (stdin/stdout) |

The `ActionExecutor` interface defines the contract all platform executors must implement:

```go
type ActionExecutor interface {
    Click(ctx context.Context, x, y int) error
    Type(ctx context.Context, text string) error
    Scroll(ctx context.Context, direction string, amount int) error
    LongPress(ctx context.Context, x, y int) error
    Swipe(ctx context.Context, fromX, fromY, toX, toY int) error
    KeyPress(ctx context.Context, key string) error
    Back(ctx context.Context) error
    Home(ctx context.Context) error
    Screenshot(ctx context.Context) ([]byte, error)
}
```

ADB (Android) executor lives in the `detector` package as `android.go` since crash detection and action execution share the ADB connection.

#### `pkg/session/`

Session lifecycle management:

| File | Role |
|------|------|
| `recorder.go` | Session event recording |
| `timeline.go` | Time-ordered event timeline |
| `video.go` | Video recording coordination |

#### `pkg/video/`

Low-level video frame handling:

| File | Role |
|------|------|
| `scrcpy.go` | scrcpy bridge for Android screen capture |
| `frames.go` | Frame extraction and assembly (screenshot-to-video) |

#### `pkg/evidence/`

Centralised evidence collection. The `Collector` gathers screenshots, video files, log extracts, and audio recordings into a structured directory:

| File | Role |
|------|------|
| `collector.go` | Evidence gathering and storage |
| `annotator.go` | Screenshot annotation (overlays, labels) |

#### `pkg/performance/`

Performance metrics collection during test execution:

| File | Role |
|------|------|
| `collector.go` | Metrics gathering (CPU, memory, response time) |
| `types.go` | Metric type definitions |
| `exec.go` | Command execution for metric collection |

---

### Layer 4: Intelligence

This layer uses LLM providers for test generation, vision analysis, and adaptive routing.

#### `pkg/llm/`

The LLM provider abstraction. All providers implement the `Provider` interface:

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message) (*Response, error)
    Vision(ctx context.Context, image []byte, prompt string) (*Response, error)
    Name() string
    SupportsVision() bool
}
```

| File | Role |
|------|------|
| `provider.go` | Interface definition, types, constants |
| `adaptive.go` | Adaptive provider (routes to best available) |
| `anthropic.go` | Anthropic Claude implementation |
| `openai.go` | OpenAI GPT implementation (also used by Tier 2 providers) |
| `ollama.go` | Ollama self-hosted implementation |
| `providers_registry.go` | Auto-discovery registry scanning env vars |

#### `pkg/learning/`

Project knowledge ingestion for the Learn phase:

| File | Role |
|------|------|
| `knowledge.go` | KnowledgeBase construction |
| `reader.go` | Documentation file reader (markdown, YAML) |
| `codebase.go` | Source code analysis (Gin routes, React routes) |
| `git.go` | Git history analysis (recent commits, hotspots) |

#### `pkg/planning/`

LLM-driven test plan generation for the Plan phase:

| File | Role |
|------|------|
| `planner.go` | Sends KnowledgeBase to LLM, parses test cases |
| `ranker.go` | Priority ranking (critical first, regressions boosted) |
| `reconciler.go` | Deduplicates against existing YAML test banks |
| `types.go` | Test plan type definitions |

#### `pkg/analysis/`

Vision-based screenshot analysis for the Analyse phase:

| File | Role |
|------|------|
| `vision.go` | LLM vision analysis (6 categories) |
| `types.go` | Analysis result types |

---

### Layer 5: Data and Reporting

#### `pkg/memory/`

SQLite persistence layer with 7 tables:

| File | Table(s) | Role |
|------|----------|------|
| `store.go` | (all) | Database init, connection, schema migration |
| `sessions.go` | `sessions` | Session lifecycle records |
| `findings.go` | `findings` | Issue findings with status transitions |
| `coverage.go` | `coverage` | Feature coverage tracking across sessions |
| `knowledge.go` | `knowledge` | Learned screens and navigation paths |
| `cognitive.go` | `cognitive_memory` | Cognitive memory layer (optional provider) |

#### `pkg/reporter/`

QA report generation:

| File | Role |
|------|------|
| `reporter.go` | Core report builder (markdown, JSON) |
| `enhanced.go` | Enhanced HTML report with embedded CSS |

Reuses `digital.vasic.challenges/pkg/report` for challenge result formatting.

#### `pkg/ticket/`

Issue ticket generation:

| File | Role |
|------|------|
| `ticket.go` | Base ticket generator (markdown files) |
| `enhanced_generator.go` | Enhanced tickets with YAML frontmatter |

#### `pkg/testbank/`

YAML test bank management:

| File | Role |
|------|------|
| `schema.go` | Test bank and test case type definitions |
| `loader.go` | YAML parsing and validation |
| `manager.go` | Bank loading, filtering, platform targeting |
| `generator.go` | Programmatic test case generation |

---

### Infrastructure

#### `pkg/config/`

Configuration types and parsing. Defines `Platform`, `Speed`, `ReportFormat`, and `AutonomousConfig`.

#### `pkg/types/`

Shared type definitions used across packages. Currently defines issue types.

#### `pkg/bridges/`

External tool bridge registry. Discovers which QA tools are installed on the host:

| Subdirectory | Tool |
|-------------|------|
| `scrcpy/` | scrcpy (Android screen mirroring) |
| `appium/` | Appium (mobile test automation) |
| `allure/` | Allure (test reporting) |
| `perfetto/` | Perfetto (performance tracing) |

The registry also checks for: maestro, ffmpeg, adb, npx, xdotool.

---

## Data Flow

The following diagram shows how data flows through the system during an autonomous session:

```
                        +------------------+
                        |   cmd/helixqa    |
                        | (CLI entry point)|
                        +--------+---------+
                                 |
                                 v
                        +------------------+
                        |  pkg/config      |
                        | (parse flags,    |
                        |  load .env)      |
                        +--------+---------+
                                 |
                                 v
                    +------------------------+
                    | pkg/autonomous/pipeline|
                    |   (phase sequencing)   |
                    +-----+----+----+---+----+
                          |    |    |   |
            +-------------+    |    |   +------------+
            v                  v    v                v
    +--------------+  +----------+ +---------+ +----------+
    | pkg/learning |  |pkg/plan- | |pkg/navi-| |pkg/anal- |
    | (Learn phase)|  |ning      | |gator    | |ysis      |
    |              |  |(Plan     | |(Execute | |(Analyse  |
    | codebase.go  |  | phase)   | | phase)  | | phase)   |
    | reader.go    |  |          | |         | |          |
    | git.go       |  |planner.go| |engine.go| |vision.go |
    +--------------+  +----------+ +---------+ +----------+
            |              |           |             |
            v              v           v             v
    +--------------+  +----------+ +---------+ +----------+
    | pkg/memory   |  | pkg/llm  | |pkg/     | |pkg/issue-|
    | (SQLite DB)  |  |(providers)| |detector | |detector  |
    |              |  |          | |+evidence| |+ticket   |
    | sessions     |  |adaptive  | |+video   | |+reporter |
    | findings     |  |anthropic | |+session | |          |
    | coverage     |  |openai    | |+perf    | |          |
    | knowledge    |  |ollama    | |+visual  | |          |
    +--------------+  +----------+ +---------+ +----------+
                                       |             |
                                       v             v
                                 +-----------+ +----------+
                                 |screenshots| |docs/     |
                                 |videos/    | |issues/   |
                                 |logs/      | |reports/  |
                                 +-----------+ +----------+
```

### Phase-by-Phase Data Flow

1. **Learn**: `learning/` reads CLAUDE.md, docs, source code, and git history. Produces a `KnowledgeBase` struct. Prior session data loaded from `memory/`.

2. **Plan**: `planning/planner` sends the `KnowledgeBase` to an LLM via `llm/`. The response is parsed into test cases. `planning/reconciler` deduplicates against `testbank/` banks. `planning/ranker` sorts by priority.

3. **Execute**: `autonomous/coordinator` creates executors via `autonomous/executor_factory`. Each test runs through `navigator/engine`, which calls the platform-specific `ActionExecutor`. `detector/` checks for crashes after each step. `validator/` records step results. `evidence/collector` gathers screenshots and video. `session/recorder` tracks the timeline.

4. **Curiosity**: The coordinator performs random navigation using the same executor infrastructure. New screens are recorded in `memory/knowledge`.

5. **Analyse**: `analysis/vision` sends screenshots to the LLM vision model. `issuedetector/` categorises findings. `ticket/` generates markdown issue files. `reporter/` produces session reports in all configured formats. `autonomous/findings_bridge` persists findings to `memory/`.

---

## Extension Points

HelixQA is designed for extensibility at several points:

| Extension Point | Interface/Package | What You Can Add |
|----------------|-------------------|-----------------|
| LLM providers | `llm.Provider` | New LLM backend (API client) |
| Platform executors | `navigator.ActionExecutor` | New platform (iOS, embedded, etc.) |
| Crash detectors | `detector.CommandRunner` | New detection backend |
| Report formats | `reporter/` | New output format |
| Tool bridges | `bridges/` | New external tool integration |
| Memory providers | `memory/cognitive.go` | Custom cognitive memory layer |
| Issue categories | `issuedetector/categories.go` | New issue classification rules |

See [Extending HelixQA](/developer/extending) for implementation details on each extension point.

## Related Pages

- [Architecture Overview](/architecture) -- high-level system overview with Mermaid diagram
- [Extending HelixQA](/developer/extending) -- how to add new detectors, executors, providers
- [LLM Providers](/developer/llm-providers) -- provider configuration and custom providers
- [Pipeline Phases](/pipeline) -- phase-by-phase walkthrough
