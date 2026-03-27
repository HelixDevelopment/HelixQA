---
layout: home

hero:
  name: HelixQA
  text: Autonomous QA Robot
  tagline: Fire-and-forget quality assurance that replaces human QA teams
  actions:
    - theme: brand
      text: Quick Start
      link: /quick-start
    - theme: alt
      text: Architecture
      link: /architecture

features:
  - title: Fire and Forget
    details: Point it at your project, walk away. HelixQA learns your codebase, generates tests, executes them, and creates issue tickets — all autonomously.
  - title: 40+ LLM Providers
    details: Anthropic, OpenAI, OpenRouter, DeepSeek, Groq, Ollama, and 35+ more. Auto-discovers available providers from environment variables.
  - title: Multi-Platform
    details: Android, Android TV, Web, Desktop, CLI, and REST API testing. Each platform has a dedicated executor with native tooling.
  - title: AI Vision Analysis
    details: Every screenshot analyzed by multimodal LLM for visual defects, UX issues, accessibility problems, and brand compliance.
  - title: Photographic Memory
    details: SQLite-backed persistent memory across sessions. Each pass builds on previous knowledge — coverage tracking, regression detection, issue lifecycle.
  - title: Video Evidence
    details: Automated video recording via scrcpy/screenrecord, screenshot capture at every step, logcat collection, performance metrics timeline.
---

## How It Works

```
helixqa autonomous --project /path/to/your/app --platforms all --timeout 1h
```

HelixQA runs a 4-phase pipeline:

1. **Learn** — Reads docs, codebase, git history, prior QA sessions
2. **Plan** — LLM generates comprehensive test cases, reconciles with existing banks
3. **Execute** — Runs tests with video recording, crash detection, performance monitoring
4. **Analyze** — LLM vision examines screenshots, detects memory leaks, creates issue tickets

## Results

Issue tickets automatically created in `docs/issues/HELIX-NNN.md` with YAML frontmatter, reproduction steps, and evidence references.

## Proven at Scale

- 883+ tests across 21 Go packages
- 558/558 HelixQA native test steps passing
- 22 integrated open-source tools
- 30 self-validation challenges
- Successfully QA'd Catalogizer (7 components, 275+ API endpoints)

## By the Numbers

| Metric | Value |
|--------|-------|
| Go packages | 24 |
| Test bank cases | 517 |
| LLM providers | 40+ |
| Supported platforms | 4 (Android, Web, Desktop, API) |
| Go test count | 883+ |
| Self-validation challenges | 30 |
| Integrated open-source tools | 22 |
| Issue ticket format | Markdown + YAML frontmatter |

## Quick Start

Get from zero to first autonomous QA session in three commands:

```bash
# 1. Build
cd HelixQA && go build -o bin/helixqa ./cmd/helixqa

# 2. Set an LLM provider key
export OPENROUTER_API_KEY="sk-or-v1-..."

# 3. Run an autonomous session against your project
helixqa autonomous \
  --project /path/to/your/app \
  --platforms web \
  --timeout 10m
```

HelixQA reads your project documentation, source code, and git history, then generates and executes test cases, captures screenshots and video evidence, analyzes every screenshot with LLM vision, and files structured issue tickets in `docs/issues/HELIX-NNN.md` -- all without human intervention.

```bash
# Review the results
ls docs/issues/HELIX-*.md              # Issue tickets
cat qa-results/session-*/pipeline-report.json  # Session report
ls qa-results/session-*/screenshots/   # Screenshot evidence
```

## Feature Highlights

### Autonomous QA

Point HelixQA at your project and walk away. The robot learns your codebase from documentation, source code, and git history. An LLM generates comprehensive test cases tailored to your actual application. Tests execute across all target platforms with full evidence collection. No test scripts to write. No manual interaction required.

### Real-Time Crash Detection

Continuous monitoring during test execution catches crashes the moment they happen. On Android, logcat is filtered for `FATAL EXCEPTION`, `ANR`, and `Force Close` patterns. On web, browser console errors and failed network requests are captured. On desktop, process exit codes and X11 error events are tracked. Every crash is documented with a full stack trace and reproduction steps.

### Evidence Collection

Every test step produces evidence: a screenshot, a video segment, performance metrics, and device logs. Screenshots are analyzed by LLM vision for visual defects, UX issues, accessibility problems, and brand compliance. Videos are recorded via scrcpy (Android), Playwright (web), or ffmpeg x11grab (desktop). Memory and CPU metrics are tracked to detect leaks and performance regressions.

### Multi-Platform Coverage

A single `helixqa autonomous` command can target Android, Android TV, Web, and Desktop simultaneously. Each platform has a dedicated executor that uses native tooling: ADB for Android, Playwright for web, xdotool and X11 for desktop. Platform-specific concerns (SDK version differences, browser quirks, display configuration) are handled transparently.

### Photographic Memory

A SQLite-backed memory store persists all session data indefinitely. Each new session has full awareness of every prior session: which screens were tested, which issues were found, which areas lack coverage, and which fixes have been verified. Coverage accumulates across passes toward a configurable target. Regressions are automatically detected when previously-fixed issues reappear.

### 40+ LLM Providers

HelixQA auto-discovers available providers by scanning environment variables at startup. Commercial providers (Anthropic, OpenAI, DeepSeek, Groq), specialized inference providers (Cerebras, Fireworks, NVIDIA NIM, SambaNova), and self-hosted solutions (Ollama) are all supported. The adaptive provider selects the best available model for each request type: fast models for planning, vision-capable models for screenshot analysis.

## Supported Platforms

| Platform | Executor | What Gets Tested |
|----------|----------|-----------------|
| Android | ADB + scrcpy | Native apps on phones and tablets |
| Android TV | ADB + screenrecord | TV applications with D-pad navigation |
| Web | Playwright | Single-page apps, dashboards, admin panels |
| Desktop | xdotool + X11 | Tauri, Electron, and native desktop apps |
| CLI | stdin/stdout | Terminal applications and TUI interfaces |
| REST API | HTTP client | API endpoint validation and schema checks |

## Architecture at a Glance

HelixQA is built as 24 Go packages organized around a 4-phase pipeline:

```
Learn  -->  Plan  -->  Execute  -->  Analyze
  |           |           |             |
  v           v           v             v
Project    LLM-driven   Platform     LLM vision
knowledge  test case    executors    analysis +
ingestion  generation   + evidence   ticket creation
```

All phases share a persistent SQLite memory store and a unified evidence collection layer. The LLM provider is abstracted behind an adaptive interface that supports 40+ backends. Platform executors are pluggable, with each executor implementing a common interface for navigation, interaction, and screenshot capture.

See [Architecture](/architecture) for the full system design.
