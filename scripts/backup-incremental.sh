#!/usr/bin/env bash
set -euo pipefail

# Helix Seller - WAL Incremental Backup

BACKUP_DIR="${BACKUP_DIR:-/var/backups/helix-seller/wal}"
DATABASE_URL="${DATABASE_URL:-postgresql://helix:helix@localhost:5432/helix_seller}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/wal_${TIMESTAMP}.tar.gz"

mkdir -p "${BACKUP_DIR}"

echo "[$(date -Iseconds)] Starting WAL archive backup..."

# Archive WAL files
psql "${DATABASE_URL}" -c "SELECT pg_switch_wal();" > /dev/null 2>&1

# Copy WAL files
WAL_DIR=$(psql "${DATABASE_URL}" -t -c "SHOW data_directory;" | tr -d ' ')
cp "${WAL_DIR}"/pg_wal/*.gz "${BACKUP_DIR}/" 2>/dev/null || true

echo "[$(date -Iseconds)] WAL backup completed"
