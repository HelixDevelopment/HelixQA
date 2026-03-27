# Introduction

## What is HelixQA?

HelixQA is an autonomous, fire-and-forget QA system that replaces manual testing workflows. Point it at your project, set a timeout, and walk away. The robot learns your codebase, generates test cases, executes them across all target platforms, records video evidence, detects crashes, analyzes screenshots with AI vision, and files detailed issue tickets — all without human intervention.

### The Problem with Manual QA

Modern software ships on multiple platforms simultaneously: web, Android, Android TV, desktop, REST API. Each platform requires dedicated test effort. As teams grow and release velocity increases, manual QA becomes the bottleneck:

- Testing is repetitive and error-prone under time pressure
- Coverage is inconsistent across platforms and releases
- Evidence collection (screenshots, video) is time-consuming
- Regression detection requires running the same tests repeatedly
- QA engineers spend time on execution rather than strategy

### The HelixQA Solution

HelixQA automates the entire QA execution pipeline. A single command launches a session that:

1. **Learns** your project from documentation, source code, and git history
2. **Plans** test cases using an LLM informed by your actual codebase
3. **Executes** tests with video recording, screenshots, and crash detection
4. **Explores** additional paths through curiosity-driven random navigation
5. **Analyzes** every screenshot with LLM vision for UI/UX defects
6. **Reports** all findings as structured issue tickets ready for developer action

## Key Capabilities

### Multi-Platform Execution

| Platform | Executor | Recording | Screenshots |
|----------|----------|-----------|-------------|
| Web | Playwright | Playwright video | Playwright screenshot |
| Android | ADB | scrcpy / screenrecord | adb screencap |
| Android TV | ADB | screenrecord | adb screencap |
| Desktop | X11 / xdotool | ffmpeg x11grab | ImageMagick import |
| CLI / TUI | stdin/stdout | — | — |
| REST API | HTTP client | — | — |

### 40+ LLM Providers

HelixQA auto-discovers available providers by scanning environment variables at startup. Supported providers span commercial, open-source, and self-hosted:

- **Commercial cloud**: Anthropic, OpenAI, OpenRouter, DeepSeek, Groq, Mistral, xAI
- **Specialized inference**: Cerebras, Fireworks, NVIDIA NIM, SambaNova, Together AI
- **Self-hosted**: Ollama (any model)
- **And 30+ more** — see [LLM Providers](/providers) for the full list

### Photographic Memory

A SQLite-backed memory store persists all session data across runs. Each new session has full awareness of prior sessions: which screens were tested, which issues were found, which areas lack coverage, and what fixes have been verified. This enables [Multi-Pass QA](/manual/multi-pass) where each pass builds on the previous.

### Evidence-Based Issue Tickets

Every finding is filed as a structured markdown ticket in `docs/issues/HELIX-NNN.md` with YAML frontmatter for machine-readable metadata, reproduction steps, and links to supporting screenshots and video recordings.

## Design Philosophy

**Fire and forget.** Once launched, HelixQA requires no human input during execution. The only interaction is reviewing the generated tickets and fixing the reported issues.

**Evidence over opinion.** Every ticket includes concrete evidence: the exact screenshot showing the defect, the performance metrics showing the leak, the video recording showing the crash. LLM analysis findings are grounded in visual evidence, not inference.

**Multi-pass accumulation.** A single QA pass is a starting point. Successive passes cover more ground, re-verify previous fixes, and explore new areas discovered through curiosity navigation. Coverage accumulates toward a configurable target.

**Zero lock-in.** HelixQA uses open interfaces: standard ADB for Android, Playwright for web, standard ffmpeg for video. The LLM provider is swappable at any time by setting a different environment variable.

## Next Steps

- [Installation](/installation) — build from source or use the container image
- [Quick Start](/quick-start) — run your first autonomous session in 5 minutes
- [Architecture](/architecture) — understand the 4-phase pipeline in depth
- [LLM Providers](/providers) — choose and configure your AI backend
