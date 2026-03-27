# Documentation

Complete documentation for HelixQA, organized by audience and topic. Use this page as a table of contents to navigate to the guide, reference, or manual you need.

## Getting Started

New to HelixQA? Start here.

| Page | Description |
|------|-------------|
| [Introduction](/introduction) | What HelixQA is, the problem it solves, design philosophy |
| [Installation](/installation) | Build from source, container setup, LLM provider configuration |
| [Quick Start](/quick-start) | First autonomous session in 5 minutes |
| [Getting Started](/getting-started) | Platform-specific quick starts, first test bank, understanding output |

## Guides

In-depth guides for specific workflows and capabilities.

### Autonomous QA

| Page | Description |
|------|-------------|
| [Autonomous QA Guide](/guides/autonomous-qa) | Complete guide to autonomous sessions: configuration, multi-pass strategy, curiosity mode, timeout management |
| [Test Banks Guide](/guides/test-banks) | Writing, organizing, and managing YAML test banks for structured testing |

### Platform Guides

| Platform | Key Topics |
|----------|-----------|
| [Android / Android TV](/executors) | ADB connection, scrcpy video, crash detection via logcat, SDK version handling |
| [Web (Playwright)](/executors) | Browser configuration, console error capture, network request monitoring |
| [Desktop (X11)](/executors) | xdotool interaction, ffmpeg recording, Xvfb for headless environments |
| [REST API](/executors) | HTTP endpoint testing, schema validation, authentication flows |

### Advanced Topics

| Page | Description |
|------|-------------|
| [Challenges](/advanced/challenges) | 30 self-validation challenges covering every major subsystem |
| [Containers](/advanced/containers) | Containerized deployment with Podman, docker-compose.qa-robot.yml |
| [Open-Source Tools](/advanced/tools) | 22 integrated tools: ADB, Playwright, ffmpeg, scrcpy, and more |

## Reference

Precise specifications for CLI commands, configuration options, schemas, and APIs.

### CLI Reference

| Page | Description |
|------|-------------|
| [CLI Reference](/reference/cli) | All commands and flags: `autonomous`, `run`, `list`, `report`, `version` |
| [CLI Manual](/manual/cli) | Extended CLI usage guide with examples for each command |

### Configuration Reference

| Page | Description |
|------|-------------|
| [Configuration Reference](/reference/config) | All environment variables, config file options, and precedence rules |
| [Configuration Manual](/manual/config) | Configuration guide with examples for common setups |

### Schema Reference

| Page | Description |
|------|-------------|
| [Test Bank Schema](/reference/test-bank-schema) | YAML schema for test bank files: fields, types, validation rules |

### API Reference

| Endpoint Group | Description |
|---------------|-------------|
| Pipeline API | Session management: start, stop, status, results |
| Memory API | Query the SQLite memory store: sessions, findings, coverage |
| Test Bank API | List, load, and validate test bank files |
| Report API | Generate and retrieve session reports |

## User Manuals

Detailed operational manuals for day-to-day use.

| Page | Description |
|------|-------------|
| [CLI Manual](/manual/cli) | Full CLI reference with usage examples |
| [Configuration Manual](/manual/config) | Environment variables, config files, precedence |
| [Memory Manual](/manual/memory) | SQLite memory store: schema, querying, maintenance |
| [Ticket Manual](/manual/tickets) | Issue ticket format, YAML frontmatter, lifecycle management |
| [Multi-Pass Manual](/manual/multi-pass) | Running successive sessions for cumulative coverage |

## Architecture and Design

Understanding the internal architecture of HelixQA.

| Page | Description |
|------|-------------|
| [Architecture](/architecture) | System design: 24 packages, pipeline phases, data flow |
| [Pipeline Phases](/pipeline) | Detailed walkthrough of Learn, Plan, Execute, Analyze phases |
| [LLM Providers](/providers) | 40+ supported providers: configuration, model selection, failover |
| [Platform Executors](/executors) | Per-platform execution: Android, Web, Desktop, CLI, API |

### Developer Guides

For contributors and teams extending HelixQA.

#### Extending HelixQA

| Topic | Description |
|-------|-------------|
| Adding a new executor | Implement the `Executor` interface for a new platform |
| Adding a new LLM provider | Implement the `Provider` interface with model discovery |
| Adding a new detector | Implement the `Detector` interface for a new crash signal source |
| Writing custom analyzers | Add new analysis categories beyond the built-in six |

#### LLM Provider Integration

| Topic | Description |
|-------|-------------|
| Provider interface | The `Provider` interface: `Generate()`, `GenerateWithVision()`, `ListModels()` |
| Adaptive provider | How the adaptive provider selects models per request type |
| Failover chain | Automatic fallback when a provider returns an error or times out |
| Rate limiting | Per-provider rate limit enforcement and backoff strategy |
| Cost tracking | Token usage counting and cost estimation per provider |

#### Package Architecture

HelixQA is organized into 24 Go packages:

| Layer | Packages |
|-------|----------|
| CLI | `cmd/helixqa` |
| Pipeline | `pkg/pipeline`, `pkg/orchestrator` |
| Learning | `pkg/learning`, `pkg/knowledge` |
| Planning | `pkg/planning`, `pkg/testbank` |
| Execution | `pkg/executor`, `pkg/navigator`, `pkg/evidence` |
| Analysis | `pkg/analysis`, `pkg/vision` |
| Memory | `pkg/memory`, `pkg/findings` |
| LLM | `pkg/llm`, `pkg/provider`, `pkg/adaptive` |
| Detection | `pkg/detector`, `pkg/crash` |
| Reporting | `pkg/reporter`, `pkg/ticket` |
| Bridges | `pkg/bridge/scrcpy`, `pkg/bridge/appium`, `pkg/bridge/allure`, `pkg/bridge/perfetto` |
| Config | `pkg/config` |
| Validation | `pkg/validator` |

## Video Course

| Page | Description |
|------|-------------|
| [Video Course](/course) | 12-module course (4 hours): from first session to real-world case study |

## Additional Resources

| Page | Description |
|------|-------------|
| [Features](/features) | Complete feature overview with technical details |
| [FAQ](/faq) | Frequently asked questions and answers |
| [Changelog](/changelog) | Version history and release notes |
| [Download](/download) | Installation methods: source, container, binary |
| [Support](/support) | Troubleshooting, debug mode, reporting issues |
