# Database Overview

## Database Engine

| Environment | Engine | Version |
|-------------|--------|---------|
| Production  | PostgreSQL | 16+ |
| Development | SQLite | 3.x |

PostgreSQL is the primary data store. SQLite is used for local development and testing only; features such as partitioning, JSONB operators, and `INET` types are emulated or skipped in SQLite.

## Schema Summary

16 entities organized into five functional groups:

| Group | Entities |
|-------|----------|
| **Tenant & Users** | Merchant, Customer, User, ApiKey |
| **Payments** | PaymentMethod, Transaction, Subscription, Invoice, Payout, Dispute |
| **Configuration** | WebhookConfig, ProviderConfig |
| **Infrastructure** | AuditLog, ExchangeRate, IdempotencyKey, BackgroundTask |

## Global Conventions

| Convention | Detail |
|------------|--------|
| Primary keys | `UUID` v7 (time-ordered) unless noted otherwise |
| Timestamps | `TIMESTAMP WITH TIME ZONE` (`TIMESTAMPTZ`) for all timestamp columns |
| Metadata | `JSONB DEFAULT '{}'` for arbitrary key-value extensions |
| Soft delete | `deleted_at TIMESTAMPTZ NULL` on Merchant and Customer; rows with non-NULL `deleted_at` are excluded by default queries |
| Partitioning | Monthly range partition on `created_at` for Transaction and AuditLog |
| Amounts | `BIGINT` storing smallest currency unit (e.g., cents) |
| Currencies | `CHAR(3)` ISO 4217 codes |
| Enums | PostgreSQL `ENUM` types for bounded value sets |

## Entity Relationship Diagram

```mermaid
erDiagram
    MERCHANT ||--o{ CUSTOMER : owns
    MERCHANT ||--o{ PAYMENT_METHOD : owns
    MERCHANT ||--o{ TRANSACTION : owns
    MERCHANT ||--o{ SUBSCRIPTION : owns
    MERCHANT ||--o{ INVOICE : owns
    MERCHANT ||--o{ PAYOUT : owns
    MERCHANT ||--o{ DISPUTE : owns
    MERCHANT ||--o{ WEBHOOK_CONFIG : owns
    MERCHANT ||--o{ PROVIDER_CONFIG : owns
    MERCHANT ||--o{ USER : has
    MERCHANT ||--o{ API_KEY : owns
    MERCHANT ||--o{ AUDIT_LOG : scoped_by

    CUSTOMER ||--o{ PAYMENT_METHOD : has
    CUSTOMER ||--o{ TRANSACTION : linked_to
    CUSTOMER ||--o{ SUBSCRIPTION : has
    CUSTOMER ||--o{ INVOICE : receives

    PAYMENT_METHOD ||--o{ TRANSACTION : used_in
    SUBSCRIPTION ||--o{ INVOICE : generates
    TRANSACTION ||--o{ DISPUTE : may_have

    USER ||--o{ API_KEY : creates

    MERCHANT {
        uuid id PK
        varchar name
        varchar email UK
        varchar slug UK
        enum status
        char default_currency
        varchar timezone
        jsonb branding
        jsonb settings
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    CUSTOMER {
        uuid id PK
        uuid merchant_id FK
        varchar external_id
        varchar name
        varchar email
        varchar phone
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    PAYMENT_METHOD {
        uuid id PK
        uuid merchant_id FK
        uuid customer_id FK
        enum type
        varchar provider
        varchar provider_token
        varchar fingerprint
        varchar brand
        char last4
        smallint exp_month
        smallint exp_year
        boolean is_default
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    TRANSACTION {
        uuid id PK
        uuid merchant_id FK
        uuid customer_id FK
        varchar provider
        varchar provider_transaction_id
        enum type
        bigint amount
        char currency
        enum status
        uuid payment_method_id FK
        varchar idempotency_key
        text description
        jsonb metadata
        varchar error_code
        text error_message
        bigint fee_amount
        bigint net_amount
        timestamptz processed_at
        timestamptz created_at
        timestamptz updated_at
    }

    SUBSCRIPTION {
        uuid id PK
        uuid merchant_id FK
        uuid customer_id FK
        varchar provider
        varchar provider_subscription_id
        varchar plan_id
        enum status
        bigint amount
        char currency
        enum interval
        smallint interval_count
        timestamptz current_period_start
        timestamptz current_period_end
        timestamptz cancel_at
        timestamptz cancelled_at
        timestamptz trial_start
        timestamptz trial_end
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    INVOICE {
        uuid id PK
        uuid merchant_id FK
        uuid customer_id FK
        uuid subscription_id FK
        varchar provider
        varchar provider_invoice_id
        bigint amount
        char currency
        enum status
        date due_date
        timestamptz paid_at
        timestamptz period_start
        timestamptz period_end
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    PAYOUT {
        uuid id PK
        uuid merchant_id FK
        varchar provider
        varchar provider_payout_id
        bigint amount
        char currency
        enum status
        enum method
        date arrival_date
        bigint fee_amount
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    DISPUTE {
        uuid id PK
        uuid transaction_id FK
        uuid merchant_id FK
        varchar provider
        varchar provider_dispute_id
        varchar reason
        enum status
        bigint amount
        timestamptz evidence_deadline
        timestamptz evidence_submitted_at
        varchar resolution
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    WEBHOOK_CONFIG {
        uuid id PK
        uuid merchant_id FK
        varchar url
        varchar secret
        text[] events
        boolean is_active
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    PROVIDER_CONFIG {
        uuid id PK
        uuid merchant_id FK
        varchar provider
        boolean is_active
        jsonb config
        smallint fallback_order
        enum health_status
        timestamptz last_health_check
        jsonb metadata
        timestamptz created_at
        timestamptz updated_at
    }

    USER {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar name
        enum role
        uuid merchant_id FK
        boolean is_active
        boolean mfa_enabled
        varchar mfa_secret
        jsonb mfa_recovery_codes
        timestamptz last_login_at
        inet last_login_ip
        timestamptz created_at
        timestamptz updated_at
    }

    API_KEY {
        uuid id PK
        uuid merchant_id FK
        uuid user_id FK
        varchar name
        varchar key_hash UK
        varchar key_prefix
        jsonb scopes
        integer rate_limit
        boolean is_active
        timestamptz last_used_at
        timestamptz expires_at
        timestamptz created_at
    }

    AUDIT_LOG {
        uuid id PK
        uuid merchant_id FK
        uuid actor_id FK
        enum actor_type
        varchar action
        varchar resource_type
        uuid resource_id
        jsonb changes
        inet ip_address
        text user_agent
        timestamptz created_at
    }

    EXCHANGE_RATE {
        serial id PK
        char base_currency
        char quote_currency
        numeric rate
        varchar source
        timestamptz fetched_at
        timestamptz expires_at
    }

    IDEMPOTENCY_KEY {
        serial id PK
        varchar key_hash UK
        jsonb response
        smallint status_code
        uuid merchant_id FK
        timestamptz created_at
        timestamptz expires_at
    }

    BACKGROUND_TASK {
        uuid id PK
        varchar type
        jsonb payload
        enum status
        smallint priority
        smallint attempts
        smallint max_attempts
        text last_error
        timestamptz next_run_at
        uuid locked_by
        timestamptz locked_at
        timestamptz created_at
        timestamptz updated_at
    }
```

## Indexing Strategy

All indexes are B-tree unless otherwise noted.

| Principle | Detail |
|-----------|--------|
| Tenant isolation | Every query-scoped table has a `merchant_id` index as the leading column |
| Unique constraints | `slug`, `email`, `key_hash`, `idempotency_key`, `provider_transaction_id`, `provider_subscription_id` are unique |
| Composite indexes | `provider_configs(merchant_id, provider)`, `audit_logs(resource_type, resource_id)` |
| Status filtering | Index on `status` columns for Transaction, Subscription, Invoice, Payout, Dispute |
| Time-range queries | Index on `created_at` and `processed_at` for partitioned tables |
| Background tasks | Composite index `(status, next_run_at)` for efficient worker claiming |
| Idempotency cleanup | Index on `expires_at` for TTL-based garbage collection |

Full index definitions per entity are in [schema.md](schema.md).

## Partitioning Strategy

Two tables use monthly range partitioning on `created_at`:

| Table | Partition Key | Rationale |
|-------|--------------|-----------|
| Transaction | `created_at` | High write volume; time-range queries dominate |
| AuditLog | `created_at` | Append-only; time-range queries for compliance |

- Partitions are created monthly via a scheduled job (e.g., `pg_partman`).
- Old partitions can be detached and archived per retention policy.
- Foreign key constraints are deferred to the partition key to avoid cross-partition FK issues.

## Soft Delete Pattern

Applied to **Merchant** and **Customer** only.

```sql
-- Query pattern: exclude soft-deleted rows
SELECT * FROM merchants WHERE deleted_at IS NULL;

-- Restore a soft-deleted entity
UPDATE merchants SET deleted_at = NULL WHERE id = $1;
```

Cascade behavior: when a Merchant is soft-deleted, associated entities are not cascade-deleted but are excluded from active queries via `merchant_id` joins filtered through the merchant's `deleted_at`.

## JSONB Usage

| Table | Column | Content |
|-------|--------|---------|
| Merchant | `branding` | `{ logo_url, colors: { primary, accent }, slogan }` |
| Merchant | `settings` | Merchant-specific configuration knobs |
| Customer | `metadata` | Arbitrary key-value pairs from the merchant |
| PaymentMethod | `metadata` | Custom data attached to the payment instrument |
| Transaction | `metadata` | Custom data attached to the transaction |
| Subscription | `metadata` | Custom data attached to the subscription |
| Invoice | `metadata` | Custom data attached to the invoice |
| Payout | `metadata` | Custom data attached to the payout |
| Dispute | `metadata` | Custom data attached to the dispute |
| WebhookConfig | `metadata` | Custom data |
| ProviderConfig | `config` | Encrypted API keys and webhook secrets |
| ProviderConfig | `metadata` | Custom data |
| User | `mfa_recovery_codes` | Hashed recovery codes array |
| ApiKey | `scopes` | Array of allowed operation scopes |
| AuditLog | `changes` | `{ before: {...}, after: {...} }` diff object |
| IdempotencyKey | `response` | Cached response body |
| BackgroundTask | `payload` | Task-specific data |

## Data Retention

| Data | Retention | Mechanism |
|------|-----------|-----------|
| IdempotencyKey | 24-48 hours | Background job deletes expired rows hourly |
| AuditLog | Indefinite (configurable) | Merchant-level retention setting; partition detach for archival |
| Transaction | Indefinite | System of record; never hard-deleted |
| BackgroundTask completed | 7 days | Background job purges completed/dead tasks |
| ExchangeRate | 7 days | Background job purges expired rates |

## Security Considerations

- **Payment tokens**: `provider_token` stores provider-issued tokens, never raw card data. PCI DSS scope is minimized.
- **API keys**: Only `key_hash` (SHA-256) and `key_prefix` are stored. Full key is shown once at creation.
- **Provider credentials**: `ProviderConfig.config` is encrypted at rest; decryption happens only in the provider adapter layer.
- **MFA secrets**: `User.mfa_secret` is encrypted; recovery codes are hashed.
- **Row-level security**: Enforced via `merchant_id` checks in the application layer (RLS policies in PostgreSQL are optional).
