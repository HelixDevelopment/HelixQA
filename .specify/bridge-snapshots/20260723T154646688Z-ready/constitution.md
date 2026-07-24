# Helix Seller — Project Constitution

> This constitution extends the Helix Universal Constitution at `constitution/Constitution.md`.
> All clauses there apply unless explicitly overridden below.

## 1. Core Principles

### 1.1 Governance-First Development
All changes must be traceable to documented decisions. No silent drift from established patterns. Every code change must pass through the Spec Kit workflow: spec → plan → tasks → implement.

### 1.2 Deterministic Execution
Agent and human actions must be predictable and auditable. No implicit assumptions. Every implementation must be verified against the plan before completion.

### 1.3 RED-First Testing
Tests must be written BEFORE implementation. TDD is mandatory for all features. Coverage floors must be established and enforced before any code is merged. Reproduce-first: write the failing (RED) test that reproduces the requirement/bug; the same test confirms the fix; extend to all cases.

### 1.4 No Silent Drift
The codebase must never silently deviate from established patterns. All deviations must be documented, reviewed, and approved before implementation.

### 1.5 Local-First Architecture
The application runs on the seller's own infrastructure. No mandatory cloud dependencies. All data stays under the seller's control.

### 1.6 API-First Design
All features must be accessible via REST/HTTP3 API. UI is a thin client. CLI and programmatic access are first-class citizens.

### 1.7 Security by Default
All data is encrypted at rest and in transit. Authentication and authorization are mandatory. No security shortcuts. PCI DSS-aware: no raw card storage, tokenization via providers, AES-256-GCM encryption.

## 2. Governance & Execution Rules

### 2.1 Spec Kit Workflow
- **Before writing code:** Create a spec using `speckit-specify`
- **Before planning:** Review the spec with `speckit-clarify` and `speckit-checklist`
- **Before implementing:** Generate a plan with `speckit-plan` and tasks with `speckit-tasks`
- **Before completing:** Verify with `speckit-converge` and `speckit-analyze`

### 2.2 Decision Documentation
All significant decisions must be recorded in the constitution or project documentation with rationale. Provenance tags: [CONSTITUTION §x], [IN-HOUSE: module], [RESEARCH], [OPERATOR], [DEFAULT — adjustable], [BUILD-NEW].

### 2.3 Agent Hierarchy
- Primary agent (OpenCode): Handles all development tasks
- Subagents: Dispatched for parallel work when needed
- All agents must follow the constitution

## 3. Scope & Invariants

### 3.1 In-Scope — MVP
- Go/Gin web application with HTTP3 support
- PostgreSQL database layer (SQLite for dev)
- Redis caching and session management
- OpenDesign integration for all design work
- REST API endpoints (/v1 path-versioned)
- WebSocket support for real-time features
- Payment provider adapters: Stripe, PayPal, Square
- Unified payment model (charges, refunds, subscriptions, invoices, payouts, disputes)
- Merchant onboarding and configuration UI
- Webhook handling (ingress + outgoing)
- Multi-currency support (150+ currencies)
- PCI DSS-aware architecture (tokenization, no raw card storage)
- User/account management (three-tier: Root Admin, Account Admin, Standard User)
- White-labeling (per-account branding)
- SDK generation (Go, TypeScript, Python)
- Web portal (Angular 19)
- CLI (Go/Cobra)
- Full technical documentation

### 3.2 In-Scope — Phase 2
- Additional payment providers (Adyen, Mollie)
- TUI (Bubble Tea + Cobra + Lipgloss)
- Desktop (Tauri 2)
- Mobile (native per platform + KMP)
- Advanced analytics and reporting
- Dispute evidence automation
- Multi-currency settlement optimization

### 3.3 Out-of-Scope
- Holding funds (platform facilitates routing, not custody)
- Banking license requirements
- Cryptocurrency payments (phase 3)
- Point-of-sale hardware integration

### 3.4 Non-Negotiable Invariants
- All code must be tested before merge (TDD mandatory)
- All changes must be documented
- All designs must use OpenDesign
- All work must follow the Spec Kit workflow
- No raw card storage — tokenization only
- All payment operations must be idempotent
- All transactions must be fully auditable
- No credentials in public repos

## 4. Content Classification & Workflow

### 4.1 Code Quality
- Go standard formatting (gofmt)
- Linting with golangci-lint
- Test coverage minimum: 80%
- No race conditions (go test -race mandatory)
- SonarQube static analysis

### 4.2 Documentation
- All API endpoints documented (OpenAPI 3.1)
- All configuration options documented
- All deployment steps documented
- Markdown primary; kept in sync with PDF + HTML siblings
- Diagrams via Mermaid with multi-paragraph explanations

### 4.3 Design
- All UI/UX work in OpenDesign
- Responsive design required
- Accessibility standards (WCAG 2.1 AA)
- Light + dark mode mandatory

## 5. Work Item Lifecycle

### 5.1 States
1. **Specified:** Requirements documented
2. **Planned:** Implementation plan created
3. **Tasked:** Tasks broken down
4. **In Progress:** Actively being worked
5. **Verifying:** Tests running, review pending
6. **Complete:** Merged and deployed

### 5.2 Transitions
- Each transition requires passing quality gates
- No skipping states
- All transitions must be documented

## 6. Payment System Specifics

### 6.1 Provider Adapter Pattern
Every payment provider has a dedicated adapter implementing a common interface (Charge, Refund, Subscription, Invoice, Payout, Dispute). Providers are enabled/configured per merchant. Fallback routing is configurable per merchant. Health monitoring on each adapter; circuit-breaker on consecutive failures.

### 6.2 Unified Payment Model
All provider-specific data normalized into: Transaction, PaymentMethod, Subscription, Invoice, Payout, Dispute. Every transaction carries: id, merchant_id, provider, provider_transaction_id, type, amount, currency, status, payment_method, metadata, timestamps.

### 6.3 Webhook Processing
Dedicated ingress per provider. Signature verification mandatory. Idempotent processing (duplicate detection). Event normalization to platform events. Outgoing webhooks: merchants configure URLs; platform sends signed payloads.

### 6.4 Concurrency & Idempotency
Every payment request carries an idempotency key. Postgres advisory lock for single-claim processing. Exponential back-off with jitter (5 retries, base 2s, factor 2.0, cap 5 min). Circuit-breaker pattern for flapping providers.

### 6.5 Multi-Currency
150+ currencies supported. Automatic conversion. Exchange rates from configurable external sources. Settlement in merchant's preferred currency.

### 6.6 PCI DSS Compliance
No raw card storage — all card data tokenized via provider. Token-based operations only. AES-256-GCM for all sensitive data at rest. TLS 1.3 for all connections. No sensitive data in logs. Full audit trail for all payment operations.

## 7. User & Account Management

### 7.1 Three-Tier Hierarchy
| Tier | Role | Permissions |
|------|------|-------------|
| 1 | Root Admin | Full system control, all accounts; only one exists |
| 2 | Account Admin | Full control of their account and its users |
| 3 | Standard User | Consumer access to assigned accounts |

### 7.2 Authentication
- JWT access + refresh (RS256)
- API keys with scopes for SDK/CLI
- OAuth2 for external service linking
- MFA (TOTP) mandatory for admin tiers
- Argon2id password hashing (min 12 chars, breach-list check)
- Sessions: access 15 min, refresh 7 d, idle 30 min

## 8. Infrastructure

### 8.1 Hosting
- Provider: Hetzner dedicated host
- Containerized: rootless Podman Compose
- Three environments: dev, staging, production (subdomains)

### 8.2 Security
- No credentials in public repos
- Runtime-load-only from gitignored .env/secrets
- AES-256-GCM at rest, TLS 1.3 in transit
- OpenTelemetry + Prometheus + Grafana monitoring
- Structured logging with correlation IDs
- Audit trail for all admin/user actions

### 8.3 Backup/DR
- Daily full + hourly DB incrementals
- Assets daily snapshot/dedup
- RPO ≈ 1 h, RTO ≈ 4 h
- Documented restore runbook

## 9. Testing

### 9.1 Mandated Test Types
Unit, integration, e2e, security, performance, benchmarking, UI/UX. Mocks/stubs allowed only in unit tests. 100% test-type coverage per feature × platform.

### 9.2 Frameworks
- Go: stdlib testing + testify, go test -race, go-sqlmock/miniredis
- TypeScript/Angular: Jasmine + Karma (unit) + Cypress/Playwright (e2e)
- Independent AI review on every change

## 10. Development Phases

### Phase 1: Foundation
- 1.1 Infrastructure (containers, DB, deployment)
- 1.2 Core Services (User Service, Event Bus, Auth)
- 1.3 Integration (payment provider adapters, API)

### Phase 2: Payment Engine
- 2.1 Payment Router
- 2.2 Provider Adapters (Stripe, PayPal, Square)
- 2.3 Webhook Processing
- 2.4 Reconciliation

### Phase 3: Client Applications
- 3.1 Web Portal
- 3.2 Desktop
- 3.3 Mobile
- 3.4 CLI/TUI
- 3.5 Design System

### Phase 4: Testing & Deployment
- 4.1 Unit
- 4.2 Integration
- 4.3 System
- 4.4 Security
- 4.5 Performance
- 4.6 Production Deployment

## 11. Compliance Check

Before completing any work, verify:
- [ ] Spec exists and is complete
- [ ] Plan exists and is approved
- [ ] Tasks are defined and ordered
- [ ] Tests are written and passing (TDD: RED → GREEN → extend)
- [ ] Documentation is updated
- [ ] Design is approved in OpenDesign
- [ ] Payment operations are idempotent
- [ ] No raw card data stored
- [ ] Audit trail complete

---

**Last Updated:** 2026-07-23
**Project:** Helix Seller
**Tech Stack:** Go 1.22+, Gin Gonic, HTTP3 (QUIC/Cronet), PostgreSQL 16+, Redis 7+
**Design Tool:** OpenDesign
**Branding:** Helix Seller / HelixSeller / helix_seller
**Deployment:** Hetzner dedicated, rootless Podman Compose, 3 environments
**Payment Providers:** Stripe, PayPal, Square (MVP); Adyen, Mollie (phase 2)
