# HelixQA Architecture

**Module:** `digital.vasic.helixqa`

HelixQA is a cross-platform QA orchestration framework built on top of
`digital.vasic.challenges` and `digital.vasic.containers`. It provides real-time
crash detection, step-by-step validation, evidence collection, and automated ticket
generation for Android, Web, and Desktop targets.

---

## Package Overview

| Package | Role |
|---------|------|
| `pkg/config` | Configuration types and validation |
| `pkg/testbank` | YAML test bank loading with platform/priority filtering |
| `pkg/detector` | Platform-specific crash/ANR detection |
| `pkg/validator` | Step-by-step validation with per-step evidence |
| `pkg/evidence` | Centralized screenshot, video, log, and trace collection |
| `pkg/ticket` | Markdown issue ticket generation for AI fix pipelines |
| `pkg/reporter` | Report generation (reuses `challenges/pkg/report`) |
| `pkg/orchestrator` | Main QA pipeline coordinator |
| `pkg/autonomous` | SessionCoordinator, PlatformWorker, PhaseManager |
| `pkg/navigator` | NavigationEngine with ADB, Playwright, X11 executors |
| `pkg/issuedetector` | LLM-powered visual/UX/accessibility/functional bug detection |
| `pkg/session` | SessionRecorder, Timeline, VideoManager |

---

## Orchestrator Flow

```mermaid
flowchart TD
    A[CLI: helixqa run] --> B[Load Config]
    B --> C[TestBank: load YAML banks]
    C --> D[Filter by platform / priority]
    D --> E[Orchestrator.Run]
    E --> F[Detector.Start]
    E --> G[For each test case]
    G --> H[Validator.ValidateStep]
    H --> I[Evidence.Collect]
    I --> J{Step passed?}
    J -- yes --> G
    J -- no --> K[Ticket.Generate]
    K --> L[Reporter.Write]
    G --> L
```

1. **Config** — `pkg/config` validates platform selection, speed mode (Slow/Normal/Fast),
   and report format from CLI flags and `.env`.
2. **TestBank** — `pkg/testbank` loads one or more YAML bank files, filters test cases
   by `--platform` and `--priority`, and returns an ordered execution list.
3. **Detector** — Starts in the background before test execution. Platform implementations:
   - `detector/android.go` — ADB `logcat` tail for FATAL/ANR lines + `pidof` liveness.
   - `detector/web.go` — `pgrep` on browser process name; JS console error injection.
   - `detector/desktop.go` — JVM/native `pgrep`; `kill -0` liveness probe.
   All detectors expose a `CommandRunner` interface so they are testable without
   a real device.
4. **Validator** — Executes each `Step` sequentially, calling `Evidence.Collect` before
   and after the action to produce a before/after screenshot pair.
5. **Ticket** — On step failure, `pkg/ticket` renders a Markdown issue ticket embedding
   the failing step, expected vs. actual output, stack trace, and evidence paths. Tickets
   are structured for consumption by AI fix pipelines.
6. **Reporter** — Delegates to `challenges/pkg/report` for Markdown/HTML/JSON output,
   augmenting the standard challenge report with QA-specific fields (crash log, video
   timestamp, ticket list).

---

## Detector Pipeline

```
Detector.Start()
  └─ goroutine: poll / tail platform log source
       └─ on match: CrashEvent{Platform, Message, Timestamp, StackTrace}
            └─ EventCh <-CrashEvent (buffered channel)

Orchestrator reads EventCh
  └─ attaches crash to current test case
  └─ marks step FAILED, triggers ticket generation
```

---

## Evidence Collection

`pkg/evidence.Collector` is the single write point for all artefacts:

| Evidence type | Source |
|---------------|--------|
| Screenshot | ADB `screencap`, Playwright `screenshot()`, X11 `scrot` |
| Video | ffmpeg screen capture started at session begin |
| Logcat / Console log | Buffered tail from detector |
| Stack trace | Extracted from crash event |

All artefacts are written under a per-session directory and referenced by relative
path in tickets and reports.

---

## Ticket Generation

`pkg/ticket.Generator` renders Markdown tickets with sections:

- **Summary** — one-line description from the failing step name.
- **Steps to Reproduce** — ordered list from the test case definition.
- **Expected / Actual** — from the step `expected` field vs. detected state.
- **Evidence** — embedded screenshot links and video timestamp.
- **Environment** — platform, device/browser, app version.

Tickets are self-contained so they can be filed directly into issue trackers or fed
to LLM-based auto-fix agents.

---

## Autonomous Session Architecture

The `autonomous` subcommand extends the standard pipeline with four phases:

1. **Setup** — LLMsVerifier selects LLMs; DocProcessor builds a FeatureMap;
   LLMOrchestrator spawns CLI agents; VisionEngine initialises screen analyser.
2. **Doc-Driven Verification** — `PlatformWorker` iterates every `Feature` in the
   FeatureMap, navigating to the relevant screen via `NavigationEngine` and capturing
   evidence at each step.
3. **Curiosity-Driven Exploration** — Workers explore screens not covered by docs,
   submitting edge-case inputs and rapid interactions; `IssueDetector` flags anomalies.
4. **Report & Cleanup** — `SessionCoordinator` aggregates coverage metrics, ticket
   list, and navigation map; `Reporter` writes the final QA report.

`PhaseManager` enforces phase transitions and propagates context cancellation so a
`--timeout` flag cleanly terminates all workers.
