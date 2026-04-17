---
title: Helix Nexus — Architecture
---

# Architecture

```
+--------------------------------------------------------------------------+
|                    Layer 1 — AI orchestration                            |
|  test-generator | visual-analyzer | llm-navigator | predictive-healing   |
+--------------------------------------------------------------------------+
|                Layer 2 — unified automation API (Go)                     |
|     Browser  |  Mobile  |  Desktop  |  API / gRPC / WebSocket            |
+--------------------------------------------------------------------------+
|                  Layer 3 — platform drivers                              |
|  chromedp + go-rod  |  Appium (UiAutomator2 / XCUITest)  |  WinAppDriver  |
|  XCUITest + osascript  |  AT-SPI / Wayland  |  k6 browser  |  axe-core   |
+--------------------------------------------------------------------------+
|         Layer 4 — evidence, observability, compliance                    |
|  Grafana dashboard  |  OTel tracer  |  S3/MinIO evidence vault          |
|  WCAG / Section 508 reports  |  SLA / SLO tracking                       |
+--------------------------------------------------------------------------+
```

## Design principles

- **Pure Go by default**. CGo-free, no native SDK required for the
  default build.
- **Build tags for heavy deps**. Chromedp and go-rod are opt-in.
- **Injection over inheritance**. Every driver takes a command-runner
  or httptest.Server so unit tests never touch real hardware.
- **Security first**. URL allowlists, scheme blocks, no inline-script
  execution, SSRF guards on LLM-returned URLs.
- **Budget first**. Every LLM call reserves against a `CostTracker`
  and aborts if the reservation would breach the session budget.
- **Observability everywhere**. `observability.Instrument` wraps hot
  paths; the in-memory tracer doubles as a test fixture.

## Package map

| Package | Role |
|---|---|
| `pkg/nexus` | Adapter, Session, Action, Snapshot, Element, Rect, SessionOptions |
| `pkg/nexus/browser` | Engine + chromedp/rod drivers + Pool + Snapshot parser + security |
| `pkg/nexus/userflow` | `NexusBrowserAdapter` bridges to `digital.vasic.challenges/pkg/userflow` |
| `pkg/nexus/mobile` | Appium client + capability builders + gestures + accessibility tree |
| `pkg/nexus/desktop` | Windows / macOS / Linux drivers behind a shared Engine interface |
| `pkg/nexus/ai` | Navigator, Healer, Generator, Predictor, CostTracker |
| `pkg/nexus/a11y` | axe-core Report + Assert + Section 508 filter + InjectionScript |
| `pkg/nexus/perf` | Metrics envelope + ParseK6JSON + GenerateScript |
| `pkg/nexus/orchestrator` | Flow + Step + ExecutionContext + EvidenceStore + RBAC + AuditLog |
| `pkg/nexus/observability` | Tracer + Span + InMemoryTracer + NoopTracer + Instrument |

## Links

- Plan: `docs/plans/2026-04-17-helix-nexus-open-clawed-integration-plan.md` (main repo).
- Charter: `docs/nexus/charter.md`.
- Status tracker: `docs/nexus/status.md`.
