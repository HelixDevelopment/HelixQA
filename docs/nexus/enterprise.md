---
title: Helix Nexus — Enterprise
phase: 5
status: ready
---

# Helix Nexus — Enterprise

- **SSO**: SAML + OIDC hook points live in `pkg/nexus/orchestrator` via
  the `User` struct. Integrations ship their IdP adapter and hand
  Nexus a fully-populated `User`.
- **RBAC**: four roles (`viewer`, `runner`, `operator`, `admin`) gate
  six built-in `Action` constants. Operators extend the policy via a
  configurable map — `Action` is a plain `string` type so additions
  require no code changes in the orchestrator core.
- **Audit log**: every access-control decision writes to an append-only
  log, keyed by user / action / resource / timestamp / allowed-or-not /
  reason.
- **Evidence vault**: pluggable `EvidenceStore` with a file-backed
  default; S3 / MinIO backends implement the same three-method
  interface. Screenshot + log helpers use per-session directories.
- **Cost discipline**: the AI layer's `CostTracker` enforces a session
  budget that aborts rather than exceeds.

SQL schemas for the enterprise tables (`helixqa_audit_log`,
`helixqa_evidence_items`) ship under `docs/nexus/sql/`.
