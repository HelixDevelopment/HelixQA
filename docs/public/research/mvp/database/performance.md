# Performance Considerations

## Connection Pooling

### pgx Pool (Go)

The application uses `pgxpool` for connection pooling:

```go
config, err := pgxpool.ParseConfig(databaseURL)
if err != nil {
    log.Fatal(err)
}

// Pool sizing formula:
// connections = (CPU cores * 2) + effective_spindle_count
// For SSD-backed PostgreSQL: (CPU cores * 2) + 1
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute

pool, err := pgxpool.NewWithConfig(context.Background(), config)
```

### Pool Sizing Guidelines

| Environment | Recommended Pool Size | Rationale |
|-------------|----------------------|-----------|
| Development | 5 | Single user, minimal concurrency |
| Staging | 10 | Load testing with simulated traffic |
| Production | 25-50 | Based on CPU cores and workload |
| High-throughput | 50-100 | With read replicas offloading queries |

### Connection Monitoring

```sql
-- Current connection count by state
SELECT state, COUNT(*)
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY state;

-- Connection count by application
SELECT application_name, state, COUNT(*)
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY application_name, state;
```

### PgBouncer (Optional)

For deployments exceeding 100 connections, use PgBouncer in transaction mode:

```ini
[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
min_pool_size = 5
reserve_pool_size = 5
```

---

## Index Optimization

### Index Types Used

| Type | Use Case | Example |
|------|----------|---------|
| B-tree | Default; equality and range queries | `merchants(slug)`, `transactions(created_at)` |
| GIN | JSONB containment queries | `(metadata)`, `(changes)` |
| BRIN | Large append-only tables with natural ordering | `audit_logs(created_at)` |

### Index Best Practices

1. **Lead with tenant isolation**: Every query-scoped table has `merchant_id` as the leading index column.

2. **Cover frequent queries**: Include columns used in `WHERE`, `ORDER BY`, and `SELECT` to enable index-only scans.

3. **Avoid over-indexing**: Each index adds write overhead. Review quarterly:
   ```sql
   -- Find unused indexes
   SELECT indexrelname, idx_scan
   FROM pg_stat_user_indexes
   WHERE idx_scan = 0
   ORDER BY pg_relation_size(indexrelid) DESC;
   ```

4. **Monitor index bloat**:
   ```sql
   SELECT indexname, pg_size_pretty(pg_relation_size(indexname::regclass))
   FROM pg_indexes
   WHERE tablename = 'transactions'
   ORDER BY pg_relation_size(indexname::regclass) DESC;
   ```

5. **Partial indexes** for common filtered queries:
   ```sql
   -- Only index active subscriptions (reduces index size)
   CREATE INDEX subscriptions_active_period_idx
       ON subscriptions (merchant_id, current_period_end)
       WHERE status = 'active';

   -- Only index pending tasks (reduces index size)
   CREATE INDEX background_tasks_pending_idx
       ON background_tasks (next_run_at, priority)
       WHERE status = 'pending';
   ```

### Index Size Estimates

| Table | Index | Estimated Size (1M rows) |
|-------|-------|--------------------------|
| transactions | merchant_id | ~20 MB |
| transactions | created_at | ~20 MB |
| transactions | provider_transaction_id (unique) | ~30 MB |
| audit_logs | merchant_id | ~20 MB |
| audit_logs | created_at | ~20 MB |
| subscriptions | current_period_end | ~15 MB |

---

## Query Plan Analysis

### EXPLAIN ANALYZE

Always use `EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)` for query tuning:

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT * FROM transactions
WHERE merchant_id = $1
  AND status = 'succeeded'
  AND created_at >= $2
  AND created_at < $3
ORDER BY created_at DESC
LIMIT 20;
```

### Common Anti-Patterns

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| `SELECT *` | Fetches unnecessary columns | Select only needed columns |
| `LIKE '%term%'` | Prevents index usage | Use full-text search or trigram index |
| `OFFSET` for deep pagination | Scans and discards rows | Use keyset pagination |
| N+1 queries | Multiple round trips | Use JOINs or batch loading |
| Unparameterized queries | Plan cache misses | Always use prepared statements |

### Keyset Pagination

```sql
-- Instead of OFFSET-based pagination:
-- BAD: Slow for large offsets
SELECT * FROM transactions
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT 20 OFFSET 10000;

-- GOOD: Constant-time regardless of page
SELECT * FROM transactions
WHERE merchant_id = $1
  AND created_at < $2  -- cursor from previous page
ORDER BY created_at DESC
LIMIT 20;
```

### Prepared Statement Caching

pgxpool automatically caches prepared statements. For manual caching:

```go
// pgx caches plans per connection
rows, err := pool.Query(ctx, `
    SELECT id, amount, status
    FROM transactions
    WHERE merchant_id = $1 AND status = $2
    ORDER BY created_at DESC
    LIMIT $3
`, merchantID, status, limit)
```

---

## Partitioning Benefits

### Transaction Table Partitioning

Monthly range partitioning on `created_at`:

| Benefit | Detail |
|---------|--------|
| Query performance | Partition pruning eliminates scanning old data |
| Maintenance | `VACUUM` and `ANALYZE` run per-partition |
| Archival | Old partitions detach without locking the main table |
| Index efficiency | Smaller indexes per partition fit in memory |

### Partition Pruning

PostgreSQL automatically prunes partitions based on `WHERE` clauses:

```sql
-- Only scans transactions_2026_07 (single partition)
SELECT * FROM transactions
WHERE created_at >= '2026-07-01' AND created_at < '2026-08-01';

-- Scans transactions_2026_06 and transactions_2026_07 (two partitions)
SELECT * FROM transactions
WHERE created_at >= '2026-06-15' AND created_at < '2026-07-15';
```

### Partition Management

```sql
-- Create next month's partition
CREATE TABLE transactions_2026_08
    PARTITION OF transactions
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

-- Detach old partition for archival
ALTER TABLE transactions DETACH PARTITION transactions_2025_01;

-- Drop detached partition (after backup)
DROP TABLE transactions_2025_01;
```

### pg_partman Automation

```sql
-- Install pg_partman
CREATE EXTENSION pg_partman;

-- Configure automatic partition creation
SELECT partman.create_parent(
    p_parent_table := 'public.transactions',
    p_control := 'created_at',
    p_type := 'native',
    p_interval := 'monthly',
    p_premake := 3  -- Create 3 months ahead
);

-- Schedule partition maintenance (run daily via pg_cron)
SELECT partman.run_maintenance();
```

### Partition Key Constraints

For foreign key compatibility with partitioned tables:

```sql
-- Defer FK checks to transaction end
SET CONSTRAINTS ALL DEFERRED;

-- Or use partition key in FK (PG 12+)
ALTER TABLE transactions
    ADD CONSTRAINT fk_merchant
    FOREIGN KEY (merchant_id) REFERENCES merchants(id)
    ON DELETE RESTRICT;
```

---

## Materialized Views for Analytics

### Revenue Summary View

```sql
CREATE MATERIALIZED VIEW mv_revenue_summary AS
SELECT
    merchant_id,
    DATE_TRUNC('day', created_at) as day,
    COUNT(*) as transaction_count,
    SUM(amount) as gross_volume,
    SUM(fee_amount) as total_fees,
    SUM(net_amount) as net_volume,
    COUNT(DISTINCT customer_id) as unique_customers
FROM transactions
WHERE type = 'charge' AND status = 'succeeded'
GROUP BY merchant_id, DATE_TRUNC('day', created_at);

CREATE UNIQUE INDEX mv_revenue_summary_idx
    ON mv_revenue_summary (merchant_id, day);
```

### Refresh Strategy

```sql
-- Refresh concurrently (non-blocking, requires unique index)
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_revenue_summary;

-- Schedule refresh every 5 minutes via pg_cron
SELECT cron.schedule(
    'refresh-revenue-summary',
    '*/5 * * * *',
    'REFRESH MATERIALIZED VIEW CONCURRENTLY mv_revenue_summary'
);
```

### Subscription Analytics View

```sql
CREATE MATERIALIZED VIEW mv_subscription_analytics AS
SELECT
    merchant_id,
    DATE_TRUNC('month', created_at) as month,
    status,
    COUNT(*) as count,
    SUM(amount) as total_amount,
    AVG(amount) as avg_amount
FROM subscriptions
GROUP BY merchant_id, DATE_TRUNC('month', created_at), status;

CREATE UNIQUE INDEX mv_subscription_analytics_idx
    ON mv_subscription_analytics (merchant_id, month, status);
```

### Materialized View Maintenance

| View | Refresh Interval | Trigger |
|------|-----------------|---------|
| `mv_revenue_summary` | Every 5 minutes | pg_cron |
| `mv_subscription_analytics` | Every 15 minutes | pg_cron |
| `mv_customer_lifetime_value` | Hourly | pg_cron |

### When to Use Materialized Views

- **Use**: Dashboard aggregations, reporting queries, data that tolerates slight staleness
- **Don't use**: Real-time data, frequently changing data, small tables (just query directly)

---

## Redis Caching Strategy

### Cache Layers

| Layer | TTL | Purpose |
|-------|-----|---------|
| L1: Application memory | 30s | Hot data (current user, active merchant) |
| L2: Redis | 5min | Frequent lookups (merchant config, provider status) |
| L3: Materialized views | 5-15min | Aggregated analytics |

### Merchant Config Cache

```go
func GetMerchantConfig(ctx context.Context, rdb *redis.Client, merchantID string) (*MerchantConfig, error) {
    cacheKey := fmt.Sprintf("merchant:%s:config", merchantID)

    // Try cache first
    cached, err := rdb.Get(ctx, cacheKey).Bytes()
    if err == nil {
        var config MerchantConfig
        json.Unmarshal(cached, &config)
        return &config, nil
    }

    // Cache miss: query database
    config, err := queryMerchantConfig(ctx, merchantID)
    if err != nil {
        return nil, err
    }

    // Populate cache
    data, _ := json.Marshal(config)
    rdb.Set(ctx, cacheKey, data, 5*time.Minute)

    return config, nil
}
```

### Cache Invalidation

```go
// Invalidate on update
func UpdateMerchant(ctx context.Context, rdb *redis.Client, db *pgxpool.Pool, id string, update MerchantUpdate) error {
    // Update database
    _, err := db.Exec(ctx, `UPDATE merchants SET name = $1, updated_at = NOW() WHERE id = $2`, update.Name, id)
    if err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("merchant:%s:config", id)
    rdb.Del(ctx, cacheKey)

    return nil
}
```

### Cache Patterns

| Pattern | Use Case | Implementation |
|---------|----------|----------------|
| Cache-aside | Merchant config, provider status | Read-through with TTL |
| Write-through | Exchange rates | Write to cache and DB simultaneously |
| Write-behind | Audit logs | Write to cache, async flush to DB |
| Refresh-ahead | Revenue summaries | Pre-refresh before TTL expiry |

### Cache Key Naming

```
merchant:{id}:config          # Merchant configuration
merchant:{id}:providers       # Active provider configs
transaction:{id}:status       # Transaction status (short TTL)
rate:{base}:{quote}           # Exchange rate
task:claim:{worker_id}        # Worker task assignment
```

### Redis Memory Management

```yaml
# redis.conf
maxmemory 512mb
maxmemory-policy allkeys-lru
```

### Cache Hit Rate Monitoring

```bash
# Redis CLI
redis-cli info stats | grep keyspace_hits
redis-cli info stats | grep keyspace_misses

# Hit rate = hits / (hits + misses)
```

Target: > 90% hit rate for merchant config, > 80% for transaction status.

---

## Query Performance Checklist

- [ ] All queries use `merchant_id` for tenant isolation (partition pruning)
- [ ] Indexes exist for all `WHERE` clause columns
- [ ] `EXPLAIN ANALYZE` confirms index usage (no sequential scans on large tables)
- [ ] Pagination uses keyset for deep pages
- [ ] Prepared statements are used for repeated queries
- [ ] Connection pool size is appropriate for workload
- [ ] Materialized views are refreshed at acceptable intervals
- [ ] Redis cache hit rates meet targets
- [ ] No `SELECT *` in production queries
- [ ] Batch operations use `BULK INSERT` or `UNION ALL` instead of loops
- [ ] Partition pruning is verified for time-range queries
- [ ] Vacuum and analyze are scheduled for all tables

---

## Monitoring Queries

### Slow Query Detection

```sql
-- Enable slow query logging (postgresql.conf)
-- log_min_duration_statement = 1000  -- 1 second

-- Find slow queries from pg_stat_statements
SELECT
    query,
    calls,
    mean_exec_time,
    total_exec_time,
    rows
FROM pg_stat_statements
WHERE mean_exec_time > 1000
ORDER BY mean_exec_time DESC
LIMIT 20;
```

### Table Bloat

```sql
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size(tablename::regclass)) as total_size,
    pg_size_pretty(pg_relation_size(tablename::regclass)) as table_size,
    pg_size_pretty(pg_indexes_size(tablename::regclass::oid)) as index_size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(tablename::regclass) DESC;
```

### Index Usage

```sql
SELECT
    indexrelname as index_name,
    idx_scan as times_used,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;
```
