---
title: Helix Nexus — Cross-Platform Orchestration
phase: 5
status: ready
---

# Helix Nexus — Cross-Platform Orchestration

`pkg/nexus/orchestrator` lets a single test flow span web → mobile →
desktop. Each `Step` is tagged with a `Platform` and carries its own
`Action` (and optional `Verify`). The `ExecutionContext` is a
concurrency-safe key/value map + an attached `Evidence` vault so every
step can share state and every step's artefacts land under one
per-session folder.

## Evidence vault

The default backend is a file store rooted at a per-session directory.
Swap to S3 / MinIO by implementing the `EvidenceStore` interface (three
methods: `Put`, `PutStream`, `List`).

## RBAC + audit

`AccessControl.Check(user, action, resource)` enforces the default role
ladder (`viewer` < `runner` < `operator` < `admin`) and records every
decision in the `AuditLog`. Failed checks still produce a log row, so
compliance reviewers can prove which users attempted forbidden actions.
