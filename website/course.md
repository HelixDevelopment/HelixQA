# Video Course

## Mastering HelixQA — Autonomous QA for Modern Software

**Duration:** 12 modules, approximately 4 hours total

**Audience:** QA engineers, developers, and DevOps teams who want to eliminate manual testing bottlenecks and achieve continuous, evidence-based quality assurance.

---

## Module 1: Introduction to Autonomous QA (15 min)

### 1.1 What is HelixQA?
- The problem: manual QA does not scale across multiple platforms and release cadences
- Fire-and-forget philosophy — point it at your project, walk away
- How HelixQA replaces repetitive human QA execution
- Live demo: running a first autonomous session from scratch

### 1.2 Architecture Overview
- The 4-phase pipeline: Learn → Plan → Execute → Analyze
- Diagram walkthrough: pipeline, providers, executors, memory, tickets
- Package structure overview (`pkg/llm/`, `pkg/memory/`, `pkg/navigator/`, etc.)

### 1.3 Supported Platforms
- Android, Android TV, Web, Desktop (X11), CLI, REST API
- Platform executor architecture — one executor per platform
- Decision guide: when to use which executor

---

## Module 2: Quick Start (20 min)

### 2.1 Installation
- Building from source: `go build -o helixqa ./cmd/helixqa`
- Container setup with Podman / Docker
- Prerequisites check: ADB, Playwright, ffmpeg, Go 1.24+

### 2.2 LLM Provider Setup
- Choosing a provider (OpenRouter recommended for beginners)
- Setting environment variables
- Testing provider connectivity with `helixqa version`
- Overview of 40+ supported providers

### 2.3 First Autonomous Run
- Live demo: `helixqa autonomous --project . --platforms web --timeout 5m`
- Reading the pipeline output in real time
- Understanding the `pipeline-report.json`
- Reviewing the generated issue tickets in `docs/issues/`

---

## Module 3: The Learning Engine (20 min)

### 3.1 Project Knowledge Ingestion
- How HelixQA reads `CLAUDE.md`, `docs/`, source code, and git history
- `ProjectReader`: documentation and constraint parsing
- `CodebaseMapper`: route and screen extraction from Go, React, Kotlin
- `GitAnalyzer`: recent change hotspots and commit frequency

### 3.2 KnowledgeBase Structure
- Screens, API endpoints, components, constraints
- Prior session awareness: findings, coverage, discovered screens
- Code walkthrough: `pkg/learning/knowledge.go`

### 3.3 Structuring Your Project for Optimal Learning
- Best practices for `CLAUDE.md` content
- Organising `docs/` for maximum ingestion value
- Adding project-specific test hints

---

## Module 4: The Planning Engine (20 min)

### 4.1 LLM-Driven Test Generation
- How the KnowledgeBase becomes an LLM prompt
- Test categories: `functional`, `security`, `edge_case`, `performance`, `accessibility`, `visual`
- Priority ranking: critical regressions first, coverage gaps second

### 4.2 Test Bank Reconciliation
- YAML test bank format
- How newly generated tests reconcile with existing banks
- Avoiding duplicate test coverage
- Code walkthrough: `pkg/planning/reconciler.go`

### 4.3 Multi-Pass Strategy
- How each pass generates different, complementary tests
- Coverage accumulation across sessions
- Pass number tracking in the memory database

---

## Module 5: Platform Executors (30 min)

### 5.1 Android / TV Executor (ADB)
- Connecting devices over Wi-Fi ADB
- Screenshot capture and video recording (scrcpy vs screenrecord)
- Crash and ANR detection via logcat
- SDK version considerations (Android 9 vs 15)

### 5.2 Web Executor (Playwright)
- Playwright setup and browser configuration
- Page navigation, form interaction, and assertions
- Screenshot and video capture
- Console error and failed request detection

### 5.3 Desktop Executor (X11)
- xdotool-based mouse and keyboard interaction
- ffmpeg x11grab for video recording
- ImageMagick for screenshot capture
- Display configuration for headless environments

### 5.4 CLI and API Executors
- `CLIExecutor`: stdin/stdout automation for terminal applications
- `APIExecutor`: REST API testing with schema validation
- When to use each executor type

---

## Module 6: Evidence Collection (20 min)

### 6.1 Screenshots
- Automatic capture at every test step
- Naming conventions: `test-NNN-screen-step-N.png`
- Before/after comparison for regression detection

### 6.2 Video Recording
- scrcpy for Android (all SDK versions, preferred)
- Playwright's built-in video for web
- ffmpeg x11grab for desktop
- Frame extraction for analysis: key frame selection strategies

### 6.3 Performance Metrics
- Memory monitoring: `adb shell dumpsys meminfo`
- CPU tracking: `adb shell dumpsys cpuinfo`
- Memory leak detection algorithm: monotonic heap growth analysis
- `MetricsTimeline` and `LeakIndicator` data structures

### 6.4 Log Collection
- Logcat capture and filtering for Android
- Browser console log collection for web
- Log pattern matching for crash signatures

---

## Module 7: LLM Vision Analysis (25 min)

### 7.1 How Vision Analysis Works
- Screenshot → LLM Vision API → structured findings
- Prompt engineering for UI defect detection
- Analysis categories: `visual`, `ux`, `accessibility`, `brand`, `content`, `performance`

### 7.2 Video Frame Analysis
- ffmpeg frame extraction pipeline
- Key frame selection: interval-based and motion-based
- Batch analysis with provider rate limiting
- Correlating video frames with test steps

### 7.3 Understanding Findings
- Severity levels: `critical` → `cosmetic`
- Category classification and ticket routing
- Managing false positives
- Code walkthrough: `pkg/analysis/vision.go`

---

## Module 8: Photographic Memory (20 min)

### 8.1 SQLite Memory Store
- 7-table database schema: sessions, test_results, findings, screenshots, metrics, knowledge, coverage
- What persists between sessions
- Querying the database directly with sqlite3

### 8.2 Issue Lifecycle
- Ticket creation in `docs/issues/HELIX-NNN.md`
- Status flow: `open → fixed → verified → reopened`
- Finding deduplication across sessions
- Automatic regression detection

### 8.3 Multi-Pass Intelligence
- How Pass N+1 uses Pass N results
- Coverage gap prioritisation
- Performance trend tracking across passes
- Code walkthrough: `pkg/memory/store.go`

---

## Module 9: Issue Ticket System (15 min)

### 9.1 Ticket Format
- YAML frontmatter: id, severity, category, platform, screen, status
- Markdown body: description, steps to reproduce, expected/actual behaviour
- Evidence references: screenshot and video paths

### 9.2 FindingsBridge
- Analysis → Memory → Markdown pipeline
- Automatic severity classification from LLM output
- Evidence path resolution

### 9.3 Integrating with External Issue Trackers
- Converting HELIX tickets to GitHub Issues, Jira, or Linear
- Parsing YAML frontmatter programmatically
- Webhook notifications on new ticket creation

---

## Module 10: Containerised Deployment (20 min)

### 10.1 Dockerfile
- Multi-stage build: Go builder + Ubuntu runtime
- Required system dependencies: ADB, Playwright, ffmpeg, xdotool
- Image optimisation and layer caching

### 10.2 Docker Compose / Podman Compose
- `docker-compose.qa-robot.yml` walkthrough
- Android USB device passthrough (`/dev/bus/usb`)
- Volume mounting for persistent results and memory
- Environment variable injection via `.env` file

### 10.3 Kubernetes Deployment
- Pod and Job configuration for one-shot QA passes
- CronJob for nightly scheduled passes
- PersistentVolumeClaims for memory database and results
- Resource limits and quotas

---

## Module 11: Advanced Configuration (20 min)

### 11.1 Curiosity-Driven Exploration
- How random navigation discovers unplanned screens
- Configuring `--curiosity-timeout`
- Screenshot capture during exploration
- How curiosity findings feed into the next pass

### 11.2 Custom Test Banks
- YAML test bank format in detail
- Writing domain-specific test cases
- Bank loading, filtering by tag, and platform targeting

### 11.3 LLM Provider Optimisation
- Choosing between cloud, hybrid, and self-hosted providers
- Cost optimisation: cheap models for planning, quality models for vision
- Model selection per request type with the AdaptiveProvider
- Fallback chain configuration

---

## Module 12: Real-World Case Study — Catalogizer (30 min)

### 12.1 The Target Application
- Catalogizer: multi-platform media management system
- 7 components: Go API, React web, Tauri desktop, Android, Android TV, installer wizard, API client
- 275+ REST API endpoints, 5 filesystem protocols, 40+ database migrations

### 12.2 Running the Full Suite
- HelixQA native test steps: 834 steps across all platforms
- Autonomous robot: 30 tests per pass, 4 passes
- Cumulative coverage progression across passes

### 12.3 Issue Discovery and Resolution
- 16 real issues found by LLM vision analysis
- Contrast ratio violations, accessibility gaps, UX inconsistencies
- Workflow: ticket created → developer fixes → next pass verifies

### 12.4 Results and Metrics
- 558/558 HelixQA native test steps passing
- 124/132 API tests passing
- 0 crashes, 0 ANRs across all devices and sessions
- 883+ Go tests in HelixQA itself, 21 packages, 0 race conditions

---

## Appendix A: All 40+ Supported LLM Providers

See [LLM Providers](/providers) for the complete reference including environment variables, default models, and base URLs.

## Appendix B: 22 Integrated Open-Source Tools

See [Open-Source Tools](/advanced/tools) for the full tool index with descriptions and usage examples.

## Appendix C: Self-Validation Challenges

See [Challenges](/advanced/challenges) for the 30 HQA challenges covering every major subsystem.
