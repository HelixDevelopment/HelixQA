# Data Model: Helix Seller Platform

**Date**: 2026-07-23
**Spec**: [spec.md](spec.md)

## Entity Relationship Overview

```
Merchant ─┬─ Customer ─┬─ PaymentMethod
           │            ├─ Subscription
           │            └─ Invoice
           ├─ Transaction ─┬─ Refund
           │                └─ Dispute
           ├─ Payout
           ├─ WebhookConfig
           ├─ ProviderConfig
           └─ AuditLog
```

## Entities

### Merchant

The top-level tenant entity. All data is scoped to a merchant.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| name | VARCHAR(255) | NOT NULL | Business name |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Primary contact email |
| slug | VARCHAR(100) | UNIQUE, NOT NULL | URL-safe identifier |
| status | ENUM | NOT NULL | active, suspended, pending_verification |
| default_currency | CHAR(3) | NOT NULL, DEFAULT 'USD' | ISO 4217 currency code |
| timezone | VARCHAR(50) | NOT NULL, DEFAULT 'UTC' | IANA timezone |
| branding | JSONB | DEFAULT '{}' | White-label config (colors, logo_url, slogan) |
| settings | JSONB | DEFAULT '{}' | Merchant-specific settings |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |
| deleted_at | TIMESTAMPTZ | NULL | Soft delete timestamp |

**Indexes**: slug (unique), email (unique), status

---

### Customer

First-class entity owned by merchants. Required for subscriptions, invoices, and recurring billing.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| external_id | VARCHAR(255) | NULL | Merchant's own customer identifier |
| name | VARCHAR(255) | NOT NULL | Customer name |
| email | VARCHAR(255) | NULL | Customer email |
| phone | VARCHAR(50) | NULL | Customer phone |
| metadata | JSONB | DEFAULT '{}' | Custom key-value pairs |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |
| deleted_at | TIMESTAMPTZ | NULL | Soft delete timestamp |

**Indexes**: merchant_id, email (per merchant), external_id (per merchant)

---

### PaymentMethod

Tokenized payment instrument. Never stores raw card data.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| customer_id | UUID | FK → Customer, NULL | Associated customer (nullable for merchant defaults) |
| type | ENUM | NOT NULL | card, bank_account, wallet |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_token | VARCHAR(500) | NOT NULL | Provider-specific token |
| fingerprint | VARCHAR(255) | NULL | Card fingerprint for dedup |
| brand | VARCHAR(50) | NULL | Card brand (visa, mastercard) |
| last4 | CHAR(4) | NULL | Last 4 digits of card |
| exp_month | SMALLINT | NULL | Expiration month |
| exp_year | SMALLINT | NULL | Expiration year |
| is_default | BOOLEAN | DEFAULT false | Default payment method |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, customer_id, fingerprint, provider_token

---

### Transaction

Unified payment transaction record. System of record for all payment operations.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| customer_id | UUID | FK → Customer, NULL | Associated customer |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_transaction_id | VARCHAR(255) | NOT NULL | Provider's transaction ID |
| type | ENUM | NOT NULL | charge, refund, payout |
| amount | BIGINT | NOT NULL | Amount in smallest currency unit (cents) |
| currency | CHAR(3) | NOT NULL | ISO 4217 currency code |
| status | ENUM | NOT NULL | pending, processing, succeeded, failed, cancelled, reversed |
| payment_method_id | UUID | FK → PaymentMethod, NULL | Payment method used |
| idempotency_key | VARCHAR(255) | UNIQUE | Client-provided idempotency key |
| description | TEXT | NULL | Transaction description |
| metadata | JSONB | DEFAULT '{}' | Custom key-value pairs |
| error_code | VARCHAR(100) | NULL | Provider error code if failed |
| error_message | TEXT | NULL | Human-readable error |
| fee_amount | BIGINT | DEFAULT 0 | Platform fee in smallest currency unit |
| net_amount | BIGINT | NULL | Amount after fees |
| processed_at | TIMESTAMPTZ | NULL | When transaction completed |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, customer_id, provider_transaction_id (unique per provider), idempotency_key (unique), status, type, created_at, processed_at

**Partitioning**: By created_at (monthly) for performance at scale.

---

### Subscription

Recurring payment subscription.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| customer_id | UUID | FK → Customer, NOT NULL | Subscriber |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_subscription_id | VARCHAR(255) | NOT NULL | Provider's subscription ID |
| plan_id | VARCHAR(255) | NOT NULL | Provider plan/price ID |
| status | ENUM | NOT NULL | active, past_due, cancelled, unpaid, trialing |
| amount | BIGINT | NOT NULL | Recurring amount in smallest unit |
| currency | CHAR(3) | NOT NULL | ISO 4217 currency code |
| interval | ENUM | NOT NULL | day, week, month, year |
| interval_count | SMALLINT | DEFAULT 1 | Units per interval |
| current_period_start | TIMESTAMPTZ | NOT NULL | Current billing period start |
| current_period_end | TIMESTAMPTZ | NOT NULL | Current billing period end |
| cancel_at | TIMESTAMPTZ | NULL | Scheduled cancellation |
| cancelled_at | TIMESTAMPTZ | NULL | Actual cancellation timestamp |
| trial_start | TIMESTAMPTZ | NULL | Trial period start |
| trial_end | TIMESTAMPTZ | NULL | Trial period end |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, customer_id, provider_subscription_id (unique per provider), status, current_period_end

---

### Invoice

Payment invoice for a customer.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| customer_id | UUID | FK → Customer, NOT NULL | Invoice recipient |
| subscription_id | UUID | FK → Subscription, NULL | Related subscription |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_invoice_id | VARCHAR(255) | NULL | Provider's invoice ID |
| amount | BIGINT | NOT NULL | Invoice amount in smallest unit |
| currency | CHAR(3) | NOT NULL | ISO 4217 currency code |
| status | ENUM | NOT NULL | draft, open, paid, void, uncollectible |
| due_date | DATE | NOT NULL | Payment due date |
| paid_at | TIMESTAMPTZ | NULL | When invoice was paid |
| period_start | TIMESTAMPTZ | NOT NULL | Billing period start |
| period_end | TIMESTAMPTZ | NOT NULL | Billing period end |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, customer_id, status, due_date, paid_at

---

### Payout

Settlement payout from platform to merchant.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Recipient merchant |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_payout_id | VARCHAR(255) | NULL | Provider's payout ID |
| amount | BIGINT | NOT NULL | Payout amount in smallest unit |
| currency | CHAR(3) | NOT NULL | ISO 4217 currency code |
| status | ENUM | NOT NULL | pending, in_transit, paid, failed, cancelled |
| method | ENUM | NOT NULL | standard, instant |
| arrival_date | DATE | NOT NULL | Expected arrival date |
| fee_amount | BIGINT | DEFAULT 0 | Provider payout fee |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, status, arrival_date, created_at

---

### Dispute

Payment dispute/chargeback.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| transaction_id | UUID | FK → Transaction, NOT NULL | Disputed transaction |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| provider_dispute_id | VARCHAR(255) | NOT NULL | Provider's dispute ID |
| reason | VARCHAR(255) | NOT NULL | Dispute reason code |
| status | ENUM | NOT NULL | warning_needs_response, under_review, lost, won, closed |
| amount | BIGINT | NOT NULL | Disputed amount in smallest unit |
| evidence_deadline | TIMESTAMPTZ | NULL | Deadline to submit evidence |
| evidence_submitted_at | TIMESTAMPTZ | NULL | When evidence was submitted |
| resolution | VARCHAR(255) | NULL | Final resolution |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: transaction_id, merchant_id, status, evidence_deadline

---

### WebhookConfig

Merchant-configured outgoing webhook endpoints.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| url | VARCHAR(2048) | NOT NULL | Webhook endpoint URL |
| secret | VARCHAR(255) | NOT NULL | Signing secret for payload verification |
| events | TEXT[] | NOT NULL | Array of event types to subscribe to |
| is_active | BOOLEAN | DEFAULT true | Whether webhook is active |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id, is_active

---

### ProviderConfig

Payment provider configuration per merchant.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owner merchant |
| provider | VARCHAR(50) | NOT NULL | stripe, paypal, square |
| is_active | BOOLEAN | DEFAULT true | Whether provider is active |
| config | JSONB | NOT NULL | Encrypted provider credentials (API keys, webhook secrets) |
| fallback_order | SMALLINT | DEFAULT 0 | Priority in fallback chain (0 = primary) |
| health_status | ENUM | DEFAULT healthy | healthy, degraded, unhealthy |
| last_health_check | TIMESTAMPTZ | NULL | Last health check timestamp |
| metadata | JSONB | DEFAULT '{}' | Custom data |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: merchant_id + provider (unique), fallback_order

**Note**: The `config` field contains encrypted API keys and secrets. Decryption happens only in the provider adapter layer.

---

### User

System users with role-based access. Multiple users per merchant; one root admin across the platform.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Login email |
| password_hash | VARCHAR(255) | NOT NULL | Argon2id hash |
| name | VARCHAR(255) | NOT NULL | Display name |
| role | ENUM | NOT NULL | root_admin, account_admin, user |
| merchant_id | UUID | FK → Merchant, NULL | Associated merchant (NULL for root_admin) |
| is_active | BOOLEAN | NOT NULL, DEFAULT true | Account enabled |
| mfa_enabled | BOOLEAN | NOT NULL, DEFAULT false | MFA activated |
| mfa_secret | VARCHAR(64) | NULL | TOTP secret (encrypted) |
| mfa_recovery_codes | JSONB | NULL | Hashed recovery codes |
| last_login_at | TIMESTAMPTZ | NULL | Last successful login |
| last_login_ip | INET | NULL | Last login IP |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: email (unique), merchant_id, role

**Note**: Root admin (role = root_admin) has merchant_id = NULL and exists as a single row. All other users belong to a merchant.

---

### ApiKey

API keys for programmatic access (SDK/CLI). Scoped per merchant with rate limiting.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owning merchant |
| user_id | UUID | FK → User, NOT NULL | Created by user |
| name | VARCHAR(255) | NOT NULL | Human-readable name |
| key_hash | VARCHAR(64) | UNIQUE, NOT NULL | SHA-256 hash of full key |
| key_prefix | VARCHAR(8) | NOT NULL | First 8 chars for identification |
| scopes | JSONB | NOT NULL, DEFAULT '[]' | Allowed operations |
| rate_limit | INTEGER | NOT NULL, DEFAULT 0 | Requests per second (0 = unlimited) |
| is_active | BOOLEAN | NOT NULL, DEFAULT true | Key enabled |
| last_used_at | TIMESTAMPTZ | NULL | Last usage timestamp |
| expires_at | TIMESTAMPTZ | NULL | Expiration (NULL = never) |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |

**Indexes**: key_hash (unique), merchant_id, user_id, is_active

**Note**: Full API key is shown only at creation time. Only the hash and prefix are stored.

---

### AuditLog

Immutable audit trail for all actions.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Scope merchant |
| actor_id | UUID | FK → User, NOT NULL | Who performed the action |
| actor_type | ENUM | NOT NULL | root_admin, account_admin, user, system, api_key |
| action | VARCHAR(100) | NOT NULL | Action performed (e.g., transaction.charge, merchant.update) |
| resource_type | VARCHAR(50) | NOT NULL | Resource type affected |
| resource_id | UUID | NULL | Resource ID affected |
| changes | JSONB | NULL | Before/after diff for updates |
| ip_address | INET | NULL | Client IP address |
| user_agent | TEXT | NULL | Client user agent |
| created_at | TIMESTAMPTZ | NOT NULL | Event timestamp |

**Indexes**: merchant_id, actor_id, action, resource_type + resource_id, created_at

**Partitioning**: By created_at (monthly) for performance. Retention: indefinite (configurable per merchant).

---

### ExchangeRate

Cached exchange rates for multi-currency conversion.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | SERIAL | PK | Auto-increment |
| base_currency | CHAR(3) | NOT NULL | Base currency code |
| quote_currency | CHAR(3) | NOT NULL | Quote currency code |
| rate | NUMERIC(18,8) | NOT NULL | Exchange rate |
| source | VARCHAR(50) | NOT NULL | frankfurter, exchangerate-api, open-exchange-rates |
| fetched_at | TIMESTAMPTZ | NOT NULL | When rate was fetched |
| expires_at | TIMESTAMPTZ | NOT NULL | When rate expires |

**Indexes**: base_currency + quote_currency (unique), expires_at

---

### IdempotencyKey

Request deduplication store.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | SERIAL | PK | Auto-increment |
| key_hash | VARCHAR(64) | UNIQUE, NOT NULL | SHA-256 hash of idempotency key |
| response | JSONB | NOT NULL | Cached response body |
| status_code | SMALLINT | NOT NULL | Cached HTTP status code |
| merchant_id | UUID | FK → Merchant, NOT NULL | Request owner |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| expires_at | TIMESTAMPTZ | NOT NULL | TTL expiration |

**Indexes**: key_hash (unique), expires_at

**Cleanup**: Background job deletes expired rows every hour.

---

### BackgroundTask

Job queue for async processing.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| type | VARCHAR(100) | NOT NULL | Task type (e.g., webhook.send, reconciliation.run) |
| payload | JSONB | NOT NULL | Task-specific data |
| status | ENUM | NOT NULL | pending, running, completed, failed, dead |
| priority | SMALLINT | DEFAULT 0 | Higher = processed first |
| attempts | SMALLINT | DEFAULT 0 | Number of attempts made |
| max_attempts | SMALLINT | DEFAULT 5 | Maximum retry attempts |
| last_error | TEXT | NULL | Last error message |
| next_run_at | TIMESTAMPTZ | NOT NULL | When to next attempt |
| locked_by | UUID | NULL | Worker ID that claimed this task |
| locked_at | TIMESTAMPTZ | NULL | When task was locked |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: status + next_run_at (for worker queries), locked_by, type
