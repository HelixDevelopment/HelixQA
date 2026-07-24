---
title: backup-verify
---

# backup-verify.sh

Validates the most recent full backup by restoring it into a temporary
database and checking that tables exist. The test database is dropped
after verification. Exits non-zero if no backup is found or restore
fails.

Usage:

```bash
./scripts/backup-verify.sh
```
