# Pipeline Phases

## Overview

Every HelixQA autonomous session runs the same 4-phase pipeline (plus an optional curiosity phase). Each phase feeds into the next, and all intermediate data is persisted to the memory database so that subsequent sessions can build on it.

```
┌──────────────────────────────────────────────────────────┐
│                     SESSION PIPELINE                      │
├────────────┬────────────┬────────────┬───────────────────┤
│  Phase 1   │  Phase 2   │  Phase 3   │  Phase 4          │
│  LEARN     │  PLAN      │  EXECUTE   │  ANALYZE          │
│            │            │            │                   │
│ Read docs, │ LLM-driven │ Run tests, │ LLM vision,       │
│ code, git, │ test case  │ record     │ leak detection,   │
│ prior QA   │ generation │ video,     │ crash analysis,   │
│ sessions   │ + ranking  │ screenshots│ issue tickets     │
└────────────┴────────────┴────────────┴───────────────────┘
       │                                         │
       ▼                                         ▼
┌─────────────┐                       ┌──────────────────┐
│  Memory DB  │                       │  docs/issues/    │
│  (SQLite)   │                       │  HELIX-NNN.md    │
└─────────────┘                       └──────────────────┘
```

## Phase 1: Learn

The learning phase builds a `KnowledgeBase` by ingesting all available project information.

### What Gets Read

| Source | What Is Extracted |
|--------|------------------|
| `CLAUDE.md` | Architecture constraints, tech stack, test requirements |
| `docs/` directory | Feature descriptions, API documentation, design decisions |
| Go source files | Gin route handlers, service boundaries |
| React source files | Page components, router paths |
| Kotlin/Compose | Navigation graphs, screen names |
| Git history | Recent change hotspots, frequently modified files |
| Memory DB | Prior session results, known issues, coverage gaps |

### KnowledgeBase Structure

```go
type KnowledgeBase struct {
    Screens     []Screen        // UI screens/pages discovered
    Endpoints   []APIEndpoint   // REST API endpoints
    Components  []Component     // UI components
    Constraints []string        // From CLAUDE.md
    RecentChanges []GitChange   // Last N commits
    PriorFindings []Finding     // From memory DB
    CoverageGaps  []string      // Under-tested areas
}
```

### Customizing Ingestion

Structure your project documentation for optimal learning:

- Keep `CLAUDE.md` at the project root with architecture, constraints, and known issues
- Place feature docs in `docs/features/`
- Put API docs in `docs/api/`
- HelixQA reads all `.md` files recursively under `docs/`

## Phase 2: Plan

The planning phase uses the `KnowledgeBase` to generate a prioritized list of test cases via LLM.

### Test Generation

The planner constructs a detailed prompt from the `KnowledgeBase` and asks the LLM to generate test cases covering:

| Category | Examples |
|----------|---------|
| `functional` | Login flow, media playback, collection management |
| `security` | Auth bypass attempts, input injection, token expiry |
| `edge_case` | Empty states, network failure, malformed data |
| `performance` | Page load time, memory growth under load |
| `accessibility` | Color contrast, touch target sizes, screen reader labels |
| `visual` | Layout correctness, icon rendering, responsive breakpoints |

### Test Bank Reconciliation

Before finalizing the plan, the planner reconciles generated tests against existing YAML test banks in `challenges/helixqa-banks/`. This prevents duplicate test generation and ensures new tests complement existing coverage.

### Priority Ranking

Tests are ranked before execution:

1. `critical` severity items from prior sessions (regressions)
2. Recently changed code paths (from git analysis)
3. Areas with zero or low coverage (from memory DB)
4. Standard functional coverage
5. Curiosity-driven exploration items

## Phase 3: Execute

The execution phase runs each planned test case using the appropriate platform executor.

### Executor Selection

| Platform flag | Executor used |
|--------------|--------------|
| `web` | Playwright |
| `android` | ADB |
| `androidtv` | ADB |
| `desktop` | X11 / xdotool |
| `cli` | CLIExecutor (stdin/stdout) |
| `api` | APIExecutor (HTTP client) |

### Per-Test Execution Flow

For each test case:

1. Start video recording (platform-appropriate method)
2. Navigate to the starting screen
3. Execute each test step
4. Capture a screenshot after each step
5. Check for crashes or ANRs continuously via logcat / console
6. Record performance metrics (memory, CPU) at defined intervals
7. Stop video recording and save evidence

### Evidence Naming Convention

```
qa-results/session-<timestamp>/
├── screenshots/
│   ├── test-001-login-step-1.png
│   ├── test-001-login-step-2.png
│   └── ...
├── videos/
│   ├── test-001-login.mp4
│   └── ...
└── pipeline-report.json
```

## Phase 3.5: Curiosity Exploration

After planned tests complete, the curiosity phase performs random navigation to discover screens and states not covered by the planned test suite.

```bash
# Enable and configure curiosity
helixqa autonomous \
  --project . \
  --platforms android \
  --curiosity=true \
  --curiosity-timeout 5m
```

The curiosity phase:
- Randomly taps interactive elements (buttons, list items, nav icons)
- Captures screenshots of every new screen reached
- Feeds discovered screens back into the knowledge base for future passes
- Stays within the configured `--curiosity-timeout` budget

## Phase 4: Analyze

The analysis phase processes all collected evidence using LLM vision and rule-based detectors.

### Screenshot Analysis

Every screenshot is sent to the configured LLM vision model with a structured prompt requesting analysis across these categories:

| Category | What Is Checked |
|----------|----------------|
| `visual` | Misaligned elements, clipped text, wrong colors, broken layouts |
| `ux` | Unresponsive buttons, confusing navigation, missing feedback |
| `accessibility` | Contrast ratios, touch target sizes, missing labels |
| `brand` | Logo placement, color scheme compliance, typography |
| `content` | Empty screens that should have data, placeholder text in production |
| `performance` | Visible jank, slow loading indicators, frozen frames |

### Leak Detection

Memory metrics collected during execution are analyzed for leak indicators:

- Monotonically increasing heap allocations across test steps
- Heap size at end significantly higher than at start
- Per-step memory delta trending upward

### Crash Detection

Real-time crash monitoring runs throughout Phase 3 and feeds findings into Phase 4:

- **Android**: `adb logcat` filtered for `FATAL EXCEPTION`, `ANR`, `Force Close`
- **Web**: Browser console errors, uncaught exceptions, failed network requests
- **Desktop**: Process exit codes, X11 error events

### Issue Ticket Creation

All findings from vision analysis, leak detection, and crash detection flow through `FindingsBridge` into the memory store and are written as markdown tickets. See [Issue Tickets](/manual/tickets) for the ticket format.

## Running the Pipeline

```bash
# Full autonomous session
helixqa autonomous \
  --project /path/to/project \
  --platforms "android,web" \
  --timeout 30m \
  --curiosity=true \
  --curiosity-timeout 5m \
  --output qa-results

# After the session completes
ls docs/issues/HELIX-*.md      # Issue tickets
cat qa-results/session-*/pipeline-report.json
```

## Related Pages

- [Platform Executors](/executors) — per-platform execution details
- [LLM Providers](/providers) — configuring the AI backend
- [Multi-Pass QA](/manual/multi-pass) — running successive sessions
- [Issue Tickets](/manual/tickets) — ticket format and lifecycle
