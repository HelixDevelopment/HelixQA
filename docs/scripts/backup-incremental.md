---
title: backup-incremental
---

# backup-incremental.sh

WAL (Write-Ahead Log) incremental backup. Forces a WAL switch via
`pg_switch_wal()` and copies archived WAL segments to `$BACKUP_DIR`
for point-in-time recovery between full backups.

Usage:

```bash
./scripts/backup-incremental.sh
```
