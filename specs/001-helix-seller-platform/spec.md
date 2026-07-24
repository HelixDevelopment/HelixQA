# Feature Specification: Helix Seller Platform

**Feature Branch**: `001-helix-seller-platform`

**Created**: 2026-07-23

**Status**: Draft

**Input**: User description: "Platform as facade over other payment systems with unique interface and maximally simple UX for easy configuration and integration of payment systems into every project/system. Whole system MUST BE fully developed into technical documentation in hundreds of pages, then from it we MUST in iterations develop functionality by functionality."

## Executive Overview

### 1.1 Document Purpose

This document is the comprehensive research and planning request for the Helix Seller (helix_seller) project. It defines scope, requirements, technical specifications, and development methodology for an enterprise-grade system that acts as a unified facade over multiple payment systems, providing a single interface and maximally simple UX for configuring and integrating payment processing into any project or system.

Classification: INTERNAL. It lives under docs/private/research/mvp for sensitive material and docs/public/research/mvp for publishable documentation.

Main repository: helix_seller.

### 1.2 System Overview

Helix Seller connects to multiple payment providers — initially Stripe, PayPal, and Square, with more to follow — using merchant API credentials, provides a unified REST API and SDK layer for payment operations (charges, refunds, subscriptions, invoicing, payouts), stores transaction and merchant data in relational and analytical databases, and offers a maximally simple web UI for configuring payment systems, managing merchants, and monitoring transactions. The system reacts to payment events (webhooks) and supports scheduled operations (payouts, reconciliation), all fully event-driven with strict prevention of race conditions and duplicate processing.

### 1.3 Key Capabilities

- Multi-provider payment facade (Stripe, PayPal, Square, Adyen, Mollie; extensible).
- Unified API for charges, refunds, subscriptions, invoices, payouts, disputes.
- Maximally simple UX for configuration and integration.
- Merchant onboarding and multi-account management with white-labeling.
- Cross-platform clients: Web, CLI, TUI, Desktop, Mobile.
- REST API + real-time Event Bus; multi-language SDKs.
- PCI DSS-aware architecture (card data tokenization, no raw card storage).
- Multi-currency support with automatic conversion.
- Reconciliation and financial reporting.

### 1.4 Process & Quality Covenant

All documentation and materials are created in multiple iterations, divided logically and hierarchically (parents/children; phases → sub-phases → micro-sections), each with its own deep research. Every iteration must be enterprise-grade, implementation-ready, with no gaps, weak spots, danger zones or bluff — validated by multiple passes and independent agent review. Workable items (phases/tasks/subtasks) are governed by the Constitution submodule and its workable-items system.

## Clarifications

### Session 2026-07-23

- Q: How are customers modeled in the system? → A: Customer is a first-class entity (id, merchant_id, name, email, phone, payment_methods[], metadata) owned by merchants; required for subscriptions, invoices, and recurring billing.
- Q: What exchange rate sources are used for multi-currency? → A: Free tier primary (exchangerate-api.com free, frankfurter.app) with paid fallback (Open Exchange Rates) — zero-cost MVP with reliability.
- Q: What is the outgoing webhook retry policy? → A: Exponential back-off: 5 retries over 24h (1m, 5m, 30m, 2h, 24h) — balanced reliability without hammering down endpoints.
- Q: How is the platform billing model structured? → A: Transaction-fee: percentage + fixed fee per transaction (e.g., 1% + $0.10) — industry standard, simple to meter.
- Q: When does provider fallback trigger? → A: Automatic fallback on timeout/5xx/unavailable, merchant configures fallback order (e.g., Stripe → PayPal → Square), circuit-breaker trips after 3 consecutive failures.

## Operator Decisions

| Decision | Value | Consequence |
|----------|-------|-------------|
| Scale | Target scale for MVP | Large / multi-tenant (100+ merchants, 10k+ transactions/day, 100+ users, multi-currency) |
| Compliance | Regulatory posture | PCI DSS-aware — tokenization, no raw card storage; formal PCI certification deferred (design remains PCI-aware) |
| Clients | Client priority | Web + CLI first; TUI → Desktop (wrapped web) → Mobile |
| Payment types | Transaction priority | All payment types in parallel — charges, refunds, subscriptions, invoicing, payouts |
| Retention | Data retention | Keep indefinitely, per-account overrides; root sets global default |
| Billing | Billing in MVP | Transaction-fee (percentage + fixed fee per transaction, e.g., 1% + $0.10) — metered per transaction, invoiced monthly |
| SLO | Performance tier | Aggressive — API p95 < 150 ms, dashboard load < 1.5 s; processing async with progress events |
| Backup | Backup / DR | Daily full + hourly DB incrementals; RPO ≈ 1 h, RTO ≈ 4 h |

## Canonical Technology Decision Matrix

| Concern | Decision | Provenance |
|---------|----------|------------|
| Primary language | Go 1.22+ | [CONSTITUTION] |
| Web framework | Gin Gonic | [CONSTITUTION] |
| Transport | HTTP/3 (QUIC/Cronet) | [CONSTITUTION] |
| Relational DB | PostgreSQL 16+ (production) / SQLite (dev) | [IN-HOUSE: database] |
| Vector DB / semantic | pgvector backend, cosine similarity | [IN-HOUSE: vectordb] |
| Event bus | NATS JetStream | [IN-HOUSE: eventbus] |
| Durable queue / jobs | Postgres-backed task queue | [IN-HOUSE: background] |
| Cache | Redis 7+ | [IN-HOUSE: cache] |
| Object storage | MinIO/S3-compatible | [IN-HOUSE: storage] |
| Auth | JWT + API keys + OAuth2 | [IN-HOUSE: auth] |
| Encryption | AES-256-GCM at rest, TLS 1.3 in transit | [IN-HOUSE: security] |
| Observability | OpenTelemetry + Prometheus + Grafana | [IN-HOUSE: observability] |
| TLS / certificates | Let's Encrypt (acme.sh) | [IN-HOUSE: lets_encrypt] |
| Containers | Rootless Podman Compose | [CONSTITUTION] |
| Frontend (Web) | Angular 19 (product) / Angular 22 (marketing) | [IN-HOUSE: design_system] |
| Desktop | Tauri 2 (Rust core + Angular UI) | [DEFAULT — adjustable] |
| Mobile | Native per platform + KMP shared logic | [DEFAULT — adjustable] |
| TUI | Bubble Tea + Cobra + Lipgloss | [IN-HOUSE] |
| CLI | Cobra headless | [IN-HOUSE] |
| Docs pipeline | Markdown → HTML/PDF via pandoc | [CONSTITUTION] |
| Design system | OpenDesign (nexu-io/open-design) | [CONSTITUTION] |
| CI/CD | Local git-hooks + pre-tag retest | [CONSTITUTION] |
| Code review | Independent AI review | [CONSTITUTION] |
| Deploy target | Hetzner dedicated host, rootless Podman Compose | [OPERATOR] |

---

## Technical Architecture

### 2.1 Core Components

#### 2.1.1 Data Layer

- **Relational Database** — PostgreSQL 16+ for production, SQLite for development. Stores merchants, payment methods, transactions, subscriptions, invoices, payouts, disputes, webhooks, audit logs. Migrations via versioned migration runner.
- **Cache** — Redis 7+ for session management, rate limiting, frequently accessed merchant data, idempotency keys.
- **Object Storage** — MinIO/S3-compatible for invoice PDFs, receipts, export files, reconciliation reports.
- **Analytics Store** — PostgreSQL with materialized views for reporting; optional ClickHouse for high-volume analytics.

**Relational ↔ Analytics relationship.** The relational store is the system of record for all payment data. The analytics store holds aggregated, denormalized data for reporting and dashboards. Data flows from relational → analytics via event-driven ETL.

#### 2.1.2 Processing Engine

- **Payment Router** — Routes payment operations to the correct provider based on merchant configuration, currency, payment method, and fallback rules.
- **Webhook Processor** — Receives, validates, and processes webhook events from payment providers. Idempotent processing with exactly-once semantics.
- **Reconciliation Engine** — Periodic reconciliation of platform records against provider records. Detects discrepancies and generates alerts.
- **Retry / Back-off Engine** — Exponential back-off with jitter for failed API calls to payment providers. Circuit-breaker pattern for flapping providers.

#### 2.1.3 Event System

- **Event Bus** — NATS JetStream for durable, distributed delivery of payment events. At-least-once delivery with idempotent consumers.
- **Webhook Outgoing** — Sends webhook events to merchant-configured endpoints for real-time payment notifications.
- **Event Types** — One-time events (payment completed, refund issued) and sticky events (merchant status, balance).

#### 2.1.4 API Layer

- **REST API** — Full CRUD, /v1 path-versioned, OpenAPI/Swagger documented, served over HTTP/3 (QUIC) + HTTP/2 fallback. Auth via JWT/API keys. Rate limiting. Security headers. CORS.
- **Webhook Ingress** — Dedicated endpoints for receiving provider webhooks with signature verification.
- **Real-time** — Event Bus surfaced to clients via WebSocket/SSE.
- **SDK** — Multi-language (Go, TypeScript, Python, Java, Ruby, PHP).

#### 2.1.5 Client Applications

- **Web Portal** — Angular 19 on OpenDesign design_system. Merchant dashboard, configuration, transaction monitoring.
- **CLI** — Go/Cobra headless. Pipeline-friendly for automation.
- **TUI** — Go/Cobra + Bubble Tea. Terminal-based merchant management.
- **Desktop** — Tauri 2 (Rust core + Angular UI) for Linux/macOS/Windows.
- **Mobile** — Native per platform with KMP shared logic.

### 2.2 Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                      Clients                            │
│  (Web Browsers, Mobile Apps, CLI, API Consumers)        │
└─────────────────────────────────────────────────────────┘
                           │
                           │ HTTP/3 (QUIC) / HTTPS
                           ▼
┌─────────────────────────────────────────────────────────┐
│                   Load Balancer                         │
│              (Nginx / Traefik / Caddy)                  │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                 Go/Gin Application                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Router     │  │ Middleware  │  │  Handlers   │    │
│  │  (Gin)      │  │  Stack      │  │             │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Payment    │  │  Services   │  │  Models     │    │
│  │  Router     │  │  (Business  │  │  (Domain)   │    │
│  │             │  │   Logic)    │  │             │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Provider   │  │  Webhook    │  │  Reconcile  │    │
│  │  Adapters   │  │  Processor  │  │  Engine     │    │
│  └─────────────┘  └─────────────┘  └─────────────┘    │
└─────────────────────────────────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Postgres │ │  Redis   │ │  MinIO   │
        │ (Primary)│ │ (Cache)  │ │ (S3)     │
        └──────────┘ └──────────┘ └──────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────┐
│              Payment Providers                          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐     │
│  │ Stripe  │ │ PayPal  │ │ Square  │ │ Adyen   │ ... │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘     │
└─────────────────────────────────────────────────────────┘
```

**Explanation.** Clients (web browsers, mobile apps, CLI, API consumers) connect to the Go/Gin application through a load balancer over HTTP/3. The application routes requests through a middleware stack to handlers, which delegate to services (business logic) and the Payment Router. The Payment Router selects the correct payment provider adapter based on merchant configuration, currency, and payment method. Provider adapters communicate with external payment systems (Stripe, PayPal, Square, Adyen, etc.) via their APIs. Webhook events from providers are received through dedicated ingress endpoints, validated, and processed idempotently. All transaction data is persisted in PostgreSQL, cached in Redis, and exported to MinIO for documents and reports. The Event Bus (NATS JetStream) distributes payment events to subscribers (internal services and merchant webhook endpoints).

---

## Payment Processing Workflow

### 3.1 Payment Acquisition

1. System receives a payment request via REST API or SDK call.
2. Request includes: amount, currency, payment method, merchant ID, idempotency key, metadata.
3. Payment Router determines the target provider based on merchant configuration and rules.
4. Provider Adapter formats the request and calls the external provider API.
5. Response is normalized into the platform's unified transaction model.
6. Transaction is persisted; events are fired; merchant webhook is sent (if configured).

### 3.2 Payment Processing Pipeline

#### 3.2.1 Pre-processing

- Validate request parameters (amount, currency, payment method format).
- Check merchant balance/limits and risk thresholds.
- Resolve idempotency key (return cached result if duplicate).
- Select provider via Payment Router rules.

#### 3.2.2 Provider Execution

Each payment type triggers one or more processing steps based on the payment method and provider:

| Payment Type | Action |
|-------------|--------|
| Charge (card) | Tokenize → Authorize → Capture (or split) |
| Charge (wallet) | Redirect/QR → Poll/Callback → Confirm |
| Charge (bank) | ACH/SEPA → Verify → Debit |
| Refund | Partial/Full → Provider API → Status update |
| Subscription | Create plan → Subscribe → Schedule renewals |
| Invoice | Generate PDF → Send → Track payment |
| Payout | Aggregate → Schedule → Execute → Verify |
| Dispute | Receive evidence → Submit → Track resolution |

#### 3.2.3 Post-processing

- Normalize response to unified transaction model.
- Persist transaction with full audit trail.
- Fire events (payment.completed, payment.failed, etc.).
- Send merchant webhook (if configured).
- Update merchant balance and analytics.
- Generate receipt/invoice (if configured).

### 3.3 Concurrency, Idempotency, Retry

- **Idempotency** — Every payment request carries an idempotency key. The system claims work with a Postgres advisory lock, so a given request is processed exactly once even under retry storms.
- **Retry / back-off** — Exponential back-off with jitter and max-retries ceiling. Circuit-breaker pattern guards flapping external providers. Default: 5 retries, base 2s, factor 2.0, cap 5 min.
- **Timeout** — Per-request soft budget (30s for standard charges, 5min for complex operations like disputes). Long-running operations delegate to provider callbacks.

### 3.4 Events & Triggers

- **Transport** — NATS JetStream for durable/distributed delivery.
- **Delivery guarantee** — At-least-once, with idempotent consumers.
- **One-time vs sticky** — One-time events fire and are consumed; sticky events retain last-value with explicit invalidation.
- **Disconnected clients** — Durable consumers replay missed events on reconnect; clients also reconcile via REST snapshots.

### 3.5 Provider Abstraction

- **Adapter pattern** — Each payment provider has a dedicated adapter implementing a common interface (Charge, Refund, Subscription, etc.).
- **Config-driven** — Providers are enabled/configured per merchant via API or UI.
- **Fallback** — Automatic fallback on provider timeout, 5xx errors, or unavailable status. Merchant configures fallback order (e.g., Stripe → PayPal → Square). Circuit-breaker trips after 3 consecutive failures, temporarily removing provider from rotation. Fallback events logged and visible in dashboard.
- **Health monitoring** — Each adapter exposes health/status; circuit-breaker trips on consecutive failures.

---

## User and Account Management

### 4.1 Three-Tier Hierarchy

| Tier | Role | Permissions |
|------|------|-------------|
| 1 | Root Admin | Full system control, all accounts; only one exists; can edit all accounts/users/roles/permissions |
| 2 | Account Admin | Full control of their account and its users |
| 3 | Standard User | Consumer access to assigned accounts |

A user may belong to multiple Accounts, create their own Account (becoming its Admin), or be invited to others as Admin or user.

### 4.2 Account Features

Multi-tenancy; white-labeling (per-account branding — colors, logo, slogan — configurable by Root Admin; new accounts default to Helix Seller/Helix Development branding); role-based access control; cross-account membership; subscription + metered billing.

### 4.3 Authentication & Security

- **Auth** — JWT access + refresh (RS256 for multi-service verification), API keys with scopes for SDK/CLI, OAuth2 for linking external services. RBAC (root/account-admin/user) via policy enforcer.
- **MFA** — TOTP mandatory for Root Admin and Account Admin; optional for users.
- **Sessions** — Access token 15 min, refresh 7 d, idle timeout 30 min (web); revocation via token store.
- **Passwords** — Argon2id hashing, min 12 chars, breach-list check.
- **Encrypted storage** for all sensitive data; audit trail for all actions.

---

## Payment System Integration

### 5.1 Supported Providers (MVP)

| Provider | Capabilities | Status |
|----------|-------------|--------|
| Stripe | Charges, refunds, subscriptions, invoices, payouts, disputes, Connect | Production-ready adapter |
| PayPal | Charges, refunds, subscriptions, payouts | Production-ready adapter |
| Square | Charges, refunds, subscriptions, payouts | Production-ready adapter |
| Adyen | Charges, refunds, subscriptions, payouts, disputes | Extension required |
| Mollie | Charges, refunds, subscriptions | Extension required |

### 5.2 Unified Payment Model

All provider-specific data is normalized into a unified model:

- **Customer** — id, merchant_id, name, email, phone, payment_methods[], metadata, created_at, updated_at. First-class entity owned by merchants; required for subscriptions, invoices, and recurring billing.
- **Transaction** — id, merchant_id, customer_id (nullable for one-time charges), provider, provider_transaction_id, type (charge/refund/payout), amount, currency, status, payment_method, metadata, created_at, updated_at.
- **PaymentMethod** — id, merchant_id, customer_id (nullable for merchant-level defaults), type (card/wallet/bank), provider, tokenized_data, fingerprint, metadata, is_default.
- **Subscription** — id, merchant_id, customer_id, plan_id, status, current_period_start/end, cancel_at, metadata.
- **Invoice** — id, merchant_id, customer_id, amount, currency, status, due_date, paid_at, metadata.
- **Payout** — id, merchant_id, amount, currency, status, method, arrival_date, metadata.
- **Dispute** — id, transaction_id, reason, status, evidence_deadline, amount, metadata.

### 5.3 Webhook Handling

- **Ingress** — Dedicated endpoints per provider for receiving webhooks.
- **Signature verification** — Every webhook is verified against provider-specific signatures.
- **Idempotent processing** — Duplicate webhooks are detected and deduplicated.
- **Event normalization** — Provider-specific events are normalized to platform events.
- **Outgoing webhooks** — Merchants configure endpoint URLs; platform sends signed webhook payloads. Retry policy: exponential back-off with 5 retries over 24h (1m, 5m, 30m, 2h, 24h). Failed deliveries logged; merchant can replay via REST API.

### 5.4 Multi-Currency

- Support for 150+ currencies with automatic conversion.
- Merchant can set default currency and accept payments in multiple currencies.
- Exchange rates updated from free tier primary (exchangerate-api.com, frankfurter.app) with paid fallback (Open Exchange Rates). Rates cached in Redis with configurable TTL.
- Settlement in merchant's preferred currency.

### 5.5 PCI DSS Compliance

- **No raw card storage** — All card data tokenized via provider tokenization.
- **Token-based operations** — All card operations use provider tokens, never raw PAN.
- **Encryption** — AES-256-GCM for all sensitive data at rest.
- **Audit logging** — All payment operations logged with full trace.
- **Network security** — TLS 1.3 for all connections; no sensitive data in logs.

---

## Merchant Configuration

### 6.1 Onboarding

1. Merchant creates account (or is invited by Admin).
2. Merchant configures payment providers (API keys, webhooks).
3. Merchant sets default currency, payment methods, and policies.
4. System verifies provider credentials via test transactions.
5. Merchant is activated for live processing.

### 6.2 Configuration UI

- **Provider setup** — Step-by-step wizard for adding payment providers.
- **API key management** — Secure storage and rotation of provider credentials.
- **Webhook configuration** — Easy URL setup with test/verify functionality.
- **Payment method selection** — Toggle which payment methods to accept.
- **Branding** — White-label configuration (colors, logo, receipts).

### 6.3 Dashboard

- **Transaction overview** — Real-time transaction volume, success rate, revenue.
- **Recent transactions** — Searchable, filterable transaction list.
- **Provider status** — Health and status of connected providers.
- **Settlement tracking** — Upcoming and completed payouts.
- **Dispute management** — Active disputes with evidence submission.

### 6.4 Platform Billing

- **Model** — Transaction-fee: percentage + fixed fee per transaction (e.g., 1% + $0.10).
- **Metering** — Every processed transaction (charge, refund, subscription renewal) is metered.
- **Invoicing** — Monthly invoice generated for each merchant; deducted from settlement payouts.
- **Transparency** — Fee breakdown visible in merchant dashboard and on each transaction.
- **Adjustable** — Fee percentages configurable per merchant (Root Admin override for enterprise deals).

---

## SDK and API Development

### 7.1 SDKs

| Language | Priority |
|----------|----------|
| Go (primary) | Critical |
| TypeScript/JavaScript | Critical |
| Python | High |
| Java/Kotlin | High |
| Ruby | Medium |
| PHP | Medium |
| C# | Medium |
| Rust | Low |

Generation strategy: define REST surface in OpenAPI 3.1, codegen per language, thin hand-written idiomatic layer over generated core.

### 7.2 REST API

Full CRUD; authn/authz; real-time subscription; comprehensive error handling; URL-path versioning /v1; OpenAPI/Swagger docs.

Key endpoints:

- `/v1/merchants` — Merchant management
- `/v1/transactions` — Transaction operations (charge, refund, void)
- `/v1/subscriptions` — Subscription management
- `/v1/invoices` — Invoice generation and management
- `/v1/payouts` — Payout scheduling and status
- `/v1/disputes` — Dispute management
- `/v1/payment-methods` — Payment method tokenization
- `/v1/webhooks` — Webhook configuration
- `/v1/analytics` — Reporting and analytics
- `/v1/search` — Cross-entity search

### 7.3 Event Bus

Real-time distribution; multiple subscription patterns; persistence; sticky events + invalidation; horizontal scaling (NATS JetStream).

---

## Client Applications

### 8.1 Application Matrix

| Platform | Technology | Notes |
|----------|-----------|-------|
| Web | Angular 19 (product) / Angular 22 (marketing) on OpenDesign design_system | Primary management surface |
| Desktop | Tauri 2 (Rust + Angular) | Linux/macOS/Windows |
| Mobile | Native per platform (Compose/SwiftUI/ArkTS/Qt) + KMP shared logic | Android/iOS/HarmonyOS/Aurora |
| TUI | Go/Cobra + Bubble Tea + Lipgloss | Terminal management |
| CLI | Go/Cobra headless | Pipeline automation |

### 8.2 Design System

A single OpenDesign source fans out to per-platform variants. Web/CSS + Angular = design_system (tokens, components, Angular adapters). Light + dark mandatory. Consumed as a dependency, never vendored/forked.

### 8.3 User Experience

Full Figma design + interactive prototypes; transition effects; clear navigation; form validations, hints, tooltips. Websites meet engineering-quality bar: fully responsive, SEO-complete, WCAG AA, Core Web Vitals.

---

## Documentation Requirements

### 9.1 Structure

```
docs/
├── public/research/mvp/
│   ├── architecture/
│   ├── api/
│   ├── database/
│   ├── deployment/
│   ├── development/
│   ├── user-guides/
│   ├── testing/
│   └── design/
└── private/research/mvp/
    ├── credentials/
    ├── access/
    └── sensitive/
```

### 9.2 Types

- **Technical** — Architecture diagrams, component specs, API, DB schemas, integration, dev, testing.
- **User** — Installation, configuration, manuals, admin guide, FAQs, troubleshooting, tutorials — one per consumer group (root admin / account admin / user) and per surface.
- **Design** — UI/UX specs, wireframes, Figma, interaction flows, component library, branding.

### 9.3 Formats & Tooling

- Markdown primary; every .md kept in sync with PDF + HTML siblings.
- Diagrams via Mermaid + exported PNG/SVG, each with a multi-paragraph Markdown explanation.
- README is the canonical entry point.

---

## Localization and Internationalization

### 10.1 Supported Languages

| Language | Code | Priority |
|----------|------|----------|
| English | en | Full |
| Russian | ru | Full |
| Serbian (Cyrillic) | sr-Cyrl | Full |

### 10.2 Process & Scope

- UI fully localized (en/ru/sr-Cyrl) via i18n framework.
- Transaction data stored in original language; on-demand translation available.
- Generated reports authored primarily in English with on-demand translation.

---

## Infrastructure Requirements

### 11.1 Hosting

Provider: Hetzner; dedicated instance; fully containerized (rootless Podman Compose).

### 11.2 Environments

| Environment | Domain | Purpose |
|------------|--------|---------|
| Development | dev.seller.hxd3v.com | Development testing |
| Staging | sta.seller.hxd3v.com | Pre-production |
| Production | seller.hxd3v.com | Live system |

### 11.3 Security, Monitoring, Logging, Backup

- No credentials in public repos; runtime-load-only from gitignored .env/secrets.
- Encryption — AES-256-GCM at rest, TLS 1.3 in transit.
- Monitoring — OpenTelemetry + Prometheus + Grafana.
- Logging — Structured logging with correlation IDs.
- Audit trail — All admin/user actions logged, queryable.
- Backup/DR — Daily full + hourly DB incrementals; RPO ≈ 1 h, RTO ≈ 4 h.

---

## Testing Strategy

### 12.1 Mandated Test Types

Unit, integration, e2e, full-automation, security, DDoS, scaling, chaos, stress, performance, benchmarking, UI, UX. Mocks/stubs allowed only in unit tests; every other test type exercises the real system.

### 12.2 TDD

Reproduce-first: failing RED test → implement → same test asserts GREEN → extend to all cases.

### 12.3 Static Analysis & Continuous Quality

- SonarQube (CLI + rootless Podman server).
- Independent AI review on every change.
- Security testing covers authn/authz, secret-leak scans, fuzzing, dependency-CVE, DDoS simulations.

### 12.4 Test Frameworks

- **Go** — stdlib testing + testify, go test -race, go-sqlmock/miniredis for unit isolation.
- **TypeScript/Angular** — Jasmine + Karma (unit) + Cypress/Playwright (e2e).
- **Test banks** — Scenario engine for payment flow testing.

---

## Development Methodology

### 13.1 Process Overview

#### 13.1.1 Documentation First

All documentation is created before implementation begins: full technical specs, system diagrams, database schemas, API definitions, UI/UX designs, and test plans.

#### 13.1.2 Hierarchical Development Structure

```
Phase 1: Foundation
├── 1.1 Infrastructure (containers, DB, deployment)
├── 1.2 Core Services (User Service, Event Bus, Auth)
└── 1.3 Integration (payment provider adapters, API)

Phase 2: Payment Engine
├── 2.1 Payment Router
├── 2.2 Provider Adapters (Stripe, PayPal, Square)
├── 2.3 Webhook Processing
└── 2.4 Reconciliation

Phase 3: Client Applications
├── 3.1 Web Portal
├── 3.2 Desktop
├── 3.3 Mobile
├── 3.4 CLI/TUI
└── 3.5 Design System

Phase 4: Testing & Deployment
├── 4.1 Unit
├── 4.2 Integration
├── 4.3 System
├── 4.4 Security
├── 4.5 Performance
└── 4.6 Production Deployment
```

### 13.2 Development Principles

- **TDD** — reproduce-first: write the failing test first.
- **Design patterns** — Observer, Facade, Factory, Circuit-Breaker, Proxy, Adapter, Mediator, Strategy.
- **Principles** — DRY, KISS, Open-Closed, Single-Responsibility, SOLID.
- **Concurrency** — Heavy use of atomic, non-blocking operations.

### 13.3 Decoupling Strategy

Every component is a decoupled module, fully reusable, generic interfaces + abstract factories for variants, no hard coupling.

---

## Resolved Research Areas

| Area | Resolution |
|------|-----------|
| Payment abstraction | Adapter pattern with unified interface; config-driven provider selection |
| Transaction storage | PostgreSQL with full audit trail; pgvector for semantic search on metadata |
| Event system | NATS JetStream; at-least-once; idempotent consumers |
| Webhook handling | Dedicated ingress per provider; signature verification; idempotent processing |
| Multi-currency | 150+ currencies; automatic conversion; configurable exchange rates |
| PCI compliance | Tokenization via providers; no raw card storage; AES-256-GCM encryption |
| SDK generation | OpenAPI 3.1 + codegen per language |
| Desktop framework | Tauri 2 (Rust + Angular) — org standard |
| TUI framework | Bubble Tea + Cobra + Lipgloss |
| CI/CD | Local git-hooks + pre-tag retest; no server-side CI |
| Documentation | Markdown primary; Mermaid diagrams; PDF/HTML sync |
| Design tools | OpenDesign mandatory |

---

## Answers to Key Questions

**Q1 — Payment providers.** Stripe, PayPal, Square at MVP; Adyen, Mollie as extensions. Adapter pattern allows adding any provider.

**Q2 — Scale.** Large / multi-tenant: 100+ merchants, 10k+ transactions/day, 100+ users, multi-currency.

**Q3 — Event bus.** NATS JetStream. Not Redis/Kafka.

**Q4 — Concurrency.** Bounded by worker pool (configurable; default 32 workers), per-provider concurrency caps, idempotent single-claim per transaction.

**Q5 — Hardware.** Dev on operator workstation. Production on Hetzner dedicated host: ≥16 vCPU, ≥64 GB RAM, NVMe for Postgres, large object storage.

**Q6 — Development organization.** Multi-track subagent orchestration with workable items; no fixed sprint cadence.

**Q7 — Compliance.** PCI DSS-aware — tokenization, no raw card storage; formal PCI certification deferred.

**Q8 — Deployment architecture.** Single Hetzner host; three environments as containers behind subdomains; rootless Podman Compose.

**Q9 — Security policies.** MFA for admin tiers; Argon2id passwords; access 15 min / refresh 7 d / idle 30 min; data classification: public/internal/sensitive.

**Q10 — Auth flow.** JWT (access+refresh) for interactive clients, API keys for SDK/CLI, OAuth2 for external services; RBAC.

**Q11 — Billing.** Subscription + metered from day one.

**Q12 — Retention / DB.** PostgreSQL (production) / SQLite (dev); keep indefinitely, per-account overrides.

**Q13 — Client priorities.** Web + CLI first, then TUI, Desktop, Mobile.

**Q14 — Performance SLOs.** Aggressive: API p95 < 150 ms, dashboard load < 1.5 s.

**Q15 — Go version.** Go 1.22+ (latest stable).

**Q16 — Frontend framework.** Angular 19 (product) / Angular 22 (marketing) on OpenDesign design_system.

**Q17 — Desktop framework.** Tauri 2 (Rust + Angular UI).

**Q18 — TUI framework.** Bubble Tea + Cobra + Lipgloss.

**Q19 — CI/CD.** No server-side CI; local git-hooks + pre-tag retest + all-upstreams push.

**Q20 — Documentation priority.** Architecture → API → DB → deployment → development → testing → user-guides → design.

**Q21 — Testing priorities.** Critical-path + security first; then all mandated test types.

**Q22 — API versioning.** URL-path /v1 (+ OpenAPI).

**Q23 — Data migration.** Automated, versioned migrations; expand-contract for zero-downtime.

**Q24 — Encryption.** AES-256-GCM at rest; TLS 1.3 in transit.

**Q25 — Monitoring.** Prometheus + Grafana + OpenTelemetry.

**Q26 — Backup frequency.** Daily full + hourly DB incrementals.

**Q27 — Recovery plan.** RPO ≈ 1 h, RTO ≈ 4 h; documented restore runbook.

---

## Assumptions

- Users have stable internet connectivity.
- Payment providers have reliable APIs with documented webhook formats.
- Merchants have existing provider accounts (Stripe, PayPal, etc.) before using the platform.
- The platform does not hold funds — it facilitates routing to providers.
- Mobile support is phase 2 after Web + CLI are production-ready.
- The system runs on a single dedicated host for MVP; horizontal scaling is phase 3.
- Exchange rates are fetched from a reliable external source and cached.
- All payment operations are idempotent and auditable.
- The platform processes payments in real-time but settlement/reconciliation is batch-oriented.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Merchants can configure a payment provider in under 5 minutes.
- **SC-002**: First payment can be processed within 10 minutes of account creation.
- **SC-003**: API response time p95 < 150 ms for standard operations.
- **SC-004**: System handles 100+ concurrent merchants without degradation.
- **SC-005**: 99.9% uptime during business hours.
- **SC-006**: All payment operations are idempotent — retrying a request produces the same result.
- **SC-007**: Webhook delivery success rate > 99.5% within 30 seconds.
- **SC-008**: Merchant dashboard loads in under 1.5 seconds.
- **SC-009**: All transactions are fully auditable with complete trace.
- **SC-010**: SDKs available for Go, TypeScript, Python at launch.

---

**Last Updated:** 2026-07-23
**Project:** Helix Seller
**Tech Stack:** Go, Gin Gonic, HTTP3 (QUIC/Cronet), PostgreSQL, Redis
**Design Tool:** OpenDesign
**Branding:** Helix Seller / HelixSeller / helix_seller
