---
module: 01
title: "From Playwright to CDP — Nexus browser engine"
length: 12 minutes
---

# 01 — From Playwright to CDP

## Shot list

1. **00:00 — Cold open.** Red screenshot of a blurry cover in the
   Catalogizer web app. VO: "every day, that's what our users saw."
2. **00:30 — Architecture diagram.** Layered Nexus stack; highlight
   browser engine.
3. **01:15 — Live coding.** `NewEngine(ChromedpDriver, Config{...})`;
   `engine.Open(ctx, SessionOptions{Headless: true})`.
4. **03:00 — Snapshot demo.** Navigate to a fixture page, call
   `engine.Snapshot`, print `e1..eN` refs, click `e5`.
5. **05:30 — Security hardening.** Try to navigate to `file:///` —
   show the denial message. Try a host off the allowlist — show the
   denial. Talk about Content-Security-Policy injection.
6. **07:30 — Pool demo.** Acquire 4 browsers simultaneously; show the
   fifth block until a release.
7. **09:00 — AI-friendly errors.** Trigger a net::ERR_NAME_NOT_RESOLVED
   and show `ToAIFriendlyError` rewriting it.
8. **10:30 — Cleanup.** `defer session.Close()`, `pool.Close()`,
   `engine.ActiveSessions()` returning zero.
9. **11:30 — Outro.** Next module.

## VO script

> "Until now, HelixQA leaned on Playwright for browser automation.
> It worked, but the 3.5-second round-trip per action slowed us down.
> Nexus gives us a Go-native CDP layer in the form of the `browser`
> package — the same ergonomic API, 10× faster, with security baked in
> by default."

*... continued in the recorded script; see the video output for the
full narration.*

## Demo script (copy-paste)

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.helixqa/pkg/nexus"
    "digital.vasic.helixqa/pkg/nexus/browser"
)

func main() {
    // Build the Engine with a test-owned mock driver for the demo.
    eng, _ := browser.NewEngine(&demoDriver{}, browser.Config{
        Engine: browser.EngineChromedp,
        AllowedHosts: []string{"helixqa.vasic.digital"},
    })
    sess, _ := eng.Open(context.Background(), nexus.SessionOptions{Headless: true})
    defer sess.Close()

    _ = eng.Navigate(context.Background(), sess, "https://helixqa.vasic.digital/")
    snap, _ := eng.Snapshot(context.Background(), sess)
    fmt.Printf("%d interactable elements\n", len(snap.Elements))
}
```

## Exercise

Give the viewer a failing test: they must write a `browser.Engine`
allowlist that blocks `evil.test` but lets `catalogizer.local`
through. Starter file: `tools/nexus-course/01-browser/exercise.go`.
Completion criteria: `go test ./tools/nexus-course/01-browser/...`
passes.
