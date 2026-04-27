#!/bin/bash
# install-host-suspend-guard.sh
#
# MANUAL PREREQUISITE — run ONCE per host, with sudo, BEFORE running
# any project test / challenge / build that boots containers or
# spawns long-running CLI agents.
#
# Background (CONST-032 / CONST-033): on 2026-04-26 18:23:43 the host
# suspended mid-session, killing HelixAgent + 41 services + the user's
# SSH session. journalctl showed:
#   systemd-logind[1183]: The system will suspend now!
# Root cause: the GDM greeter session at the local console has its own
# power policy; SSH sessions don't count as activity. User-level
# `sleep-inactive-ac-type=nothing` is necessary but not sufficient.
#
# This script applies defence in depth so neither the greeter, nor any
# DE, nor any user with logind privileges, can suspend the host while
# it's running mission-critical workloads.
#
# Verification (re-run the challenge after this script):
#   bash challenges/scripts/host_no_auto_suspend_challenge.sh
# All 4 assertions must PASS.

set -euo pipefail

if [[ "$EUID" -ne 0 ]]; then
    echo "ERROR: must be run as root (sudo)." >&2
    exit 1
fi

echo "[1/3] Masking sleep / suspend / hibernate / hybrid-sleep targets..."
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target

echo "[2/3] Setting AllowSuspend=no in /etc/systemd/sleep.conf.d/..."
mkdir -p /etc/systemd/sleep.conf.d
cat > /etc/systemd/sleep.conf.d/00-no-suspend.conf <<'EOF'
# CONST-033: host runs mission-critical parallel CLI-agent + container
# workloads; auto-suspend is unsafe. Defence in depth — see also the
# masked targets above and the logind drop-in below.
[Sleep]
AllowSuspend=no
AllowHibernation=no
AllowSuspendThenHibernate=no
AllowHybridSleep=no
EOF

echo "[3/3] Setting logind IdleAction=ignore + HandleLidSwitch=ignore..."
mkdir -p /etc/systemd/logind.conf.d
cat > /etc/systemd/logind.conf.d/00-no-idle-suspend.conf <<'EOF'
# CONST-033: do not suspend the host on idle (SSH sessions don't count
# as activity; the GDM greeter's idle policy was the historical
# trigger).
[Login]
IdleAction=ignore
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
EOF

echo "Reloading systemd..."
systemctl daemon-reload
systemctl reload-or-restart systemd-logind || true

echo
echo "DONE. Verify with:"
echo "  bash challenges/scripts/host_no_auto_suspend_challenge.sh"
echo "All 4 assertions must PASS."
