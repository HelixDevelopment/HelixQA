#!/usr/bin/env bash
set -euo pipefail

# Helix Seller - Backup Schedule Setup
# Creates systemd service and timer units for automated backups

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="${SERVICE_DIR:-/etc/systemd/system}"

echo "Installing backup schedule..."

# Full backup service
cat > "${SERVICE_DIR}/helix-backup-full.service" << 'EOF'
[Unit]
Description=Helix Seller Full Database Backup
After=postgresql.service

[Service]
Type=oneshot
ExecStart=/opt/helix-seller/scripts/backup-full.sh
User=helix
Environment=DATABASE_URL=postgresql://helix:helix@localhost:5432/helix_seller
EOF

# Full backup timer (daily at 2 AM)
cat > "${SERVICE_DIR}/helix-backup-full.timer" << 'EOF'
[Unit]
Description=Helix Seller Full Backup Timer

[Timer]
OnCalendar=daily 02:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

# Incremental backup service
cat > "${SERVICE_DIR}/helix-backup-incremental.service" << 'EOF'
[Unit]
Description=Helix Seller WAL Incremental Backup
After=postgresql.service

[Service]
Type=oneshot
ExecStart=/opt/helix-seller/scripts/backup-incremental.sh
User=helix
Environment=DATABASE_URL=postgresql://helix:helix@localhost:5432/helix_seller
EOF

# Incremental backup timer (hourly)
cat > "${SERVICE_DIR}/helix-backup-incremental.timer" << 'EOF'
[Unit]
Description=Helix Seller Incremental Backup Timer

[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now helix-backup-full.timer
systemctl enable --now helix-backup-incremental.timer

echo "Backup schedule installed:"
echo "  - Full backup: daily at 2 AM"
echo "  - Incremental: hourly"
