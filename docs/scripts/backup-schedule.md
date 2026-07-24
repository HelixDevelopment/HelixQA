---
title: backup-schedule
---

# backup-schedule.sh

Installs systemd service and timer units for automated full and
incremental backups. Full backup runs daily at 02:00; incremental
backup runs hourly. Units are installed under `$SERVICE_DIR`
(default: `/etc/systemd/system`).

Usage:

```bash
sudo ./scripts/backup-schedule.sh
```
