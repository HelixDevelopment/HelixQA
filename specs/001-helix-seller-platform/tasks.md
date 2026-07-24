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
  - **Content**: DATABASE_URL, REDIS_URL, SERVER_PORT, JWT_PRIVATE_KEY_PATH, JWT_PUBLIC_KEY_PATH, JWT_ACCESS_EXPIRY, JWT_REFRESH_EXPIRY, LOG_LEVEL, STRIPE_API_KEY, PAYPAL_CLIENT_ID, PAYPAL_SECRET, SQUARE_ACCESS_TOKEN, NATS_URL, ENCRYPTION_KEY

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

- [ ] T053 [P] Create `pkg/errors/errors.go` — Comprehensive error wrapping, HTTP status mapping, error codes
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
  - **Context Files**: `internal/config/config.go` (JWTPublicKeyPath, JWTAccessExpiry), `internal/model/errors.go`
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

## Phase 2.5: Auth & User Service (Blocking Prerequisites)

**Purpose**: User model, authentication (JWT + API keys), RBAC, MFA — ALL user stories depend on these

**⚠️ CRITICAL**: No user story work can begin until this phase is complete (alongside Phase 2)

### User Model & Migration

- [ ] T069A [P] Create `internal/model/user.go` — User struct with role enum (root_admin, account_admin, user), MFA fields, password hash
  - **Context Files**: `data-model.md` (User entity), `contracts/api-v1.yaml` (User schema)
  - **Creates**: `internal/model/user.go`
  - **Dependencies**: T052

- [ ] T069B [P] Create `migrations/015_create_users.up.sql` — Users table with roles, MFA, password hash
  - **Context Files**: `data-model.md` (User entity)
  - **Creates**: `migrations/015_create_users.up.sql`
  - **Dependencies**: T035

- [ ] T069C Create `internal/repository/user_repo.go` — User CRUD + role-based queries
  - **Context Files**: `internal/model/user.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/user_repo.go`
  - **Dependencies**: T007, T069A

### Authentication Services

- [ ] T069D [P] Create `internal/service/auth.go` — Password hashing (Argon2id), breach-list check, credential validation
  - **Context Files**: `internal/model/user.go`, `pkg/logger/logger.go`
  - **Creates**: `internal/service/auth.go`
  - **Dependencies**: T069A, T054
  - **External**: `golang.org/x/crypto/argon2`

- [ ] T069E [P] Create `internal/service/jwt.go` — JWT RS256 access/refresh token generation, validation, refresh flow (access 15 min, refresh 7 d)
  - **Context Files**: `internal/config/config.go` (JWTPrivateKeyPath, JWTPublicKeyPath, JWTAccessExpiry, JWTRefreshExpiry), `internal/model/user.go`
  - **Creates**: `internal/service/jwt.go`
  - **Dependencies**: T069A
  - **External**: `github.com/golang-jwt/jwt/v5`

- [ ] T069F [P] Create `internal/service/apikey.go` — API key generation, scoping, validation, rotation
  - **Context Files**: `internal/model/user.go`, `pkg/crypto/encrypt.go`
  - **Creates**: `internal/service/apikey.go`
  - **Dependencies**: T069A, T066

- [ ] T069G Create `internal/middleware/rbac.go` — Role-based access enforcement (root_admin > account_admin > user)
  - **Context Files**: `internal/model/user.go`, `internal/middleware/auth.go`
  - **Creates**: `internal/middleware/rbac.go`
  - **Dependencies**: T059, T069C

- [ ] T069H [P] Create `internal/service/mfa.go` — TOTP generation, QR code, verification, recovery codes
  - **Context Files**: `internal/model/user.go`
  - **Creates**: `internal/service/mfa.go`
  - **Dependencies**: T069A

### Auth Handlers & Routes

- [ ] T069I Create `internal/handler/auth.go` — Login, register, refresh, logout, MFA challenge/verify
  - **Context Files**: `internal/service/auth.go`, `internal/service/jwt.go`, `internal/service/mfa.go`, `contracts/api-v1.yaml`
  - **Creates**: `internal/handler/auth.go`
  - **Dependencies**: T069D, T069E, T069H

- [ ] T069J [P] Create `internal/handler/user.go` — User CRUD, role assignment, profile management
  - **Context Files**: `internal/repository/user_repo.go`, `contracts/api-v1.yaml`
  - **Creates**: `internal/handler/user.go`
  - **Dependencies**: T069C

- [ ] T069K [P] Create `internal/handler/apikey.go` — API key management (create, list, revoke)
  - **Context Files**: `internal/service/apikey.go`, `contracts/api-v1.yaml`
  - **Creates**: `internal/handler/apikey.go`
  - **Dependencies**: T069F

- [ ] T069L Register auth/user/apikey routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/auth.go`, `internal/handler/user.go`, `internal/handler/apikey.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T069I, T069J, T069K

**Checkpoint**: Auth & user service ready — JWT, API keys, RBAC, MFA functional

- [ ] T-C0 PCI Compliance Gate: Verify no raw card storage anywhere in codebase
  - **Context Files**: All model and service files
  - **Creates**: None (verification only)
  - **Verifies**: PCI DSS §3.4 — no PAN storage, tokenization only

---

## Phase 3: User Story 1 — Merchant Management (Priority: P1) 🎯 MVP

**Goal**: Merchant can create account, configure payment providers, and manage settings

**Independent Test**: Create merchant → configure Stripe → verify credentials via test transaction → list merchants

### Repository Layer

- [ ] T070-T RED: Write failing tests for Merchant repository CRUD
  - **Context Files**: `internal/model/merchant.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/merchant_repo_test.go`
  - **Dependencies**: T037, T007
  - **Verifies**: T070

- [ ] T070 Create `internal/repository/merchant_repo.go` — CRUD operations for Merchant
  - **Context Files**: `internal/model/merchant.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/merchant_repo.go`
  - **Dependencies**: T007, T037

- [ ] T071-T RED: Write failing tests for ProviderConfig repository CRUD
  - **Context Files**: `internal/model/provider_config.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/provider_config_repo_test.go`
  - **Dependencies**: T046, T007
  - **Verifies**: T071

- [ ] T071 [P] Create `internal/repository/provider_config_repo.go` — CRUD for ProviderConfig
  - **Context Files**: `internal/model/provider_config.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/provider_config_repo.go`
  - **Dependencies**: T007, T046

### Service Layer

- [ ] T072-T RED: Write failing tests for Merchant service (create, update, get, list)
  - **Context Files**: `internal/model/merchant.go`, `internal/repository/merchant_repo.go`
  - **Creates**: `tests/unit/merchant_service_test.go`
  - **Dependencies**: T070, T037
  - **Verifies**: T072

- [ ] T072 Create `internal/service/merchant.go` — Merchant business logic (create, update, get, list)
  - **Context Files**: `internal/repository/merchant_repo.go`, `internal/model/merchant.go`
  - **Creates**: `internal/service/merchant.go`
  - **Dependencies**: T070

- [ ] T073-T RED: Write failing tests for Provider service (configure, verify, health check)
  - **Context Files**: `internal/model/provider_config.go`, `internal/repository/provider_config_repo.go`
  - **Creates**: `tests/unit/provider_service_test.go`
  - **Dependencies**: T071, T046
  - **Verifies**: T073

- [ ] T073 Create `internal/service/provider.go` — Provider configuration logic (configure, verify, health check)
  - **Context Files**: `internal/repository/provider_config_repo.go`, `internal/model/provider_config.go`
  - **Creates**: `internal/service/provider.go`
  - **Dependencies**: T071

### Handler Layer

- [ ] T074-T RED: Write failing tests for Merchant HTTP handlers
  - **Context Files**: `internal/service/merchant.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/merchant_handler_test.go`
  - **Dependencies**: T072
  - **Verifies**: T074

- [ ] T074 [P] Create `internal/handler/merchant.go` — Merchant HTTP handlers
  - **Context Files**: `internal/service/merchant.go`, `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `internal/handler/merchant.go`
  - **Dependencies**: T072

- [ ] T075-T RED: Write failing tests for Provider HTTP handlers
  - **Context Files**: `internal/service/provider.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/provider_handler_test.go`
  - **Dependencies**: T073
  - **Verifies**: T075

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

- [ ] T077-T RED: Write failing tests for PaymentProvider interface contract
  - **Context Files**: `data-model.md` (Transaction entity), `research.md` (Payment Provider Integration)
  - **Creates**: `tests/unit/adapter_test.go`
  - **Dependencies**: T040
  - **Verifies**: T077

- [ ] T077 Create `internal/provider/adapter.go` — PaymentProvider interface definition
  - **Context Files**: `data-model.md` (Transaction entity), `research.md` (Payment Provider Integration)
  - **Creates**: `internal/provider/adapter.go`
  - **Dependencies**: T040

- [ ] T078-T RED: Write failing tests for Stripe adapter (charge, refund, verify)
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Stripe Integration)
  - **Creates**: `tests/unit/stripe_adapter_test.go`
  - **Dependencies**: T077
  - **Verifies**: T078

- [ ] T078 Create `internal/provider/stripe.go` — Stripe adapter (charge, refund, verify)
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Stripe Integration)
  - **Creates**: `internal/provider/stripe.go`
  - **Dependencies**: T077
  - **External**: `github.com/stripe/stripe-go/v76`

- [ ] T079-T RED: Write failing tests for PayPal adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (PayPal Integration)
  - **Creates**: `tests/unit/paypal_adapter_test.go`
  - **Dependencies**: T077
  - **Verifies**: T079

- [ ] T079 [P] Create `internal/provider/paypal.go` — PayPal adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (PayPal Integration)
  - **Creates**: `internal/provider/paypal.go`
  - **Dependencies**: T077

- [ ] T080-T RED: Write failing tests for Square adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Square Integration)
  - **Creates**: `tests/unit/square_adapter_test.go`
  - **Dependencies**: T077
  - **Verifies**: T080

- [ ] T080 [P] Create `internal/provider/square.go` — Square adapter
  - **Context Files**: `internal/provider/adapter.go`, `research.md` (Square Integration)
  - **Creates**: `internal/provider/square.go`
  - **Dependencies**: T077

### Payment Router

- [ ] T081-T RED: Write failing tests for Payment Router (selection, fallback, circuit breaker)
  - **Context Files**: `internal/provider/adapter.go`, `internal/model/provider_config.go`
  - **Creates**: `tests/unit/payment_router_test.go`
  - **Dependencies**: T077, T078, T079, T080
  - **Verifies**: T081

- [ ] T081 Create `internal/service/payment_router.go` — Provider selection, fallback, circuit breaker
  - **Context Files**: `internal/provider/adapter.go`, `internal/model/provider_config.go`, `research.md` (Circuit Breaker)
  - **Creates**: `internal/service/payment_router.go`
  - **Dependencies**: T077, T078, T079, T080, T073

### Transaction Repository

- [ ] T082-T RED: Write failing tests for Transaction repository CRUD + idempotency
  - **Context Files**: `internal/model/transaction.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/transaction_repo_test.go`
  - **Dependencies**: T040, T007
  - **Verifies**: T082

- [ ] T082 Create `internal/repository/transaction_repo.go` — Transaction CRUD + idempotency
  - **Context Files**: `internal/model/transaction.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/transaction_repo.go`
  - **Dependencies**: T007, T040

### Transaction Service

- [ ] T083-T RED: Write failing tests for Transaction service (charge, refund, get, list)
  - **Context Files**: `internal/repository/transaction_repo.go`, `internal/service/payment_router.go`
  - **Creates**: `tests/unit/transaction_service_test.go`
  - **Dependencies**: T082, T081
  - **Verifies**: T083

- [ ] T083 Create `internal/service/transaction.go` — Transaction business logic (charge, refund, get, list)
  - **Context Files**: `internal/repository/transaction_repo.go`, `internal/service/payment_router.go`, `internal/model/transaction.go`
  - **Creates**: `internal/service/transaction.go`
  - **Dependencies**: T082, T081

### Transaction Handler

- [ ] T084-T RED: Write failing tests for Transaction HTTP handlers
  - **Context Files**: `internal/service/transaction.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/transaction_handler_test.go`
  - **Dependencies**: T083
  - **Verifies**: T084

- [ ] T084 [P] Create `internal/handler/transaction.go` — Transaction HTTP handlers
  - **Context Files**: `internal/service/transaction.go`, `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `internal/handler/transaction.go`
  - **Dependencies**: T083

### Webhook Ingress

- [ ] T085-T RED: Write failing tests for Webhook ingress endpoints (signature verification, idempotency)
  - **Context Files**: `internal/service/payment_router.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/webhook_handler_test.go`
  - **Dependencies**: T081
  - **Verifies**: T085

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

- [ ] T-C1 Payment Audit Trail Verification: Every charge/refund/subscription creates audit log entry
  - **Context Files**: `internal/middleware/audit.go`, `internal/service/transaction.go`
  - **Creates**: None (verification only)
  - **Verifies**: PCI DSS §10.1 — full audit trail for all payment operations

---

## Phase 5: User Story 3 — Customer Management (Priority: P1) 🎯 MVP

**Goal**: Manage customers, payment methods, and tokenized card storage

**Independent Test**: Create customer → add payment method → list payment methods → delete payment method

### Repository Layer

- [ ] T087-T RED: Write failing tests for Customer repository CRUD
  - **Context Files**: `internal/model/customer.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/customer_repo_test.go`
  - **Dependencies**: T038, T007
  - **Verifies**: T087

- [ ] T087 Create `internal/repository/customer_repo.go` — Customer CRUD operations
  - **Context Files**: `internal/model/customer.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/customer_repo.go`
  - **Dependencies**: T007, T038

- [ ] T088-T RED: Write failing tests for PaymentMethod repository CRUD
  - **Context Files**: `internal/model/payment_method.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/payment_method_repo_test.go`
  - **Dependencies**: T039, T007
  - **Verifies**: T088

- [ ] T088 [P] Create `internal/repository/payment_method_repo.go` — PaymentMethod CRUD
  - **Context Files**: `internal/model/payment_method.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/payment_method_repo.go`
  - **Dependencies**: T007, T039

### Service Layer

- [ ] T089-T RED: Write failing tests for Customer service (create, update, get, list)
  - **Context Files**: `internal/repository/customer_repo.go`, `internal/model/customer.go`
  - **Creates**: `tests/unit/customer_service_test.go`
  - **Dependencies**: T087, T038
  - **Verifies**: T089

- [ ] T089 Create `internal/service/customer.go` — Customer business logic
  - **Context Files**: `internal/repository/customer_repo.go`, `internal/model/customer.go`
  - **Creates**: `internal/service/customer.go`
  - **Dependencies**: T087

- [ ] T090-T RED: Write failing tests for PaymentMethod service (tokenize, list, delete)
  - **Context Files**: `internal/repository/payment_method_repo.go`, `internal/model/payment_method.go`
  - **Creates**: `tests/unit/payment_method_service_test.go`
  - **Dependencies**: T088, T039
  - **Verifies**: T090

- [ ] T090 Create `internal/service/payment_method.go` — PaymentMethod business logic
  - **Context Files**: `internal/repository/payment_method_repo.go`, `internal/model/payment_method.go`
  - **Creates**: `internal/service/payment_method.go`
  - **Dependencies**: T088

### Handler Layer

- [ ] T091-T RED: Write failing tests for Customer HTTP handlers
  - **Context Files**: `internal/service/customer.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/customer_handler_test.go`
  - **Dependencies**: T089
  - **Verifies**: T091

- [ ] T091 [P] Create `internal/handler/customer.go` — Customer HTTP handlers
  - **Context Files**: `internal/service/customer.go`, `contracts/api-v1.yaml` (Customer endpoints)
  - **Creates**: `internal/handler/customer.go`
  - **Dependencies**: T089

- [ ] T091A-T RED: Write failing tests for Customer update handler (updateCustomer)
  - **Context Files**: `internal/handler/customer.go`, `contracts/api-v1.yaml` (updateCustomer endpoint)
  - **Creates**: `tests/integration/customer_handler_test.go` (extend)
  - **Dependencies**: T091
  - **Verifies**: T091A

- [ ] T091A Add updateCustomer handler logic to `internal/handler/customer.go`
  - **Context Files**: `internal/service/customer.go`, `contracts/api-v1.yaml` (UpdateCustomerRequest)
  - **Modifies**: `internal/handler/customer.go`
  - **Dependencies**: T091, T089

- [ ] T092-T RED: Write failing tests for PaymentMethod HTTP handlers
  - **Context Files**: `internal/service/payment_method.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/payment_method_handler_test.go`
  - **Dependencies**: T090
  - **Verifies**: T092

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

- [ ] T094-T RED: Write failing tests for Subscription repository CRUD
  - **Context Files**: `internal/model/subscription.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/subscription_repo_test.go`
  - **Dependencies**: T041, T007
  - **Verifies**: T094

- [ ] T094 Create `internal/repository/subscription_repo.go` — Subscription CRUD
  - **Context Files**: `internal/model/subscription.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/subscription_repo.go`
  - **Dependencies**: T007, T041

- [ ] T095-T RED: Write failing tests for Invoice repository CRUD
  - **Context Files**: `internal/model/invoice.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/invoice_repo_test.go`
  - **Dependencies**: T042, T007
  - **Verifies**: T095

- [ ] T095 [P] Create `internal/repository/invoice_repo.go` — Invoice CRUD
  - **Context Files**: `internal/model/invoice.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/invoice_repo.go`
  - **Dependencies**: T007, T042

### Service Layer

- [ ] T096-T RED: Write failing tests for Subscription service (create, cancel, renew)
  - **Context Files**: `internal/repository/subscription_repo.go`, `internal/model/subscription.go`
  - **Creates**: `tests/unit/subscription_service_test.go`
  - **Dependencies**: T094, T041
  - **Verifies**: T096

- [ ] T096 Create `internal/service/subscription.go` — Subscription business logic
  - **Context Files**: `internal/repository/subscription_repo.go`, `internal/model/subscription.go`
  - **Creates**: `internal/service/subscription.go`
  - **Dependencies**: T094

- [ ] T097-T RED: Write failing tests for Invoice service (generate, send, track payment)
  - **Context Files**: `internal/repository/invoice_repo.go`, `internal/model/invoice.go`
  - **Creates**: `tests/unit/invoice_service_test.go`
  - **Dependencies**: T095, T042
  - **Verifies**: T097

- [ ] T097 Create `internal/service/invoice.go` — Invoice business logic
  - **Context Files**: `internal/repository/invoice_repo.go`, `internal/model/invoice.go`
  - **Creates**: `internal/service/invoice.go`
  - **Dependencies**: T095

### Handler Layer

- [ ] T098-T RED: Write failing tests for Subscription HTTP handlers
  - **Context Files**: `internal/service/subscription.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/subscription_handler_test.go`
  - **Dependencies**: T096
  - **Verifies**: T098

- [ ] T098 [P] Create `internal/handler/subscription.go` — Subscription HTTP handlers
  - **Context Files**: `internal/service/subscription.go`, `contracts/api-v1.yaml` (Subscription endpoints)
  - **Creates**: `internal/handler/subscription.go`
  - **Dependencies**: T096

- [ ] T098A-T RED: Write failing tests for Subscription update handler (updateSubscription)
  - **Context Files**: `internal/handler/subscription.go`, `contracts/api-v1.yaml` (updateSubscription endpoint)
  - **Creates**: `tests/integration/subscription_handler_test.go` (extend)
  - **Dependencies**: T098
  - **Verifies**: T098A

- [ ] T098A Add updateSubscription handler logic to `internal/handler/subscription.go`
  - **Context Files**: `internal/service/subscription.go`, `contracts/api-v1.yaml` (UpdateSubscriptionRequest)
  - **Modifies**: `internal/handler/subscription.go`
  - **Dependencies**: T098, T096

- [ ] T099-T RED: Write failing tests for Invoice HTTP handlers
  - **Context Files**: `internal/service/invoice.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/invoice_handler_test.go`
  - **Dependencies**: T097
  - **Verifies**: T099

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

- [ ] T101-T RED: Write failing tests for Payout repository CRUD
  - **Context Files**: `internal/model/payout.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/payout_repo_test.go`
  - **Dependencies**: T043, T007
  - **Verifies**: T101

- [ ] T101 Create `internal/repository/payout_repo.go` — Payout CRUD
  - **Context Files**: `internal/model/payout.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/payout_repo.go`
  - **Dependencies**: T007, T043

- [ ] T102-T RED: Write failing tests for Dispute repository CRUD
  - **Context Files**: `internal/model/dispute.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/dispute_repo_test.go`
  - **Dependencies**: T044, T007
  - **Verifies**: T102

- [ ] T102 [P] Create `internal/repository/dispute_repo.go` — Dispute CRUD
  - **Context Files**: `internal/model/dispute.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/dispute_repo.go`
  - **Dependencies**: T007, T044

### Service Layer

- [ ] T103-T RED: Write failing tests for Payout service (create, schedule, verify)
  - **Context Files**: `internal/repository/payout_repo.go`, `internal/model/payout.go`
  - **Creates**: `tests/unit/payout_service_test.go`
  - **Dependencies**: T101, T043
  - **Verifies**: T103

- [ ] T103 Create `internal/service/payout.go` — Payout business logic
  - **Context Files**: `internal/repository/payout_repo.go`, `internal/model/payout.go`
  - **Creates**: `internal/service/payout.go`
  - **Dependencies**: T101

- [ ] T104-T RED: Write failing tests for Dispute service (create, submit evidence, track resolution)
  - **Context Files**: `internal/repository/dispute_repo.go`, `internal/model/dispute.go`
  - **Creates**: `tests/unit/dispute_service_test.go`
  - **Dependencies**: T102, T044
  - **Verifies**: T104

- [ ] T104 Create `internal/service/dispute.go` — Dispute business logic
  - **Context Files**: `internal/repository/dispute_repo.go`, `internal/model/dispute.go`
  - **Creates**: `internal/service/dispute.go`
  - **Dependencies**: T102

### Handler Layer

- [ ] T105-T RED: Write failing tests for Payout HTTP handlers
  - **Context Files**: `internal/service/payout.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/payout_handler_test.go`
  - **Dependencies**: T103
  - **Verifies**: T105

- [ ] T105 [P] Create `internal/handler/payout.go` — Payout HTTP handlers
  - **Context Files**: `internal/service/payout.go`, `contracts/api-v1.yaml` (Payout endpoints)
  - **Creates**: `internal/handler/payout.go`
  - **Dependencies**: T103

- [ ] T106-T RED: Write failing tests for Dispute HTTP handlers
  - **Context Files**: `internal/service/dispute.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/dispute_handler_test.go`
  - **Dependencies**: T104
  - **Verifies**: T106

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

- [ ] T108-T RED: Write failing tests for WebhookConfig repository CRUD
  - **Context Files**: `internal/model/webhook_config.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/webhook_config_repo_test.go`
  - **Dependencies**: T045, T007
  - **Verifies**: T108

- [ ] T108 Create `internal/repository/webhook_config_repo.go` — WebhookConfig CRUD
  - **Context Files**: `internal/model/webhook_config.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/webhook_config_repo.go`
  - **Dependencies**: T007, T045

### Service Layer

- [ ] T109-T RED: Write failing tests for Webhook service (deliver, retry, signature verification)
  - **Context Files**: `internal/repository/webhook_config_repo.go`, `internal/model/webhook_config.go`
  - **Creates**: `tests/unit/webhook_service_test.go`
  - **Dependencies**: T108, T045
  - **Verifies**: T109

- [ ] T109 Create `internal/service/webhook.go` — Webhook delivery, retry, signature verification
  - **Context Files**: `internal/repository/webhook_config_repo.go`, `internal/model/webhook_config.go`
  - **Creates**: `internal/service/webhook.go`
  - **Dependencies**: T108

### Handler Layer

- [ ] T110-T RED: Write failing tests for WebhookConfig HTTP handlers
  - **Context Files**: `internal/service/webhook.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/webhook_config_handler_test.go`
  - **Dependencies**: T109
  - **Verifies**: T110

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

- [ ] T112-T RED: Write failing tests for ExchangeRate repository CRUD
  - **Context Files**: `internal/model/exchange_rate.go`, `internal/database/postgres.go`
  - **Creates**: `tests/unit/exchange_rate_repo_test.go`
  - **Dependencies**: T049, T007
  - **Verifies**: T112

- [ ] T112 Create `internal/repository/exchange_rate_repo.go` — ExchangeRate CRUD
  - **Context Files**: `internal/model/exchange_rate.go`, `internal/database/postgres.go`
  - **Creates**: `internal/repository/exchange_rate_repo.go`
  - **Dependencies**: T007, T049

### Service Layer

- [ ] T113-T RED: Write failing tests for ExchangeRate service (fetch, cache, convert)
  - **Context Files**: `internal/repository/exchange_rate_repo.go`, `internal/model/exchange_rate.go`
  - **Creates**: `tests/unit/exchange_rate_service_test.go`
  - **Dependencies**: T112, T049
  - **Verifies**: T113

- [ ] T113 Create `internal/service/exchange_rate.go` — Exchange rate fetching, caching, conversion
  - **Context Files**: `internal/repository/exchange_rate_repo.go`, `internal/model/exchange_rate.go`, `research.md` (Exchange Rate Research)
  - **Creates**: `internal/service/exchange_rate.go`
  - **Dependencies**: T112

### Handler Layer

- [ ] T114-T RED: Write failing tests for ExchangeRate HTTP handlers
  - **Context Files**: `internal/service/exchange_rate.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/exchange_rate_handler_test.go`
  - **Dependencies**: T113
  - **Verifies**: T114

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

- [ ] T116-T RED: Write failing tests for Analytics repository queries (aggregations, trends)
  - **Context Files**: `internal/database/postgres.go`, `data-model.md` (Transaction entity)
  - **Creates**: `tests/unit/analytics_repo_test.go`
  - **Dependencies**: T040, T007
  - **Verifies**: T116

- [ ] T116 Create `internal/repository/analytics_repo.go` — Analytics queries (aggregations, trends)
  - **Context Files**: `internal/database/postgres.go`, `data-model.md` (Transaction entity)
  - **Creates**: `internal/repository/analytics_repo.go`
  - **Dependencies**: T007, T040

### Service Layer

- [ ] T117-T RED: Write failing tests for Analytics service (revenue, volume, trends)
  - **Context Files**: `internal/repository/analytics_repo.go`
  - **Creates**: `tests/unit/analytics_service_test.go`
  - **Dependencies**: T116
  - **Verifies**: T117

- [ ] T117 Create `internal/service/analytics.go` — Analytics business logic
  - **Context Files**: `internal/repository/analytics_repo.go`
  - **Creates**: `internal/service/analytics.go`
  - **Dependencies**: T116

### Handler Layer

- [ ] T118-T RED: Write failing tests for Analytics HTTP handlers
  - **Context Files**: `internal/service/analytics.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/analytics_handler_test.go`
  - **Dependencies**: T117
  - **Verifies**: T118

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

- [ ] T120-T RED: Write failing e2e tests for Angular app initialization and routing
  - **Context Files**: `plan.md` (Frontend section)
  - **Creates**: `web/tests/e2e/app.spec.ts`
  - **Dependencies**: None
  - **Verifies**: T120

- [ ] T120 Initialize Angular 19 project in `web/`
  - **Context Files**: `plan.md` (Frontend section)
  - **Creates**: `web/src/`, `web/package.json`, `web/angular.json`

- [ ] T121-T RED: Write failing tests for Core module (auth guard, interceptors)
  - **Context Files**: `contracts/api-v1.yaml` (auth endpoints)
  - **Creates**: `web/tests/unit/core/` 
  - **Dependencies**: T120
  - **Verifies**: T121

- [ ] T121 [P] Create `web/src/app/core/` — Core module (auth guard, interceptors, services)
  - **Context Files**: `contracts/api-v1.yaml` (auth endpoints)
  - **Creates**: `web/src/app/core/`

- [ ] T122-T RED: Write failing tests for Shared module components
  - **Context Files**: None
  - **Creates**: `web/tests/unit/shared/`
  - **Dependencies**: T120
  - **Verifies**: T122

- [ ] T122 [P] Create `web/src/app/shared/` — Shared module (components, pipes, directives)
  - **Context Files**: None
  - **Creates**: `web/src/app/shared/`

### OpenDesign Integration

- [ ] T122A-T RED: Write failing tests for OpenDesign theme and component rendering
  - **Context Files**: None
  - **Creates**: `web/tests/unit/opendesign/`
  - **Dependencies**: T120
  - **Verifies**: T122A

- [ ] T122A [P] Initialize OpenDesign design tokens, theme configuration, light/dark modes
  - **Context Files**: `spec.md` (Design System section)
  - **Creates**: `web/src/assets/themes/`, `web/src/styles/opendesign.scss`

- [ ] T122B [P] Set up OpenDesign Angular component library adapters
  - **Context Files**: None
  - **Creates**: `web/src/app/opendesign/`

- [ ] T122C Create OpenDesign layout system (navigation, sidebar, header, responsive grid)
  - **Context Files**: None
  - **Creates**: `web/src/app/shared/layout/`

- [ ] T122D Verify all Web Portal pages use OpenDesign components (no custom CSS for core UI)
  - **Context Files**: None
  - **Creates**: None (verification only)

- [ ] T122E OpenDesign responsive design audit — mobile/tablet/desktop breakpoints
  - **Context Files**: None
  - **Creates**: None (verification only)

### Dashboard

- [ ] T123-T RED: Write failing e2e tests for Dashboard page (transaction overview, charts)
  - **Context Files**: `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `web/tests/e2e/dashboard.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T123

- [ ] T123 Create `web/src/app/pages/dashboard/` — Dashboard page (transaction overview, charts)
  - **Context Files**: `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `web/src/app/pages/dashboard/`

### Merchant Management

- [ ] T124-T RED: Write failing e2e tests for Merchant management pages
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `web/tests/e2e/merchants.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T124

- [ ] T124 [P] Create `web/src/app/pages/merchants/` — Merchant list, create, edit pages
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `web/src/app/pages/merchants/`

### Transaction Management

- [ ] T125-T RED: Write failing e2e tests for Transaction management pages
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `web/tests/e2e/transactions.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T125

- [ ] T125 [P] Create `web/src/app/pages/transactions/` — Transaction list, detail pages
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `web/src/app/pages/transactions/`

### Provider Configuration

- [ ] T126-T RED: Write failing e2e tests for Provider configuration wizard
  - **Context Files**: `contracts/api-v1.yaml` (Provider endpoints)
  - **Creates**: `web/tests/e2e/providers.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T126

- [ ] T126 [P] Create `web/src/app/pages/providers/` — Provider configuration wizard
  - **Context Files**: `contracts/api-v1.yaml` (Provider endpoints)
  - **Creates**: `web/src/app/pages/providers/`

### Customer Management

- [ ] T127-T RED: Write failing e2e tests for Customer management pages
  - **Context Files**: `contracts/api-v1.yaml` (Customer endpoints)
  - **Creates**: `web/tests/e2e/customers.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T127

- [ ] T127 [P] Create `web/src/app/pages/customers/` — Customer list, detail pages
  - **Context Files**: `contracts/api-v1.yaml` (Customer endpoints)
  - **Creates**: `web/src/app/pages/customers/`

### Webhook Configuration

- [ ] T128-T RED: Write failing e2e tests for Webhook configuration page
  - **Context Files**: `contracts/api-v1.yaml` (Webhook endpoints)
  - **Creates**: `web/tests/e2e/webhooks.spec.ts`
  - **Dependencies**: T121
  - **Verifies**: T128

- [ ] T128 [P] Create `web/src/app/pages/webhooks/` — Webhook configuration page
  - **Context Files**: `contracts/api-v1.yaml` (Webhook endpoints)
  - **Creates**: `web/src/app/pages/webhooks/`

**Checkpoint**: Web portal fully functional — merchants can manage everything via UI

---

## Phase 12: User Story 10 — CLI/TUI (Priority: P3)

**Goal**: Command-line interface for pipeline automation and terminal-based management

**Independent Test**: List merchants → create transaction → view analytics → export report

### CLI Setup

- [ ] T129-T RED: Write failing tests for CLI entry point and command routing
  - **Context Files**: `plan.md` (CLI section)
  - **Creates**: `tests/unit/cli_test.go`
  - **Dependencies**: None
  - **Verifies**: T129

- [ ] T129 Create `cmd/cli/main.go` — CLI entry point with Cobra
  - **Context Files**: `plan.md` (CLI section)
  - **Creates**: `cmd/cli/main.go`
  - **External**: `github.com/spf13/cobra`

- [ ] T130-T RED: Write failing tests for Merchant CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `tests/unit/cli_merchant_test.go`
  - **Dependencies**: T129
  - **Verifies**: T130

- [ ] T130 [P] Create `internal/cli/merchant.go` — Merchant CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Merchant endpoints)
  - **Creates**: `internal/cli/merchant.go`

- [ ] T131-T RED: Write failing tests for Transaction CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `tests/unit/cli_transaction_test.go`
  - **Dependencies**: T129
  - **Verifies**: T131

- [ ] T131 [P] Create `internal/cli/transaction.go` — Transaction CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Transaction endpoints)
  - **Creates**: `internal/cli/transaction.go`

- [ ] T132-T RED: Write failing tests for Analytics CLI commands
  - **Context Files**: `contracts/api-v1.yaml` (Analytics endpoints)
  - **Creates**: `tests/unit/cli_analytics_test.go`
  - **Dependencies**: T129
  - **Verifies**: T132

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

- [ ] T-C2 Security Audit: Full security review — auth, input sanitization, SQL injection, secret scanning, dependency CVE check
  - **Context Files**: All handler, middleware, and repository files
  - **Creates**: None (verification only)
  - **Verifies**: PCI DSS §6.5 — secure coding practices

- [ ] T-C3 OpenDesign Compliance Check: Verify all Web Portal pages use OpenDesign components
  - **Context Files**: `web/src/`
  - **Creates**: None (verification only)

**Checkpoint**: Polish complete — system ready for SDK generation and deployment

---

## Phase 14: Reconciliation & Platform Billing

**Purpose**: Reconcile platform records with providers; meter and invoice platform fees

### Reconciliation

- [ ] T146-T RED: Write failing tests for Reconciliation service (platform vs provider record comparison)
  - **Context Files**: `internal/repository/transaction_repo.go`, `internal/provider/adapter.go`
  - **Creates**: `tests/unit/reconciliation_service_test.go`
  - **Dependencies**: T083, T077
  - **Verifies**: T146

- [ ] T146 Create `internal/service/reconciliation.go` — Reconciliation service (compare platform vs provider records)
  - **Context Files**: `internal/repository/transaction_repo.go`, `internal/provider/adapter.go`, `internal/service/background.go`
  - **Creates**: `internal/service/reconciliation.go`
  - **Dependencies**: T083, T077, T064

- [ ] T147-T RED: Write failing tests for Discrepancy detection and alerting
  - **Context Files**: `internal/service/reconciliation.go`, `internal/eventbus/eventbus.go`
  - **Creates**: `tests/unit/reconciliation_alert_test.go`
  - **Dependencies**: T146
  - **Verifies**: T147

- [ ] T147 Create `internal/service/discrepancy.go` — Discrepancy detection and NATS event alerting
  - **Context Files**: `internal/service/reconciliation.go`, `internal/eventbus/eventbus.go`
  - **Creates**: `internal/service/discrepancy.go`
  - **Dependencies**: T146

### Platform Billing

- [ ] T148-T RED: Write failing tests for Platform billing service (metered transaction fees)
  - **Context Files**: `internal/model/transaction.go`, `internal/model/merchant.go`
  - **Creates**: `tests/unit/billing_service_test.go`
  - **Dependencies**: T083, T072
  - **Verifies**: T148

- [ ] T148 Create `internal/service/billing.go` — Platform billing service (metered transaction fees: percentage + fixed)
  - **Context Files**: `internal/model/transaction.go`, `internal/model/merchant.go`, `internal/repository/transaction_repo.go`
  - **Creates**: `internal/service/billing.go`
  - **Dependencies**: T083, T072

- [ ] T149-T RED: Write failing tests for Monthly invoice generation per merchant
  - **Context Files**: `internal/service/billing.go`, `internal/model/merchant.go`
  - **Creates**: `tests/unit/billing_invoice_test.go`
  - **Dependencies**: T148
  - **Verifies**: T149

- [ ] T149 Create `internal/service/billing_invoice.go` — Monthly invoice generation per merchant
  - **Context Files**: `internal/service/billing.go`, `internal/model/merchant.go`
  - **Creates**: `internal/service/billing_invoice.go`
  - **Dependencies**: T148

- [ ] T150-T RED: Write failing tests for Settlement reconciliation (payouts vs expected amounts)
  - **Context Files**: `internal/service/reconciliation.go`, `internal/service/payout.go`
  - **Creates**: `tests/unit/settlement_test.go`
  - **Dependencies**: T146, T103
  - **Verifies**: T150

- [ ] T150 Create `internal/service/settlement.go` — Settlement reconciliation
  - **Context Files**: `internal/service/reconciliation.go`, `internal/service/payout.go`
  - **Creates**: `internal/service/settlement.go`
  - **Dependencies**: T146, T103

### Billing Handler

- [ ] T151-T RED: Write failing tests for Billing HTTP handlers
  - **Context Files**: `internal/service/billing.go`, `contracts/api-v1.yaml`
  - **Creates**: `tests/integration/billing_handler_test.go`
  - **Dependencies**: T148
  - **Verifies**: T151

- [ ] T151 Create `internal/handler/billing.go` — Billing endpoints (fee breakdown, invoice history)
  - **Context Files**: `internal/service/billing.go`, `contracts/api-v1.yaml`
  - **Creates**: `internal/handler/billing.go`
  - **Dependencies**: T148

- [ ] T152 Register billing and reconciliation routes in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/handler/billing.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T151

**Checkpoint**: Reconciliation and billing fully functional — platform fees metered, discrepancies detected

---

## Phase 15: SDK Generation

**Purpose**: Generate multi-language SDKs from OpenAPI contract

**MVP Scope**: Go (Critical), TypeScript (Critical), Python (High).
Remaining languages (Java/Kotlin, Ruby, PHP, C#, Rust) deferred to Phase 2 — consistent with spec §7.1 priority tiers.

### Go SDK

- [ ] T153-T RED: Write failing tests for Go SDK (authenticate, list merchants, create transaction)
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/go/sdk_test.go`
  - **Dependencies**: None
  - **Verifies**: T154

- [ ] T153 Generate Go SDK from `contracts/api-v1.yaml` using oapi-codegen
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/go/`
  - **External**: `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`

- [ ] T154 Add idiomatic Go wrapper layer over generated SDK (error handling, pagination helpers, retry logic)
  - **Context Files**: `sdk/go/`
  - **Modifies**: `sdk/go/`
  - **Dependencies**: T153

### TypeScript SDK

- [ ] T155-T RED: Write failing tests for TypeScript SDK
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/typescript/src/__tests__/`
  - **Dependencies**: None
  - **Verifies**: T156

- [ ] T155 Generate TypeScript SDK from `contracts/api-v1.yaml` using openapi-typescript-codegen
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/typescript/`
  - **External**: `openapi-typescript-codegen`

- [ ] T156 Add TypeScript wrapper (typed errors, pagination helpers, retry with backoff)
  - **Context Files**: `sdk/typescript/`
  - **Modifies**: `sdk/typescript/`
  - **Dependencies**: T155

### Python SDK

- [ ] T157-T RED: Write failing tests for Python SDK
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/python/tests/`
  - **Dependencies**: None
  - **Verifies**: T158

- [ ] T157 Generate Python SDK from `contracts/api-v1.yaml` using openapi-python-client
  - **Context Files**: `contracts/api-v1.yaml`
  - **Creates**: `sdk/python/`
  - **External**: `openapi-python-client`

- [ ] T158 Add Python wrapper (typed exceptions, pagination helpers, retry logic)
  - **Context Files**: `sdk/python/`
  - **Modifies**: `sdk/python/`
  - **Dependencies**: T157

### SDK Validation

- [ ] T159 SDK smoke tests — verify each SDK can authenticate and list merchants against running server
  - **Context Files**: `sdk/go/`, `sdk/typescript/`, `sdk/python/`
  - **Creates**: `tests/contract/sdk_smoke_test.go`
  - **Dependencies**: T154, T156, T158, T063

**Checkpoint**: SDKs available for Go, TypeScript, Python — all pass smoke tests

---

## Phase 16: Observability & Monitoring

**Purpose**: OpenTelemetry + Prometheus + Grafana monitoring stack — mandated by constitution §8.2

### OpenTelemetry Setup

- [ ] T160 [P] Create `internal/observability/otel.go` — OpenTelemetry SDK initialization, trace/meter provider setup
  - **Context Files**: `internal/config/config.go`
  - **Creates**: `internal/observability/otel.go`
  - **Dependencies**: T005
  - **External**: `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/sdk/metric`

- [ ] T161 Create `internal/middleware/tracing.go` — OTel HTTP request tracing middleware
  - **Context Files**: `internal/observability/otel.go`
  - **Creates**: `internal/middleware/tracing.go`
  - **Dependencies**: T160
  - **External**: `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`

- [ ] T162 [P] Create `internal/observability/metrics.go` — Custom Prometheus metrics (request duration, error rate, provider latency, webhook delivery, active merchants)
  - **Context Files**: `internal/observability/otel.go`
  - **Creates**: `internal/observability/metrics.go`
  - **Dependencies**: T160
  - **External**: `github.com/prometheus/client_golang/prometheus`

### Prometheus & Grafana

- [ ] T163 [P] Create `configs/prometheus.yml` — Prometheus scrape config for app metrics
  - **Context Files**: `internal/observability/metrics.go`
  - **Creates**: `configs/prometheus.yml`

- [ ] T164 [P] Create `configs/grafana/dashboards/helix-overview.json` — Grafana dashboard (request rate, error rate, latency, provider health, active merchants)
  - **Context Files**: `internal/observability/metrics.go`
  - **Creates**: `configs/grafana/dashboards/helix-overview.json`

- [ ] T165 [P] Create `configs/grafana/provisioning/datasources.yml` — Prometheus datasource auto-config
  - **Context Files**: `configs/prometheus.yml`
  - **Creates**: `configs/grafana/provisioning/datasources.yml`

### Integration & Alerting

- [ ] T166 Register OTel tracing + metrics middleware in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/middleware/tracing.go`, `internal/observability/metrics.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T161, T162

- [ ] T167 [P] Create `configs/grafana/alerting/provider-failure.yml` — Alert rules (provider circuit-breaker, error rate spike, latency breach)
  - **Context Files**: `internal/observability/metrics.go`
  - **Creates**: `configs/grafana/alerting/provider-failure.yml`

- [ ] T168 Create `scripts/generate-alerts.sh` — Generate Prometheus alerting rules from config
  - **Context Files**: `configs/prometheus.yml`
  - **Creates**: `scripts/generate-alerts.sh`

- [ ] T169 Expand `internal/handler/health.go` — Add OTel/Prometheus readiness indicator
  - **Context Files**: `internal/handler/health.go`, `internal/observability/otel.go`
  - **Modifies**: `internal/handler/health.go`
  - **Dependencies**: T133, T160

- [ ] T170 Add observability tests — Verify OTel initialization, metrics registration, tracing middleware
  - **Context Files**: `internal/observability/otel.go`, `internal/middleware/tracing.go`, `internal/observability/metrics.go`
  - **Creates**: `tests/unit/otel_test.go`, `tests/unit/tracing_test.go`, `tests/unit/metrics_test.go`
  - **Dependencies**: T160, T161, T162

**Checkpoint**: Observability stack operational — traces, metrics, dashboards, alerts

---

## Phase 17: Backup & Disaster Recovery

**Purpose**: Database backup infrastructure — mandated by constitution §8.3 (RPO ≈ 1 h, RTO ≈ 4 h)

### Backup Scripts

- [ ] T171 [P] Create `scripts/backup-full.sh` — Full database backup via pg_dump with compression + timestamp
  - **Context Files**: `internal/config/config.go` (DatabaseURL)
  - **Creates**: `scripts/backup-full.sh`

- [ ] T172 [P] Create `scripts/backup-incremental.sh` — WAL-based incremental backup
  - **Context Files**: `internal/config/config.go` (DatabaseURL)
  - **Creates**: `scripts/backup-incremental.sh`
  - **External**: PostgreSQL WAL archiving

- [ ] T173 [P] Create `scripts/backup-verify.sh` — Restore backup to temporary DB and run validation queries
  - **Context Files**: `scripts/backup-full.sh`
  - **Creates**: `scripts/backup-verify.sh`

### Scheduling & Runbook

- [ ] T174 [P] Create `scripts/backup-schedule.sh` — Systemd timer unit files for daily full + hourly incremental
  - **Context Files**: `scripts/backup-full.sh`, `scripts/backup-incremental.sh`
  - **Creates**: `scripts/backup-schedule.sh`, `scripts/helix-backup-full.service`, `scripts/helix-backup-incremental.service`, `scripts/helix-backup-*.timer`

- [ ] T175 [P] Create `docs/operations/RESTORE_RUNBOOK.md` — Step-by-step restore procedures, RPO/RTO validation, disaster recovery checklist
  - **Context Files**: `scripts/backup-full.sh`, `scripts/backup-incremental.sh`, `scripts/backup-verify.sh`
  - **Creates**: `docs/operations/RESTORE_RUNBOOK.md`

- [ ] T176 Add backup integration test — Verify backup creation, restore, and data integrity
  - **Context Files**: `scripts/backup-full.sh`, `scripts/backup-verify.sh`
  - **Creates**: `tests/integration/backup_test.go`
  - **Dependencies**: T171, T173

**Checkpoint**: Backup infrastructure complete — daily full + hourly incrementals scheduled, restore runbook documented

---

## Phase 18: Real-Time, Load Testing & Polish

**Purpose**: WebSocket real-time events, performance testing, quality gates, documentation pipeline

### WebSocket

- [ ] T177 [P] Create `internal/websocket/hub.go` — WebSocket connection hub (gorilla/websocket), room-based pub/sub
  - **Context Files**: `internal/eventbus/eventbus.go`
  - **Creates**: `internal/websocket/hub.go`
  - **Dependencies**: T058
  - **External**: `github.com/gorilla/websocket`

- [ ] T178 Create `internal/websocket/handler.go` — WebSocket upgrade handler, auth middleware, message routing
  - **Context Files**: `internal/websocket/hub.go`, `internal/middleware/auth.go`
  - **Creates**: `internal/websocket/handler.go`
  - **Dependencies**: T177, T059

- [ ] T179 Register WebSocket endpoint in `internal/handler/router.go`
  - **Context Files**: `internal/handler/router.go`, `internal/websocket/handler.go`
  - **Modifies**: `internal/handler/router.go`
  - **Dependencies**: T063, T178

- [ ] T180 Add WebSocket integration tests — Connection, auth, message delivery, reconnection
  - **Context Files**: `internal/websocket/hub.go`, `internal/websocket/handler.go`
  - **Creates**: `tests/integration/websocket_test.go`
  - **Dependencies**: T177, T178

### Load & Performance Testing

- [ ] T181 [P] Create `tests/load/k6-merchant-config.js` — k6 script: configure payment provider in < 5 min (SC-001)
  - **Context Files**: `specs/001-helix-seller-platform/quickstart.md`
  - **Creates**: `tests/load/k6-merchant-config.js`
  - **External**: `k6`

- [ ] T182 [P] Create `tests/load/k6-api-latency.js` — k6 script: API p95 < 150ms under 100 concurrent merchants (SC-003, SC-004)
  - **Context Files**: `specs/001-helix-seller-platform/spec.md` (SC-003, SC-004)
  - **Creates**: `tests/load/k6-api-latency.js`

- [ ] T183 [P] Create `tests/load/k6-dashboard.js` — k6 script: dashboard loads in < 1.5s (SC-008)
  - **Context Files**: `specs/001-helix-seller-platform/spec.md` (SC-008)
  - **Creates**: `tests/load/k6-dashboard.js`

- [ ] T184 Create `tests/load/run-all.sh` — Run all load tests and generate reports
  - **Context Files**: `tests/load/k6-merchant-config.js`, `tests/load/k6-api-latency.js`, `tests/load/k6-dashboard.js`
  - **Creates**: `tests/load/run-all.sh`
  - **Dependencies**: T181, T182, T183

### Quality Gates

- [ ] T185 [P] Create `configs/sonar-project.properties` — SonarQube scanner configuration (coverage gate, quality profiles)
  - **Context Files**: None
  - **Creates**: `configs/sonar-project.properties`

- [ ] T186 [P] Create `scripts/sonar-scan.sh` — Run SonarQube analysis in container
  - **Context Files**: `configs/sonar-project.properties`
  - **Creates**: `scripts/sonar-scan.sh`

- [ ] T187 Add coverage gate check to quickstart validation — Verify 80% minimum coverage (constitution §4.1)
  - **Context Files**: `specs/001-helix-seller-platform/quickstart.md`, `scripts/sonar-scan.sh`
  - **Modifies**: `specs/001-helix-seller-platform/quickstart.md`
  - **Dependencies**: T185, T186

### Documentation Pipeline

- [ ] T188 [P] Create `scripts/docs-build.sh` — pandoc pipeline: Markdown → PDF + HTML sync (constitution §4.2)
  - **Context Files**: None
  - **Creates**: `scripts/docs-build.sh`
  - **External**: `pandoc`

- [ ] T189 Add documentation build validation — Verify all .md files generate valid PDF/HTML
  - **Context Files**: `scripts/docs-build.sh`
  - **Creates**: `tests/integration/docs_test.go`
  - **Dependencies**: T188

**Checkpoint**: Real-time events working, load tests pass SLA, quality gates configured, docs pipeline operational

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **Auth & User (Phase 2.5)**: Depends on Phase 2 — BLOCKS all user stories
- **User Stories (Phase 3-12)**: All depend on Phase 2 + Phase 2.5 completion
  - US1 (Merchant) → US2 (Payments) → US3 (Customers) → US4 (Subscriptions) → US5 (Payouts) → US6 (Webhooks) → US7 (Multi-Currency) → US8 (Analytics) → US9 (Web Portal) → US10 (CLI)
- **Polish (Phase 13)**: Depends on all desired user stories being complete
- **Reconciliation & Billing (Phase 14)**: Depends on Phase 4 (Payments) + Phase 5 (Customers)
- **SDK Generation (Phase 15)**: Depends on Phase 13 (Polish) + API contract stability
- **Observability (Phase 16)**: Depends on Phase 2 (config) — can run in parallel with user stories
- **Backup/DR (Phase 17)**: Independent — can run anytime after Phase 1
- **Real-Time & Load Testing (Phase 18)**: WebSocket depends on Phase 2.5 (auth); load tests depend on Phase 13 (polish); docs pipeline is independent

### User Story Dependencies

- **US1 (Merchant)**: Can start after Phase 2.5 — No dependencies on other stories
- **US2 (Payments)**: Can start after Phase 2.5 — May integrate with US1 but independently testable
- **US3 (Customers)**: Can start after Phase 2.5 — May integrate with US1/US2 but independently testable
- **US4 (Subscriptions)**: Can start after Phase 2.5 — May integrate with US1/US3 but independently testable
- **US5 (Payouts)**: Can start after Phase 2.5 — May integrate with US1/US2 but independently testable
- **US6 (Webhooks)**: Can start after Phase 2.5 — May integrate with US1/US2 but independently testable
- **US7 (Multi-Currency)**: Can start after Phase 2.5 — May integrate with US2 but independently testable
- **US8 (Analytics)**: Can start after Phase 2.5 — May integrate with US2/US5 but independently testable
- **US9 (Web Portal)**: Can start after Phase 2.5 — May integrate with US1/US2/US3 but independently testable
- **US10 (CLI)**: Can start after Phase 2.5 — May integrate with US1/US2/US8 but independently testable

### TDD Flow (Per Task)

Each implementation task in Phases 3-12 follows:
1. **RED** — Write failing test (`T0XX-T`)
2. **GREEN** — Implement task (`T0XX`) to make test pass
3. **REFACTOR** — Clean up while keeping tests green

### Compliance Gates

- **T-C0**: PCI compliance check after Phase 2.5 (no raw card storage)
- **T-C1**: Payment audit trail verification after Phase 4
- **T-C2**: Full security audit after Phase 13
- **T-C3**: OpenDesign compliance check after Phase 13

### Within Each User Story

- Models before repositories
- Repositories before services
- Services before handlers
- Handlers before registration
- Registration before checkpoint

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Auth/User tasks marked [P] can run in parallel (within Phase 2.5)
- Once Phase 2.5 completes, all user stories can start in parallel (if team capacity allows)
- Different user stories can be worked on in parallel by different team members
- SDK generation (Phase 15) can run in parallel across languages (Go, TypeScript, Python)
- Reconciliation & Billing (Phase 14) is independent of user stories 6-10
- Observability (Phase 16) can run in parallel with user stories
- Backup/DR (Phase 17) is fully independent
- WebSocket (Phase 18 partial) can start after Phase 2.5
- Load tests (Phase 18) require Phase 13 completion
- Documentation pipeline (Phase 18) is fully independent

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
3. Complete Phase 2.5: Auth & User Service (CRITICAL — blocks all stories)
4. Complete Phase 3: User Story 1 (Merchant Management)
5. Complete Phase 4: User Story 2 (Process Payments)
6. Complete Phase 5: User Story 3 (Customer Management)
7. Complete Phase 6: User Story 4 (Subscriptions & Invoicing)
8. **STOP and VALIDATE**: Test all MVP stories independently
9. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational + Auth → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Add User Story 4 → Test independently → Deploy/Demo
6. Add Reconciliation & Billing → Test → Deploy/Demo
7. Each story adds value without breaking previous stories

### Full Delivery

1. Complete Phases 1-2.5 → Foundation ready
2. Complete Phases 3-6 → MVP functional
3. Complete Phase 14 → Reconciliation & billing active
4. Complete Phases 7-10 → All payment features
5. Complete Phase 11 → Web portal
6. Complete Phase 12 → CLI
7. Complete Phase 13 → Polish & security audit
8. Complete Phase 15 → SDKs generated
9. Complete Phase 16 → Observability operational
10. Complete Phase 17 → Backup infrastructure ready
11. Complete Phase 18 → WebSocket, load tests, quality gates, docs pipeline
12. **FINAL VALIDATION**: Run quickstart.md scenarios, security audit, SDK smoke tests, load tests

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
- **TDD mandatory**: Every implementation task has a preceding RED test task (`T0XX-T`)
- **OpenDesign mandatory**: All UI/UX work uses OpenDesign (constitution §4.3)
- **PCI compliance gates**: Run compliance checks at designated checkpoints
- **SDK validation**: All SDKs must pass smoke tests before marking Phase 15 complete
