# CLI Reference

## Commands

### `helixqa autonomous`

Run a full autonomous QA session: learn, plan, execute, explore, analyze, and report.

```bash
helixqa autonomous --project /path/to/project --platforms web --timeout 30m
```

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | `.` | Path to the project root directory |
| `--platforms` | `android,desktop,web` | Comma-separated list of target platforms |
| `--timeout` | `2h` | Maximum total session duration |
| `--output` | `qa-results` | Directory for screenshots, videos, and reports |
| `--curiosity` | `true` | Enable curiosity-driven random exploration |
| `--curiosity-timeout` | `30m` | Time budget for curiosity phase |
| `--coverage-target` | `0.9` | Desired coverage ratio (0.0–1.0) |
| `--report` | `markdown,html,json` | Comma-separated report output formats |
| `--verbose` | `false` | Enable verbose logging |
| `--env` | `.env` | Path to environment file |
| `--dry-run` | `false` | Run learn and plan phases only; skip execution |

#### Platform Values

| Value | Executor Activated |
|-------|-------------------|
| `web` | Playwright (requires `HELIX_WEB_URL`) |
| `android` | ADB Android (requires `HELIX_ANDROID_DEVICE`) |
| `androidtv` | ADB Android TV (requires `HELIX_ANDROID_DEVICE`) |
| `desktop` | X11 / xdotool (requires X11 display) |
| `cli` | stdin/stdout executor |
| `api` | HTTP REST executor (requires `HELIX_API_URL`) |
| `all` | All executors with configured environment |

#### Examples

```bash
# Minimal web run
helixqa autonomous --project . --platforms web --timeout 10m

# Full cross-platform run with curiosity
helixqa autonomous \
  --project /path/to/project \
  --platforms "android,web" \
  --timeout 30m \
  --curiosity=true \
  --curiosity-timeout 5m \
  --output qa-results \
  --report markdown,json

# Quick dry run to preview planned tests
helixqa autonomous \
  --project . \
  --platforms web \
  --timeout 1m \
  --dry-run \
  --verbose
```

---

### `helixqa run`

Execute existing YAML test banks without autonomous learning and planning.

```bash
helixqa run --banks challenges/helixqa-banks/ --platforms web --output qa-results
```

| Flag | Default | Description |
|------|---------|-------------|
| `--banks` | — | Path to directory containing YAML test bank files |
| `--platforms` | `web` | Target platforms |
| `--output` | `qa-results` | Output directory |
| `--filter` | — | Filter test cases by tag or name substring |
| `--verbose` | `false` | Verbose logging |

#### Examples

```bash
# Run all banks against web
helixqa run --banks challenges/helixqa-banks/ --platforms web

# Run only tests tagged "smoke"
helixqa run --banks ./banks/ --platforms android --filter smoke

# Run against multiple platforms
helixqa run --banks ./banks/ --platforms "android,web" --output results
```

---

### `helixqa list`

List test cases available in the specified test banks.

```bash
helixqa list --banks challenges/helixqa-banks/ --platform android
```

| Flag | Default | Description |
|------|---------|-------------|
| `--banks` | — | Path to test bank directory |
| `--platform` | — | Filter by target platform |
| `--json` | `false` | Output as JSON instead of table |

#### Examples

```bash
# List all test cases in table format
helixqa list --banks ./banks/

# List Android tests as JSON
helixqa list --banks ./banks/ --platform android --json
```

---

### `helixqa report`

Generate reports from existing session result directories.

```bash
helixqa report --input qa-results/session-20260327-143022 --format html
```

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | — | Path to session directory or glob pattern |
| `--format` | `markdown` | Output format: `markdown`, `html`, `json` |
| `--output` | same as `--input` | Directory to write generated reports |

#### Examples

```bash
# Generate HTML report from latest session
helixqa report --input qa-results/session-* --format html

# Generate all formats
helixqa report --input qa-results/session-20260327-143022 --format "markdown,html,json"
```

---

### `helixqa version`

Print the HelixQA version, build information, and detected providers.

```bash
helixqa version
```

Sample output:

```
HelixQA v1.0.0 (build 42, 2026-03-27)
Detected providers: anthropic, openrouter, groq
Platforms available: web, android
```

---

## Output Structure

After a session completes, the `--output` directory contains:

```
qa-results/
└── session-20260327-143022/
    ├── pipeline-report.json     # machine-readable session summary
    ├── pipeline-report.md       # human-readable markdown report
    ├── pipeline-report.html     # HTML report
    ├── screenshots/
    │   ├── test-001-login-step-1.png
    │   ├── test-001-login-step-2.png
    │   └── ...
    └── videos/
        ├── test-001-login.mp4
        └── ...
```

Issue tickets are written to `docs/issues/` in the project root:

```
docs/issues/
├── HELIX-001-login-button-not-visible.md
├── HELIX-002-contrast-ratio-too-low.md
└── ...
```

## Related Pages

- [Configuration](/manual/config) — environment variables and `.env` files
- [Memory System](/manual/memory) — understanding session persistence
- [Issue Tickets](/manual/tickets) — ticket format and lifecycle
- [Multi-Pass QA](/manual/multi-pass) — running successive sessions
