# Implementation Plan: Helix Seller Platform

**Branch**: `001-helix-seller-platform` | **Date**: 2026-07-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-helix-seller-platform/spec.md`

## Summary

Helix Seller is a unified payment facade providing a single REST API and maximally simple UX for configuring and integrating payment processing (Stripe, PayPal, Square) into any project. Built in Go/Gin with HTTP3, PostgreSQL, Redis, and NATS JetStream. Merchant-managed, PCI DSS-aware, multi-currency, with automatic provider fallback and idempotent processing.

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)

**Primary Dependencies**: Gin Gonic (web framework), pgx/v5 (PostgreSQL driver), go-redis/v9, quic-go/http3, golang-jwt/v5, golang.org/x/crypto (Argon2id), NATS JetStream client

**Storage**: PostgreSQL 16+ (production) / SQLite (dev), Redis 7+ (cache/sessions), MinIO/S3 (documents)

**Testing**: stdlib testing + testify, go-sqlmock/miniredis (unit isolation), go test -race, Cypress/Playwright (e2e web)

**Target Platform**: Linux server (Hetzner dedicated), rootless Podman Compose, 3 environments (dev/staging/prod)

**Project Type**: Web service (REST API + SDK + Web Portal + CLI)

**Performance Goals**: API p95 < 150ms, dashboard load < 1.5s, webhook delivery >99.5% within 30s

**Constraints**: PCI DSS-aware (no raw card storage), idempotent payment operations, full audit trail, TLS 1.3

**Scale/Scope**: 100+ merchants, 10k+ txns/day, 100+ users, multi-currency (150+), 3 payment providers at MVP

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Governance-First Development | ✅ PASS | Spec Kit workflow followed: spec → plan → tasks |
| RED-First Testing | ✅ PASS | TDD mandatory; test frameworks defined |
| No Silent Drift | ✅ PASS | All decisions documented with provenance |
| Local-First Architecture | ✅ PASS | No mandatory cloud dependencies; Hetzner self-hosted |
| API-First Design | ✅ PASS | REST API /v1 primary interface; UI is thin client |
| Security by Default | ✅ PASS | AES-256-GCM, TLS 1.3, tokenization, no raw cards |
| PCI DSS-Aware | ✅ PASS | Token-based operations only; full audit trail |
| Documentation First | ✅ PASS | Full technical docs before implementation |
| Decoupling Strategy | ✅ PASS | Adapter pattern for providers; generic interfaces |

**Gate Result**: All principles satisfied. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/001-helix-seller-platform/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── api-v1.yaml      # OpenAPI 3.1 contract
└── tasks.md             # Phase 2 output (NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
├── server/              # Main API server entry point
└── migrate/             # Database migration runner

internal/
├── config/              # Configuration loading
├── database/            # Database connection and migrations
├── handler/             # HTTP handlers (controllers)
│   ├── merchant.go
│   ├── transaction.go
│   ├── subscription.go
│   ├── invoice.go
│   ├── payout.go
│   ├── dispute.go
│   ├── payment_method.go
│   ├── webhook.go
│   ├── analytics.go
│   └── customer.go
├── middleware/           # HTTP middleware (auth, CORS, rate-limit)
├── model/               # Domain models
├── repository/          # Data access layer
├── service/             # Business logic layer
│   ├── payment_router.go
│   ├── provider_adapter.go
│   ├── webhook_processor.go
│   ├── reconciliation.go
│   └── billing.go
├── provider/            # Payment provider adapters
│   ├── adapter.go       # Common interface
│   ├── stripe.go
│   ├── paypal.go
│   └── square.go
├── eventbus/            # NATS JetStream event bus
├── validator/           # Custom validators
├── observability/       # OpenTelemetry, metrics, tracing
├── websocket/           # WebSocket hub and handlers
└── cli/                 # CLI command implementations

pkg/
├── errors/              # Error types and handling
├── logger/              # Structured logging
├── crypto/              # Encryption utilities
└── utils/               # Utility functions

web/                     # Angular web portal
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

tests/
├── contract/            # API contract tests
├── integration/         # Integration tests
└── unit/                # Unit tests

migrations/              # Database migrations
scripts/                 # Build and deployment scripts
configs/                 # Prometheus, Grafana, SonarQube configs
├── prometheus.yml
├── grafana/
│   ├── dashboards/
│   └── provisioning/
├── sonar-project.properties
docs/                    # Project documentation
└── operations/          # Operational runbooks
    └── RESTORE_RUNBOOK.md
```

**Structure Decision**: Web service with separate cmd/ entry points, internal/ for private code, pkg/ for shared libraries, web/ for Angular frontend. Provider adapters are isolated in internal/provider/ with a common interface. Event bus is in internal/eventbus/ for NATS integration.

## Complexity Tracking

> No constitution violations — section not needed.
