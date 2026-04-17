---
title: Helix Nexus — Browser Engine
phase: 1
status: in-progress
---

# Helix Nexus — Browser Engine

The browser engine is HelixQA's CDP-native automation layer. It absorbs
OpenClaw's `browser-tool.ts` semantics (role-based element refs,
AI-friendly errors, hardened profile) into a Go package that runs on top
of `chromedp` and `go-rod`.

## Packages

- `pkg/nexus/adapter.go` — cross-platform Adapter / Session / Action / Snapshot types.
- `pkg/nexus/browser/engine.go` — facade Engine that implements `nexus.Adapter` and wraps a `Driver`.
- `pkg/nexus/browser/snapshot.go` — pure-Go ARIA-role snapshot parser with OpenClaw-style `e1`, `e2` refs.
- `pkg/nexus/browser/errors.go` — `ToAIFriendlyError(err) string` translator.
- `pkg/nexus/browser/pool.go` — size-capped `Pool` for concurrent sessions.
- `pkg/nexus/browser/chromedp_driver.go` — real Chromium driver, guarded by `nexus_chromedp` build tag.
- `pkg/nexus/browser/rod_driver.go` — go-rod driver, guarded by `nexus_rod` build tag.

## Why build tags

The default build of HelixQA does **not** depend on a browser binary.
Unit tests on CI-less workstations use a mock `Driver`. Operators enable
the real drivers when they need them:

```sh
# Use chromedp (more debuggable, slower)
go build -tags=nexus_chromedp ./...

# Use go-rod (faster, lower memory)
go build -tags=nexus_rod ./...
```

Both drivers can compile together if the operator passes both tags.

## Element references

`Snapshot.Elements[i].Ref` is a stable OpenClaw-style string (`e1`, `e2`,
...) assigned in document order over interactable nodes. The AI navigator
asks for `click(e5)` rather than a brittle CSS selector; the driver
resolves the ref via the accompanying `Selector` field.

A snapshot must be retaken after any action that mutates the DOM. Refs
are stable **within** a snapshot, not across snapshots.

## Action kinds

Today: `click`, `type`, `scroll`, `screenshot`. Additional kinds land in
P1-05:

- `drag`, `hover`, `select`, `wait_for`, `tab_open`, `tab_close`,
  `pdf`, `console_read`.

Each kind round-trips through `Action`, keeping the navigator API and
the underlying Driver decoupled.

## Security guarantees

- `Engine` rejects `file://`, `javascript:`, `data:`, and `vbscript:`
  targets before they touch the Driver.
- An `AllowedHosts` allowlist (non-empty) caps navigation to the listed
  hosts only. This is the backstop for challenge suites that must not
  accidentally reach the wider internet.
- CDP flags are restricted to `127.0.0.1` on both drivers.
- `MaxBodyBytes` caps downloaded responses (default 32 MiB) so a hostile
  page cannot exhaust host RAM.

## SQL

Table `helixqa_browser_sessions` records one row per `Engine.Open()`:

```sql
CREATE TABLE helixqa_browser_sessions (
    session_id      TEXT PRIMARY KEY,
    engine          TEXT NOT NULL,
    pool_slot       INTEGER,
    started_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at        TIMESTAMP,
    platform        TEXT NOT NULL,
    user_data_dir   TEXT
);
```

The migration ships in Phase 1 task P1-13.

## Migrating from the Playwright adapter

1. Register `browser.NewEngine(driver, cfg)` as a `BrowserAdapter` in
   `pkg/userflow/browser` via a small shim (P1-10).
2. In bank YAML, select the adapter by name, e.g.
   `platform: web-nexus-chromedp`.
3. Existing Playwright banks continue to run untouched.

## Observability

Every call path surfaces:

- `Engine.ActiveSessions()` — current in-flight session count.
- `Pool.Active()` — current pool usage.
- `ToAIFriendlyError(err)` — short, LLM-readable error message for logs.

OpenTelemetry spans and Grafana dashboards arrive in Phase 5 (P5-08,
P5-10); the Engine currently emits structured logs only.
