# Helix Nexus — Task Status

One line per task from the execution plan. Update this file in every
merge that touches a task. The executable plan itself lives at
`docs/plans/2026-04-17-helix-nexus-open-clawed-integration-plan.md` in the
main Catalogizer repo.

Legend: `[ ]` pending, `[~]` in progress, `[x]` done, `[-]` deferred.

## Phase 0 — Kickoff

- [x] P0-01 Create `pkg/nexus/` namespace + `doc.go`
- [x] P0-02 go.mod dependencies (chromedp, go-rod, goquery, agouti)
- [x] P0-03 Adapter interface file `pkg/nexus/adapter.go`
- [x] P0-04 Charter at `docs/nexus/charter.md`
- [~] P0-05 Scaffold Nexus SQL migration namespace
- [x] P0-06 5 kickoff challenges `CH-NX-KICKOFF-*`
- [ ] P0-07 Publish Helix Nexus Vision page on helixqa.vasic.digital

## Phase 1 — Browser engine

- [x] P1-01 chromedp_driver.go (build-tagged nexus_chromedp)
- [x] P1-02 rod_driver.go (build-tagged nexus_rod)
- [x] P1-03 engine.go unified interface
- [x] P1-04 snapshot.go role-based refs
- [~] P1-05 actions.go (click, type, scroll, screenshot landed; drag/hover/select/wait_for/tab_open/tab_close/pdf/console_read pending)
- [x] P1-06 errors.go (ToAIFriendlyError)
- [x] P1-07 pool.go warm pool
- [x] P1-08 Security hardening pass (scheme blocks + allowlist + max-body cap + empty-URL refuser, security_test.go)
- [x] P1-09 `CH-NX-BROWSER-*` challenges (15 cases in banks/nexus-browser.{yaml,json})
- [x] P1-10 Integrate as BrowserAdapter in pkg/nexus/userflow (NexusBrowserAdapter satisfies uf.BrowserAdapter)
- [x] P1-11 `docs/nexus/browser.md`
- [ ] P1-12 Video module 01 (content-production task)
- [x] P1-13 `helixqa_browser_sessions` schema in docs/nexus/sql/

## Phase 2 — Mobile engine
Pending.

## Phase 3 — Desktop engine
Pending.

## Phase 4 — AI navigation + self-healing
Pending.

## Phase 5 — Accessibility, performance, cross-platform, enterprise
Pending.
