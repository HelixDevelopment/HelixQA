#!/usr/bin/env bash
set -euo pipefail

# Helix Seller - Full Database Backup
# Constitution §8.3: RPO ≈ 1h, RTO ≈ 4h

BACKUP_DIR="${BACKUP_DIR:-/var/backups/helix-seller}"
DATABASE_URL="${DATABASE_URL:-postgresql://helix:helix@localhost:5432/helix_seller}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/helix_seller_full_${TIMESTAMP}.sql.gz"
RETENTION_DAYS="${RETENTION_DAYS:-30}"

mkdir -p "${BACKUP_DIR}"

echo "[$(date -Iseconds)] Starting full backup..."

pg_dump "${DATABASE_URL}" | gzip > "${BACKUP_FILE}"

echo "[$(date -Iseconds)] Backup created: ${BACKUP_FILE}"
echo "[$(date -Iseconds)] Size: $(du -h "${BACKUP_FILE}" | cut -f1)"

# Cleanup old backups
find "${BACKUP_DIR}" -name "helix_seller_full_*.sql.gz" -mtime +${RETENTION_DAYS} -delete
echo "[$(date -Iseconds)] Old backups cleaned (retention: ${RETENTION_DAYS} days)"
