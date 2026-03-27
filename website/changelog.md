# Changelog

All notable changes to HelixQA are documented in this file. Releases follow [Semantic Versioning](https://semver.org/).

## v0.9.0 - 2026-03-27

### Security Scanning Infrastructure

- Security scanning infrastructure is now fully operational across all packages
- Integrated govulncheck for Go standard library and dependency vulnerability scanning
- Added Semgrep static analysis for security anti-pattern detection
- Container image scanning via Trivy for base image vulnerabilities
- All 24 packages pass security scans with zero critical or high findings

### Safety Hardening

- Goroutine lifecycle management: all spawned goroutines now respect context cancellation and have explicit shutdown paths
- Context timeouts enforced on every LLM API call, preventing indefinite hangs when a provider is unresponsive
- Added `BoundedSemaphore` for concurrency control across parallel test execution, preventing resource exhaustion on constrained hosts
- Lazy provider initialization: LLM providers are initialized on first use rather than at startup, reducing memory footprint and startup time for sessions that only use a subset of configured providers
- Memory store connections use WAL mode with busy timeout to prevent database lock contention during concurrent read/write operations
- All HTTP clients configured with connection pool limits, idle timeouts, and retry logic with exponential backoff

### Test Bank Expansion

- Added 220 new test bank cases, bringing the total to 517 cases across all platforms
- New cases cover: accessibility compliance (WCAG 2.1 AA), performance thresholds, error state handling, network failure resilience, and multi-language support
- Test bank cases organized by platform: 156 web, 138 Android, 94 API, 72 desktop, 57 cross-platform
- Added tag-based filtering for selective test bank execution (`--tags smoke`, `--tags a11y,perf`)

### Video Course

- Added 6 new video course modules (Modules 7-12), bringing the course to 12 modules and approximately 4 hours of content
- Module 7: LLM Vision Analysis -- prompt engineering for defect detection, video frame analysis, false positive management
- Module 8: Photographic Memory -- SQLite schema, issue lifecycle, multi-pass intelligence
- Module 9: Issue Ticket System -- ticket format, FindingsBridge pipeline, external tracker integration
- Module 10: Containerised Deployment -- Dockerfile, Podman Compose, Kubernetes Job and CronJob
- Module 11: Advanced Configuration -- curiosity exploration, custom test banks, provider optimization
- Module 12: Real-World Case Study -- Catalogizer QA with 558 test steps across 7 components

### Website

- Added 5 new website content pages: features, getting-started, documentation, faq, support
- Expanded existing pages (index, changelog, download) with comprehensive content
- Documentation page now serves as a complete table of contents linking all guides, references, and manuals
- FAQ covers autonomous QA, test banks, LLM providers, performance, and troubleshooting
- Support page includes troubleshooting guide, log locations, debug mode, and issue reporting

### Bridge Packages

- Added Go bridge packages for scrcpy, Appium, Allure, and Perfetto integration
- scrcpy bridge: manages video recording lifecycle, device connection, and frame extraction
- Appium bridge: provides an alternative Android executor for teams with existing Appium infrastructure
- Allure bridge: exports session results in Allure-compatible format for integration with existing reporting dashboards
- Perfetto bridge: manages trace collection on Android devices with configurable trace categories

### Cognitive Memory

- Added CognitiveMemory layer with optional provider interface for advanced session-to-session knowledge transfer
- Wired CognitiveMemory into the pipeline learning phase for improved test generation based on accumulated project understanding
- Memory-backed coverage tracking identifies under-tested screens and prioritizes them in subsequent passes

### Other Changes

- Updated Go module to require Go 1.24+
- All 883+ tests pass with zero race conditions (`-race` flag)
- 558/558 HelixQA native test steps passing
- 30 self-validation challenges all passing
- Reduced container image size by 18% through multi-stage build optimization

## v0.8.0 - 2026-02-15

### Pipeline

- Initial implementation of the 4-phase autonomous pipeline (Learn, Plan, Execute, Analyze)
- Pipeline orchestrator with phase transitions, timeout enforcement, and graceful shutdown
- Session timeline recording with millisecond-precision event logging

### LLM Providers

- Support for 40+ LLM providers via environment variable auto-discovery
- Adaptive provider with per-request model selection (fast models for planning, vision models for analysis)
- Provider failover chain with automatic retry on error or timeout

### Platform Executors

- Android executor via ADB with scrcpy video recording and logcat crash detection
- Web executor via Playwright with console error and network failure capture
- Desktop executor via xdotool and X11 with ffmpeg video recording
- CLI executor for terminal application testing
- API executor for REST endpoint validation

### Memory

- SQLite-backed persistent memory store with 7-table schema
- Session, test result, finding, screenshot, metric, knowledge, and coverage tables
- Multi-pass awareness: each session has full access to all prior session data

### Evidence Collection

- Automatic screenshot capture at every test step
- Video recording per test case (platform-appropriate method)
- Performance metric sampling (memory, CPU) at configurable intervals
- Log collection and filtering (logcat, browser console)

### Analysis

- LLM vision analysis of every captured screenshot
- Six analysis categories: visual, ux, accessibility, brand, content, performance
- Memory leak detection via monotonic heap growth analysis
- Real-time crash and ANR detection during execution

### Issue Tickets

- Automated ticket creation in `docs/issues/HELIX-NNN.md`
- YAML frontmatter with machine-readable metadata
- Ticket lifecycle: open, fixed, verified, reopened, wontfix, false_positive
- Finding deduplication across sessions

### Test Banks

- YAML test bank format with platform, category, priority, and tag fields
- Test bank reconciliation with LLM-generated tests during planning
- 297 initial test bank cases

### Reporting

- JSON session report with test counts, finding summaries, coverage, and phase durations
- LLM token usage tracking and cost estimation
- Timeline export for post-session analysis

## v0.7.0 - 2026-01-10

### Initial Release

- Core framework with challenge integration from `digital.vasic.challenges`
- Container integration from `digital.vasic.containers`
- Basic crash detection for Android (logcat) and Web (console errors)
- Step-by-step validation with evidence collection
- Markdown ticket generation
- CLI entry point with `run`, `list`, `report`, and `version` subcommands
- 21 Go packages, 500+ tests
