# API Reference

HelixQA is primarily a CLI tool. Its main interface is the `helixqa` binary with subcommands (`run`, `autonomous`, `list`, `report`, `version`). However, when integrated with the `digital.vasic.challenges` framework or when running as a service, several API endpoints become available. This page documents those interfaces.

## Primary Interface: CLI

The `helixqa` binary is the canonical way to interact with HelixQA. For the complete CLI reference, see [CLI Reference](/reference/cli).

```bash
helixqa run --banks banks/ --platform web --output qa-results
helixqa autonomous --project . --platforms web --timeout 30m
helixqa list --banks banks/ --platform android --json
helixqa report --input qa-results/session-123 --format html
helixqa version
```

All session data is written to the filesystem. Reports, screenshots, videos, and issue tickets are files on disk. This makes integration straightforward -- any tool that can read files can consume HelixQA output.

---

## Challenge Execution API

When the `digital.vasic.challenges` framework is wired into a running service (e.g., `catalog-api`), challenges are exposed via REST endpoints at `/api/v1/challenges`. HelixQA's self-validation challenges (HQA-001 through HQA-081) are registered alongside application challenges.

### List All Challenges

```
GET /api/v1/challenges
```

Returns the full list of registered challenges with metadata.

**Response** `200 OK`:

```json
{
  "challenges": [
    {
      "id": "HQA-001",
      "name": "ProviderDiscovery",
      "description": "Auto-discovery finds at least one configured provider",
      "category": "llm-providers",
      "severity": "critical",
      "status": "pending"
    },
    {
      "id": "HQA-016",
      "name": "MemoryInit",
      "description": "Database initialises with correct schema",
      "category": "memory",
      "severity": "critical",
      "status": "pending"
    }
  ],
  "total": 30
}
```

### List Challenges by Category

```
GET /api/v1/challenges?category=memory
```

**Query Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `category` | string | Filter by challenge category |
| `severity` | string | Filter by severity: `critical`, `high`, `medium`, `low` |

**Response** `200 OK`:

```json
{
  "challenges": [
    {
      "id": "HQA-016",
      "name": "MemoryInit",
      "category": "memory",
      "severity": "critical",
      "status": "pending"
    },
    {
      "id": "HQA-017",
      "name": "SessionPersistence",
      "category": "memory",
      "severity": "critical",
      "status": "pending"
    }
  ],
  "total": 7
}
```

### Run a Single Challenge

```
POST /api/v1/challenges/{id}/run
```

Executes the specified challenge synchronously and returns the result.

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Challenge ID (e.g., `HQA-001`) |

**Response** `200 OK`:

```json
{
  "id": "HQA-001",
  "name": "ProviderDiscovery",
  "status": "passed",
  "message": "discovered 3 provider(s): [anthropic groq deepseek]",
  "duration": "45ms",
  "timestamp": "2026-03-27T14:30:00Z"
}
```

**Response** `200 OK` (failure):

```json
{
  "id": "HQA-001",
  "name": "ProviderDiscovery",
  "status": "failed",
  "message": "no LLM providers discovered -- set at least one API key",
  "duration": "2ms",
  "timestamp": "2026-03-27T14:30:00Z"
}
```

**Response** `404 Not Found`:

```json
{
  "error": "challenge not found: HQA-999"
}
```

### Run All Challenges

```
POST /api/v1/challenges/run-all
```

Executes all registered challenges sequentially. This endpoint is **synchronous and blocking** -- no other challenge can run until all challenges complete.

**Important constraints**:

- The `write_timeout` in `config.json` must be set to `900` (15 minutes) for long-running challenge suites
- Progress-based liveness detection: if no challenge makes progress for 5 minutes, the runner is killed
- Challenges run sequentially, never in parallel

**Response** `200 OK`:

```json
{
  "total": 30,
  "passed": 27,
  "failed": 2,
  "skipped": 1,
  "duration": "2m34s",
  "results": [
    {
      "id": "HQA-001",
      "name": "ProviderDiscovery",
      "status": "passed",
      "message": "discovered 3 provider(s)",
      "duration": "45ms"
    },
    {
      "id": "HQA-048",
      "name": "ADBConnectivity",
      "status": "skipped",
      "message": "HELIX_ANDROID_DEVICE not set",
      "duration": "1ms"
    }
  ]
}
```

### Get Challenge Status

```
GET /api/v1/challenges/{id}
```

Returns the current status of a challenge (pending, running, passed, failed, skipped).

**Response** `200 OK`:

```json
{
  "id": "HQA-001",
  "name": "ProviderDiscovery",
  "status": "passed",
  "last_run": "2026-03-27T14:30:00Z",
  "message": "discovered 3 provider(s): [anthropic groq deepseek]"
}
```

---

## Report Generation

Report generation is performed via the CLI (`helixqa report`) rather than a REST endpoint. The output is written to disk in the requested format.

### Programmatic Report Access

After a session completes, read the JSON report for programmatic access:

```bash
cat qa-results/session-<timestamp>/pipeline-report.json
```

### Report Schema

The JSON report follows this structure:

```json
{
  "session_id": "session-1711547422",
  "pass_number": 2,
  "platforms": ["android", "web"],
  "start_time": "2026-03-27T14:00:00Z",
  "end_time": "2026-03-27T14:30:00Z",
  "duration": "30m0s",
  "total_tests": 28,
  "passed": 25,
  "failed": 3,
  "skipped": 0,
  "coverage_ratio": 0.87,
  "findings": [
    {
      "id": "HELIX-001",
      "title": "Login button not visible on dark theme",
      "severity": "high",
      "category": "visual",
      "platform": "web",
      "status": "open",
      "evidence": {
        "screenshots": [
          "screenshots/test-001-login-step-2-post.png"
        ],
        "video": "videos/test-001-login.mp4"
      },
      "description": "The login button blends into the dark background...",
      "reproduction_steps": [
        "Navigate to login page",
        "Enable dark theme",
        "Observe login button visibility"
      ]
    }
  ],
  "platform_results": [
    {
      "platform": "web",
      "tests": 15,
      "passed": 13,
      "failed": 2,
      "crash_count": 0,
      "anr_count": 0,
      "duration": "12m30s"
    },
    {
      "platform": "android",
      "tests": 13,
      "passed": 12,
      "failed": 1,
      "crash_count": 1,
      "anr_count": 0,
      "duration": "17m30s"
    }
  ],
  "llm_usage": {
    "anthropic": {
      "input_tokens": 45000,
      "output_tokens": 12000,
      "requests": 28,
      "estimated_cost_usd": 0.42
    }
  }
}
```

### Key Report Fields

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | Unique session identifier (directory name) |
| `pass_number` | int | Increments across sessions (tracked in memory DB) |
| `platforms` | string[] | Platforms tested in this session |
| `total_tests` | int | Total test cases executed |
| `passed` | int | Tests that passed all steps |
| `failed` | int | Tests with at least one failure |
| `coverage_ratio` | float | Features tested / features discovered (0.0 to 1.0) |
| `findings` | object[] | Detected issues with evidence |
| `platform_results` | object[] | Per-platform breakdown |
| `llm_usage` | object | Token usage and estimated cost per provider |

---

## Test Bank Management

Test bank operations are performed via the CLI. There is no REST endpoint for bank management.

### List Test Cases

```bash
helixqa list --banks banks/ --json
```

Returns a JSON array of all test cases:

```json
[
  {
    "id": "AUTH-001",
    "name": "Successful login with valid credentials",
    "category": "functional",
    "priority": "critical",
    "platforms": ["android", "web", "desktop"],
    "tags": ["auth", "login", "smoke"],
    "steps": 3,
    "estimated_duration": "15s"
  }
]
```

### Filter Test Cases

```bash
# By platform
helixqa list --banks banks/ --platform android --json

# By category
helixqa list --banks banks/ --category security --json

# By priority
helixqa list --banks banks/ --priority critical --json

# By tag
helixqa list --banks banks/ --tag smoke --json
```

---

## Session Management

Session data is stored in the SQLite memory database at `HelixQA/data/memory.db`. There is no REST endpoint for session management -- access is via the CLI and filesystem.

### Session Data Location

```
HelixQA/data/memory.db          # SQLite memory database
qa-results/session-<timestamp>/ # Session output directory
docs/issues/                    # Generated issue tickets
```

### Memory Database Tables

| Table | Contents |
|-------|----------|
| `sessions` | Session metadata (start time, duration, platforms, pass number) |
| `test_results` | Per-test pass/fail results with evidence paths |
| `findings` | Issue findings with status transitions (open, verified, fixed) |
| `screenshots` | Screenshot metadata and analysis results |
| `metrics` | Performance metrics collected during execution |
| `knowledge` | Discovered screens and navigation paths |
| `coverage` | Feature coverage tracking across sessions |

### Querying the Memory Database

For debugging or analysis, query the memory database directly with SQLite:

```bash
sqlite3 HelixQA/data/memory.db

-- List all sessions
SELECT id, start_time, pass_number, platforms
FROM sessions ORDER BY start_time DESC;

-- Count findings by severity
SELECT severity, COUNT(*) as count
FROM findings GROUP BY severity;

-- Check coverage progress
SELECT platform, coverage_ratio, updated_at
FROM coverage ORDER BY updated_at DESC;
```

---

## Health and Status

### Version Check

```bash
helixqa version
```

Output:

```
helixqa v0.2.0
```

### Provider Discovery

When running an autonomous session, HelixQA prints discovered providers at startup:

```
LLM provider:     anthropic
Available providers: [anthropic groq deepseek]
Vision providers:   [anthropic]
```

### Tool Discovery

Check which external tools are available on the host:

```bash
helixqa run --banks /dev/null --verbose 2>&1 | grep "tool"
```

The bridge registry reports tool availability at startup when verbose mode is enabled:

```
Tool discovery:
  scrcpy:    available (v2.4)
  appium:    not found
  allure:    available (v2.25.0)
  perfetto:  not found
  maestro:   available (v1.37.0)
  ffmpeg:    available (v6.1)
  adb:       available (v35.0.1)
  xdotool:   available (v3.20211022.1)
```

---

## Error Responses

All API error responses follow a consistent format:

```json
{
  "error": "descriptive error message"
}
```

### Common HTTP Status Codes

| Code | Meaning | Example |
|------|---------|---------|
| `200` | Success | Challenge passed or failed (result in body) |
| `400` | Bad request | Invalid parameter value |
| `404` | Not found | Challenge ID does not exist |
| `408` | Timeout | Challenge exceeded its timeout |
| `500` | Internal error | Unexpected server error |
| `503` | Unavailable | Runner busy (RunAll in progress) |

---

## Integration Patterns

### Script Integration

Parse CLI output with standard Unix tools:

```bash
# Run and check exit code
helixqa run --banks banks/ --platform web && echo "PASS" || echo "FAIL"

# Parse JSON report
FINDINGS=$(cat qa-results/session-*/pipeline-report.json | \
    jq '.findings | length')
echo "Found $FINDINGS issues"
```

### CI Pipeline Integration

See [CI Integration](/guides/ci-integration) for complete pipeline scripts.

### Webhook Notification (External)

HelixQA does not have built-in webhook support. Use a wrapper script to send notifications:

```bash
#!/usr/bin/env bash
helixqa autonomous --project . --platforms web --timeout 30m
EXIT=$?

# Send notification
curl -X POST "https://hooks.slack.com/services/..." \
    -H "Content-Type: application/json" \
    -d "{\"text\": \"HelixQA session complete. Exit: $EXIT\"}"

exit $EXIT
```

## Related Pages

- [CLI Reference](/reference/cli) -- complete command-line reference
- [Configuration](/reference/config) -- environment variables and config fields
- [Test Bank Schema](/reference/test-bank-schema) -- YAML test bank format
- [Challenges](/guides/challenges) -- challenge development guide
- [CI Integration](/guides/ci-integration) -- pipeline integration
