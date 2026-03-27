# Frequently Asked Questions

## General

### What is HelixQA?

HelixQA is an autonomous, fire-and-forget QA system that replaces manual testing workflows. Point it at your project, set a timeout, and walk away. It learns your codebase, generates test cases using LLMs, executes them across all target platforms, records video evidence, detects crashes, analyzes screenshots with AI vision, and files detailed issue tickets -- all without human intervention.

### What platforms does HelixQA support?

HelixQA supports Android (phones and tablets), Android TV, Web (via Playwright), Desktop (X11-based applications including Tauri and Electron), CLI/TUI applications, and REST APIs. Each platform has a dedicated executor that uses native tooling for maximum reliability.

### Does HelixQA replace human QA engineers?

HelixQA replaces the repetitive execution portion of QA work: running the same tests across platforms, capturing screenshots, checking for crashes, and filing tickets. Human QA engineers remain essential for test strategy, exploratory thinking, usability judgment, and prioritization of findings. HelixQA handles execution so humans can focus on strategy.

### How is HelixQA different from Selenium, Appium, or Playwright alone?

Traditional test frameworks require you to write and maintain test scripts. HelixQA generates test cases automatically by reading your project documentation, source code, and git history. It also provides LLM vision analysis of every screenshot, persistent memory across sessions, automatic issue ticket creation, and multi-platform execution from a single command. You can still use traditional test banks alongside the LLM-generated tests.

## Autonomous QA

### How does curiosity mode work?

After all planned tests complete, curiosity mode takes over and performs random exploration of the application. It identifies interactive elements on the current screen (buttons, list items, navigation icons), randomly taps one, waits for the screen to settle, and captures a screenshot. If a new screen is discovered (not seen in any prior session), it is added to the knowledge base for future structured testing.

Curiosity mode has several safety constraints:
- It respects the `--curiosity-timeout` budget and stops when time expires
- It avoids destructive actions by filtering button labels containing words like "delete", "remove", "logout", and "uninstall"
- Crashes during curiosity exploration are recorded with the full navigation path for reproduction
- It runs after planned tests to avoid consuming the main session timeout

Enable it with:

```bash
helixqa autonomous \
  --project . \
  --platforms android \
  --curiosity=true \
  --curiosity-timeout 5m
```

### How does multi-pass QA work?

Each autonomous session (pass) stores its complete results in a SQLite memory database. When the next session starts, the learning phase reads all prior session data: which screens were tested, which issues were found, which areas have low coverage, and which fixes have been verified.

The planner uses this history to generate different, complementary test cases for each pass:

- **Pass 1**: Broad coverage of major screens and critical user paths
- **Pass 2**: Deeper testing in areas where issues were found, plus coverage gaps
- **Pass 3**: Edge cases, error states, and exploratory paths
- **Pass N**: Regression verification of fixed issues, new feature coverage

Coverage accumulates across passes toward a configurable target (default 80%). Run multiple passes with:

```bash
# Pass 1
helixqa autonomous --project . --platforms web --timeout 10m

# Pass 2 (automatically builds on Pass 1)
helixqa autonomous --project . --platforms web --timeout 10m

# Continue until coverage target is met
```

### How does HelixQA learn about my project?

During the learning phase, HelixQA reads:

- `CLAUDE.md` or `README.md` at the project root for architecture constraints and tech stack
- All `.md` files under `docs/` for feature descriptions and API documentation
- Go source files for Gin route handlers and service boundaries
- React/TypeScript source files for page components and router paths
- Kotlin/Compose source files for navigation graphs and screen names
- Git history for recent change hotspots and frequently modified files
- The SQLite memory database for prior session results, known issues, and coverage gaps

The more documentation your project has, the better HelixQA's test generation will be. A well-structured `CLAUDE.md` with architecture overview, screen descriptions, and known constraints significantly improves test quality.

### Can I control which screens or features get tested?

Yes, through several mechanisms:

- **Test banks**: Write specific test cases in YAML that target exact screens and features
- **Tags**: Use `--tags` to filter test bank cases by tag (e.g., `--tags smoke,auth`)
- **Platform filtering**: Use `--platforms` to restrict testing to specific platforms
- **Priority**: The planner prioritizes recently changed code, so features in active development get tested first
- **Documentation hints**: Add test hints in your project documentation that the learning phase picks up

## Test Banks

### How do I write custom test banks?

Test banks are YAML files placed in `challenges/helixqa-banks/`. Each file contains a bank header and a list of test cases:

```yaml
bank:
  name: my-custom-tests
  version: "1.0"
  description: Custom tests for my application

cases:
  - id: CUSTOM-001
    title: "Login with valid credentials"
    platform: web
    category: functional
    priority: critical
    tags: [auth, login, smoke]
    steps:
      - action: navigate
        url: "${HELIX_WEB_URL}/login"
      - action: fill
        selector: "#email"
        value: "test@example.com"
      - action: fill
        selector: "#password"
        value: "password123"
      - action: click
        selector: "#login-button"
      - action: wait_for_selector
        selector: "[data-testid='dashboard']"
        timeout: 10s
      - action: screenshot
        name: "login-success"
    expected:
      - url_contains: "/dashboard"
      - no_console_errors: true
```

See [Test Bank Schema](/reference/test-bank-schema) for the complete field reference.

### How are test bank cases different from LLM-generated tests?

Test bank cases are deterministic: they execute the same steps every time and are ideal for smoke tests, regression tests, and compliance checks that must run identically in every session. LLM-generated tests are dynamic: they change based on the project's current state, coverage gaps, and prior findings.

During the planning phase, the reconciler merges both sources and removes duplicates. Test bank cases always execute; LLM-generated tests fill in the remaining coverage gaps.

### How many test bank cases does HelixQA include?

The current release includes 517 test bank cases distributed across platforms:

| Platform | Cases |
|----------|-------|
| Web | 156 |
| Android | 138 |
| REST API | 94 |
| Desktop | 72 |
| Cross-platform | 57 |

These cover functional testing, accessibility compliance (WCAG 2.1 AA), performance thresholds, error state handling, network failure resilience, and multi-language support.

### Can I run only test bank cases without LLM generation?

Yes. Use the `run` command instead of `autonomous`:

```bash
helixqa run \
  --bank challenges/helixqa-banks/smoke.yaml \
  --platforms web \
  --timeout 5m
```

This skips the learning and planning phases entirely and executes only the specified test bank cases.

## LLM Providers

### Which LLM providers are supported?

HelixQA supports 40+ providers. The major categories:

| Category | Providers |
|----------|----------|
| Commercial cloud | Anthropic (Claude), OpenAI (GPT-4), Google (Gemini), DeepSeek, Mistral, xAI (Grok) |
| Inference platforms | OpenRouter, Groq, Cerebras, Fireworks, Together AI, SambaNova |
| Specialized | NVIDIA NIM, Cohere, AI21 Labs, Perplexity |
| Self-hosted | Ollama (any model), vLLM, LocalAI |

See [LLM Providers](/providers) for the complete list with environment variables and default models.

### How does provider auto-discovery work?

At startup, HelixQA scans all environment variables for patterns matching known provider API key formats (e.g., `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`). Every detected key is registered as an available provider. No configuration file is needed -- just set the environment variable.

For Ollama, set `HELIX_OLLAMA_URL` to the Ollama server address (default: `http://localhost:11434`). No API key is required.

### Which provider should I use?

It depends on your priorities:

| Priority | Recommended Provider | Why |
|----------|---------------------|-----|
| Best quality | Anthropic (Claude) | Strongest vision analysis and code understanding |
| Lowest cost | DeepSeek | Very competitive pricing for planning and analysis |
| Fastest speed | Groq | Hardware-optimized inference with sub-second latency |
| Most models | OpenRouter | Access to 100+ models through a single API key |
| Full privacy | Ollama | Runs entirely on your hardware, no data leaves your network |

### Can I use multiple providers simultaneously?

Yes. Set multiple API keys and the adaptive provider will select the best available model for each request type: fast models for planning, vision-capable models for screenshot analysis, and cost-effective models for ticket generation. If one provider fails, the next in the fallback chain is tried automatically.

### How much does a session cost in LLM API fees?

Cost varies by provider, model, and session length. Typical ranges for a 10-minute web session:

| Provider | Approximate Cost |
|----------|-----------------|
| DeepSeek | $0.02 - $0.10 |
| OpenRouter (budget models) | $0.05 - $0.20 |
| Groq | $0.05 - $0.15 |
| OpenAI (GPT-4o) | $0.20 - $0.80 |
| Anthropic (Claude Sonnet) | $0.15 - $0.60 |

The session report includes exact token usage and estimated cost for the session.

## Performance

### How many concurrent tests can run?

HelixQA runs tests sequentially within a single session to ensure reliable evidence collection and crash detection. Concurrent test execution across multiple devices or browsers is supported through multiple simultaneous sessions:

```bash
# Session 1: Android phone
helixqa autonomous --project . --platforms android --timeout 10m &

# Session 2: Web browser
helixqa autonomous --project . --platforms web --timeout 10m &

# Session 3: Android TV
helixqa autonomous --project . --platforms androidtv --timeout 10m &
```

Each session maintains its own evidence directory and writes to the shared memory database using WAL mode for safe concurrent access.

### What are the resource requirements?

| Component | CPU | Memory | Disk |
|-----------|-----|--------|------|
| HelixQA binary | 1-2 cores | 512 MB - 1 GB | Minimal |
| Playwright browser | 1-2 cores | 1-2 GB | 500 MB |
| ADB + scrcpy | < 1 core | 256 MB | Minimal |
| ffmpeg recording | 1 core | 256 MB | Varies with duration |
| SQLite memory store | Minimal | Proportional to history | 10-100 MB |

For containerized execution, recommended limits are `--cpus=2 --memory=4g`.

### How long does a typical session take?

Session duration depends on the number of tests generated and the target platform:

| Session Type | Typical Duration |
|-------------|-----------------|
| Quick smoke test (10 cases) | 2-5 minutes |
| Standard autonomous pass (30 cases) | 10-20 minutes |
| Comprehensive pass with curiosity (50+ cases) | 20-40 minutes |
| Multi-platform full suite | 30-60 minutes |

Use `--timeout` to set a hard upper limit. HelixQA will complete the current test and run analysis when the timeout is reached.

### Does HelixQA slow down my application?

HelixQA interacts with your application the same way a human user would: through the UI (taps, clicks, keyboard input) or HTTP requests. It does not instrument your application code or inject any agents. The only additional load comes from screenshot capture and video recording, which have negligible impact on modern hardware.

On Android, scrcpy uses hardware video encoding on the device itself, adding minimal CPU overhead. Playwright's screenshot and video capture is built into the browser automation layer and does not affect page performance.

## Troubleshooting

### My session produces no findings -- is that normal?

Yes, if your application has no detectable issues. Check the session report (`pipeline-report.json`) to confirm:
- Tests were actually executed (check `tests_executed` count)
- Screenshots were captured (check `screenshots_captured` count)
- The analysis phase ran (check `phase_durations.analyze`)

If tests executed but no findings were generated, your application passed all checks. This is the desired outcome.

### The LLM generates irrelevant test cases. How do I improve it?

Improve the quality of your project documentation:
- Add a detailed `CLAUDE.md` or `README.md` describing your application's screens, features, and known constraints
- Put feature documentation in `docs/` where HelixQA can find it
- Include screen names, route paths, and key UI elements in your documentation
- Add test hints like "The login page requires email and password fields"

### Why is vision analysis slow?

Vision analysis sends each screenshot to the LLM API, which involves network latency and model inference time. If you have many screenshots, this phase can take several minutes. Strategies to speed it up:
- Use a faster provider (Groq, Cerebras) for vision analysis
- Reduce screenshot count by increasing the step interval
- Use `--skip-analysis` to skip vision analysis entirely (useful for debugging execution issues)

### Can I use HelixQA behind a corporate proxy?

Yes. Set the standard proxy environment variables:

```bash
export HTTP_PROXY="http://proxy.corp.com:8080"
export HTTPS_PROXY="http://proxy.corp.com:8080"
export NO_PROXY="localhost,127.0.0.1"
```

These are respected by both the Go HTTP client (for LLM API calls) and Playwright (for web testing).
