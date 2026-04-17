---
title: Helix Nexus — Getting Started
---

# Getting Started

Helix Nexus is a Go package family under `digital.vasic.helixqa/pkg/nexus`.
Every sub-package is independently importable; start with the one
matching the surface you test.

## Install

```bash
go get digital.vasic.helixqa/pkg/nexus
```

Optional build tags unlock real browser drivers (they are off by
default so CI-less workstations do not need a Chromium binary):

```bash
# chromedp driver
go build -tags=nexus_chromedp ./...

# go-rod driver
go build -tags=nexus_rod ./...
```

## Hello, Nexus

```go
package main

import (
    "context"

    "digital.vasic.helixqa/pkg/nexus"
    "digital.vasic.helixqa/pkg/nexus/browser"
)

func main() {
    eng, _ := browser.NewEngine(browser.NewChromedpDriver(), browser.Config{
        Engine: browser.EngineChromedp,
        Headless: true,
    })
    sess, _ := eng.Open(context.Background(), nexus.SessionOptions{})
    defer sess.Close()

    _ = eng.Navigate(context.Background(), sess, "https://example.com")
    snap, _ := eng.Snapshot(context.Background(), sess)
    _ = snap.Elements // [{Ref:"e1", Role:"link", Name:"More information..."}]
}
```

## Next steps

- [Architecture](./architecture) — layers, interfaces, trade-offs.
- [Browser](./browser) — CDP drivers, snapshot refs, security.
- [Mobile](./mobile) — Appium, gestures, accessibility tree.
- [Desktop](./desktop) — Windows / macOS / Linux drivers.
- [AI](./ai) — Navigator, Healer, Generator, Predictor.
- [Accessibility](./a11y) — axe-core + WCAG 2.2.
- [Performance](./perf) — Core Web Vitals + k6.
- [Cross-platform](./cross-platform) — flows + evidence vault.
- [Enterprise](./enterprise) — RBAC, audit, observability.
- [Video course](./video-course) — 8-module walkthrough.
