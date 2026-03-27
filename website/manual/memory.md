# Memory System

HelixQA maintains a persistent SQLite database that accumulates knowledge across every QA session. This "photographic memory" is what enables each new session to build on the results of all previous ones, rather than starting from scratch.

## Database Location

```
HelixQA/data/memory.db
```

Override the path with the `HELIX_MEMORY_DB` environment variable:

```bash
export HELIX_MEMORY_DB="/path/to/custom/memory.db"
```

## Schema

The memory database has 7 tables:

| Table | Contents |
|-------|---------|
| `sessions` | One row per QA session: timestamp, platforms, pass number, duration, coverage ratio, pass/fail counts |
| `test_results` | One row per test execution: session ID, test name, platform, status, duration, screenshot paths |
| `findings` | Every issue discovered: severity, category, platform, screen, status, description, evidence paths |
| `screenshots` | Screenshot index: file path, test name, session ID, timestamp |
| `metrics` | Performance data points: session ID, test step, memory usage, CPU usage, timestamp |
| `knowledge` | Learned project facts: screen count, endpoint count, component list, last ingested |
| `coverage` | Per-screen/platform coverage tracking: times tested, last tested, pass rate |

## What Gets Persisted

### Sessions

Every run of `helixqa autonomous` creates a session record:

```sql
sessions(
    id, started_at, finished_at,
    platforms, pass_number,
    total_tests, passed, failed,
    coverage_ratio, output_dir
)
```

The `pass_number` increments automatically — the first run is pass 1, the second is pass 2, and so on for the same project.

### Findings

Every issue discovered (from LLM vision, crash detection, or leak detection) is stored with full lifecycle tracking:

```sql
findings(
    id, session_id,
    helix_id,       -- e.g. "HELIX-042"
    severity,       -- critical | high | medium | low | cosmetic
    category,       -- visual | ux | accessibility | performance | functional | brand
    platform, screen,
    status,         -- open | in_progress | fixed | verified | reopened | wontfix
    description,
    screenshot_path, video_path,
    found_at, updated_at
)
```

### Coverage

The coverage table tracks how thoroughly each screen/platform combination has been tested:

```sql
coverage(
    platform, screen,
    times_tested, times_passed, times_failed,
    last_tested_at, last_pass_number
)
```

## How Memory Drives Each New Session

At the start of Phase 1 (Learn), HelixQA queries the memory database and feeds the results into the `KnowledgeBase`:

- **Coverage gaps** — screens with zero or low coverage are flagged for prioritized testing
- **Recent failures** — tests that failed in the last session are re-run first (regression check)
- **Open findings** — known open issues are tested to check if they have been fixed
- **Performance baselines** — prior metrics provide baseline comparison for leak detection
- **Discovered screens** — screens found via curiosity in prior passes are added to the test plan

This means pass 2 is always more targeted than pass 1, and pass 3 more targeted still.

## Querying the Memory Database

You can inspect the memory database directly with any SQLite client:

```bash
sqlite3 HelixQA/data/memory.db

# Show all sessions
SELECT id, started_at, pass_number, total_tests, passed, failed
FROM sessions ORDER BY started_at DESC;

# Show open findings
SELECT helix_id, severity, platform, screen, description
FROM findings WHERE status = 'open'
ORDER BY severity;

# Show coverage by platform
SELECT platform, screen, times_tested, times_passed
FROM coverage ORDER BY times_tested ASC;

# Show memory leak indicators across sessions
SELECT session_id, test_name, memory_mb
FROM metrics WHERE metric_type = 'heap_end'
ORDER BY session_id, memory_mb DESC;
```

## Issue Lifecycle in Memory

When a finding is first created, its status is `open`. As the team works on it, the status flows through these states:

```
open → in_progress → fixed → verified
                           ↘ reopened → open
                   → wontfix
```

HelixQA automatically sets a finding to `reopened` if it detects the same issue in a subsequent session after it was previously marked `fixed`. This provides automatic regression detection with no manual tracking.

## Resetting Memory

To start completely fresh (e.g. after a major refactor):

```bash
rm HelixQA/data/memory.db
```

The database is recreated automatically on the next run. Note that all historical session data, findings, and coverage information will be lost.

To reset only coverage (keep findings history):

```bash
sqlite3 HelixQA/data/memory.db "DELETE FROM coverage;"
```

## Related Pages

- [Multi-Pass QA](/manual/multi-pass) — using memory for progressive coverage
- [Issue Tickets](/manual/tickets) — how findings become markdown tickets
- [Pipeline Phases](/pipeline) — how memory feeds into the Learn phase
