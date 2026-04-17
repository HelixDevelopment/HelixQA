---
title: Helix Nexus — Charter
date: 2026-04-17
status: approved
program: helix-nexus
---

# Helix Nexus — Charter

## 1. Mission

Make HelixQA bleeding-edge enterprise-grade at controlling and navigating
application UIs and UX flows across web, mobile (Android phone, Android TV,
iOS) and desktop (Windows, macOS, Linux), driven autonomously by LLMs,
with zero-shot test generation, self-healing selectors, cross-platform
orchestration, accessibility and performance verification, and real-time
operator-facing observability.

## 2. Scope (in)

- CDP-native Go browser automation via `chromedp` and `go-rod`, patterned
  after OpenClaw's `browser-tool.ts`.
- Appium 2.0 WebDriver client in Go for iOS + Android + Android TV.
- Windows (WinAppDriver), macOS (XCUITest + AppleScript / osascript),
  Linux (AT-SPI over DBus, X11 fallback, Wayland detection).
- AI navigation + self-healing layer reusing the existing `LLMOrchestrator`
  and `LLMProvider` submodules and their providers.
- Accessibility auditing via vendored axe-core.
- Performance validation via `k6` browser and in-process Core Web Vitals.
- Cross-platform orchestration with shared `ExecutionContext`.
- Evidence vault (file-store + S3/MinIO), OpenTelemetry, Grafana dashboard.

## 3. Scope (out)

- Re-implementing a browser or an automation protocol from scratch.
- Running full Playwright (Node.js) inside Go. Playwright remains an
  adapter option for cross-browser coverage (Firefox, WebKit), but is not
  required for primary flows.
- Shipping a cloud offering. Helix Nexus targets on-prem + developer
  workstations.
- Bringing the entire OpenClaw monorepo into the repository. We absorb
  patterns and rebuild the 725-line `browser-tool.ts` as Go code.
- Deprecating existing adapters until two consecutive full QA campaigns
  pass on Nexus alone.

## 4. Non-negotiable constraints

- Every task ships with tests across all ten constitution categories
  (unit, integration, E2E, full automation, stress, security, DDoS,
  benchmark, challenges, HelixQA banks).
- No sudo, no root, no privileged container flags anywhere in Helix Nexus.
- All API-side surfaces (gRPC, HTTP) run HTTP/3 + Brotli where applicable
  (matches catalog-api constraint).
- Every feature carries a user guide, operator manual, architecture
  diagram, API reference, CHANGELOG entry, video-course module and
  website section. Code without docs does not merge.
- LLM calls are cost-tracked and budget-capped. A budget breach is a hard
  abort, not a warning.
- Browser pools, device pools, and the evidence vault fit inside the 4
  CPU / 8 GB host resource budget defined in the project CLAUDE.md.

## 5. Success metrics (program-level)

| Metric | Target |
|---|---|
| Snapshot → action round-trip (local workstation) | p50 < 250 ms, p95 < 500 ms |
| Cover-quality gate passes on every Catalogizer screen | 100% |
| WCAG 2.2 AA violations on every shipped public Catalogizer screen | 0 |
| Autonomous scenario success (no human keystroke) | ≥ 90% |
| Flake predictor false-positive rate | < 2% |
| Test categories green per feature at merge | 10 / 10 |

## 6. Roles

| Role | Owner |
|---|---|
| Program lead | HelixQA core |
| Browser engine | HelixQA core |
| Mobile engine | HelixQA core |
| Desktop engine | HelixQA core + SRE for device farm |
| AI navigation | HelixQA core + LLMOrchestrator maintainers |
| Accessibility + performance | Compliance + Performance guilds |
| Cross-platform orchestration | HelixQA core |
| Evidence + observability | SRE |
| Documentation + video course | Content guild |
| Website | Web guild |

## 7. Reference

- Research source: `docs/research/open-clawed/Open-Clawed.md`
- Execution plan:
  `docs/plans/2026-04-17-helix-nexus-open-clawed-integration-plan.md`
- Status tracker (one line per task):
  `docs/nexus/status.md` (to be created by Phase 0 deliverable)
