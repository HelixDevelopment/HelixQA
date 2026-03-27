# Multi-Pass QA

A single HelixQA session gives you a broad first look at your application's quality. Multiple passes, run over time, build toward complete coverage — each pass smarter and more targeted than the last.

## The Core Idea

HelixQA's [Memory System](/manual/memory) persists everything between sessions: which screens were tested, which tests passed or failed, which issues were found, and what paths were explored via curiosity. Every new session loads this history and uses it to generate a better test plan.

This means:
- Pass 1 explores broadly with limited prior knowledge
- Pass 2 digs deeper into areas that had failures or low coverage
- Pass 3 verifies that reported fixes actually work
- Pass N+1 always has more context than Pass N

## Running Multiple Passes

The command is identical for every pass — HelixQA automatically detects the current pass number from the memory database:

```bash
# Pass 1: initial scan
helixqa autonomous --project . --platforms all --timeout 1h

# Pass 2: deeper exploration (runs automatically after Pass 1 completes)
helixqa autonomous --project . --platforms all --timeout 1h

# Pass 3: regression verification
helixqa autonomous --project . --platforms all --timeout 30m
```

Each invocation increments the `pass_number` in the memory database. No special flags are needed.

## Recommended Pass Strategy

### Three-Pass Standard Cycle

| Pass | Timeout | Curiosity | Goal |
|------|---------|-----------|------|
| 1 | 1–2 hours | 10–15 min | Broad coverage, initial issue discovery |
| 2 | 1–2 hours | 15–20 min | Deep exploration, re-test failures from Pass 1 |
| 3 | 30–60 min | 5 min | Regression check, verify fixes from Passes 1–2 |

```bash
# Pass 1
helixqa autonomous \
  --project . \
  --platforms "android,web" \
  --timeout 1h \
  --curiosity-timeout 10m

# Pass 2
helixqa autonomous \
  --project . \
  --platforms "android,web" \
  --timeout 1h \
  --curiosity-timeout 15m

# Pass 3 (after fixes are deployed)
helixqa autonomous \
  --project . \
  --platforms "android,web" \
  --timeout 30m \
  --curiosity-timeout 5m
```

### Continuous Integration Cycle

For projects with frequent releases, run a pass after each deployment:

```bash
# Post-deploy pass (short, focused on recent changes)
helixqa autonomous \
  --project . \
  --platforms "android,web" \
  --timeout 20m \
  --curiosity=false
```

HelixQA's git change detection ensures recent code changes receive higher test priority automatically.

## How Pass Intelligence Works

### Coverage Prioritization

The planner queries the coverage table and identifies:

1. Screens with `times_tested = 0` (never tested) — highest priority
2. Screens with `times_failed > 0` in recent sessions — re-test for regression
3. Screens with `times_tested < 3` — need more coverage samples
4. Screens with `last_tested_at` older than 7 days — stale coverage

### Regression Detection

If Pass 2 discovers a finding on a screen that was clean in Pass 1, it is flagged as a regression. If a finding marked `fixed` reappears, its status is automatically set to `reopened` in the memory database and the ticket frontmatter is updated.

### Curiosity Discovery

Screens discovered via curiosity exploration in Pass N are added to the `knowledge` table. In Pass N+1 they appear in the KnowledgeBase as known screens, so the planner can generate deliberate test cases for them rather than relying on random discovery again.

## Coverage Target

Set a coverage target to know when you have achieved sufficient quality:

```bash
helixqa autonomous \
  --project . \
  --platforms all \
  --timeout 2h \
  --coverage-target 0.95
```

When the memory database shows coverage at or above the target across all platforms, the session exits early with a green status. Check current coverage:

```bash
sqlite3 HelixQA/data/memory.db \
  "SELECT platform, COUNT(*) as screens, AVG(times_passed * 1.0 / MAX(times_tested, 1)) as pass_rate
   FROM coverage GROUP BY platform;"
```

## Interpreting Multi-Pass Results

After several passes, the `pipeline-report.json` for each session shows progression:

```json
{
  "pass_number": 3,
  "total_tests": 28,
  "passed": 27,
  "failed": 1,
  "new_findings": 0,
  "verified_fixed": 3,
  "reopened": 1,
  "coverage_ratio": 0.94
}
```

Key metrics to track across passes:
- `new_findings` should decrease toward zero as the codebase stabilises
- `verified_fixed` should increase as the team resolves reported issues
- `coverage_ratio` should increase with each pass
- `reopened` indicates regressions that need attention

## Related Pages

- [Memory System](/manual/memory) — the database that enables multi-pass intelligence
- [Issue Tickets](/manual/tickets) — how reopened regressions are tracked
- [CLI Reference](/manual/cli) — full flag reference for `helixqa autonomous`
- [Pipeline Phases](/pipeline) — how prior session data feeds into Phase 1 (Learn)
