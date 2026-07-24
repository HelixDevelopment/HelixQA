# Restore Runbook

Disaster recovery procedures for the Helix Seller platform.

## Recovery Objectives

| Metric | Target |
|--------|--------|
| RPO (Recovery Point Objective) | ~1 hour |
| RTO (Recovery Time Objective) | ~4 hours |

---

## Prerequisites

### Access Required

- SSH access to production server(s)
- PostgreSQL superuser or `helix` role credentials
- Access to backup storage location (S3 / NAS / local)
- DNS provider console access (for failover)
- Application environment file (`.env`)

### Tools

```bash
# Verify tools are installed
pg_dump --version       # PostgreSQL 16+
psql --version          # PostgreSQL client
redis-cli --version     # Redis 7+
nats-server --version   # NATS (if JetStream persistence used)
curl                    # Health check
make                    # Build toolchain
```

### Backup Location

Backups are stored at:

```
/backups/helix_seller/
  ├── daily/
  │   ├── helix_seller_YYYYMMDD_HHMMSS.dump
  │   └── helix_seller_YYYYMMDD_HHMMSS.sql.gz
  ├── wal/
  │   └── helix_seller_wal_YYYYMMDD_HHMMSS/
  └── redis/
      └── dump_YYYYMMDD_HHMMSS.rdb
```

S3 backup path: `s3://helix-seller-backups/`

---

## Full Database Restore

Use when the database is corrupted or lost entirely.

### Step 1: Stop the Application

```bash
# Stop the helix-seller service
sudo systemctl stop helix-seller.service

# Verify it's stopped
sudo systemctl status helix-seller.service
```

### Step 2: Terminate Active Connections

```bash
psql -U helix -d postgres -c "
  SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
  WHERE datname = 'helix_seller' AND pid <> pg_backend_pid();
"
```

### Step 3: Drop and Recreate the Database

```bash
psql -U helix -d postgres -c "DROP DATABASE IF EXISTS helix_seller;"
psql -U helix -d postgres -c "CREATE DATABASE helix_seller OWNER helix;"
```

### Step 4: Restore from Backup

```bash
# For .dump format (custom, compressed)
pg_restore -U helix -d helix_seller -Fc --verbose \
  /backups/helix_seller/daily/helix_seller_YYYYMMDD_HHMMSS.dump

# For .sql.gz format
gunzip -c /backups/helix_seller/daily/helix_seller_YYYYMMDD_HHMMSS.sql.gz \
  | psql -U helix -d helix_seller -v ON_ERROR_STOP=1
```

### Step 5: Apply WAL Replay (if available)

```bash
# If WAL archives exist beyond the base backup
pg_wal_replay -D /var/lib/postgresql/data/pg_wal/
```

### Step 6: Run Migrations

```bash
make migrate-up
```

### Step 7: Verify Table Counts

```bash
psql -U helix -d helix_seller -c "
  SELECT schemaname, relname, n_live_tup
  FROM pg_stat_user_tables
  ORDER BY n_live_tup DESC;
"
```

---

## Incremental Backup Restore

Use when only recent data loss has occurred (within the last ~1 hour).

### Step 1: Identify the Recovery Target

```bash
# Find the last known good backup
ls -lt /backups/helix_seller/daily/

# Check WAL archive timestamp
ls -lt /backups/helix_seller/wal/
```

### Step 2: Restore Base Backup

Follow Steps 1-5 from Full Database Restore above using the most recent base backup.

### Step 3: Configure Recovery

```bash
# Create recovery configuration
cat > /var/lib/postgresql/data/postgresql.conf << EOF
restore_command = 'cp /backups/helix_seller/wal/%f %p'
recovery_target_time = 'YYYY-MM-DD HH:MM:SS+00'
recovery_target_action = 'promote'
EOF

# Create recovery signal file
touch /var/lib/postgresql/data/recovery.signal
```

### Step 4: Restart PostgreSQL

```bash
sudo systemctl restart postgresql
```

### Step 5: Verify Recovery

```bash
psql -U helix -d helix_seller -c "SELECT pg_is_in_recovery();"
# Should return: f

psql -U helix -d helix_seller -c "
  SELECT max(created_at) FROM orders;
"
```

---

## Data Integrity Verification

Run these checks after any restore.

### Row Count Verification

```bash
psql -U helix -d helix_seller -c "
  SELECT 'users' as tbl, count(*) FROM users
  UNION ALL SELECT 'products', count(*) FROM products
  UNION ALL SELECT 'orders', count(*) FROM orders
  UNION ALL SELECT 'order_items', count(*) FROM order_items
  UNION ALL SELECT 'payments', count(*) FROM payments
  UNION ALL SELECT 'sessions', count(*) FROM sessions
  UNION ALL SELECT 'webhook_events', count(*) FROM webhook_events;
"
```

### Foreign Key Integrity

```bash
psql -U helix -d helix_seller -c "
  SELECT conname, conrelid::regclass, confrelid::regclass
  FROM pg_constraint
  WHERE contype = 'f';
"
```

### Sequence Continuity

```bash
psql -U helix -d helix_seller -c "
  SELECT sequencename, last_value
  FROM pg_sequences
  WHERE schemaname = 'public';
"
```

---

## Application Restart

```bash
# Rebuild if needed
make build

# Start the application
sudo systemctl start helix-seller.service

# Verify health
curl -s http://localhost:8080/health | jq .
# Expected: {"status":"healthy","time":"..."}

# Watch logs for errors
sudo journalctl -u helix-seller.service -f --lines=50
```

---

## DNS Failover

Use when the primary server is unreachable.

### Step 1: Update DNS Record

```bash
# If using Cloudflare API
curl -X PATCH "https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records/{record_id}" \
  -H "Authorization: Bearer {cf_api_token}" \
  -H "Content-Type: application/json" \
  --data '{"content": "BACKUP_SERVER_IP"}'

# If using AWS Route53
aws route53 change-resource-record-sets \
  --hosted-zone-id {zone_id} \
  --change-batch '{
    "Changes": [{
      "Action": "UPSERT",
      "ResourceRecordSet": {
        "Name": "api.helixseller.com",
        "Type": "A",
        "TTL": 60,
        "ResourceRecords": [{"Value": "BACKUP_SERVER_IP"}]
      }
    }]
  }'
```

### Step 2: Verify DNS Propagation

```bash
dig +short api.helixseller.com
curl -I https://api.helixseller.com/health
```

### Step 3: Monitor

Watch for 15-30 minutes to confirm traffic is routing correctly.

---

## Communication Checklist

| Who | When | Channel |
|-----|------|---------|
| Engineering team | Immediately | Slack #incidents |
| Operations lead | Immediately | Phone call |
| Customer support | Within 15 min | Slack #cs-alerts |
| Management | Within 30 min | Email |
| Customers (if outage > 30 min) | Within 30 min | Status page / email |

### Status Page Update Template

```
[Investigating] We are aware of issues affecting the platform.
Our team is actively working on resolution.
Next update in 30 minutes.

[Identified] The issue has been identified as [ROOT CAUSE].
We are implementing a fix. ETA: [TIME].

[Monitoring] A fix has been deployed. We are monitoring for stability.

[Resolved] The issue has been resolved. Total downtime: [DURATION].
A post-incident review will follow.
```

---

## Rollback Procedure

If the restore introduces issues:

### Step 1: Stop Application

```bash
sudo systemctl stop helix-seller.service
```

### Step 2: Restore Previous Database State

```bash
# If the restore was the problem, restore from the backup taken BEFORE the failed restore
psql -U helix -d postgres -c "DROP DATABASE IF EXISTS helix_seller;"
psql -U helix -d postgres -c "CREATE DATABASE helix_seller OWNER helix;"

pg_restore -U helix -d helix_seller -Fc --verbose \
  /backups/helix_seller/daily/helix_seller_PREVIOUS_BACKUP.dump
```

### Step 3: Restart

```bash
sudo systemctl start helix-seller.service
curl -s http://localhost:8080/health | jq .
```

---

## Post-Restore Validation

Complete these checks before declaring recovery successful.

| # | Check | Command | Expected |
|---|-------|---------|----------|
| 1 | Health endpoint | `curl localhost:8080/health` | `{"status":"healthy"}` |
| 2 | Database connectivity | `psql -U helix -d helix_seller -c "SELECT 1"` | Returns `1` |
| 3 | Recent orders accessible | API test: `GET /api/v1/orders?limit=5` | Returns data |
| 4 | Payment provider status | Check Stripe/PayPal dashboards | No stuck charges |
| 5 | Background workers running | `ps aux \| grep helix-seller` | Worker processes active |
| 6 | Redis connected | `redis-cli ping` | `PONG` |
| 7 | NATS connected | `nats-server --help` or check logs | Connected |
| 8 | Logs clean | `journalctl -u helix-seller --since "5 min ago"` | No errors |
| 9 | Response times normal | Check Grafana dashboards | <200ms p99 |

---

## Appendix: Backup File Naming Convention

```
helix_seller_{TYPE}_{YYYYMMDD}_{HHMMSS}.{EXT}
```

| Component | Description |
|-----------|-------------|
| `helix_seller` | Database name prefix |
| `TYPE` | `full`, `wal`, `redis`, `incremental` |
| `YYYYMMDD` | Date (e.g., `20260723`) |
| `HHMMSS` | Time in UTC (e.g., `143022`) |
| `EXT` | `dump`, `sql.gz`, `rdb`, `tar.gz` |

Examples:

```
helix_seller_full_20260723_020000.dump
helix_seller_wal_20260723_030000.tar.gz
helix_seller_redis_20260723_020000.rdb
```

---

## Appendix: Common Failure Scenarios and Fixes

### Scenario: `pg_restore: error: connection to server failed`

**Cause:** PostgreSQL not running or wrong port.

```bash
sudo systemctl status postgresql
# If not running:
sudo systemctl start postgresql

# Check port
grep port /etc/postgresql/16/main/postgresql.conf
```

### Scenario: `pg_restore: error: could not open input file`

**Cause:** Wrong path or file doesn't exist.

```bash
# List available backups
ls -la /backups/helix_seller/daily/
# Verify file permissions
stat /backups/helix_seller/daily/helix_seller_*.dump
```

### Scenario: `FATAL: password authentication failed`

**Cause:** Wrong password or pg_hba.conf misconfigured.

```bash
# Check pg_hba.conf
cat /etc/postgresql/16/main/pg_hba.conf | grep helix
# Should have: local all helix md5

# Reset password if needed
psql -U postgres -c "ALTER USER helix PASSWORD 'helix';"
```

### Scenario: `ERROR: could not create directory`

**Cause:** Disk full.

```bash
df -h /var/lib/postgresql
# Free space:
sudo journalctl --vacuum-size=100M
# Or move old backups off disk
```

### Scenario: `ERROR: database "helix_seller" already exists`

**Cause:** Partial previous restore attempt.

```bash
# Force drop
psql -U helix -d postgres -c "
  SELECT pg_terminate_backend(pid)
  FROM pg_stat_activity
  WHERE datname = 'helix_seller';
"
psql -U helix -d postgres -c "DROP DATABASE helix_seller;"
# Then retry restore
```

### Scenario: Application won't start after restore

**Cause:** Missing environment file or wrong config.

```bash
# Verify .env exists
cat /path/to/helix-seller/.env | grep DATABASE_URL
# Should be: DATABASE_URL=postgresql://helix:helix@localhost:5432/helix_seller

# Check for missing migrations
make migrate-up
```

### Scenario: Redis data inconsistency

**Cause:** Redis wasn't backed up at same time as PostgreSQL.

```bash
# Flush stale Redis cache (sessions will be invalidated)
redis-cli FLUSHALL

# Restart application to rebuild cache
sudo systemctl restart helix-seller.service
```
