---
module: 07
title: "Cross-platform flows + evidence vault"
length: 13 minutes
---

# 07 — Cross-platform flows + evidence vault

## Shot list

1. **00:00 — Cold open.** A single flow going web → mobile → desktop.
2. **01:00 — ExecutionContext.** Shared Data map; concurrency-safe;
   Snapshot returns a copy.
3. **03:00 — Flow.** Build a three-step flow; run it; observe Verify
   abort semantics.
4. **06:00 — EvidenceStore.** File-backed default; layout under
   `sessions/<id>/<step>/`.
5. **08:30 — RBAC.** Four roles; default policy. Show a viewer being
   blocked; switch to operator.
6. **11:00 — AuditLog.** Inspect entries; exported to
   `helixqa_audit_log` table.
7. **12:30 — Outro.**

## Exercise

Write a cross-platform flow: register a user on web, verify email on
mobile, confirm presence on desktop. Each step must fail the flow on
its own regression.
