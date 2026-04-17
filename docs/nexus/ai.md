---
title: Helix Nexus — AI Navigation + Self-Healing
phase: 4
status: ready
---

# Helix Nexus — AI Navigation + Self-Healing

`pkg/nexus/ai` is four capabilities on top of the existing
`LLMOrchestrator` / `LLMProvider` submodules:

| Capability | Type | Purpose |
|---|---|---|
| Navigator | `*ai.Navigator` | Pick the next Action from goal + screenshot + tree |
| Healer    | `*ai.Healer`    | Recover a broken selector using the fresh Snapshot |
| Generator | `*ai.Generator` | Convert a user story into bank YAML |
| Predictor | `*ai.Predictor` | Flag flaky tests before they run |

A single `*ai.CostTracker` enforces the session budget (`NEXUS_LLM_BUDGET_USD`). Every capability reserves cost up-front; a reservation over budget returns `ErrBudgetExceeded` and the session aborts.

## Prompts

- Navigator and Healer use deterministic JSON / plain-text response contracts so the downstream parser is trivial.
- Generator asks for YAML, strips code fences, and validates required fields (`id`, `name`, `steps`) before returning the text to the caller.

## Predictor

A plain logistic function over four features (retries, duration, hour-of-day late-night flag, RSS). Defaults are a sensible baseline; `Observe(sample)` nudges the bias toward failure or pass so the predictor slowly personalises to the operator's suite. Swap to an ONNX runtime when you need real training.

## SQL

- `docs/nexus/sql/helixqa_ai_decisions.sql` — one row per LLM call
- `docs/nexus/sql/helixqa_flake_predictions.sql` — one row per flake prediction

## Cost + safety guarantees

- Calls without a prior `Reserve()` never bypass the budget; every capability reserves **before** recording the entry.
- Temperature defaults to `0` for Healer + JSON-returning callers so replies are deterministic.
- The Generator refuses empty stories and malformed YAML.
- The Predictor's `Decide()` returns `false` for clean samples even at the 0.5 threshold; callers can tighten to 0.9 for high-sensitivity lanes.
