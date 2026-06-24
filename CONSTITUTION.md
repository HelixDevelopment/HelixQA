# HelixQA Constitution

## INHERITED FROM constitution/Constitution.md

All rules in `constitution/Constitution.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in its
`Constitution.md` (and the `CLAUDE.md` / `AGENTS.md` it references) apply
unconditionally. The submodule-scoped rules below extend the universal
clauses — they MUST NOT weaken any inherited rule. Locate the constitution
from any nested depth via its `find_constitution.sh` helper — do NOT
hardcode a path (this module stays fully decoupled and project-agnostic per
§11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

## Anti-bluff — the binding mandate for this module

HelixQA is an anti-bluff QA orchestration framework. Tests and Challenges
exist for exactly one purpose: to confirm a feature genuinely works for a
real end user, end-to-end. A test that passes while the feature is broken
is a bluff test and is forbidden (§11.4). Every PASS HelixQA emits — and
every PASS in HelixQA's own suite — MUST carry positive runtime evidence
captured during execution (screenshots, logcat, video, stack traces,
reports). CI green is necessary, never sufficient.

This binds the whole module: mocks are unit-test-only (§11.4.27); every
gate carries a paired §1.1 mutation; `t.Skip()` requires a topology
justification (§11.4.3); no guessing language (§11.4.6); credentials are
never committed (§11.4.10). See [`CLAUDE.md`](CLAUDE.md) and
[`AGENTS.md`](AGENTS.md) for the full operating manual.
