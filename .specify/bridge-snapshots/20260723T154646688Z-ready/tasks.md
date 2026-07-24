# Tasks: Helix Seller Platform

**Input**: Design documents from `/specs/001-helix-seller-platform/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: TDD approach (RED → GREEN → REFACTOR). Each task includes test task before implementation.

**Organization**: Tasks grouped by user story for independent implementation. Each task is self-contained — a local model (32GB VRAM) can implement it by reading ONLY the task description + referenced context files.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3...)
- File paths are absolute from project root
- Each task references ONLY the files it needs to create or modify

## Context Files Per Task

Each task lists its **Context Files** — the minimum set of files a model must read to implement the task. Models should NOT load other project files.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, Go module, directory structure, config

- [ ] T001 Initialize Go module `go.mod` with module path `github.com/helix-seller/helix-seller` and Go 1.22
  - **Context Files**: `go.mod`
  - **Creates**: `go.mod`

- [ ] T002 Create directory structure per plan.md layout
  - **Context Files**: `plan.md`
  - **Creates**: `cmd/server/`, `cmd/migrate/`, `internal/config/`, `internal/database/`, `internal/handler/`, `internal/middleware/`, `internal/model/`, `internal/repository/`, `internal/service/`, `internal/provider/`, `internal/eventbus/`, `internal/validator/`, `pkg/errors/`, `pkg/logger/`, `pkg/crypto/`, `pkg/utils/`, `web/src/`, `tests/contract/`, `tests/integration/`, `tests/unit/`, `migrations/`, `scripts/`

- [ ] T003 [P] Create `.env.example` with all environment variables
  - **Context Files**: `plan.md` (Technical Context section)
  - **Creates**: `.env.example`
  - **Content**: DATABASE_URL, REDIS_URL, SERVER_PORT, JWT_SECRET, LOG_LEVEL, STRIPE_API_KEY, PAYPAL_CLIENT_ID, PAYPAL_SECRET, SQUARE_ACCESS_TOKEN, NATS_URL, ENCRYPTION_KEY

- [ ] T004 [P] Create `Makefile` with build, run, test, lint, deps targets
  - **Context Files**: `plan.md`
  - **Creates**: `Makefile`

- [ ] T005 [P] Create `internal/config/config.go` — Config struct + `Load()` from env
  - **Context Files**: `internal/config/config.go` (existing skeleton), `.env.example`
  - **Modifies**: `internal/config/config.go`
  - **Dependencies**: T003

- [ ] T006 [P] Create `cmd/server/main.go` — Server entry point with graceful shutdown
  - **Context Files**: `cmd/server/main.go` (existing skeleton), `internal/config/config.go`
  - **Modifies**: `cmd/server/main.go`
  - **Dependencies**: T005

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database, auth, middleware, error handling — ALL user stories depend on these

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Database Layer

- [ ] T007 Create `internal/database/postgres.go` — PostgreSQL connection pool with health check
  - **Context Files**: `internal/config/config.go` (Config struct)
  - **Creates**: `internal/database/postgres.go`
  - **Dependencies**: T005
  - **External**: `github.com/jackc/pgx/v5`

- [ ] T008 [P] Create `internal/database/redis.go` — Redis client connection
  - **Context Files**: `internal/config/config.go`
  - **Creates**: `internal/database/redis.go`
  - **Dependencies**: T005
  - **External**: `github.com/redis/go-redis/v9`

- [ ] T009 [P] Create `migrations/001_create_merchants.up.sql` — Merchant table migration
  - **Context Files**: `data-model.md` (Merchant entity)
  - **Creates**: `migrations/001_create_merchants.up.sql`
  - **Content**: CREATE TABLE merchants with all fields from data-model.md

- [ ] T010 [P] Create `migrations/001_create_merchants.down.sql` — Rollback migration
  - **Context Files**: `migrations/001_create_merchants.up.sql`
  - **Creates**: `migrations/001_create_merchants.down.sql`

- [ ] T011 [P] Create `migrations/002_create_customers.up.sql` — Customer table
  - **Context Files**: `data-model.md` (Customer entity)
  - **Creates**: `migrations/002_create_customers.up.sql`

- [ ] T012 [P] Create `migrations/002_create_customers.down.sql`
  - **Context Files**: `migrations/002_create_customers.up.sql`
  - **Creates**: `migrations/002_create_customers.down.sql`

- [ ] T013 [P] Create `migrations/003_create_payment_methods.up.sql`
  - **Context Files**: `data-model.md` (PaymentMethod entity)
  - **Creates**: `migrations/003_create_payment_methods.up.sql`

- [ ] T014 [P] Create `migrations/003_create_payment_methods.down.sql`
  - **Context Files**: `migrations/003_create_payment_methods.up.sql`
  - **Creates**: `migrations/003_create_payment_methods.down.sql`

- [ ] T015 [P] Create `migrations/004_create_transactions.up.sql`
  - **Context Files**: `data-model.md` (Transaction entity)
  - **Creates**: `migrations/004_create_transactions.up.sql`
  - **Note**: Include monthly partitioning by created_at

- [ ] T016 [P] Create `migrations/004_create_transactions.down.sql`
  - **Context Files**: `migrations/004_create_transactions.up.sql`
  - **Creates**: `migrations/004_create_transactions.down.sql`

- [ ] T017 [P] Create `migrations/005_create_subscriptions.up.sql`
  - **Context Files**: `data-model.md` (Subscription entity)
  - **Creates**: `migrations/005_create_subscriptions.up.sql`

- [ ] T018 [P] Create `migrations/005_create_subscriptions.down.sql`
  - **Context Files**: `migrations/005_create_subscriptions.up.sql`
  - **Creates**: `migrations/005_create_subscriptions.down.sql`

- [ ] T019 [P] Create `migrations/006_create_invoices.up.sql`
  - **Context Files**: `data-model.md` (Invoice entity)
  - **Creates**: `migrations/006_create_invoices.up.sql`

- [ ] T020 [P] Create `migrations/006_create_invoices.down.sql`
  - **Context Files**: `migrations/006_create_invoices.up.sql`
  - **Creates**: `migrations/006_create_invoices.down.sql`

- [ ] T021 [P] Create `migrations/007_create_payouts.up.sql`
  - **Context Files**: `data-model.md` (Payout entity)
  - **Creates**: `migrations/007_create_payouts.up.sql`

- [ ] T022 [P] Create `migrations/007_create_payouts.down.sql`
  - **Context Files**: `migrations/007_create_payouts.up.sql`
  - **Creates**: `migrations/007_create_payouts.down.sql`

- [ ] T023 [P] Create `migrations/008_create_disputes.up.sql`
  - **Context Files**: `data-model.md` (Dispute entity)
  - **Creates**: `migrations/008_create_disputes.up.sql`

- [ ] T024 [P] Create `migrations/008_create_disputes.down.sql`
  - **Context Files**: `migrations/008_create_disputes.up.sql`
  - **Creates**: `migrations/008_create_disputes.down.sql`

- [ ] T025 [P] Create `migrations/009_create_webhook_configs.up.sql`
  - **Context Files**: `data-model.md` (WebhookConfig entity)
  - **Creates**: `migrations/009_create_webhook_configs.up.sql`

- [ ] T026 [P] Create `migrations/009_create_webhook_configs.down.sql`
  - **Context Files**: `migrations/009_create_webhook_configs.up.sql`
  - **Creates**: `migrations/009_create_webhook_configs.down.sql`

- [ ] T027 [P] Create `migrations/010_create_provider_configs.up.sql`
  - **Context Files**: `data-model.md` (ProviderConfig entity)
  - **Creates**: `migrations/010_create_provider_configs.up.sql`

- [ ] T028 [P] Create `migrations/010_create_provider_configs.down.sql`
  - **Context Files**: `migrations/010_create_provider_configs.up.sql`
  - **Creates**: `migrations/010_create_provider_configs.down.sql`

- [ ] T029 [P] Create `migrations/011_create_audit_logs.up.sql`
  - **Context Files**: `data-model.md` (AuditLog entity)
  - **Creates**: `migrations/011_create_audit_logs.up.sql`

- [ ] T030 [P] Create `migrations/011_create_audit_logs.down.sql`
  - **Context Files**: `migrations/011_create_audit_logs.up.sql`
  - **Creates**: `migrations/011_create_audit_logs.down.sql`

- [ ] T031 [P] Create `migrations/012_create_exchange_rates.up.sql`
  - **Context Files**: `data-model.md` (ExchangeRate entity)
  - **Creates**: `migrations/012_create_exchange_rates.up.sql`

- [ ] T032 [P] Create `migrations/012_create_exchange_rates.down.sql`
  - **Context Files**: `migrations/012_create_exchange_rates.up.sql`
  - **Creates**: `migrations/012_create_exchange_rates.down.sql`

- [ ] T033 [P] Create `migrations/013_create_idempotency_keys.up.sql`
  - **Context Files**: `data-model.md` (IdempotencyKey entity)
  - **Creates**: `migrations/013_create_idempotency_keys.up.sql`

- [ ] T034 [P] Create `migrations/013_create_idempotency_keys.down.sql`
  - **Context Files**: `migrations/013_create_idempotency_keys.up.sql`
  - **Creates**: `migrations/013_create_idempotency_keys.down.sql`

- [ ] T035 [P] Create `migrations/014_create_background_tasks.up.sql`
  - **Context Files**: `data-model.md` (BackgroundTask entity)
  - **Creates**: `migrations/014_create_background_tasks.up.sql`

- [ ] T036 [P] Create `migrations/014_create_background_tasks.down.sql`
  - **Context Files**: `migrations/014_create_background_tasks.up.sql`
  - **Creates**: `migrations/014_create_background_tasks.down.sql`

### Domain Models

- [ ] T037 [P] Create `internal/model/merchant.go` — Merchant struct + validation
  - **Context Files**: `data-model.md` (Merchant entity), `contracts/api-v1.yaml` (Merchant schema)
  - **Creates**: `internal/model/merchant.go`

- [ ] T038 [P] Create `internal/model/customer.go` — Customer struct + validation
  - **Context Files**: `data-model.md` (Customer entity), `contracts/api-v1.yaml` (Customer schema)
  - **Creates**: `internal/model/customer.go`

- [ ] T039 [P] Create `internal/model/payment_method.go` — PaymentMethod struct
  - **Context Files**: `data-model.md` (PaymentMethod entity)
  - **Creates**: `internal/model/payment_method.go`

- [ ] T040 [P] Create `internal/model/transaction.go` — Transaction struct + enums
  - **Context Files**: `data-model.md` (Transaction entity), `contracts/api-v1.yaml` (Transaction schema)
  - **Creates**: `internal/model/transaction.go`

- [ ] T041 [P] Create `internal/model/subscription.go` — Subscription struct
  - **Context Files**: `data-model.md` (Subscription entity)
  - **Creates**: `internal/model/subscription.go`

- [ ] T042 [P] Create `internal/model/invoice.go` — Invoice struct
  - **Context Files**: `data-model.md` (Invoice entity)
  - **Creates**: `internal/model/invoice.go`

- [ ] T043 [P] Create `internal/model/payout.go` — Payout struct
  - **Context Files**: `data-model.md` (Payout entity)
  - **Creates**: `internal/model/payout.go`

- [ ] T044 [P] Create `internal/model/dispute.go` — Dispute struct
  - **Context Files**: `data-model.md` (Dispute entity)
  - **Creates**: `internal/model/dispute.go`

- [ ] T045 [P] Create `internal/model/webhook_config.go` — WebhookConfig struct
  - **Context Files**: `data-model.md` (WebhookConfig entity)
  - **Creates**: `internal/model/webhook_config.go`

- [ ] T046 [P] Create `internal/model/provider_config.go` — ProviderConfig struct
  - **Context Files**: `data-model.md` (ProviderConfig entity)
  - **Creates**: `internal/model/provider_config.go`

- [ ] T047 [P] Create `internal/model/audit_log.go` — AuditLog struct
  - **Context Files**: `data-model.md` (AuditLog entity)
  - **Creates**: `internal/model/audit_log.go`

- [ ] T048 [P] Create `internal/model/background_task.go` — BackgroundTask struct
  - **Context Files**: `data-model.md` (BackgroundTask entity)
  - **Creates**: `internal/model/background_task.go`

- [ ] T049 [P] Create `internal/model/exchange_rate.go` — ExchangeRate struct
  - **Context Files**: `data-model.md` (ExchangeRate entity)
  - **Creates**: `internal/model/exchange_rate.go`

- [ ] T050 [P] Create `internal/model/idempotency_key.go` — IdempotencyKey struct
  - **Context Files**: `data-model.md` (IdempotencyKey entity)
  - **Creates**: `internal/model/idempotency_key.go`

- [ ] T051 [P] Create `internal/model/pagination.go` — PaginationParams + PaginatedResponse
  - **Context Files**: `contracts/api-v1.yaml` (PaginatedResponse schemas)
  - **Creates**: `internal/model/pagination.go`

- [ ] T052 [P] Create `internal/model/errors.go` — Domain error types (NotFoundError, ValidationError, ConflictError, etc.)
  - **Context Files**: `contracts/api-v1.yaml` (ErrorResponse schema)
  - **Creates**: `internal/model/errors.go`

### Error Handling & Logging

- [ ] T053 [P] Create `pkg/errors/errors.go` — Error wrapping, HTTP status mapping
  - **Context Files**: `internal/model/errors.go`
  - **Creates**: `pkg/errors/errors.go`

- [ ] T054 [P] Create `pkg/logger/logger.go` — Structured logging (zap)
  - **Context Files**: `internal/config/config.go` (LogLevel field)
  - **Creates**: `pkg/logger/logger.go`

### Middleware

- [ ] T055 Create `internal/middleware/recovery.go` — Panic recovery middleware
  - **Context Files**: None (standard Go middleware)
  - **Creates**: `internal/middleware/recovery.go`

- [ ] T056 [P] Create `internal/middleware/cors.go` — CORS configuration
  - **Context Files**: None (standard Go middleware)
  - **Creates**: `internal/middleware/cors.go`

- [ ] T057 [P] Create `internal/middleware/request_id.go` — Request ID generation
  - **Context Files**: None
  - **Creates**: `internal/middleware/request_id.go`

- [ ] T058 [P] Create `internal/middleware/logger.go` — Request/response logging
  - **Context Files**: `pkg/logger/logger.go`
  - **Creates**: `internal/middleware/logger.go`
  - **Dependencies**: T054

- [ ] T059 Create `internal/middleware/auth.go` — JWT + API key authentication
  - **Context Files**: `internal/config/config.go` (JWTSecret), `internal/model/errors.go`
  - **Creates**: `internal/middleware/auth.go`
  - **Dependencies**: T052, T053
  - **External**: `github.com/golang-jwt/jwt/v5`

- [ ] T060 [P] Create `internal/middleware/rate_limit.go` — Rate limiting middleware
  - **Context Files**: `internal/database/redis.go`
  - **Creates**: `internal/middleware/rate_limit.go`
  - **Dependencies**: T008

- [ ] T061 [P] Create `internal/middleware/audit.go` — Audit logging middleware
  - **Context Files**: `internal/model/audit_log.go`, `internal/database/postgres.go`
  - **Creates**: `internal/middleware/audit.go`
  - **Dependencies**: T007, T047

### Validation

- [ ] T062 [P] Create `internal/validator/validator.go` — Custom validators (amount > 0, valid currency, etc.)
  - **Context Files**: `data-model.md` (validation rules)
  - **Creates**: `internal/validator/validator.go`

### Router Setup

- [ ] T063 Create `internal/handler/router.go` — Gin router with all route groups
  - **Context Files**: `contracts/api-v1.yaml` (all endpoints), `internal/middleware/auth.go`
  - **Creates**: `internal/handler/router.go`
  - **Dependencies**: T055, T056, T057, T058, T059, T060, T061

### Background Task Queue

- [ ] T064 Create `internal/service/background.go` — Postgres-backed task queue (claim, execute, retry)
  - **Context Files**: `internal/model/background_task.go`, `internal/database/postgres.go`
  - **Creates**: `internal/service/background.go`
  - **Dependencies**: T007, T048

### Event Bus

- [ ] T065 Create `internal/eventbus/eventbus.go` — NATS JetStream event bus interface + implementation
  - **Context Files**: `internal/config/config.go`
  - **Creates**: `internal/eventbus/eventbus.go`
  - **External**: `github.com/nats-io/nats.go`

### Utility Packages

- [ ] T066 [P] Create `pkg/crypto/encrypt.go` — AES-256-GCM encryption/decryption helpers
  - **Context Files**: None (standard Go crypto)
  - **Creates**: `pkg/crypto/encrypt.go`

- [ ] T067 [P] Create `pkg/utils/uuid.go` — UUID generation helper
  - **Context Files**: None
  - **Creates**: `pkg/utils/uuid.go`
  - **External**: `github.com/google/uuid`

- [ ] T068 [P] Create `pkg/utils/currency.go` — Currency validation helper (ISO 4217)
  - **Context Files**: None
  - **Creates**: `pkg/utils/currency.go`

- [ ] T069 [P] Create `pkg/utils/time.go` — Time parsing helpers
  - **Context Files**: None
  - **Creates**: `pkg/utils/time.go`

**Checkpoint**: Foundation ready — user story implementation can now begin

---

## Phase 3: User Story 1 — Merchant Management (Priority: P1) 🎯 MVP

**Goal**: Merchant can create account, configure payment providers, and manage settings

**Independent Test**: Create merchant → configure Stripe → verify credentials via test transaction → list merchants

### Repository Layer

- [ ] T070 Create `internal/repository/merchant_repo.go` — CRUD operations for Merchant
  - **Context Files**: `internal/model/merchant.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/merchant_repo.go`
  - **Dependencies**: T007, T037

- [ ] T071 [P] Create `internal/repository/provider_config_repo.go` — CRUD for ProviderConfig
  - **Context Files**: `internal/model/provider_config.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/provider_config_repo.go`
  - **Dependencies**: T007, T046

### Service Layer

- [ ] T072 Create `internal/service/merchant.go` — Merchant business logic (create, update, get, list)
  - **Context Files**: `internal/repository/merchant_repo.go`, `internal/model/merchant.go`
  - **Creates**: `internal/service/merchant.go`
  - **Dependencies**: T070

- [ ] T073 Create `internal/service/provider.go` — Provider configuration logic (configure, verify, health check)
  - **Context Files**: `internal/repository/provider_config_repo.go`, `internal/model/provider_config.go`
  - **Creates**: `internal/service/provider.go`
  - **Dependencies**: T071

### Handler Layer

- [ ] T074 [P] Create `internal/handler/merchant.go` — Merchant HTTP handlers
  - **Context Files**: `internal/service/merchant.go`, `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `internal/handler/merchant.go`
  - **Dependencies**: T072

- [ ] T075 [P] Create `internal/handler/provider.go` — Provider configuration HTTP handlers
  - **Context Files**: `internal/service/provider.go`, `contracts/api-v1.yaml` (Provider endpoints)
  - **Creates**: `internal/handler/provider.go`
  - **Dependencies**: T073

### Registration

- [ ] T076 Register merchant routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/merchant.go`, `internal/handler/provider.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T074, T075

**Checkpoint**: Merchant management fully functional — merchants can be created, updated, and providers configured

---

## Phase 4: User Story 2 — Process Payments (Priority: P1) 🎯 MVP

**Goal**: Process charges, refunds through unified API across Stripe, PayPal, Square

**Independent Test**: Create customer → process charge → verify transaction → refund → verify refund

### Provider Adapters

- [ ] T077 Create `internal/provider/adapter.go` — PaymentProvider interface definition
  - **Context Files**: `data-model.md` (Transaction entity), `research.md` (Payment Provider Integration)
  - **Creates**: `internal/provider/adapter.go`
  - **Dependencies**: T040

- [ ] T078 Create `internal/provider/stripe.go` — Stripe adapter (charge, refund, verify)
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Stripe Integration)
  - **Creates**: `internal/provider/stripe.go`
  - **Dependencies**: T077
  - **External**: `github.com/stripe/stripe-go/v76`

- [ ] T079 [P] Create `internal/provider/paypal.go` — PayPal adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (PayPal Integration)
  - **Creates**: `internal/provider/paypal.go`
  - **Dependencies**: T077

- [ ] T080 [P] Create `internal/provider/square.go` — Square adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Square Integration)
  - **Creates**: `internal/provider/square.go`
  - **Dependencies**: T077

### Payment Router

- [ ] T081 Create `internal/service/payment_router.go` — Provider selection, fallback, circuit breaker
  - **Context Files**: `internal/provider/adapter.go`, `internal/model/provider_config.go`, `research.md` (Circuit Breaker)
  - **Creates**: `internal/service/payment_router.go`
  - **Dependencies**: T077, T078, T079, T080, T073

### Transaction Repository

- [ ] T082 Create `internal/repository/transaction_repo.go` — Transaction CRUD + idempotency
  - **Context Files**: `internal/model/transaction.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/transaction_repo.go`
  - **Dependencies**: T007, T040

### Transaction Service

- [ ] T083 Create `internal/service/transaction.go` — Transaction business logic (charge, refund, get, list)
  - **Context Files**: `internal/repository/transaction_repo.go`, `internal/service/payment_router.go`, `internal/model/transaction.go`
  - **Creates**: `internal/service/transaction.go`
  - **Dependencies**: T082, T081

### Transaction Handler

- [ ] T084 [P] Create `internal/handler/transaction.go` — Transaction HTTP handlers
  - **Context Files**: `internal/service/transaction.go`, `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `internal/handler/transaction.go`
  - **Dependencies**: T083

### Webhook Ingress

- [ ] T085 Create `internal/handler/webhook.go` — Webhook ingress endpoints (Stripe, PayPal, Square)
  - **Context Files**: `internal/service/payment_router.go`, `contracts/api-v1.yaml` (Webhook endpoints)
  - **Creates**: `internal/handler/webhook.go`
  - **Dependencies**: T081

### Registration

- [ ] T086 Register transaction routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/transaction.go`, `internal/handler/webhook.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T084, T085

**Checkpoint**: Payment processing fully functional — charges and refunds work across all providers

---

## Phase 5: User Story 3 — Customer Management (Priority: P1) 🎯 MVP

**Goal**: Manage customers, payment methods, and tokenized card storage

**Independent Test**: Create customer → add payment method → list payment methods → delete payment method

### Repository Layer

- [ ] T087 Create `internal/repository/customer_repo.go` — Customer CRUD operations
  - **Context Files**: `internal/model/customer.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/customer_repo.go`
  - **Dependencies**: T007, T038

- [ ] T088 [P] Create `internal/repository/payment_method_repo.go` — PaymentMethod CRUD
  - **Context Files**: `internal/model/payment_method.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/payment_method_repo.go`
  - **Dependencies**: T007, T039

### Service Layer

- [ ] T089 Create `internal/service/customer.go` — Customer business logic
  - **Context Files**: `internal/repository/customer_repo.go`, `internal/model/customer.go`
  - **Creates**: `internal/service/customer.go`
  - **Dependencies**: T087

- [ ] T090 Create `internal/service/payment_method.go` — PaymentMethod business logic
  - **Context Files**: `internal/repository/payment_method_repo.go`, `internal/model/payment_method.go`
  - **Creates**: `internal/service/payment_method.go`
  - **Dependencies**: T088

### Handler Layer

- [ ] T091 [P] Create `internal/handler/customer.go` — Customer HTTP handlers
  - **Context Files**: `internal/service/customer.go`, `contracts/api-v1.yaml` (Customer endpoints)
  - **Creates**: `internal/handler/customer.go`
  - **Dependencies**: T089

- [ ] T092 [P] Create `internal/handler/payment_method.go` — PaymentMethod HTTP handlers
  - **Context Files**: `internal/service/payment_method.go`, `contracts/api-v1.yaml` (PaymentMethod endpoints)
  - **Creates**: `internal/handler/payment_method.go`
  - **Dependencies**: T090

### Registration

- [ ] T093 Register customer routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/customer.go`, `internal/handler/payment_method.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T091, T092

**Checkpoint**: Customer management fully functional — customers and payment methods can be managed

---

## Phase 6: User Story 4 — Subscriptions & Invoicing (Priority: P1) 🎯 MVP

**Goal**: Manage recurring subscriptions and generate invoices

**Independent Test**: Create subscription → list subscriptions → cancel subscription → verify invoice generated

### Repository Layer

- [ ] T094 Create `internal/repository/subscription_repo.go` — Subscription CRUD
  - **Context Files**: `internal/model/subscription.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/subscription_repo.go`
  - **Dependencies**: T007, T041

- [ ] T095 [P] Create `internal/repository/invoice_repo.go` — Invoice CRUD
  - **Context Files**: `internal/model/invoice.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/invoice_repo.go`
  - **Dependencies**: T007, T042

### Service Layer

- [ ] T096 Create `internal/service/subscription.go` — Subscription business logic
  - **Context Files**: `internal/repository/subscription_repo.go`, `internal/model/subscription.go`
  - **Creates**: `internal/service/subscription.go`
  - **Dependencies**: T094

- [ ] T097 Create `internal/service/invoice.go` — Invoice business logic
  - **Context Files**: `internal/repository/invoice_repo.go`, `internal/model/invoice.go`
  - **Creates**: `internal/service/invoice.go`
  - **Dependencies**: T095

### Handler Layer

- [ ] T098 [P] Create `internal/handler/subscription.go` — Subscription HTTP handlers
  - **Context Files**: `internal/service/subscription.go`, `contracts/api-v1.yaml` (Subscription endpoints)
  - **Creates**: `internal/handler/subscription.go`
  - **Dependencies**: T096

- [ ] T099 [P] Create `internal/handler/invoice.go` — Invoice HTTP handlers
  - **Context Files**: `internal/service/invoice.go`, `contracts/api-v1.yaml` (Invoice endpoints)
  - **Creates**: `internal/handler/invoice.go`
  - **Dependencies**: T097

### Registration

- [ ] T100 Register subscription routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/subscription.go`, `internal/handler/invoice.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T098, T099

**Checkpoint**: Subscriptions and invoicing fully functional

---

## Phase 7: User Story 5 — Payouts & Disputes (Priority: P2)

**Goal**: Manage merchant payouts and handle payment disputes

**Independent Test**: Create payout → verify payout status → create dispute → submit evidence → verify resolution

### Repository Layer

- [ ] T101 Create `internal/repository/payout_repo.go` — Payout CRUD
  - **Context Files**: `internal/model/payout.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/payout_repo.go`
  - **Dependencies**: T007, T043

- [ ] T102 [P] Create `internal/repository/dispute_repo.go` — Dispute CRUD
  - **Context Files**: `internal/model/dispute.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/dispute_repo.go`
  - **Dependencies**: T007, T044

### Service Layer

- [ ] T103 Create `internal/service/payout.go` — Payout business logic
  - **Context Files**: `internal/repository/payout_repo.go`, `internal/model/payout.go`
  - **Creates**: `internal/service/payout.go`
  - **Dependencies**: T101

- [ ] T104 Create `internal/service/dispute.go` — Dispute business logic
  - **Context Files**: `internal/repository/dispute_repo.go`, `internal/model/dispute.go`
  - **Creates**: `internal/service/dispute.go`
  - **Dependencies**: T102

### Handler Layer

- [ ] T105 [P] Create `internal/handler/payout.go` — Payout HTTP handlers
  - **Context Files**: `internal/service/payout.go`, `contracts/api-v1.yaml` (Payout endpoints)
  - **Creates**: `internal/handler/payout.go`
  - **Dependencies**: T103

- [ ] T106 [P] Create `internal/handler/dispute.go` — Dispute HTTP handlers
  - **Context Files**: `internal/service/dispute.go`, `contracts/api-v1.yaml` (Dispute endpoints)
  - **Creates**: `internal/handler/dispute.go`
  - **Dependencies**: T104

### Registration

- [ ] T107 Register payout routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/payout.go`, `internal/handler/dispute.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T105, T106

**Checkpoint**: Payouts and disputes fully functional

---

## Phase 8: User Story 6 — Webhooks & Events (Priority: P2)

**Goal**: Configure outgoing webhooks and process incoming webhook events

**Independent Test**: Configure webhook endpoint → trigger event → verify webhook delivered → retry failed webhook

### Repository Layer

- [ ] T108 Create `internal/repository/webhook_config_repo.go` — WebhookConfig CRUD
  - **Context Files**: `internal/model/webhook_config.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/webhook_config_repo.go`
  - **Dependencies**: T007, T045

### Service Layer

- [ ] T109 Create `internal/service/webhook.go` — Webhook delivery, retry, signature verification
  - **Context Files**: `internal/repository/webhook_config_repo.go`, `internal/model/webhook_config.go`
  - **Creates**: `internal/service/webhook.go`
  - **Dependencies**: T108

### Handler Layer

- [ ] T110 [P] Create `internal/handler/webhook_config.go` — WebhookConfig HTTP handlers
  - **Context Files**: `internal/service/webhook.go`, `contracts/api-v1.yaml` (WebhookConfig endpoints)
  - **Creates**: `internal/handler/webhook_config.go`
  - **Dependencies**: T109

### Registration

- [ ] T111 Register webhook routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/webhook_config.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T110

**Checkpoint**: Webhook system fully functional — outgoing webhooks can be configured and delivered

---

## Phase 9: User Story 7 — Multi-Currency & Exchange Rates (Priority: P2)

**Goal**: Support multi-currency transactions with automatic conversion

**Independent Test**: Create transaction in EUR → verify USD conversion → verify exchange rate cached

### Repository Layer

- [ ] T112 Create `internal/repository/exchange_rate_repo.go` — ExchangeRate CRUD
  - **Context Files**: `internal/model/exchange_rate.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/exchange_rate_repo.go`
  - **Dependencies**: T007, T049

### Service Layer

- [ ] T113 Create `internal/service/exchange_rate.go` — Exchange rate fetching, caching, conversion
  - **Context Files**: `internal/repository/exchange_rate_repo.go`, `internal/model/exchange_rate.go`, `research.md` (Exchange Rate Research)
  - **Creates**: `internal/service/exchange_rate.go`
  - **Dependencies**: T112

### Handler Layer

- [ ] T114 [P] Create `internal/handler/exchange_rate.go` — ExchangeRate HTTP handlers
  - **Context Files**: `internal/service/exchange_rate.go`, `contracts/api-v1.yaml` (ExchangeRate endpoints)
  - **Creates**: `internal/handler/exchange_rate.go`
  - **Dependencies**: T113

### Registration

- [ ] T115 Register exchange rate routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/exchange_rate.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T114

**Checkpoint**: Multi-currency support fully functional

---

## Phase 10: User Story 8 — Analytics & Reporting (Priority: P3)

**Goal**: Provide transaction analytics and financial reporting

**Independent Test**: Process transactions → query analytics → verify aggregated data → export report

### Repository Layer

- [ ] T116 Create `internal/repository/analytics_repo.go` — Analytics queries (aggregations, trends)
  - **Context Files**: `internal/database/postgres.go`, `data-model.md` (Transaction entity)
  - **Creates**: `internal/repository/analytics_repo.go`
  - **Dependencies**: T007, T040

### Service Layer

- [ ] T117 Create `internal/service/analytics.go` — Analytics business logic
  - **Context Files**: `internal/repository/analytics_repo.go`
  - **Creates**: `internal/service/analytics.go`
  - **Dependencies**: T116

### Handler Layer

- [ ] T118 [P] Create `internal/handler/analytics.go` — Analytics HTTP handlers
  - **Context Files**: `internal/service/analytics.go`, `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `internal/handler/analytics.go`
  - **Dependencies**: T117

### Registration

- [ ] T119 Register analytics routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/analytics.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T118

**Checkpoint**: Analytics and reporting fully functional

---

## Phase 11: User Story 9 — Web Portal (Priority: P3)

**Goal**: Angular web portal for merchant dashboard and configuration

**Independent Test**: Login → view dashboard → configure providers → view transactions → logout

### Frontend Setup

- [ ] T120 Initialize Angular 19 project in `web/`
  - **Context Files**: `plan.md` (Frontend section)
  - **Creates**: `web/src/`, `web/package.json`, `web/angular.json`

- [ ] T121 [P] Create `web/src/app/core/` — Core module (auth guard, interceptors, services)
  - **Context Files**: `contracts/api-v1.yaml` (auth endpoints)
  - **Creates**: `web/src/app/core/`

- [ ] T122 [P] Create `web/src/app/shared/` — Shared module (components, pipes, directives)
  - **Context Files**: None
  - **Creates**: `web/src/app/shared/`

### Dashboard

- [ ] T123 Create `web/src/app/pages/dashboard/` — Dashboard page (transaction overview, charts)
  - **Context Files**: `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `web/src/app/pages/dashboard/`

### Merchant Management

- [ ] T124 [P] Create `web/src/app/pages/merchants/` — Merchant list, create, edit pages
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `web/src/app/pages/merchants/`

### Transaction Management

- [ ] T125 [P] Create `web/src/app/pages/transactions/` — Transaction list, detail pages
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `web/src/app/pages/transactions/`

### Provider Configuration

- [ ] T126 [P] Create `web/src/app/pages/providers/` — Provider configuration wizard
  - **Context Files**: `contracts/api-v1.yaml` (Provider endpoints)
  - **Creates**: `web/src/app/pages/providers/`

### Customer Management

- [ ] T127 [P] Create `web/src/app/pages/customers/` — Customer list, detail pages
  - **Context Files**: `contracts/api-v1.yaml` (Customer endpoints)
  - **Creates**: `web/src/app/pages/customers/`

### Webhook Configuration

- [ ] T128 [P] Create `web/src/app/pages/webhooks/` — Webhook configuration page
  - **Context Files**: `contracts/api-v1.yaml` (Webhook endpoints)
  - **Creates**: `web/src/app/pages/webhooks/`

**Checkpoint**: Web portal fully functional — merchants can manage everything via UI

---

## Phase 12: User Story 10 — CLI/TUI (Priority: P3)

**Goal**: Command-line interface for pipeline automation and terminal-based management

**Independent Test**: List merchants → create transaction → view analytics → export report

### CLI Setup

- [ ] T129 Create `cmd/cli/main.go` — CLI entry point with Cobra
  - **Context Files**: `plan.md` (CLI section)
  - **Creates**: `cmd/cli/main.go`
  - **External**: `github.com/spf13/cobra`

- [ ] T130 [P] Create `internal/cli/merchant.go` — Merchant CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `internal/cli/merchant.go`

- [ ] T131 [P] Create `internal/cli/transaction.go` — Transaction CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `internal/cli/transaction.go`

- [ ] T132 [P] Create `internal/cli/analytics.go` — Analytics CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `internal/cli/analytics.go`

**Checkpoint**: CLI fully functional — merchants can manage everything via terminal

---

## Phase 13: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T133 [P] Create `internal/handler/health.go` — Health check endpoint with DB/Redis status
  - **Context Files**: `internal/database/postgres.go`, `internal/database/redis.go`
  - **Creates**: `internal/handler/health.go`

- [ ] T134 [P] Create `pkg/errors/errors.go` — Comprehensive error handling with HTTP status mapping
  - **Context Files**: `internal/model/errors.go`
  - **Creates**: `pkg/errors/errors.go`

- [ ] T135 [P] Create `internal/middleware/security.go` — Security headers middleware
  - **Context Files**: None
  - **Creates**: `internal/middleware/security.go`

- [ ] T136 [P] Create `internal/middleware/request_size.go` — Request size limiting middleware
  - **Context Files**: None
  - **Creates**: `internal/middleware/request_size.go`

- [ ] T137 Run quickstart.md validation scenarios
  - **Context Files**: `quickstart.md`
  - **Creates**: None (validation only)

- [ ] T138 Performance optimization — database query tuning, connection pooling
  - **Context Files**: `internal/database/postgres.go`, `internal/repository/`
  - **Modifies**: Multiple repository files

- [ ] T139 Security hardening — input sanitization, SQL injection prevention
  - **Context Files**: All handler and repository files
  - **Modifies**: Multiple files

- [ ] T140 Documentation updates — API docs, README, architecture diagrams
  - **Context Files**: All spec files
  - **Creates**: `docs/` files

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3-12)**: All depend on Foundational phase completion
  - US1 (Merchant) → US2 (Payments) → US3 (Customers) → US4 (Subscriptions) → US5 (Payouts) → US6 (Webhooks) → US7 (Multi-Currency) → US8 (Analytics) → US9 (Web Portal) → US10 (CLI)
- **Polish (Phase 13)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (Merchant)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **US2 (Payments)**: Can start after Foundational (Phase 2) — May integrate with US1 but should be independently testable
- **US3 (Customers)**: Can start after Foundational (Phase 2) — May integrate with US1/US2 but should be independently testable
- **US4 (Subscriptions)**: Can start after Foundational (Phase 2) — May integrate with US1/US3 but should be independently testable
- **US5 (Payouts)**: Can start after Foundational (Phase 2) — May integrate with US1/US2 but should be independently testable
- **US6 (Webhooks)**: Can start after Foundational (Phase 2) — May integrate with US1/US2 but should be independently testable
- **US7 (Multi-Currency)**: Can start after Foundational (Phase 2) — May integrate with US2 but should be independently testable
- **US8 (Analytics)**: Can start after Foundational (Phase 2) — May integrate with US2/US5 but should be independently testable
- **US9 (Web Portal)**: Can start after Foundational (Phase 2) — May integrate with US1/US2/US3 but should be independently testable
- **US10 (CLI)**: Can start after Foundational (Phase 2) — May integrate with US1/US2/US8 but should be independently testable

### Within Each User Story

- Models before repositories
- Repositories before services
- Services before handlers
- Handlers before registration
- Registration before checkpoint

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all models for User Story 1 together:
Task: "Create Merchant struct in internal/model/merchant.go"
Task: "Create ProviderConfig struct in internal/model/provider_config.go"

# Launch all repositories for User Story 1 together:
Task: "Create Merchant repository in internal/repository/merchant_repo.go"
Task: "Create ProviderConfig repository in internal/repository/provider_config_repo.go"

# Launch all handlers for User Story 1 together:
Task: "Create Merchant HTTP handlers in internal/handler/merchant.go"
Task: "Create Provider HTTP handlers in internal/handler/provider.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1-4)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (Merchant Management)
4. Complete Phase 4: User Story 2 (Process Payments)
5. Complete Phase 5: User Story 3 (Customer Management)
6. Complete Phase 6: User Story 4 (Subscriptions & Invoicing)
7. **STOP and VALIDATE**: Test all MVP stories independently
8. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Add User Story 4 → Test independently → Deploy/Demo
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Merchant Management)
   - Developer B: User Story 2 (Process Payments)
   - Developer C: User Story 3 (Customer Management)
   - Developer D: User Story 4 (Subscriptions & Invoicing)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
