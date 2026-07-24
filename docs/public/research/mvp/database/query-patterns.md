# Query Patterns

Common SQL query patterns used throughout the Helix Seller platform.

## Merchant Lookup

### By Slug (API Routing)

```sql
SELECT id, name, email, slug, status, default_currency, timezone, branding, settings
FROM merchants
WHERE slug = $1 AND deleted_at IS NULL;
```

### By Email (Login)

```sql
SELECT id, name, email, slug, status
FROM merchants
WHERE email = $1 AND deleted_at IS NULL;
```

### Active Merchant Listing

```sql
SELECT id, name, slug, status, created_at
FROM merchants
WHERE deleted_at IS NULL AND status = 'active'
ORDER BY created_at DESC;
```

---

## Customer Lookup

### By Merchant and External ID

```sql
SELECT id, merchant_id, external_id, name, email, phone, metadata
FROM customers
WHERE merchant_id = $1 AND external_id = $2 AND deleted_at IS NULL;
```

### By Merchant and Email

```sql
SELECT id, merchant_id, external_id, name, email, phone, metadata
FROM customers
WHERE merchant_id = $1 AND email = $2 AND deleted_at IS NULL;
```

### Customer Listing with Pagination

```sql
SELECT id, external_id, name, email, phone, metadata, created_at
FROM customers
WHERE merchant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

### Customer Search (Name/Email)

```sql
SELECT id, external_id, name, email, phone, metadata, created_at
FROM customers
WHERE merchant_id = $1
  AND deleted_at IS NULL
  AND (name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%')
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

---

## Transaction Queries

### By Merchant (Paginated)

```sql
SELECT id, customer_id, provider, type, amount, currency, status,
       payment_method_id, description, metadata, error_code,
       fee_amount, net_amount, processed_at, created_at
FROM transactions
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

### By Customer

```sql
SELECT id, provider, type, amount, currency, status,
       description, metadata, processed_at, created_at
FROM transactions
WHERE merchant_id = $1 AND customer_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### By Status

```sql
SELECT id, customer_id, provider, type, amount, currency, status, created_at
FROM transactions
WHERE merchant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### By Date Range

```sql
SELECT id, customer_id, provider, type, amount, currency, status,
       fee_amount, net_amount, processed_at, created_at
FROM transactions
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at < $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
```

### By Provider Transaction ID

```sql
SELECT id, merchant_id, customer_id, provider, type, amount, currency,
       status, payment_method_id, metadata, processed_at, created_at
FROM transactions
WHERE provider_transaction_id = $1;
```

### Idempotency Check

```sql
SELECT id, status, response_body, created_at
FROM transactions
WHERE idempotency_key = $1 AND merchant_id = $2;
```

### Successful Charges (Revenue)

```sql
SELECT id, customer_id, amount, currency, fee_amount, net_amount,
       processed_at, created_at
FROM transactions
WHERE merchant_id = $1
  AND type = 'charge'
  AND status = 'succeeded'
  AND created_at >= $2
  AND created_at < $3
ORDER BY processed_at DESC;
```

### Transaction Summary by Status

```sql
SELECT status, COUNT(*) as count, SUM(amount) as total_amount
FROM transactions
WHERE merchant_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY status;
```

---

## Subscription Queries

### By Merchant (Active Subscriptions)

```sql
SELECT s.id, s.customer_id, s.provider, s.plan_id, s.status,
       s.amount, s.currency, s.interval, s.interval_count,
       s.current_period_start, s.current_period_end,
       s.cancel_at, s.trial_start, s.trial_end,
       s.metadata, s.created_at
FROM subscriptions s
WHERE s.merchant_id = $1 AND s.status = 'active'
ORDER BY s.created_at DESC;
```

### By Customer

```sql
SELECT id, provider, plan_id, status, amount, currency,
       interval, interval_count, current_period_start,
       current_period_end, cancel_at, cancelled_at,
       trial_start, trial_end, metadata, created_at
FROM subscriptions
WHERE merchant_id = $1 AND customer_id = $2
ORDER BY created_at DESC;
```

### By Status

```sql
SELECT id, customer_id, provider, plan_id, status, amount,
       currency, current_period_end, created_at
FROM subscriptions
WHERE merchant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### Renewal Batch (Due for Renewal)

```sql
SELECT id, merchant_id, customer_id, provider, provider_subscription_id,
       plan_id, amount, currency, interval, interval_count,
       current_period_start, current_period_end
FROM subscriptions
WHERE status = 'active'
  AND current_period_end <= $1
ORDER BY current_period_end ASC
LIMIT $2;
```

### Trial Expiration

```sql
SELECT id, merchant_id, customer_id, status, amount, currency,
       trial_start, trial_end
FROM subscriptions
WHERE status = 'trialing'
  AND trial_end <= $1
ORDER BY trial_end ASC;
```

### Subscription Analytics

```sql
SELECT
    status,
    COUNT(*) as count,
    SUM(amount) as total_amount,
    AVG(amount) as avg_amount
FROM subscriptions
WHERE merchant_id = $1
GROUP BY status;
```

---

## Invoice Queries

### By Merchant (Overdue)

```sql
SELECT i.id, i.customer_id, i.subscription_id, i.amount,
       i.currency, i.status, i.due_date, i.paid_at, i.created_at
FROM invoices i
WHERE i.merchant_id = $1
  AND i.status = 'open'
  AND i.due_date < CURRENT_DATE
ORDER BY i.due_date ASC;
```

### By Customer

```sql
SELECT id, subscription_id, provider, amount, currency,
       status, due_date, paid_at, period_start, period_end,
       metadata, created_at
FROM invoices
WHERE merchant_id = $1 AND customer_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### Revenue Summary (Paid Invoices)

```sql
SELECT
    DATE_TRUNC('month', paid_at) as month,
    COUNT(*) as invoice_count,
    SUM(amount) as total_revenue,
    AVG(amount) as avg_invoice_value
FROM invoices
WHERE merchant_id = $1
  AND status = 'paid'
  AND paid_at >= $2
  AND paid_at < $3
GROUP BY DATE_TRUNC('month', paid_at)
ORDER BY month DESC;
```

---

## Payout Queries

### Upcoming Payouts

```sql
SELECT id, provider, amount, currency, status, method,
       arrival_date, fee_amount, created_at
FROM payouts
WHERE merchant_id = $1
  AND status IN ('pending', 'in_transit')
ORDER BY arrival_date ASC;
```

### Payout History

```sql
SELECT id, provider, amount, currency, status, method,
       arrival_date, fee_amount, created_at, updated_at
FROM payouts
WHERE merchant_id = $1
  AND status = 'paid'
ORDER BY arrival_date DESC
LIMIT $2 OFFSET $3;
```

---

## Analytics Aggregations

### Revenue by Day

```sql
SELECT
    DATE(created_at) as day,
    COUNT(*) as transaction_count,
    SUM(amount) as gross_volume,
    SUM(fee_amount) as total_fees,
    SUM(net_amount) as net_volume
FROM transactions
WHERE merchant_id = $1
  AND type = 'charge'
  AND status = 'succeeded'
  AND created_at >= $2
  AND created_at < $3
GROUP BY DATE(created_at)
ORDER BY day DESC;
```

### Revenue by Month

```sql
SELECT
    DATE_TRUNC('month', created_at) as month,
    COUNT(*) as transaction_count,
    SUM(amount) as gross_volume,
    SUM(fee_amount) as total_fees,
    SUM(net_amount) as net_volume
FROM transactions
WHERE merchant_id = $1
  AND type = 'charge'
  AND status = 'succeeded'
  AND created_at >= $2
  AND created_at < $3
GROUP BY DATE_TRUNC('month', created_at)
ORDER BY month DESC;
```

### Revenue by Customer

```sql
SELECT
    c.id as customer_id,
    c.name as customer_name,
    c.email as customer_email,
    COUNT(t.id) as transaction_count,
    SUM(t.amount) as total_volume,
    SUM(t.fee_amount) as total_fees
FROM transactions t
JOIN customers c ON t.customer_id = c.id
WHERE t.merchant_id = $1
  AND t.type = 'charge'
  AND t.status = 'succeeded'
  AND t.created_at >= $2
  AND t.created_at < $3
GROUP BY c.id, c.name, c.email
ORDER BY total_volume DESC
LIMIT $4;
```

### Transaction Volume by Provider

```sql
SELECT
    provider,
    COUNT(*) as transaction_count,
    SUM(amount) as total_volume,
    SUM(fee_amount) as total_fees
FROM transactions
WHERE merchant_id = $1
  AND status = 'succeeded'
  AND created_at >= $2
  AND created_at < $3
GROUP BY provider
ORDER BY total_volume DESC;
```

### Subscription MRR (Monthly Recurring Revenue)

```sql
SELECT
    SUM(
        CASE
            WHEN interval = 'month' THEN amount
            WHEN interval = 'year' THEN amount / 12
            WHEN interval = 'week' THEN amount * 4.33
            WHEN interval = 'day' THEN amount * 30
            ELSE 0
        END
    ) as mrr
FROM subscriptions
WHERE merchant_id = $1
  AND status IN ('active', 'trialing');
```

### Churn Rate

```sql
WITH active_start AS (
    SELECT COUNT(*) as count
    FROM subscriptions
    WHERE merchant_id = $1
      AND created_at < $2
      AND status IN ('active', 'trialing')
),
cancelled AS (
    SELECT COUNT(*) as count
    FROM subscriptions
    WHERE merchant_id = $1
      AND cancelled_at >= $2
      AND cancelled_at < $3
)
SELECT
    a.count as starting_active,
    c.count as churned,
    CASE WHEN a.count > 0
        THEN ROUND(c.count::numeric / a.count * 100, 2)
        ELSE 0
    END as churn_rate_pct
FROM active_start a, cancelled c;
```

---

## Idempotency Key Lookup

### Check Existing Key

```sql
SELECT id, response, status_code, created_at, expires_at
FROM idempotency_keys
WHERE key_hash = $1;
```

### Store Idempotency Key

```sql
INSERT INTO idempotency_keys (key_hash, response, status_code, merchant_id, created_at, expires_at)
VALUES ($1, $2, $3, $4, NOW(), NOW() + INTERVAL '24 hours')
ON CONFLICT (key_hash) DO NOTHING;
```

### Cleanup Expired Keys

```sql
DELETE FROM idempotency_keys
WHERE expires_at < NOW();
```

---

## Background Task Claiming

### Claim Next Task (FOR UPDATE SKIP LOCKED)

```sql
UPDATE background_tasks
SET status = 'running',
    locked_by = $1,
    locked_at = NOW()
WHERE id = (
    SELECT id
    FROM background_tasks
    WHERE status = 'pending'
      AND next_run_at <= NOW()
    ORDER BY priority DESC, next_run_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, type, payload, priority, attempts, max_attempts;
```

### Complete Task

```sql
UPDATE background_tasks
SET status = 'completed',
    updated_at = NOW()
WHERE id = $1 AND locked_by = $2;
```

### Fail Task (Retry)

```sql
UPDATE background_tasks
SET status = CASE
        WHEN attempts + 1 >= max_attempts THEN 'dead'
        ELSE 'pending'
    END,
    attempts = attempts + 1,
    last_error = $3,
    next_run_at = NOW() + ( INTERVAL '1 minute' * POWER(2, attempts) ),
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE id = $1 AND locked_by = $2
RETURNING status, attempts;
```

### Stale Lock Recovery

```sql
UPDATE background_tasks
SET status = 'pending',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE status = 'running'
  AND locked_at < NOW() - INTERVAL '5 minutes';
```

### Task Cleanup

```sql
DELETE FROM background_tasks
WHERE status IN ('completed', 'dead')
  AND updated_at < NOW() - INTERVAL '7 days';
```

---

## Exchange Rate Queries

### Get Current Rate

```sql
SELECT rate, fetched_at, expires_at
FROM exchange_rates
WHERE base_currency = $1 AND quote_currency = $2
  AND expires_at > NOW()
ORDER BY fetched_at DESC
LIMIT 1;
```

### Upsert Rate

```sql
INSERT INTO exchange_rates (base_currency, quote_currency, rate, source, fetched_at, expires_at)
VALUES ($1, $2, $3, $4, NOW(), NOW() + INTERVAL '1 hour')
ON CONFLICT (base_currency, quote_currency)
DO UPDATE SET
    rate = EXCLUDED.rate,
    source = EXCLUDED.source,
    fetched_at = EXCLUDED.fetched_at,
    expires_at = EXCLUDED.expires_at;
```

### Cleanup Expired Rates

```sql
DELETE FROM exchange_rates
WHERE expires_at < NOW() - INTERVAL '7 days';
```

---

## Audit Log Queries

### By Resource (Entity History)

```sql
SELECT id, actor_id, actor_type, action, changes,
       ip_address, user_agent, created_at
FROM audit_logs
WHERE merchant_id = $1
  AND resource_type = $2
  AND resource_id = $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
```

### By Actor (User Activity)

```sql
SELECT id, action, resource_type, resource_id, changes,
       ip_address, created_at
FROM audit_logs
WHERE merchant_id = $1 AND actor_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### By Action Type

```sql
SELECT id, actor_id, actor_type, resource_type, resource_id,
       changes, ip_address, created_at
FROM audit_logs
WHERE merchant_id = $1 AND action = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
```

### Security Investigation (By IP)

```sql
SELECT id, actor_id, actor_type, action, resource_type,
       resource_id, user_agent, created_at
FROM audit_logs
WHERE ip_address = $1
ORDER BY created_at DESC
LIMIT $2;
```

---

## Exchange Rate Query

### Currency Conversion

```sql
WITH rate AS (
    SELECT rate
    FROM exchange_rates
    WHERE base_currency = $1 AND quote_currency = $2
      AND expires_at > NOW()
    ORDER BY fetched_at DESC
    LIMIT 1
)
SELECT $3 * rate as converted_amount
FROM rate;
```
