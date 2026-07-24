# Migration Strategy

## Overview

All schema changes are managed through versioned SQL migration files. Migrations are applied in order, tracked in a `schema_migrations` table, and designed for zero-downtime deployments using the expand-contract pattern.

## Migration Runner

A custom migration runner (Go) applies migrations at application startup or via CLI:

```bash
# Apply pending migrations
helix migrate up

# Rollback the last migration
helix migrate down

# Check current version
helix migrate status

# Rollback to a specific version
helix migrate down --to 20260723_001
```

The runner:
- Connects to the database using the configured DSN
- Reads the `schema_migrations` table to determine applied versions
- Applies pending migrations in order within a transaction
- Records applied migrations with a timestamp
- Fails fast on any error (transactional rollback)

## Schema Migrations Table

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     VARCHAR(15) PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## File Naming Convention

```
migrations/
  20260723_001_create_merchants.sql
  20260723_002_create_customers.sql
  20260723_003_create_payment_methods.sql
  20260723_004_create_transactions.sql
  ...
```

Format: `YYYYMMDD_NNN_description.sql`

- `YYYYMMDD`: Date of creation
- `NNN`: Sequence number within the day (001-999)
- `description`: Snake-case description of the change

## Migration File Structure

Each migration file is a single SQL transaction:

```sql
-- Migration: 20260723_001_create_merchants
-- Description: Create merchants table with core fields

BEGIN;

CREATE TYPE merchant_status AS ENUM ('active', 'suspended', 'pending_verification');

CREATE TABLE merchants (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL,
    email             VARCHAR(255) UNIQUE NOT NULL,
    slug              VARCHAR(100) UNIQUE NOT NULL,
    status            merchant_status NOT NULL DEFAULT 'pending_verification',
    default_currency  CHAR(3) NOT NULL DEFAULT 'USD',
    timezone          VARCHAR(50) NOT NULL DEFAULT 'UTC',
    branding          JSONB DEFAULT '{}',
    settings          JSONB DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ NULL
);

CREATE INDEX merchants_slug_idx ON merchants (slug);
CREATE INDEX merchants_email_idx ON merchants (email);
CREATE INDEX merchants_status_idx ON merchants (status);

COMMIT;
```

## Expand-Contract Pattern

Zero-downtime migrations follow a three-phase approach:

### Phase 1: Expand (Additive)

Add new columns, tables, or indexes without modifying existing structures.

```sql
-- Add new column with a default (safe in PG 11+)
ALTER TABLE transactions ADD COLUMN net_amount BIGINT NULL;

-- Create new table
CREATE TABLE background_tasks (...);

-- Add new index concurrently
CREATE INDEX CONCURRENTLY transactions_net_amount_idx ON transactions (net_amount);
```

### Phase 2: Migrate (Dual-Write)

Update application code to write to both old and new structures. Backfill existing data.

```sql
-- Backfill net_amount for completed transactions
UPDATE transactions
SET net_amount = amount - fee_amount
WHERE status = 'succeeded' AND net_amount IS NULL;
```

### Phase 3: Contract (Remove)

Once all reads use the new structure, remove the old one.

```sql
-- Drop old column (only after confirming no reads depend on it)
ALTER TABLE transactions DROP COLUMN old_field;
```

## Index Operations

Indexes are created `CONCURRENTLY` to avoid locking:

```sql
-- Safe: does not block reads or writes
CREATE INDEX CONCURRENTLY merchants_status_idx ON merchants (status);

-- Drop is also concurrent
DROP INDEX CONCURRENTLY merchants_status_idx;
```

## Enum Modifications

PostgreSQL does not support `ALTER TYPE ... DROP VALUE`. For enum changes:

### Adding Values

```sql
-- Safe: additive only
ALTER TYPE transaction_status ADD VALUE IF NOT EXISTS 'reversed';
```

### Removing Values

Requires a multi-step process:

1. Add new column with the new type
2. Backfill data, converting removed enum values
3. Drop old column
4. Drop old enum type
5. Rename new column and type

```sql
-- Step 1: Create new enum type
CREATE TYPE transaction_status_v2 AS ENUM ('pending', 'processing', 'succeeded', 'failed', 'cancelled');

-- Step 2: Add new column
ALTER TABLE transactions ADD COLUMN status_v2 transaction_status_v2;

-- Step 3: Backfill (convert 'reversed' to 'failed')
UPDATE transactions SET status_v2 = status::text::transaction_status_v2;

-- Step 4: Swap columns
ALTER TABLE transactions DROP COLUMN status;
ALTER TABLE transactions RENAME COLUMN status_v2 TO status;
ALTER TABLE transactions ALTER COLUMN status SET NOT NULL;

-- Step 5: Drop old type
DROP TYPE transaction_status;
ALTER TYPE transaction_status_v2 RENAME TO transaction_status;
```

## Partition Management

New monthly partitions are created via migration or scheduled job:

```sql
-- Create partition for next month
CREATE TABLE transactions_2026_08
    PARTITION OF transactions
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

-- Detach old partition (for archival)
ALTER TABLE transactions DETACH PARTITION transactions_2025_01;
```

Automated partition management uses `pg_partman`:

```sql
-- Configure partition creation
SELECT partman.create_parent(
    p_parent_table := 'public.transactions',
    p_control := 'created_at',
    p_type := 'native',
    p_interval := 'monthly'
);
```

## Rollback Procedures

### Automatic Rollback

Each migration runs in a transaction. If any statement fails, the entire migration is rolled back.

### Manual Rollback

Rollback files are optional but recommended for complex migrations:

```
migrations/
  20260723_001_create_merchants.sql
  20260723_001_create_merchants_rollback.sql
```

The runner supports:

```bash
# Rollback using the rollback file
helix migrate down --version 20260723_001

# Rollback all migrations
helix migrate reset
```

### Rollback File Example

```sql
-- Rollback: 20260723_001_create_merchants
BEGIN;

DROP TABLE IF EXISTS merchants;
DROP TYPE IF EXISTS merchant_status;

COMMIT;
```

## Data Migration Patterns

### Column Backfill

```sql
-- Add column with default
ALTER TABLE merchants ADD COLUMN display_name VARCHAR(255) NULL;

-- Backfill from existing column
UPDATE merchants SET display_name = name WHERE display_name IS NULL;

-- Set NOT NULL after backfill
ALTER TABLE merchants ALTER COLUMN display_name SET NOT NULL;
```

### JSONB Migration

```sql
-- Move flat columns to JSONB metadata
ALTER TABLE customers ADD COLUMN metadata_v2 JSONB DEFAULT '{}';

UPDATE customers
SET metadata_v2 = jsonb_build_object(
    'legacy_phone', phone,
    'legacy_notes', notes
)
WHERE metadata_v2 = '{}';
```

### Large Table Migrations

For tables with millions of rows, batch updates to avoid long-running transactions:

```sql
-- Batch update in chunks of 10000
UPDATE transactions
SET net_amount = amount - fee_amount
WHERE id IN (
    SELECT id FROM transactions
    WHERE status = 'succeeded' AND net_amount IS NULL
    LIMIT 10000
);
```

Repeat until zero rows are affected.

## Validation

After migration, run validation queries:

```sql
-- Verify no NULL values in NOT NULL columns
SELECT COUNT(*) FROM merchants WHERE name IS NULL;

-- Verify foreign key integrity
SELECT COUNT(*) FROM customers c
LEFT JOIN merchants m ON c.merchant_id = m.id
WHERE m.id IS NULL;

-- Verify index existence
SELECT indexname FROM pg_indexes
WHERE tablename = 'merchants';
```

## Testing Migrations

1. Apply migration to a test database
2. Verify schema matches expected state
3. Insert test data and verify constraints
4. Run application test suite against the migrated schema
5. Rollback and verify clean rollback

```bash
# Test migration cycle
createdb helix_test
helix migrate up --database helix_test
go test ./...
helix migrate reset --database helix_test
```
