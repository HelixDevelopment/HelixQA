# Challenge Development Guide

Challenges are the foundation of structured, repeatable validation in HelixQA. They are Go structs with an `Execute()` method that perform a specific check and return a pass/fail result with evidence. This guide walks you through creating, registering, running, and debugging challenges.

## What Are Challenges?

A challenge is a self-contained validation scenario. Unlike test bank entries (which describe steps for a human or LLM executor), challenges are compiled Go code that directly exercises APIs, services, or infrastructure. Every challenge:

- Has a unique ID (e.g., `HQA-001`, `CH-005`)
- Embeds `challenge.BaseChallenge` from the `digital.vasic.challenges` module
- Implements an `Execute(ctx context.Context) challenge.Result` method
- Returns `Pass()`, `Fail()`, or `Skip()` with a descriptive message
- Carries metadata: name, description, category, severity

Challenges are HelixQA's mechanism for self-validation. Before trusting HelixQA to test your application, you run the HelixQA challenge suite to verify the framework itself works correctly.

---

## Creating a New Challenge

### Step 1: Define the Struct

Create a new Go file in the `challenges/` directory. Every challenge struct embeds `challenge.BaseChallenge` and holds any dependencies it needs:

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package challenges

import (
    "context"
    "fmt"

    "digital.vasic.challenges/pkg/challenge"
    "digital.vasic.helixqa/pkg/memory"
)

type MemorySchemaChallenge struct {
    challenge.BaseChallenge
    store *memory.Store
}

func NewMemorySchemaChallenge(
    store *memory.Store,
) *MemorySchemaChallenge {
    c := &MemorySchemaChallenge{store: store}
    c.SetID("HQA-016")
    c.SetName("MemoryInit")
    c.SetDescription(
        "Verify the SQLite memory store initialises " +
            "with the correct schema",
    )
    c.SetCategory("memory")
    c.SetSeverity(challenge.SeverityCritical)
    return c
}
```

### Step 2: Implement Execute

The `Execute` method receives a context (with timeout) and returns a `challenge.Result`. Use the `Pass()`, `Fail()`, and `Skip()` helpers inherited from `BaseChallenge`:

```go
func (c *MemorySchemaChallenge) Execute(
    ctx context.Context,
) challenge.Result {
    tables, err := c.store.ListTables(ctx)
    if err != nil {
        return c.Fail(fmt.Sprintf(
            "failed to list tables: %v", err,
        ))
    }

    required := []string{
        "sessions", "test_results", "findings",
        "screenshots", "metrics", "knowledge",
        "coverage",
    }

    for _, t := range required {
        if !contains(tables, t) {
            return c.Fail(fmt.Sprintf(
                "missing required table: %s", t,
            ))
        }
    }

    return c.Pass(fmt.Sprintf(
        "all %d required tables present",
        len(required),
    ))
}
```

### Step 3: Register the Challenge

Open `challenges/register.go` and add your challenge to the `RegisterAll()` function. This function is called at startup and populates the challenge runner:

```go
func RegisterAll(
    runner *runner.Runner,
    store *memory.Store,
    registry *llm.Registry,
) {
    // Existing challenges...
    runner.Register(NewProviderDiscoveryChallenge(registry))
    runner.Register(NewAnthropicConnectivityChallenge(registry))

    // Your new challenge
    runner.Register(NewMemorySchemaChallenge(store))
}
```

Every challenge must be registered here. Unregistered challenges are invisible to the runner and the REST API.

---

## Challenge Categories

HelixQA organises challenges into categories that map to the system's major subsystems. Each category covers a distinct layer of the architecture.

### API Challenges

Validate HTTP endpoints, authentication flows, and request/response contracts. These challenges make real HTTP requests against a running `catalog-api` instance:

```go
c.SetCategory("api")
```

Examples: endpoint reachability, JWT validation, CRUD operations, error response format.

### Web Challenges

Validate the React web frontend via Playwright. These challenges launch a browser, navigate to pages, and assert visual and functional correctness:

```go
c.SetCategory("web")
```

Examples: page load, navigation, form submission, WebSocket connection, responsive layout.

### Desktop Challenges

Validate Tauri desktop applications via X11/xdotool automation:

```go
c.SetCategory("desktop")
```

Examples: window launch, menu navigation, file dialogs, IPC communication.

### Mobile Challenges

Validate Android and Android TV applications via ADB:

```go
c.SetCategory("mobile")
```

Examples: app launch, touch navigation, D-pad navigation, intent handling, screen rotation.

### Infrastructure Challenges

Validate internal framework components that do not require a running application:

```go
c.SetCategory("llm-providers")
c.SetCategory("memory")
c.SetCategory("learning")
c.SetCategory("planning")
c.SetCategory("execution")
c.SetCategory("pipeline")
c.SetCategory("container")
```

---

## Running Challenges

### Via the CLI

```bash
# Run all challenges
helixqa challenges run --all

# Run a single challenge by ID
helixqa challenges run --id HQA-001

# Run all challenges in a category
helixqa challenges run --category memory

# List available challenges
helixqa challenges list

# Run with verbose output
helixqa challenges run --all --verbose
```

### Via the REST API

When HelixQA or `catalog-api` is running as a service with the challenges module wired in, challenges are exposed at `/api/v1/challenges`:

```bash
# List all challenges
curl http://localhost:8080/api/v1/challenges

# Run a specific challenge
curl -X POST http://localhost:8080/api/v1/challenges/HQA-001/run

# Run all challenges (synchronous, blocking)
curl -X POST http://localhost:8080/api/v1/challenges/run-all
```

**Important**: `RunAll` is synchronous and blocking. No other challenge can execute until it completes. The runner enforces a 5-minute stale threshold -- if a challenge makes no progress for 5 minutes, it is killed.

### Via Go Tests

For development and debugging, call `Execute` directly in a test:

```go
func TestMemorySchema(t *testing.T) {
    store, err := memory.NewStore(":memory:")
    require.NoError(t, err)
    defer store.Close()

    c := NewMemorySchemaChallenge(store)
    result := c.Execute(context.Background())
    assert.Equal(t, challenge.StatusPassed, result.Status)
}
```

---

## Debugging Failed Challenges

When a challenge fails, the result contains a descriptive message. Start your investigation from this message.

### Check the Result Message

```json
{
  "id": "HQA-016",
  "name": "MemoryInit",
  "status": "failed",
  "message": "missing required table: coverage",
  "duration": "12ms"
}
```

The message tells you exactly what went wrong. In this case, the `coverage` table is missing from the memory database.

### Run in Isolation

Run the single failing challenge with verbose output:

```bash
helixqa challenges run --id HQA-016 --verbose
```

Verbose mode prints the full execution trace, including setup steps, intermediate checks, and teardown.

### Inspect Dependencies

Some challenges depend on external services or configuration. Common failure causes:

| Failure Pattern | Likely Cause | Fix |
|-----------------|-------------|-----|
| `no LLM providers discovered` | No API key set | Export at least one `*_API_KEY` env var |
| `ADB device not reachable` | Device disconnected | Run `adb connect <device>` |
| `Playwright launch failed` | Browser not installed | Run `npx playwright install` |
| `timeout exceeded` | Service not running | Start `catalog-api` or target app |
| `connection refused` | Wrong URL/port | Check `HELIX_WEB_URL` / `HELIX_API_URL` |

### Add Diagnostic Evidence

When writing challenges, include diagnostic data in failure messages. The more context you provide, the faster debugging becomes:

```go
if err != nil {
    return c.Fail(fmt.Sprintf(
        "failed to connect to %s:%d: %v "+
            "(is the service running?)",
        host, port, err,
    ))
}
```

---

## Best Practices

### Keep Challenges Focused

Each challenge should test exactly one thing. If you find yourself testing multiple independent conditions, split them into separate challenges.

### Use Descriptive IDs

Follow the `<PREFIX>-<NNN>` convention. Group related challenges under the same prefix with sequential numbers:

| Prefix | Scope |
|--------|-------|
| `HQA-` | HelixQA self-validation |
| `CH-` | Catalogizer application challenges |
| `UF-API-` | User flow: API platform |
| `UF-WEB-` | User flow: web platform |
| `UF-DSK-` | User flow: desktop platform |
| `UF-MOB-` | User flow: mobile platform |

### Set Appropriate Timeouts

The default challenge timeout is 5 minutes. For quick checks (schema validation, API ping), reduce it. For long-running operations (full pipeline, container builds), increase it via `challenge.NewConfig()`:

```go
cfg := challenge.NewConfig()
cfg.Timeout = 2 * time.Minute
c.SetConfig(cfg)
```

### Never Skip Without Reason

If a challenge cannot run because of a missing dependency (no API key, no device connected), use `Skip()` with an explanation rather than `Fail()`:

```go
if os.Getenv("ANTHROPIC_API_KEY") == "" {
    return c.Skip("ANTHROPIC_API_KEY not set")
}
```

### Test Your Challenges

Write a standard Go test alongside each challenge. The test should verify both the pass and fail paths:

```go
func TestMemorySchema_Pass(t *testing.T) {
    store := setupTestStore(t) // creates all tables
    c := NewMemorySchemaChallenge(store)
    result := c.Execute(context.Background())
    assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestMemorySchema_MissingTable(t *testing.T) {
    store := setupBrokenStore(t) // missing coverage table
    c := NewMemorySchemaChallenge(store)
    result := c.Execute(context.Background())
    assert.Equal(t, challenge.StatusFailed, result.Status)
    assert.Contains(t, result.Message, "coverage")
}
```

## Related Pages

- [Challenges Overview](/advanced/challenges) -- the full list of HQA challenges
- [Architecture](/architecture) -- where challenges fit in the pipeline
- [Test Banks](/guides/test-banks) -- YAML-based test case authoring (complementary to challenges)
- [CI Integration](/guides/ci-integration) -- running challenges in local CI pipelines
