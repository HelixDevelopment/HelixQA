# Schema Reference

Complete field, constraint, and index definitions for all 16 entities.

---

## Merchant

Top-level tenant entity. All data is scoped to a merchant.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier (UUID v7) |
| 2 | `name` | `VARCHAR(255)` | `NOT NULL` | Business name |
| 3 | `email` | `VARCHAR(255)` | `UNIQUE, NOT NULL` | Primary contact email |
| 4 | `slug` | `VARCHAR(100)` | `UNIQUE, NOT NULL` | URL-safe identifier (e.g., `acme-corp`) |
| 5 | `status` | `ENUM('active', 'suspended', 'pending_verification')` | `NOT NULL` | Account status |
| 6 | `default_currency` | `CHAR(3)` | `NOT NULL, DEFAULT 'USD'` | ISO 4217 currency code |
| 7 | `timezone` | `VARCHAR(50)` | `NOT NULL, DEFAULT 'UTC'` | IANA timezone identifier |
| 8 | `branding` | `JSONB` | `DEFAULT '{}'` | White-label configuration: `{ logo_url, colors: { primary, accent }, slogan }` |
| 9 | `settings` | `JSONB` | `DEFAULT '{}'` | Merchant-specific settings |
| 10 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 11 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |
| 12 | `deleted_at` | `TIMESTAMPTZ` | `NULL` | Soft delete timestamp (NULL = active) |

### Constraints

- `email` must be unique across all merchants
- `slug` must be unique, lowercase, alphanumeric with hyphens only
- `default_currency` must be a valid ISO 4217 code

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `merchants_slug_idx` | `slug` | UNIQUE | Fast lookup by slug (API routing) |
| `merchants_email_idx` | `email` | UNIQUE | Login and lookup by email |
| `merchants_status_idx` | `status` | BTREE | Filter active/suspended/pending merchants |

### Notes

- Soft-deleted merchants are excluded from active queries via `WHERE deleted_at IS NULL`.
- Cascade deletion of child entities is not performed; application-layer filtering applies.

---

## Customer

First-class entity owned by merchants. Required for subscriptions, invoices, and recurring billing.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `external_id` | `VARCHAR(255)` | `NULL` | Merchant's own customer identifier |
| 4 | `name` | `VARCHAR(255)` | `NOT NULL` | Customer name |
| 5 | `email` | `VARCHAR(255)` | `NULL` | Customer email |
| 6 | `phone` | `VARCHAR(50)` | `NULL` | Customer phone |
| 7 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom key-value pairs |
| 8 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 9 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |
| 10 | `deleted_at` | `TIMESTAMPTZ` | `NULL` | Soft delete timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `external_id` is unique per merchant (application-enforced or unique partial index)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `customers_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `customers_email_per_merchant_idx` | `merchant_id, email` | BTREE | Lookup by email within a merchant |
| `customers_external_id_per_merchant_idx` | `merchant_id, external_id` | BTREE | Lookup by merchant's external ID |

### Notes

- Soft-deleted customers follow the same pattern as Merchant.
- `external_id` allows merchants to map their own customer IDs to Helix customer records.

---

## PaymentMethod

Tokenized payment instrument. Never stores raw card data.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `customer_id` | `UUID` | `NULL, FOREIGN KEY → customers(id)` | Associated customer (nullable for merchant defaults) |
| 4 | `type` | `ENUM('card', 'bank_account', 'wallet')` | `NOT NULL` | Payment instrument type |
| 5 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name (stripe, paypal, square) |
| 6 | `provider_token` | `VARCHAR(500)` | `NOT NULL` | Provider-specific token |
| 7 | `fingerprint` | `VARCHAR(255)` | `NULL` | Card fingerprint for deduplication |
| 8 | `brand` | `VARCHAR(50)` | `NULL` | Card brand (visa, mastercard, amex) |
| 9 | `last4` | `CHAR(4)` | `NULL` | Last 4 digits of card |
| 10 | `exp_month` | `SMALLINT` | `NULL` | Expiration month (1-12) |
| 11 | `exp_year` | `SMALLINT` | `NULL` | Expiration year (4-digit) |
| 12 | `is_default` | `BOOLEAN` | `DEFAULT false` | Whether this is the default payment method |
| 13 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 14 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 15 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `customer_id` references `customers(id)` with `ON DELETE SET NULL`
- `exp_month` must be 1-12 if not NULL
- Only one `is_default = true` per customer (application-enforced)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `payment_methods_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `payment_methods_customer_id_idx` | `customer_id` | BTREE | Lookup by customer |
| `payment_methods_fingerprint_idx` | `fingerprint` | BTREE | Card deduplication |
| `payment_methods_provider_token_idx` | `provider_token` | BTREE | Provider token lookup |

### Notes

- No soft delete; payment methods are hard-deleted when removed.
- `provider_token` is a provider-issued token, not raw card data (PCI compliance).

---

## Transaction

Unified payment transaction record. System of record for all payment operations.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `customer_id` | `UUID` | `NULL, FOREIGN KEY → customers(id)` | Associated customer |
| 4 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 5 | `provider_transaction_id` | `VARCHAR(255)` | `NOT NULL` | Provider's transaction ID |
| 6 | `type` | `ENUM('charge', 'refund', 'payout')` | `NOT NULL` | Transaction type |
| 7 | `amount` | `BIGINT` | `NOT NULL` | Amount in smallest currency unit (cents) |
| 8 | `currency` | `CHAR(3)` | `NOT NULL` | ISO 4217 currency code |
| 9 | `status` | `ENUM('pending', 'processing', 'succeeded', 'failed', 'cancelled', 'reversed')` | `NOT NULL` | Transaction status |
| 10 | `payment_method_id` | `UUID` | `NULL, FOREIGN KEY → payment_methods(id)` | Payment method used |
| 11 | `idempotency_key` | `VARCHAR(255)` | `UNIQUE` | Client-provided idempotency key |
| 12 | `description` | `TEXT` | `NULL` | Human-readable description |
| 13 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom key-value pairs |
| 14 | `error_code` | `VARCHAR(100)` | `NULL` | Provider error code if failed |
| 15 | `error_message` | `TEXT` | `NULL` | Human-readable error message |
| 16 | `fee_amount` | `BIGINT` | `DEFAULT 0` | Platform fee in smallest currency unit |
| 17 | `net_amount` | `BIGINT` | `NULL` | Amount after fees (computed on completion) |
| 18 | `processed_at` | `TIMESTAMPTZ` | `NULL` | When transaction completed |
| 19 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 20 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `mersters(id)` with `ON DELETE RESTRICT`
- `customer_id` references `customers(id)` with `ON DELETE SET NULL`
- `payment_method_id` references `payment_methods(id)` with `ON DELETE SET NULL`
- `amount` must be > 0 for charges, >= 0 for refunds
- `net_amount` = `amount` - `fee_amount` (computed on status = succeeded)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `transactions_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `transactions_customer_id_idx` | `customer_id` | BTREE | Customer transaction history |
| `transactions_provider_transaction_id_idx` | `provider_transaction_id` | UNIQUE | Provider dedup / webhook matching |
| `transactions_idempotency_key_idx` | `idempotency_key` | UNIQUE | Idempotency lookup |
| `transactions_status_idx` | `status` | BTREE | Status filtering |
| `transactions_type_idx` | `type` | BTREE | Type filtering |
| `transactions_created_at_idx` | `created_at` | BTREE | Time-range queries |
| `transactions_processed_at_idx` | `processed_at` | BTREE | Processing time queries |

### Partitioning

Monthly range partition on `created_at`. Managed via `pg_partman` or manual DDL.

```
transactions_2026_01
transactions_2026_02
transactions_2026_03
...
```

### Notes

- Immutable once `status` reaches a terminal state (`succeeded`, `failed`, `cancelled`, `reversed`).
- `net_amount` is NULL until the transaction completes; computed as `amount - fee_amount`.
- Foreign key constraints are deferred to the partition key for partition compatibility.

---

## Subscription

Recurring payment subscription.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `customer_id` | `UUID` | `NOT NULL, FOREIGN KEY → customers(id)` | Subscriber |
| 4 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 5 | `provider_subscription_id` | `VARCHAR(255)` | `NOT NULL` | Provider's subscription ID |
| 6 | `plan_id` | `VARCHAR(255)` | `NOT NULL` | Provider plan/price ID |
| 7 | `status` | `ENUM('active', 'past_due', 'cancelled', 'unpaid', 'trialing')` | `NOT NULL` | Subscription status |
| 8 | `amount` | `BIGINT` | `NOT NULL` | Recurring amount in smallest unit |
| 9 | `currency` | `CHAR(3)` | `NOT NULL` | ISO 4217 currency code |
| 10 | `interval` | `ENUM('day', 'week', 'month', 'year')` | `NOT NULL` | Billing interval |
| 11 | `interval_count` | `SMALLINT` | `DEFAULT 1` | Units per interval (e.g., 3 = every 3 months) |
| 12 | `current_period_start` | `TIMESTAMPTZ` | `NOT NULL` | Current billing period start |
| 13 | `current_period_end` | `TIMESTAMPTZ` | `NOT NULL` | Current billing period end |
| 14 | `cancel_at` | `TIMESTAMPTZ` | `NULL` | Scheduled cancellation time |
| 15 | `cancelled_at` | `TIMESTAMPTZ` | `NULL` | Actual cancellation timestamp |
| 16 | `trial_start` | `TIMESTAMPTZ` | `NULL` | Trial period start |
| 17 | `trial_end` | `TIMESTAMPTZ` | `NULL` | Trial period end |
| 18 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 19 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 20 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `customer_id` references `customers(id)` with `ON DELETE RESTRICT`
- `current_period_end` must be > `current_period_start`
- `trial_end` must be > `trial_start` if both are set
- `cancelled_at` must be set if `status = 'cancelled'`

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `subscriptions_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `subscriptions_customer_id_idx` | `customer_id` | BTREE | Customer subscription history |
| `subscriptions_provider_subscription_id_idx` | `provider_subscription_id` | UNIQUE | Provider dedup / webhook matching |
| `subscriptions_status_idx` | `status` | BTREE | Status filtering |
| `subscriptions_current_period_end_idx` | `current_period_end` | BTREE | Renewal batch queries |

### Notes

- No soft delete; cancelled subscriptions remain for historical reference.
- Renewal jobs query `WHERE status = 'active' AND current_period_end <= NOW()`.

---

## Invoice

Payment invoice for a customer.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `customer_id` | `UUID` | `NOT NULL, FOREIGN KEY → customers(id)` | Invoice recipient |
| 4 | `subscription_id` | `UUID` | `NULL, FOREIGN KEY → subscriptions(id)` | Related subscription |
| 5 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 6 | `provider_invoice_id` | `VARCHAR(255)` | `NULL` | Provider's invoice ID |
| 7 | `amount` | `BIGINT` | `NOT NULL` | Invoice amount in smallest unit |
| 8 | `currency` | `CHAR(3)` | `NOT NULL` | ISO 4217 currency code |
| 9 | `status` | `ENUM('draft', 'open', 'paid', 'void', 'uncollectible')` | `NOT NULL` | Invoice status |
| 10 | `due_date` | `DATE` | `NOT NULL` | Payment due date |
| 11 | `paid_at` | `TIMESTAMPTZ` | `NULL` | When invoice was paid |
| 12 | `period_start` | `TIMESTAMPTZ` | `NOT NULL` | Billing period start |
| 13 | `period_end` | `TIMESTAMPTZ` | `NOT NULL` | Billing period end |
| 14 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 15 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 16 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `customer_id` references `customers(id)` with `ON DELETE RESTRICT`
- `subscription_id` references `subscriptions(id)` with `ON DELETE SET NULL`
- `paid_at` must be set when `status = 'paid'`
- `period_end` must be > `period_start`

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `invoices_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `invoices_customer_id_idx` | `customer_id` | BTREE | Customer invoice history |
| `invoices_status_idx` | `status` | BTREE | Status filtering |
| `invoices_due_date_idx` | `due_date` | BTREE | Overdue invoice queries |
| `invoices_paid_at_idx` | `paid_at` | BTREE | Payment reporting |

### Notes

- `subscription_id` is NULL for one-off invoices not tied to a subscription.
- Invoices with `status = 'draft'` may be updated; all other statuses are effectively immutable.

---

## Payout

Settlement payout from platform to merchant.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Recipient merchant |
| 3 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 4 | `provider_payout_id` | `VARCHAR(255)` | `NULL` | Provider's payout ID |
| 5 | `amount` | `BIGINT` | `NOT NULL` | Payout amount in smallest unit |
| 6 | `currency` | `CHAR(3)` | `NOT NULL` | ISO 4217 currency code |
| 7 | `status` | `ENUM('pending', 'in_transit', 'paid', 'failed', 'cancelled')` | `NOT NULL` | Payout status |
| 8 | `method` | `ENUM('standard', 'instant')` | `NOT NULL` | Payout method |
| 9 | `arrival_date` | `DATE` | `NOT NULL` | Expected arrival date |
| 10 | `fee_amount` | `BIGINT` | `DEFAULT 0` | Provider payout fee |
| 11 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 12 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 13 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `amount` must be > 0
- `arrival_date` must be >= creation date

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `payouts_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `payouts_status_idx` | `status` | BTREE | Status filtering |
| `payouts_arrival_date_idx` | `arrival_date` | BTREE | Upcoming payout queries |
| `payouts_created_at_idx` | `created_at` | BTREE | Time-range queries |

### Notes

- Payouts are immutable once `status` reaches a terminal state.

---

## Dispute

Payment dispute/chargeback.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `transaction_id` | `UUID` | `NOT NULL, FOREIGN KEY → transactions(id)` | Disputed transaction |
| 3 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 4 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 5 | `provider_dispute_id` | `VARCHAR(255)` | `NOT NULL` | Provider's dispute ID |
| 6 | `reason` | `VARCHAR(255)` | `NOT NULL` | Dispute reason code |
| 7 | `status` | `ENUM('warning_needs_response', 'under_review', 'lost', 'won', 'closed')` | `NOT NULL` | Dispute status |
| 8 | `amount` | `BIGINT` | `NOT NULL` | Disputed amount in smallest unit |
| 9 | `evidence_deadline` | `TIMESTAMPTZ` | `NULL` | Deadline to submit evidence |
| 10 | `evidence_submitted_at` | `TIMESTAMPTZ` | `NULL` | When evidence was submitted |
| 11 | `resolution` | `VARCHAR(255)` | `NULL` | Final resolution description |
| 12 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 13 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 14 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `transaction_id` references `transactions(id)` with `ON DELETE RESTRICT`
- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `evidence_submitted_at` must be <= `evidence_deadline` if both are set

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `disputes_transaction_id_idx` | `transaction_id` | BTREE | Lookup disputes by transaction |
| `disputes_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `disputes_status_idx` | `status` | BTREE | Status filtering |
| `disputes_evidence_deadline_idx` | `evidence_deadline` | BTREE | Urgent deadline queries |

### Notes

- Disputes are created via provider webhooks and updated as they progress.
- `warning_needs_response` status requires merchant action before the deadline.

---

## WebhookConfig

Merchant-configured outgoing webhook endpoints.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `url` | `VARCHAR(2048)` | `NOT NULL` | Webhook endpoint URL |
| 4 | `secret` | `VARCHAR(255)` | `NOT NULL` | HMAC signing secret for payload verification |
| 5 | `events` | `TEXT[]` | `NOT NULL` | Array of event types to subscribe to |
| 6 | `is_active` | `BOOLEAN` | `DEFAULT true` | Whether webhook is enabled |
| 7 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 8 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 9 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `url` must be a valid URL with https scheme
- `events` must not be empty array

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `webhook_configs_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `webhook_configs_is_active_idx` | `is_active` | BTREE | Filter active webhooks |

### Notes

- The `secret` is used to compute HMAC-SHA256 signatures for outgoing payloads.
- `events` uses PostgreSQL array type for efficient contains/overlap queries.

---

## ProviderConfig

Payment provider configuration per merchant.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owner merchant |
| 3 | `provider` | `VARCHAR(50)` | `NOT NULL` | Provider name |
| 4 | `is_active` | `BOOLEAN` | `DEFAULT true` | Whether provider is enabled |
| 5 | `config` | `JSONB` | `NOT NULL` | Encrypted provider credentials |
| 6 | `fallback_order` | `SMALLINT` | `DEFAULT 0` | Priority in fallback chain (0 = primary) |
| 7 | `health_status` | `ENUM('healthy', 'degraded', 'unhealthy')` | `DEFAULT 'healthy'` | Current health status |
| 8 | `last_health_check` | `TIMESTAMPTZ` | `NULL` | Last health check timestamp |
| 9 | `metadata` | `JSONB` | `DEFAULT '{}'` | Custom data |
| 10 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 11 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- Unique constraint on `(merchant_id, provider)` — one config per provider per merchant

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `provider_configs_merchant_provider_idx` | `merchant_id, provider` | UNIQUE | One config per provider per merchant |
| `provider_configs_fallback_order_idx` | `fallback_order` | BTREE | Fallback chain ordering |

### Notes

- `config` contains encrypted API keys and webhook secrets. Decryption happens only in the provider adapter layer.
- `fallback_order` determines provider selection when the primary provider is unhealthy.

---

## User

System users with role-based access. Multiple users per merchant; one root admin across the platform.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `email` | `VARCHAR(255)` | `UNIQUE, NOT NULL` | Login email |
| 3 | `password_hash` | `VARCHAR(255)` | `NOT NULL` | Argon2id password hash |
| 4 | `name` | `VARCHAR(255)` | `NOT NULL` | Display name |
| 5 | `role` | `ENUM('root_admin', 'account_admin', 'user')` | `NOT NULL` | User role |
| 6 | `merchant_id` | `UUID` | `NULL, FOREIGN KEY → merchants(id)` | Associated merchant (NULL for root_admin) |
| 7 | `is_active` | `BOOLEAN` | `NOT NULL, DEFAULT true` | Account enabled |
| 8 | `mfa_enabled` | `BOOLEAN` | `NOT NULL, DEFAULT false` | MFA activated |
| 9 | `mfa_secret` | `VARCHAR(64)` | `NULL` | TOTP secret (encrypted) |
| 10 | `mfa_recovery_codes` | `JSONB` | `NULL` | Hashed recovery codes |
| 11 | `last_login_at` | `TIMESTAMPTZ` | `NULL` | Last successful login timestamp |
| 12 | `last_login_ip` | `INET` | `NULL` | Last login IP address |
| 13 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 14 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `email` must be unique across all users
- `merchant_id` must be NULL when `role = 'root_admin'`
- `merchant_id` must NOT be NULL when `role IN ('account_admin', 'user')`
- Exactly one `root_admin` row must exist (application-enforced)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `users_email_idx` | `email` | UNIQUE | Login lookup |
| `users_merchant_id_idx` | `merchant_id` | BTREE | Merchant user listing |
| `users_role_idx` | `role` | BTREE | Role-based filtering |

### Notes

- Root admin (`role = root_admin`) has `merchant_id = NULL` and exists as a single row.
- MFA secrets are encrypted at rest; recovery codes are hashed individually.
- Password hashing uses Argon2id with configurable parameters.

---

## ApiKey

API keys for programmatic access (SDK/CLI). Scoped per merchant with rate limiting.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Owning merchant |
| 3 | `user_id` | `UUID` | `NOT NULL, FOREIGN KEY → users(id)` | Created by user |
| 4 | `name` | `VARCHAR(255)` | `NOT NULL` | Human-readable key name |
| 5 | `key_hash` | `VARCHAR(64)` | `UNIQUE, NOT NULL` | SHA-256 hash of full API key |
| 6 | `key_prefix` | `VARCHAR(8)` | `NOT NULL` | First 8 characters for identification |
| 7 | `scopes` | `JSONB` | `NOT NULL, DEFAULT '[]'` | Allowed operations array |
| 8 | `rate_limit` | `INTEGER` | `NOT NULL, DEFAULT 0` | Requests per second (0 = unlimited) |
| 9 | `is_active` | `BOOLEAN` | `NOT NULL, DEFAULT true` | Key enabled |
| 10 | `last_used_at` | `TIMESTAMPTZ` | `NULL` | Last usage timestamp |
| 11 | `expires_at` | `TIMESTAMPTZ` | `NULL` | Expiration (NULL = never expires) |
| 12 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `user_id` references `users(id)` with `ON DELETE RESTRICT`
- `key_hash` must be unique across all API keys
- `expires_at` must be > `created_at` if set

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `api_keys_key_hash_idx` | `key_hash` | UNIQUE | Authentication lookup |
| `api_keys_merchant_id_idx` | `merchant_id` | BTREE | Merchant key listing |
| `api_keys_user_id_idx` | `user_id` | BTREE | User key listing |
| `api_keys_is_active_idx` | `is_active` | BTREE | Active key filtering |

### Notes

- Full API key is shown only at creation time. Only `key_hash` and `key_prefix` are stored.
- Authentication flow: hash incoming key, look up by `key_hash`, verify `is_active` and `expires_at`.
- `scopes` controls fine-grained access (e.g., `["transactions:read", "customers:write"]`).

---

## AuditLog

Immutable audit trail for all actions.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Scope merchant |
| 3 | `actor_id` | `UUID` | `NOT NULL, FOREIGN KEY → users(id)` | Who performed the action |
| 4 | `actor_type` | `ENUM('root_admin', 'account_admin', 'user', 'system', 'api_key')` | `NOT NULL` | Type of actor |
| 5 | `action` | `VARCHAR(100)` | `NOT NULL` | Action name (e.g., `transaction.charge`, `merchant.update`) |
| 6 | `resource_type` | `VARCHAR(50)` | `NOT NULL` | Resource type affected |
| 7 | `resource_id` | `UUID` | `NULL` | Resource ID affected |
| 8 | `changes` | `JSONB` | `NULL` | Before/after diff: `{ before: {...}, after: {...} }` |
| 9 | `ip_address` | `INET` | `NULL` | Client IP address |
| 10 | `user_agent` | `TEXT` | `NULL` | Client user agent string |
| 11 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Event timestamp |

### Constraints

- `merchant_id` references `merchants(id)` with `ON DELETE RESTRICT`
- `actor_id` references `users(id)` with `ON DELETE RESTRICT`
- Rows are append-only; UPDATE and DELETE are forbidden (application and DB-level enforcement)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `audit_logs_merchant_id_idx` | `merchant_id` | BTREE | Tenant isolation |
| `audit_logs_actor_id_idx` | `actor_id` | BTREE | Actor activity queries |
| `audit_logs_action_idx` | `action` | BTREE | Action filtering |
| `audit_logs_resource_type_resource_id_idx` | `resource_type, resource_id` | BTREE | Resource history lookup |
| `audit_logs_created_at_idx` | `created_at` | BTREE | Time-range queries |
| `audit_logs_ip_address_idx` | `ip_address` | BTREE | IP-based investigation |

### Partitioning

Monthly range partition on `created_at`. Same strategy as Transaction.

### Notes

- AuditLog is immutable. No UPDATE or DELETE operations are permitted.
- `actor_type` distinguishes between human users, system actions, and API key actions.
- Retention is configurable per merchant; default is indefinite.
- `changes` captures the full before/after state for update operations.

---

## ExchangeRate

Cached exchange rates for multi-currency conversion.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `SERIAL` | `PRIMARY KEY` | Auto-increment identifier |
| 2 | `base_currency` | `CHAR(3)` | `NOT NULL` | Base currency code (e.g., USD) |
| 3 | `quote_currency` | `CHAR(3)` | `NOT NULL` | Quote currency code (e.g., EUR) |
| 4 | `rate` | `NUMERIC(18,8)` | `NOT NULL` | Exchange rate (8 decimal places) |
| 5 | `source` | `VARCHAR(50)` | `NOT NULL` | Rate provider (frankfurter, exchangerate-api, open-exchange-rates) |
| 6 | `fetched_at` | `TIMESTAMPTZ` | `NOT NULL` | When rate was fetched |
| 7 | `expires_at` | `TIMESTAMPTZ` | `NOT NULL` | When rate expires |

### Constraints

- `rate` must be > 0
- `expires_at` must be > `fetched_at`
- Unique constraint on `(base_currency, quote_currency)` — one active rate per pair

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `exchange_rates_pair_idx` | `base_currency, quote_currency` | UNIQUE | Rate lookup by currency pair |
| `exchange_rates_expires_at_idx` | `expires_at` | BTREE | Cleanup of expired rates |

### Notes

- Uses `SERIAL` (auto-increment) instead of UUID since this is a cache, not a user-facing entity.
- Background job fetches rates periodically and upserts by currency pair.
- Expired rates are purged by a background cleanup job.

---

## IdempotencyKey

Request deduplication store.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `SERIAL` | `PRIMARY KEY` | Auto-increment identifier |
| 2 | `key_hash` | `VARCHAR(64)` | `UNIQUE, NOT NULL` | SHA-256 hash of idempotency key |
| 3 | `response` | `JSONB` | `NOT NULL` | Cached response body |
| 4 | `status_code` | `SMALLINT` | `NOT NULL` | Cached HTTP status code |
| 5 | `merchant_id` | `UUID` | `NOT NULL, FOREIGN KEY → merchants(id)` | Request owner |
| 6 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 7 | `expires_at` | `TIMESTAMPTZ` | `NOT NULL` | TTL expiration |

### Constraints

- `key_hash` must be unique across all idempotency keys
- `expires_at` must be > `created_at`
- `status_code` must be a valid HTTP status code (100-599)

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `idempotency_keys_key_hash_idx` | `key_hash` | UNIQUE | Idempotency lookup |
| `idempotency_keys_expires_at_idx` | `expires_at` | BTREE | TTL cleanup |

### Notes

- Uses `SERIAL` instead of UUID; this is infrastructure, not user-facing.
- Background job deletes expired rows every hour.
- TTL is typically 24-48 hours.

---

## BackgroundTask

Job queue for async processing.

### Fields

| # | Field | Type | Constraints | Description |
|---|-------|------|-------------|-------------|
| 1 | `id` | `UUID` | `PRIMARY KEY` | Unique identifier |
| 2 | `type` | `VARCHAR(100)` | `NOT NULL` | Task type (e.g., `webhook.send`, `reconciliation.run`) |
| 3 | `payload` | `JSONB` | `NOT NULL` | Task-specific data |
| 4 | `status` | `ENUM('pending', 'running', 'completed', 'failed', 'dead')` | `NOT NULL` | Task status |
| 5 | `priority` | `SMALLINT` | `DEFAULT 0` | Higher value = processed first |
| 6 | `attempts` | `SMALLINT` | `DEFAULT 0` | Number of attempts made |
| 7 | `max_attempts` | `SMALLINT` | `DEFAULT 5` | Maximum retry attempts |
| 8 | `last_error` | `TEXT` | `NULL` | Last error message |
| 9 | `next_run_at` | `TIMESTAMPTZ` | `NOT NULL` | When to next attempt execution |
| 10 | `locked_by` | `UUID` | `NULL` | Worker ID that claimed this task |
| 11 | `locked_at` | `TIMESTAMPTZ` | `NULL` | When task was locked |
| 12 | `created_at` | `TIMESTAMPTZ` | `NOT NULL` | Row creation timestamp |
| 13 | `updated_at` | `TIMESTAMPTZ` | `NOT NULL` | Last modification timestamp |

### Constraints

- `attempts` must be <= `max_attempts`
- `locked_at` must be set when `locked_by` is set
- `next_run_at` must be >= `created_at`

### Indexes

| Name | Columns | Type | Purpose |
|------|---------|------|---------|
| `background_tasks_status_next_run_at_idx` | `status, next_run_at` | BTREE | Worker claiming query |
| `background_tasks_locked_by_idx` | `locked_by` | BTREE | Worker task listing |
| `background_tasks_type_idx` | `type` | BTREE | Task type filtering |

### Notes

- Worker claiming uses `SELECT ... FOR UPDATE SKIP LOCKED` on the `(status, next_run_at)` index.
- `dead` status indicates a task that has exceeded `max_attempts` and requires manual intervention.
- `locked_by` stores the worker's UUID; `locked_at` is set to prevent stale locks.
- Completed and dead tasks are purged by a background cleanup job (retention: 7 days).
