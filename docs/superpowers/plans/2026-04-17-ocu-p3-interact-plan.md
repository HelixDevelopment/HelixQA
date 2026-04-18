# OCU P3 — Interaction Engine Implementation Plan

Goal: Implement `contracts.Interactor` with 4 backends (linux, web, android, androidtv) behind a pluggable factory.
Scope P3: plumbing + verifier hook + injectable production-not-wired sentinel. Real evdev/CDP/ADB wiring is P3.5.

Groups:
- A — factory + Interactor registry (mirrors capture.Factory)
- B — linux backend (uinput-planned; P3 returns ErrNotWired)
- C — web backend (CDP-planned; P3 returns ErrNotWired)
- D — android backend (ADB input; registers "android" + "androidtv")
- E — verifier hook (`pkg/nexus/interact/verify/`)
- F — bench + stress + security audit + challenge bank + integration + close + push

Contracts frozen by P0.
