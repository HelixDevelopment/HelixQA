---
title: backup-full
---

# backup-full.sh

Full PostgreSQL database backup using `pg_dump`. Output is gzip-compressed
and written to `$BACKUP_DIR` (default: `/var/backups/helix-seller`).
Old backups are pruned after `$RETENTION_DAYS` (default: 30).

Usage:

```bash
BACKUP_DIR=/custom/path ./scripts/backup-full.sh
```
