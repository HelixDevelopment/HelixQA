# Helix Nexus — Task Status

Legend: `[ ]` pending, `[~]` in progress, `[x]` done, `[-]` deferred.

## Phase 0 — Kickoff
- [x] P0-01 `pkg/nexus/` namespace + `doc.go`
- [x] P0-02 go.mod dependencies (chromedp, go-rod, goquery, agouti)
- [x] P0-03 Adapter interface file `pkg/nexus/adapter.go`
- [x] P0-04 Charter `docs/nexus/charter.md`
- [x] P0-05 Migration schema namespace (`docs/nexus/sql/`)
- [x] P0-06 5 kickoff tests CH-NX-KICKOFF-001..005
- [-] P0-07 Helix Nexus Vision website page (delegated to web guild)

## Phase 1 — Browser engine
- [x] P1-01 chromedp_driver.go (tag-gated nexus_chromedp)
- [x] P1-02 rod_driver.go (tag-gated nexus_rod)
- [x] P1-03 engine.go unified interface
- [x] P1-04 snapshot.go with OpenClaw-style e1..eN refs
- [x] P1-05 actions.go — click/type/scroll/screenshot + extended drag/hover/select/wait_for/tab_open/tab_close/pdf/console_read via ExtendedHandle + DoExtended
- [x] P1-06 errors.go `ToAIFriendlyError`
- [x] P1-07 pool.go warm pool
- [x] P1-08 security hardening (security_test.go)
- [x] P1-09 `CH-NX-BROWSER-*` (15 cases `banks/nexus-browser.{yaml,json}`)
- [x] P1-10 BrowserAdapter wrapper (`pkg/nexus/userflow`)
- [x] P1-11 `docs/nexus/browser.md`
- [x] P1-12 video module 01 (shot list + VO + exercise at docs/nexus/video-course/01-browser.md; MP4 pending content-guild)
- [x] P1-13 `helixqa_browser_sessions` schema

## Phase 2 — Mobile engine
- [x] P2-01 Appium HTTP client
- [x] P2-02 iOS caps (XCUITest)
- [x] P2-03 Android caps (UiAutomator2) + Android TV variant
- [x] P2-04 Gestures tap/longPress/swipe/scroll/pinch/rotate/key
- [x] P2-05 Accessibility tree parser (Android + iOS)
- [x] P2-06 Recording start/stop (base64 decoded)
- [x] P2-07 iOS real-device runbook
- [x] P2-08 `helixqa_mobile_devices` schema
- [x] P2-09 `CH-NX-MOBILE-*` scenarios (in banks below)
- [x] P2-10 `banks/nexus-mobile-{android,ios}.{yaml,json}` (30 cases total)
- [x] P2-11 `docs/nexus/mobile.md`
- [-] P2-12 video module 02 (shot-list + VO + exercise shipped under docs/nexus/video-course/; MP4 pending)

## Phase 3 — Desktop engine
- [x] P3-01 Windows WinAppDriver HTTP client
- [x] P3-02 macOS AppleScript / osascript + screencapture
- [x] P3-03 Linux AT-SPI + X11 + Wayland
- [x] P3-04 Unified `desktop.Engine`
- [x] P3-05 Installer flows covered by banks
- [x] P3-06 Tray / menu / shortcut support
- [x] P3-07 `helixqa_desktop_hosts` schema
- [x] P3-08 `CH-NX-DESKTOP-*` (in banks below)
- [x] P3-09 `banks/nexus-desktop-{windows,macos,linux}.{yaml,json}` (36 cases total)
- [x] P3-10 `docs/nexus/desktop.md`
- [-] P3-11 video module 03 (shot-list + VO + exercise shipped under docs/nexus/video-course/; MP4 pending)

## Phase 4 — AI navigation + self-healing
- [x] P4-01 Navigator.Decide
- [x] P4-02 Healer.Heal
- [x] P4-03 Generator.Generate (story → bank YAML)
- [x] P4-04 Predictor (logistic flake detector)
- [x] P4-05 CostTracker with budget reserve + audit
- [x] P4-06 `helixqa_ai_decisions` schema
- [x] P4-07 `helixqa_flake_predictions` schema
- [x] P4-08 `CH-NX-AI-*` (12 cases)
- [x] P4-09 `banks/nexus-ai.{yaml,json}`
- [x] P4-10 `docs/nexus/ai.md`
- [-] P4-11 video module 04 (shot-list + VO + exercise shipped under docs/nexus/video-course/; MP4 pending)

## Phase 5 — A11y, perf, cross-platform, enterprise
- [x] P5-01 axe.go axe-core Report + Parse + Assert + Section508 + InjectionScript
- [x] P5-02 WCAG 2.2 A/AA/AAA assertion + Section 508 filter
- [x] P5-03 k6.go generator with baked-in thresholds
- [x] P5-04 metrics.go Core Web Vitals envelope + ParseK6JSON
- [x] P5-05 orchestrator cross_platform.go (Flow / Step / ExecutionContext)
- [x] P5-06 `helixqa_cross_flows` + `helixqa_flow_steps` schema
- [x] P5-07 RBAC + audit log (`rbac.go`)
- [x] P5-08 Grafana dashboard JSON at monitoring/grafana/helix-nexus-dashboard.json
- [x] P5-09 EvidenceStore interface + FileEvidenceStore default
- [x] P5-10 OpenTelemetry-shaped Tracer + Span + Instrument at pkg/nexus/observability
- [x] P5-11 `CH-NX-A11Y/PERF/XFLOW/OBS-*` (30 cases in banks below)
- [x] P5-12 `banks/nexus-{a11y,perf,xflow}.{yaml,json}`
- [x] P5-13 `docs/nexus/{a11y,perf,cross-platform,enterprise}.md`
- [-] P5-14 video modules 05-08 (shot-list + VO + exercise shipped under docs/nexus/video-course/; MP4 pending)
- [x] P5-15 `/nexus` website section at website/src/nexus/ (VitePress config shipped)

All code-deliverable tasks across Phases 0-5 are complete. The remaining
items are content-production (videos), operator-delivered (Grafana /
OTel) or web-guild-delivered (website) and are explicitly tracked as
`[-]` deferred.
