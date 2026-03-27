# Challenges

HelixQA includes 30 self-validation challenges (HQA-001 to HQA-081) that verify the framework itself is working correctly. These challenges are built on the `digital.vasic.challenges` framework and cover every major subsystem: LLM providers, memory, learning, planning, execution, curiosity, analysis, pipeline orchestration, and container integration.

## What Are Challenges?

Challenges are structured, executable test scenarios defined as Go structs. Each challenge has:

- A unique ID (e.g. `HQA-001`)
- A name and description
- An `Execute()` method that runs the validation
- Pass/fail assertions with detailed evidence
- A severity level and category

They serve as HelixQA's own QA suite — before trusting HelixQA to test your application, you can verify HelixQA itself passes all its challenges.

## Running Challenges

Challenges are exposed via the `digital.vasic.challenges` REST API when HelixQA is running as a service. They can also be run directly via the challenges CLI runner:

```bash
# Run all HelixQA challenges
helixqa challenges run --all

# Run a specific challenge
helixqa challenges run --id HQA-001

# Run a category
helixqa challenges run --category llm-providers

# List all challenges
helixqa challenges list
```

## Challenge Categories

### LLM Provider Challenges (HQA-001 to HQA-015)

Verify each configured LLM provider is reachable and returns valid responses.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-001 | ProviderDiscovery | Auto-discovery finds at least one configured provider |
| HQA-002 | AnthropicConnectivity | Anthropic API returns a valid chat completion |
| HQA-003 | OpenAIConnectivity | OpenAI API returns a valid chat completion |
| HQA-004 | OpenRouterConnectivity | OpenRouter API returns a valid chat completion |
| HQA-005 | DeepSeekConnectivity | DeepSeek API returns a valid chat completion |
| HQA-006 | GroqConnectivity | Groq API returns a valid chat completion |
| HQA-007 | OllamaConnectivity | Ollama local server is reachable |
| HQA-008 | VisionAnalysis | Vision-capable provider analyses a test screenshot |
| HQA-009 | AdaptiveSelection | AdaptiveProvider selects correct provider per request type |
| HQA-010 | ProviderFallback | Falls back to next provider on simulated failure |

### Memory Challenges (HQA-016 to HQA-025)

Verify the SQLite memory store reads and writes correctly.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-016 | MemoryInit | Database initialises with correct schema |
| HQA-017 | SessionPersistence | Session records survive process restart |
| HQA-018 | FindingLifecycle | Finding status transitions work correctly |
| HQA-019 | CoverageTracking | Coverage table updates on each test execution |
| HQA-020 | PassNumberIncrement | Pass number increments correctly across sessions |
| HQA-021 | RegressionDetection | Reopens finding when re-detected after fix |
| HQA-022 | KnowledgeAccumulation | Learned screens persist and load in next session |

### Learning Challenges (HQA-026 to HQA-035)

Verify the project knowledge ingestion pipeline.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-026 | ClaudeMdIngestion | CLAUDE.md is read and constraints extracted |
| HQA-027 | DocsIngestion | All markdown files under docs/ are parsed |
| HQA-028 | RouteExtraction | Go Gin routes are extracted from source |
| HQA-029 | ReactRouteExtraction | React Router paths are extracted |
| HQA-030 | GitAnalysis | Recent commits are read and hotspots identified |
| HQA-031 | PriorSessionLoading | Prior session findings load into KnowledgeBase |

### Planning Challenges (HQA-036 to HQA-045)

Verify the LLM-driven test plan generation.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-036 | TestGeneration | LLM generates at least 5 test cases from KnowledgeBase |
| HQA-037 | CategoryCoverage | Generated tests cover functional, security, and edge_case |
| HQA-038 | BankReconciliation | New tests do not duplicate existing bank entries |
| HQA-039 | PriorityRanking | Critical severity tests rank above low severity |
| HQA-040 | CoverageGapPriority | Untested screens appear in test plan |

### Execution Challenges (HQA-046 to HQA-055)

Verify platform executor connectivity and basic operations.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-046 | PlaywrightLaunch | Playwright browser launches successfully |
| HQA-047 | PlaywrightScreenshot | Screenshot captured and saved to output directory |
| HQA-048 | ADBConnectivity | ADB device is reachable (if configured) |
| HQA-049 | ADBScreenshot | Screenshot captured from Android device |
| HQA-050 | VideoRecording | Video recording starts and produces a non-empty file |
| HQA-051 | CrashDetection | Simulated crash is detected via logcat pattern |

### Pipeline Challenges (HQA-056 to HQA-065)

Verify the full autonomous pipeline end-to-end.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-056 | PipelineLearnPhase | Phase 1 completes within timeout |
| HQA-057 | PipelinePlanPhase | Phase 2 produces a non-empty test plan |
| HQA-058 | PipelineExecutePhase | Phase 3 runs at least one test with a screenshot |
| HQA-059 | PipelineCuriosityPhase | Phase 3.5 captures additional screenshots |
| HQA-060 | PipelineAnalyzePhase | Phase 4 produces analysis output |
| HQA-061 | TicketCreation | At least one ticket written to docs/issues/ |
| HQA-062 | ReportGeneration | Markdown and JSON reports generated successfully |

### Container Challenges (HQA-066 to HQA-075)

Verify containerised execution works correctly.

| ID | Name | What It Tests |
|----|------|--------------|
| HQA-066 | ContainerRuntime | Podman or Docker is available |
| HQA-067 | ComposeValidation | docker-compose.qa-robot.yml is valid |
| HQA-068 | ImageBuild | Container image builds successfully |
| HQA-069 | ContainerExecution | HelixQA runs inside container and produces output |

## Challenge Implementation

Each challenge embeds `challenge.BaseChallenge` from the `digital.vasic.challenges` module:

```go
// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package challenges

import "digital.vasic.challenges/pkg/challenge"

type ProviderDiscoveryChallenge struct {
    challenge.BaseChallenge
    registry *llm.Registry
}

func (c *ProviderDiscoveryChallenge) Execute(ctx context.Context) challenge.Result {
    providers := c.registry.Available()
    if len(providers) == 0 {
        return c.Fail("no LLM providers discovered — set at least one API key")
    }
    return c.Pass(fmt.Sprintf("discovered %d provider(s): %v", len(providers), providers))
}
```

## Relationship to the Challenges Module

HelixQA imports `digital.vasic.challenges` — it never reimplements challenge infrastructure. The `digital.vasic.challenges` module provides:

- `challenge.BaseChallenge` — base struct with `Pass()`, `Fail()`, `Skip()` helpers
- `runner.Runner` — executes challenges with timeout and progress tracking
- `report.Reporter` — generates challenge result reports
- REST API endpoints at `/api/v1/challenges`

See the [Challenges submodule](https://github.com/vasic-digital/Challenges) for the full framework documentation.

## Related Pages

- [Pipeline Phases](/pipeline) — the 4-phase pipeline these challenges verify
- [Open-Source Tools](/advanced/tools) — tools used by execution challenges
- [Architecture](/architecture) — package structure that challenges exercise
