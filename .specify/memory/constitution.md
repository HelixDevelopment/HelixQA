<!--
Sync Impact Report
==================
Version change: [TEMPLATE] → 1.0.0 (initial ratification of this file)
Context: the Spec Kit workspace previously lived nested at
  `helix-seller/.specify/` and had reached a filled-in v1.1.0 there. An
  external move operation (commit adf0e13, "Auto-commit") relocated the
  whole workspace to the repository root and reset constitution.md back
  to the pristine template in the process — this fill supersedes that
  removed v1.1.0 content; the version counter restarts at 1.0.0 because
  this is this file's own first-ever ratification at this path, but no
  substantive ground is lost: the same research-doc grounding and
  submodule cross-reference are carried forward in full below.
Modified principles: N/A (first fill at this path)
Added sections:
  - Core Principles I–VIII (Test-First & Anti-Bluff, Payment-Provider
    Abstraction, Webhook-Only Access Activation, Subscription Lifecycle
    Integrity, Zero-Trust Security by Default, Observability &
    Simplicity, Independent Code Review Before Any Merge, Latest-Source
    Verification for Payment Integrations)
  - Security & Compliance Requirements
  - Subscription & Billing Data Model
  - Deployment & Operations
  - Development Workflow
  - Inherited Universal Governance (Cross-Reference)
  - Governance
Removed sections: none (template placeholders only)
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ reviewed — "Constitution Check" gate
    is generic ("[Gates determined based on constitution file]"), no edit needed
  - .specify/templates/spec-template.md ✅ reviewed — no constitution-specific
    references found
  - .specify/templates/tasks-template.md ✅ reviewed — no constitution-specific
    references found
  - .claude/skills/speckit-*/SKILL.md (incl. the new speckit-superpowers-bridge
    family) ✅ reviewed — no outdated CLAUDE-only or agent-specific references
    found
  - README.md (repo root) ✅ reviewed — no principle references to update
Follow-up TODOs: none — no placeholder left undefined
-->

# Helix Seller Constitution

## Core Principles

### I. Test-First & Anti-Bluff (NON-NEGOTIABLE)

Every change MUST be preceded by a failing test that reproduces the gap
before any implementation is written (RED before GREEN). A test authored
after its implementation proves only that it agrees with the code, never
that it catches a defect, and is not an acceptable substitute. Every test
MUST exercise real, observable behavior — mocks, stubs, and placeholders
are permitted only in unit tests; integration, contract, and end-to-end
tests MUST run against a real database, real Redis, and real (or
provider-sandbox) webhook payloads. A test that passes without proving
the underlying behavior actually works is a defect, not a pass. Code
coverage on the executable codebase MUST reach at least 85% (target
~100%), but the percentage is a necessary floor, never sufficient proof
by itself — a covered line whose branch was never taken, or a test with
no real assertion, does not count as tested.

Rationale: this project's payment-processing surface has zero tolerance
for a test suite that is green while a webhook silently double-charges or
a subscription silently fails to activate. This principle extends —
never weakens — the stricter test-first and anti-bluff covenant already
bound at the repository root (`constitution/Constitution.md` §1, §1.1,
§11.4.224, §11.4.43); where the two overlap, the root constitution's rule
governs.

### II. Payment-Provider Abstraction

Every Merchant-of-Record integration (Paddle, Lemon Squeezy, Gumroad, and
any future provider) MUST implement one common internal interface for
checkout creation, webhook parsing, and subscription-state mapping.
Provider-specific quirks (field names, signature schemes, event
taxonomies) stay entirely behind that interface. Adding, removing, or
swapping a provider MUST NOT require changes to webhook dispatch,
subscription state-machine logic, or any HTTP handler outside the
provider's own adapter package. Each provider adapter MUST handle, at
minimum, its subscription-created, subscription-updated,
subscription-cancelled, payment-succeeded, and payment-failed events.

Rationale: the business strategy names Paddle and Lemon Squeezy as
primary providers with Gumroad as a secondary fallback and an eventual
move to a local PSP once revenue justifies it — provider churn is an
expected, not exceptional, event, and the codebase must absorb it
without touching core logic.

### III. Webhook-Only Access Activation (NON-NEGOTIABLE)

Frontend "payment success" pages and redirects are NEVER trusted to
grant or extend access — access is activated, extended, or revoked ONLY
by a verified, provider-originated webhook. Every inbound webhook MUST
verify its provider's cryptographic signature before any further
processing; an unverified or invalid signature MUST be rejected, never
silently accepted. Every webhook event ID MUST be recorded on first
receipt (the `SETNX webhook_event_id <TTL>` pattern) so a provider's
at-least-once redelivery of the same event is acknowledged without
re-executing its side effects. Every resulting subscription state
transition MUST be logged with the event ID that caused it.

Rationale: a frontend success page is client-controlled and trivially
spoofable — it is a UX convenience, never an authorization signal. This
is also the exact mechanism preventing duplicate subscription activation
and duplicate billing side effects, the single most financially
consequential failure mode this system can have.

### IV. Subscription Lifecycle Integrity

The subscription state machine has exactly the states `NONE`,
`checkout_started`, `active`, `past_due`, and `cancelled`, with these
transition rules, and no other transition is permitted: a succeeded
payment activates the subscription and extends its expiration; a failed
payment marks the subscription `past_due` and waits for the provider's
own retry schedule — it MUST NOT immediately revoke access; a
cancellation keeps access live until the already-paid period actually
ends, it MUST NOT revoke access immediately on cancellation. Every
transition is driven exclusively by a verified webhook event (Principle
III) — no internal code path may move a subscription between states
without one.

Rationale: mis-modeling any one of these transitions either revokes
access a paying customer is still entitled to (a support/refund
incident) or grants access nobody paid for (a revenue-loss incident) —
this state machine is the single source of truth for what a customer is
entitled to at any given moment.

### V. Zero-Trust Security by Default

Internal service-to-service traffic assumes a hostile network by
default. Credentials, API keys, and webhook signing secrets are never
logged, never committed, and always git-ignored (`.env`, `.env.*`)
outside a tracked `.env.example`. Database roles are least-privilege per
service. No raw payment-card data is ever stored, logged, or transmitted
by this system — PCI scope stays entirely with the Merchant-of-Record
provider.

Rationale: a solo-developer-operated payment platform has no dedicated
security team to catch a credential leak or an over-privileged database
role after the fact — the default must be safe, not merely capable of
being made safe. Extends the repository-root constitution's credentials
mandate (`constitution/Constitution.md` §11.4.10) and its git-hygiene
mandate (§11.4.30); where the two overlap, the root constitution's rule
governs.

### VI. Observability & Simplicity

Every payment-critical code path (webhook receipt, subscription state
transition, provider API call) MUST emit structured logs and metrics
sufficient to reconstruct what happened without reproducing it live.
Complexity — a new caching layer, a new internal service, a new
abstraction — is introduced only against a measured, current need, never
speculatively for an anticipated future one (YAGNI).

Rationale: for a subscription business, the ability to answer "what
happened to this specific customer's payment" after the fact is not
optional, and unnecessary complexity is what makes that reconstruction
hard.

### VII. Independent Code Review Before Any Merge (NON-NEGOTIABLE)

Every change — no exception for "just a one-liner" or "just a doc edit"
— MUST pass an independent code-review step, structurally separated from
its author, before it is accepted, committed, or built. The review MUST
enumerate the full input/scenario space of the change (not just the
reported case), prove rather than assume any "can't happen" claim, and
verify the fix against any captured runtime evidence. Any finding MUST
be fixed and re-reviewed; the review loop repeats until it returns zero
findings and zero warnings of any kind — a single "addressed the
comments" pass is not sufficient.

Rationale: distilled from the repository-root constitution's mandatory
code-review family (`constitution/Constitution.md` §11.4.125, §11.4.134,
§11.4.142, §11.4.194, §11.4.209 — the last of which pins the review to
run on the Fable model at `xhigh` effort, Opus `xhigh` as fallback). A
solo-developer project has no second engineer to catch what the author
missed; a structurally-independent review pass is the substitute.

### VIII. Latest-Source Verification for Payment Integrations

Before implementing or documenting any integration against Paddle, Lemon
Squeezy, Gumroad, or any other external provider API, the current
official provider documentation MUST be fetched and cross-referenced —
never implemented from training-data memory or a stale prior integration
note. Any gap, silence, or contradiction found MUST be documented
explicitly rather than assumed to be agreement.

Rationale: distilled from the repository-root constitution's
latest-source cross-reference mandate (`constitution/Constitution.md`
§11.4.99) — payment-provider webhook schemas, signature schemes, and
required event sets change over time, and a stale integration is exactly
the kind of defect that stays invisible until a real customer's payment
silently fails to activate their subscription.

## Security & Compliance Requirements

- No raw card data ever touches this system; PCI scope is delegated
  entirely to the Merchant-of-Record provider (Paddle / Lemon Squeezy /
  Gumroad).
- Webhook signing secrets are rotated per each provider's documented
  rotation procedure.
- `.env` and all secret-bearing files are git-ignored; only `.env.example`
  (with placeholder values) is tracked.
- Every subscription state change carries an audit trail: the triggering
  webhook event ID, the prior state, the new state, and the timestamp.
- Redis is used for webhook-event deduplication (`SETNX` + TTL) and MAY be
  used for caching, but the deduplication use is load-bearing and MUST
  NOT be bypassed for performance reasons.

## Subscription & Billing Data Model

Baseline schema (may be extended by feature specs, never contradicted):

- **Users**: `id` (UUID), `email`, `created_at`.
- **Subscription**: `id`, `user_id`, `provider`, `external_subscription_id`,
  `status`, `plan_id`, `current_period_end`.
- **Subscription status** is a closed set: `active`, `past_due`,
  `cancelled`, `expired` (see Principle IV for the transition rules
  governing movement between these).

## Deployment & Operations

- **Development**: Docker Compose with `postgres`, `redis`, `backend`, and
  `caddy` services.
- **Production**: a VPS behind Caddy as reverse proxy, automatic TLS,
  HTTP/3 enabled, regular backups, and monitoring.
- **Pipeline stages** (commit → tests → build container → security scan →
  deploy → health check) are enforced, but NEVER via GitHub Actions,
  GitLab CI, or any equivalent hosted CI/CD automation — the repository-
  root constitution (`constitution/Constitution.md` §11.4.156) mandates
  all such automation stay disabled. These stages run through the
  project's own pre-build gates, the agentic development loop, and
  scripted/manual deploy steps instead.
- **Roadmap phasing** (for Constitution Check context on feature scope,
  not a binding technical constraint): Phase 1 — accounts, Paddle
  integration, webhook processing, subscription checks; Phase 2 — Lemon
  Squeezy fallback, admin dashboard, audit logs, email notifications;
  Phase 3 — multiple plans, teams, invoices, analytics, billing portal.

## Development Workflow

- Features are developed spec-first via Spec Kit:
  `/speckit-specify` → `/speckit-plan` → `/speckit-tasks` →
  `/speckit-implement`. The installed `speckit-superpowers-bridge`
  extension guards `/speckit-clarify`, `/speckit-plan`, `/speckit-tasks`,
  and `/speckit-implement` so Spec Kit contracts cannot be changed while
  a Superpowers implementation handoff is in flight, and records a
  handoff automatically after `/speckit-tasks`.
- Every `plan.md` MUST pass the Constitution Check gate against this
  document before Phase 0 research begins, and MUST be re-checked after
  Phase 1 design.
- This project's stack — Go, Gin Gonic, PostgreSQL, Redis, Caddy with
  HTTP/3 QUIC termination and Brotli compression — is the default target
  for `Technical Context` unless a specific feature's spec states
  otherwise.
- This constitution governs feature-level engineering discipline for the
  Spec-Kit workspace. It operates within, and never overrides, the
  broader Helix Constitution inherited via the constitution submodule at
  `constitution/Constitution.md` (referenced from the root `/CLAUDE.md`
  and `/AGENTS.md`) — where a rule here and a rule there conflict, the
  root constitution's stricter rule governs.

## Inherited Universal Governance (Cross-Reference)

This project inherits the full Helix Constitution submodule at
`constitution/` (~224 numbered `§11.4.x` anchors plus `§1`–`§12`). That
submodule is the single source of truth for every universal rule it
contains (per its own §11.4.35 — universal rules live once, in the
submodule, and are inherited by reference; duplicating them verbatim
into this file would itself violate that rule). This section names, by
anchor number, the submodule clauses most directly load-bearing for this
Go payment-backend project, so a reader of this file knows exactly where
to look without re-deriving it:

- **Testing & anti-bluff**: §1, §1.1, §11.4.1–§11.4.9, §11.4.27, §11.4.43,
  §11.4.108, §11.4.115, §11.4.224 (see Principle I).
- **Code review**: §11.4.125, §11.4.134, §11.4.142, §11.4.194, §11.4.209
  (see Principle VII).
- **Security & credentials**: §11.4.10, §11.4.30, §9 (see Principle V).
- **Latest-source verification**: §11.4.99 (see Principle VIII).
- **Git & release discipline**: §2, §2.1 (multi-upstream push),
  §11.4.113 (absolute no-force-push), §11.4.188 (regular main→feature
  merge cadence), §11.4.195 (branch taxonomy).
- **Documentation discipline**: §11.4.12, §11.4.44, §11.4.65, §11.4.106,
  §11.4.212 (README as documentation entry point).
- **Host-session safety**: §12, §12.6 (60% memory ceiling).
- **CI/CD**: §11.4.156 (hosted CI/CD automation MUST stay disabled — see
  Deployment & Operations above).
- **Completion discipline**: §11.4.197 (a started effort must reach a
  genuinely completed-and-wired or explicitly-closed terminal state,
  never sit un-wired in the backlog).

**Explicitly out of scope for this project** (present in the submodule,
not applicable): the large families of anchors governing Android/mobile
device testing, on-device video/audio capture and OCR validation,
hardware flashing and target-device safety, and multi-track
Android-build orchestration (e.g. §11.4.48–§11.4.52, §11.4.107,
§11.4.117, §11.4.133, §11.4.153–§11.4.170, §11.4.187–§11.4.196) — these
govern a different class of consuming project (mobile/embedded) and have
no analog in a Go HTTP backend. They remain correctly present in the
submodule for the projects that need them; this file does not restate
them because they do not bind here.

## Governance

This constitution supersedes ad-hoc engineering practice for the
`helix_seller` codebase. Amendments follow the standard
`/speckit-constitution` flow: a Sync Impact Report is generated, the
version is bumped per semantic-versioning rules (MAJOR for incompatible
principle removal/redefinition, MINOR for a new or materially expanded
principle, PATCH for wording/clarification), and every dependent
template and command file is checked for now-outdated references before
the change is considered complete. Every feature plan MUST demonstrate
compliance with the Core Principles above via its `plan.md` Constitution
Check gate; unresolved violations MUST be justified in that plan's
Complexity Tracking table or the plan MUST be simplified until they are
not needed.

**Version**: 1.0.0 | **Ratified**: 2026-07-22 | **Last Amended**: 2026-07-22
