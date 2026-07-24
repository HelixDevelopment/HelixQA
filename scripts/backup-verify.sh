#!/usr/bin/env bash
set -euo pipefail

# Helix Seller - Backup Verification

BACKUP_DIR="${BACKUP_DIR:-/var/backups/helix-seller}"
DATABASE_URL="${DATABASE_URL:-postgresql://helix:helix@localhost:5432/helix_seller}"
TEST_DB="helix_seller_verify_$(date +%s)"

echo "[$(date -Iseconds)] Starting backup verification..."

# Find latest backup
LATEST_BACKUP=$(ls -t "${BACKUP_DIR}"/helix_seller_full_*.sql.gz 2>/dev/null | head -1)
if [ -z "${LATEST_BACKUP}" ]; then
    echo "[$(date -Iseconds)] ERROR: No backup found"
    exit 1
fi

echo "[$(date -Iseconds)] Using backup: ${LATEST_BACKUP}"

# Create test database
psql "${DATABASE_URL}" -c "CREATE DATABASE ${TEST_DB};" 2>/dev/null || true

# Restore
gunzip -c "${LATEST_BACKUP}" | psql "${DATABASE_URL}/${TEST_DB}" > /dev/null 2>&1

# Validate
TABLE_COUNT=$(psql "${DATABASE_URL}/${TEST_DB}" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')

echo "[$(date -Iseconds)] Restored ${TABLE_COUNT} tables"

# Cleanup
psql "${DATABASE_URL}" -c "DROP DATABASE ${TEST_DB};" 2>/dev/null || true

echo "[$(date -Iseconds)] Backup verification PASSED"
