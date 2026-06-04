# Conduit — Real-Time Conductor Channel

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-03T00:00:00Z |
| Last modified | 2026-06-03T00:00:00Z |
| Status | active |

## Table of contents

- [What it is](#what-it-is)
- [Why](#why)
- [Event schema](#event-schema)
- [Producer side (the QA session)](#producer-side-the-qa-session)
- [Consumer side (the conductor)](#consumer-side-the-conductor)
- [LLMProvider bridge](#llmprovider-bridge)
- [Decoupling](#decoupling)

## What it is

`pkg/conduit` is a dependency-free, project-agnostic real-time
event/status channel that lets an **external conductor** — a human
operator, a CLI agent, or another orchestrating process — stay in sync
with everything a HelixQA autonomous QA session is doing, without
parsing free-form stdout.

Transport is two files written into the session output directory:

| File | Purpose |
|---|---|
| `conduit.events.jsonl` | append-only JSONL event stream (one JSON object per line). The conductor tails it as it grows. |
| `conduit.status.json` | latest-snapshot status, overwritten atomically on every event. The conductor reads it for an O(1) "where are we now" view. |

## Why

The autonomous pipeline previously reported progress only via
`fmt.Printf` to stdout; `session.Timeline` is an in-memory post-hoc
artefact. There was no live, structured surface a conductor could
consume. `pkg/conduit` closes that gap. It also carries the
captured-evidence signal (`evidence_captured` events with the artefact
path) so a conductor can audit each PASS against real, non-zero
artefacts — the §11.4.2 / §11.4.69 anti-bluff requirement.

## Event schema

Closed-set `EventType`: `session_start`, `session_end`, `phase_start`,
`phase_complete`, `phase_error`, `phase_progress`, `challenge_start`,
`challenge_step`, `challenge_verdict`, `evidence_captured`, `llm_call`,
`vision_call`, `error`, `log`.

Closed-set `Verdict`: `PASS`, `FAIL`, `SKIP`, `OPERATOR-BLOCKED`.

Every event carries a monotonic `seq`, a UTC `time`, the `session` id,
and the relevant optional fields (`phase`, `platform`, `challenge`,
`step`, `verdict`, `progress`, `evidence_path`, `evidence_kind`,
`reason`, `detail`, `duration_ms`, `fields`). See `pkg/conduit/event.go`.

## Producer side (the QA session)

The `SessionPipeline` gained `WithEventSink(conduit.Sink)`. When set,
it emits `session_start`/`session_end` and per-phase events. The
`SessionCoordinator` is wired via `autonomous.AttachConduit(sc, sink)`,
which registers a `PhaseListener` on the coordinator's existing
`PhaseManager` observer seam — no coupling to conduit inside the
PhaseManager.

The `helixqa` CLI enables the channel by default into the output dir
(disable with `HELIXQA_CONDUIT=0`) and prints the stream/status paths
at startup.

Typed emit helpers (`conduit.SessionStart`, `ChallengeVerdict`,
`EvidenceCaptured`, `LLMCall`, `VisionCall`, …) funnel through
`Sink.Emit` so sequencing/timestamping/status-folding happen in one
place. A `conduit.NopSink()` makes the sink optional with no nil checks.

## Consumer side (the conductor)

Two ways to consume:

1. **Go API** — `conduit.NewMonitor(streamPath, opts...).Tail(ctx, handler)`
   delivers each parsed event in order, following the file across
   growth (`tail -f` semantics). `conduit.Collect(...)` returns the
   whole session once `session_end` is seen. `conduit.ReadStatus(...)`
   reads the O(1) snapshot.

2. **CLI** — `helixqa-conduit-monitor -stream <path>` prints a live
   human-readable feed (and flags 0-byte evidence as a bluff signal);
   `-json` emits raw JSONL for piping; `-status <path>` prints the
   snapshot once. Exit code is non-zero on a FAIL / OPERATOR-BLOCKED
   final verdict so a conductor can gate on it.

A malformed stream line is surfaced to the handler as an `error` event
(`reason=malformed_stream_line`), never silently dropped.

## LLMProvider bridge

`pkg/llm/llmprovider_bridge.go` (`NewLLMProviderBridge`) adapts any
`digital.vasic.llmprovider/pkg/provider.LLMProvider` to the HelixQA
`llm.Provider` interface, so the autonomous pipeline can use the shared
LLMProvider module's production stack (circuit breakers, health
monitoring, retry/backoff) instead of re-implementing transport.
Vision is gated on the provider's advertised capability.

VisionEngine (`pkg/analyzer`, `pkg/graph`), LLMOrchestrator
(`pkg/agent`), and DocProcessor (`pkg/coverage`, `pkg/feature`) were
already bridged in `pkg/autonomous/coordinator.go`.

## Decoupling

Per CONST-051 (§11.4.28), `pkg/conduit` carries no consumer-project
knowledge — no ATMOSphere/device/package strings. The consuming project
supplies session and feature identifiers at runtime. The conductor
consumes plain files, so the conductor itself can be any process on any
platform.
