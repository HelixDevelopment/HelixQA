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
