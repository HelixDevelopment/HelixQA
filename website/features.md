# Features

HelixQA is a comprehensive autonomous QA framework with capabilities spanning test generation, multi-platform execution, evidence collection, and AI-powered analysis. This page details every major feature.

## Fire-and-Forget Autonomous QA

The core design principle of HelixQA is zero human intervention during execution. A single command launches a complete QA session:

```bash
helixqa autonomous --project /path/to/app --platforms all --timeout 30m
```

Once launched, HelixQA operates through a 4-phase pipeline without requiring any interaction:

1. **Learn** -- Reads your project documentation, source code, git history, and prior QA sessions to build a comprehensive knowledge base.
2. **Plan** -- An LLM generates prioritized test cases based on the knowledge base, reconciling against existing test banks to avoid duplication.
3. **Execute** -- Tests run on all target platforms with video recording, screenshot capture, crash detection, and performance monitoring.
4. **Analyze** -- LLM vision examines every screenshot for visual defects, UX issues, accessibility problems, and brand compliance. Findings are filed as structured issue tickets.

The entire pipeline is orchestrated by the `pkg/pipeline` package, which manages phase transitions, timeout enforcement, and graceful shutdown. If the session timeout is reached mid-execution, HelixQA completes the current test, runs the analysis phase on all collected evidence, and writes the session report.

### LLM-Driven Test Generation

Rather than requiring hand-written test scripts, HelixQA uses large language models to generate test cases from your actual codebase. The planner constructs a detailed prompt from the knowledge base -- including discovered screens, API endpoints, UI components, architecture constraints, and prior findings -- and asks the LLM to produce test cases across six categories: functional, security, edge case, performance, accessibility, and visual.

Each generated test case includes:
- A human-readable title and description
- Target platform and screen
- Step-by-step execution instructions
- Expected outcomes for validation
- Priority ranking based on risk and coverage gaps

### Multi-Pass Intelligence

A single QA pass is a starting point, not the destination. Each successive pass builds on previous sessions through the persistent memory store:

- **Pass 1**: Broad coverage of major screens and critical paths
- **Pass 2**: Deeper testing of areas where issues were found, plus coverage gaps
- **Pass 3**: Edge cases, error states, and exploratory paths
- **Pass N**: Regression verification of fixed issues, new feature coverage

Coverage accumulates across passes toward a configurable target (default 80%). The planner automatically prioritizes under-tested areas and recently changed code paths.

## AI Vision-Based Screenshot Analysis

Every screenshot captured during test execution is sent to a multimodal LLM for structured analysis. The vision model examines each screenshot across six categories:

| Category | What Is Checked |
|----------|----------------|
| `visual` | Misaligned elements, clipped text, wrong colors, broken layouts, overlapping widgets |
| `ux` | Unresponsive buttons, confusing navigation, missing loading indicators, dead-end flows |
| `accessibility` | Color contrast ratios (WCAG 2.1 AA), touch target sizes, missing labels, focus indicators |
| `brand` | Logo placement and rendering, color scheme compliance, typography consistency |
| `content` | Empty screens that should have data, placeholder text in production, truncated strings |
| `performance` | Visible jank indicators, stuck loading spinners, frozen frames, layout thrashing |

The vision analysis prompt is carefully engineered to produce structured JSON output with severity levels, category classifications, and actionable descriptions. Each finding is correlated with the specific test step and screen where it was observed.

### Video Frame Analysis

For video recordings, HelixQA extracts key frames using ffmpeg and analyzes them in batch. Two frame selection strategies are available:

- **Interval-based**: Extract a frame every N seconds (default: 2 seconds)
- **Motion-based**: Extract frames where significant visual change occurs between consecutive frames

Batch analysis respects provider rate limits and uses the adaptive provider to select the most cost-effective vision-capable model.

### False Positive Management

LLM vision analysis can produce false positives. HelixQA addresses this through:

- **Finding deduplication**: Identical findings across sessions are merged rather than duplicated
- **Confidence scoring**: Each finding includes a confidence level from the LLM
- **Status lifecycle**: Tickets can be marked as `wontfix` or `false_positive`, and future passes will not re-report them
- **Contextual grounding**: Findings reference the specific screenshot and test step, allowing quick human verification

## Performance Profiling via Perfetto Bridge

HelixQA integrates with Perfetto for deep performance profiling on Android devices. The Perfetto bridge (`pkg/bridge/perfetto`) manages trace collection during test execution:

```bash
helixqa autonomous \
  --project . \
  --platforms android \
  --perfetto=true \
  --perfetto-categories "sched,freq,idle,am,wm,gfx,view,input" \
  --timeout 15m
```

### What Perfetto Captures

| Category | Metrics |
|----------|---------|
| CPU scheduling | Per-core frequency, idle states, task migrations |
| Memory | RSS, PSS, heap allocations, page faults |
| Graphics | Frame render times, jank detection, GPU utilization |
| Activity Manager | Activity lifecycle events, broadcast delivery |
| Window Manager | Window transitions, animation timing |
| Input | Touch event latency, input dispatch timing |

### Trace Analysis

Perfetto traces are saved as `.perfetto-trace` files in the session output directory. HelixQA parses the trace data to extract:

- **Frame timing**: Identifies frames exceeding 16.67ms (60fps target) or 33.33ms (30fps target)
- **CPU hotspots**: Threads consuming disproportionate CPU time during test execution
- **Memory trends**: Allocation patterns that indicate leaks or excessive garbage collection
- **Input latency**: Time from touch event to UI response, flagging latencies above 100ms

These metrics are included in the session report and can trigger issue tickets when thresholds are exceeded.

## Curiosity Mode for Exploratory Testing

Beyond planned test cases, HelixQA includes a curiosity-driven exploration phase that discovers screens and states not covered by the test plan:

```bash
helixqa autonomous \
  --project . \
  --platforms android \
  --curiosity=true \
  --curiosity-timeout 5m
```

### How Curiosity Works

1. After all planned tests complete, the curiosity navigator takes control
2. It identifies interactive elements on the current screen (buttons, list items, navigation icons, menu entries)
3. It randomly selects and taps an element, then waits for the screen to settle
4. A screenshot is captured and compared against known screens in the memory store
5. If a new screen is discovered, it is added to the knowledge base for future passes
6. The navigator continues exploring until the curiosity timeout expires

### Discovery Tracking

Every screen discovered through curiosity exploration is persisted in the memory database with:

- A screenshot reference
- The navigation path that reached the screen (sequence of taps)
- A hash of the screen content for deduplication
- A flag indicating it was discovered via curiosity (not planned)

Future planning passes include curiosity-discovered screens as candidates for structured test coverage, gradually converting unplanned discoveries into planned test targets.

### Curiosity Constraints

- Curiosity respects the `--curiosity-timeout` budget and will not exceed it
- The navigator avoids destructive actions (delete, logout, uninstall) by filtering button labels
- If a crash or ANR occurs during exploration, it is recorded as a finding with the full navigation path
- Curiosity runs after planned tests to avoid consuming the main session timeout

## Session Recording and Timeline Management

HelixQA maintains a detailed timeline of every action taken during a session. The timeline is a structured log of events with timestamps, enabling precise correlation between actions, screenshots, metrics, and findings.

### Timeline Events

| Event Type | Description |
|------------|-------------|
| `session_start` | Session begins with configuration summary |
| `phase_start` | A pipeline phase (learn, plan, execute, analyze) begins |
| `test_start` | An individual test case begins execution |
| `step_execute` | A test step is executed (tap, type, navigate) |
| `screenshot` | A screenshot is captured with file path |
| `video_start` | Video recording begins for a test case |
| `video_stop` | Video recording ends with file path and duration |
| `crash_detected` | A crash or ANR is detected with stack trace |
| `metric_sample` | A performance metric sample (memory, CPU) is recorded |
| `finding` | An analysis finding is created |
| `phase_end` | A pipeline phase completes with summary |
| `session_end` | Session completes with overall statistics |

### Timeline Output

The timeline is written to `qa-results/session-<timestamp>/timeline.json` as a JSON array of events. Each event includes:

```json
{
  "timestamp": "2026-03-27T14:32:01.234Z",
  "type": "step_execute",
  "test_id": "test-007",
  "step": 3,
  "action": "tap",
  "target": "Login button",
  "platform": "android",
  "screenshot": "screenshots/test-007-login-step-3.png",
  "duration_ms": 1420
}
```

### Session Reports

At the end of every session, a `pipeline-report.json` is generated summarizing:

- Total tests planned, executed, passed, and failed
- Total screenshots captured and analyzed
- Total findings by severity and category
- Coverage percentage (screens tested / screens known)
- Performance metrics summary (average memory, peak CPU)
- Duration of each pipeline phase
- LLM token usage and estimated cost

## Real-Time Crash and ANR Detection

Crash detection runs continuously during the execution phase, monitoring platform-specific signals:

### Android

- `adb logcat` is filtered in real time for `FATAL EXCEPTION`, `ANR in`, `Force closing`, and `has died` patterns
- When a crash is detected, the full stack trace is captured from logcat
- A screenshot is taken immediately after the crash
- The crash is correlated with the current test step and recorded as a critical finding

### Web (Playwright)

- Browser console errors (`console.error`) are captured via Playwright's event API
- Uncaught exceptions are intercepted via `page.on('pageerror')`
- Failed network requests (4xx, 5xx, timeouts) are logged via `page.on('requestfailed')`
- JavaScript errors are captured with full stack traces

### Desktop (X11)

- Process exit codes are monitored -- non-zero exits indicate crashes
- X11 error events are captured via `xdotool` status checks
- Window disappearance (unexpected close) is detected by polling the window list

## Multi-Platform Execution

HelixQA supports testing across all major application platforms through dedicated executors:

### Android Executor

Uses ADB for device communication, scrcpy for high-quality video recording (all SDK versions), and `adb screencap` for screenshots. Supports both USB and Wi-Fi connected devices. Handles SDK version differences transparently -- screenrecord works on Android 9 and below, while screenshot-to-video assembly is used for Android 10+ where screenrecord may fail from ADB.

### Web Executor

Uses Playwright for browser automation with Chromium, Firefox, or WebKit. Supports full page navigation, form interaction, file uploads, and JavaScript evaluation. Video recording and screenshots use Playwright's built-in capabilities. Console errors and network failures are captured automatically.

### Desktop Executor

Uses xdotool for mouse and keyboard interaction on X11 displays. ffmpeg x11grab records the screen, and ImageMagick captures screenshots. Supports Tauri, Electron, and native GTK/Qt applications. Headless operation is possible via Xvfb.

### API Executor

Sends HTTP requests to REST API endpoints with configurable headers, authentication, and request bodies. Validates response status codes, response times, and JSON schema compliance. Useful for backend-only testing without a UI.

## Test Bank Management

Test banks are YAML files containing predefined test cases that complement LLM-generated tests:

```yaml
- id: TB-LOGIN-001
  title: "Login with valid credentials"
  platform: web
  category: functional
  priority: critical
  tags: [auth, login, smoke]
  steps:
    - navigate: "/login"
    - fill: { selector: "#email", value: "test@example.com" }
    - fill: { selector: "#password", value: "password123" }
    - click: "#login-button"
    - assert: { url_contains: "/dashboard" }
```

Test banks are loaded from `challenges/helixqa-banks/` and reconciled with LLM-generated tests during the planning phase. The current test bank contains 517 cases across all platforms and categories.

## 40+ LLM Provider Support

HelixQA auto-discovers available providers by scanning environment variables at startup. The adaptive provider selects the best available model for each request type:

| Request Type | Model Selection Strategy |
|-------------|------------------------|
| Test planning | Fast, cost-effective models (DeepSeek, Groq) |
| Vision analysis | Multimodal-capable models (Claude, GPT-4V) |
| Ticket generation | Mid-tier models with good writing quality |
| Code analysis | Models with strong code understanding |

Provider failover is automatic: if the primary provider returns an error or times out, the next available provider in the chain is tried. See [LLM Providers](/providers) for the complete list.
