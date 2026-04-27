#!/bin/bash
# user_session_no_suspend_bootstrap.sh
#
# Per-user defensive layer — runs without sudo. Protects ONLY the
# current GUI/CLI session of the invoking user. The GDM greeter and
# system-wide policies are NOT covered (use install-host-suspend-guard.sh
# for those, which requires sudo).
#
# Idempotent. Safe to source from a project's start.sh / setup.sh /
# bootstrap.sh.

set -uo pipefail

# 1. GNOME power-manager (only if gsettings is present)
if command -v gsettings >/dev/null 2>&1; then
  for key in sleep-inactive-ac-type sleep-inactive-battery-type; do
    cur=$(gsettings get org.gnome.settings-daemon.plugins.power "$key" 2>/dev/null || echo "")
    if [[ "$cur" != "'nothing'" ]] && [[ -n "$cur" ]]; then
      gsettings set org.gnome.settings-daemon.plugins.power "$key" "nothing" 2>/dev/null \
        && echo "[bootstrap] gsettings $key -> nothing"
    fi
  done
  for key in idle-delay sleep-inactive-ac-timeout sleep-inactive-battery-timeout; do
    gsettings set org.gnome.settings-daemon.plugins.power "$key" 0 2>/dev/null || true
    gsettings set org.gnome.desktop.session "$key" 0 2>/dev/null || true
  done
fi

# 2. xset DPMS off (X11 only — no-op on Wayland)
if command -v xset >/dev/null 2>&1 && [[ -n "${DISPLAY:-}" ]]; then
  xset -dpms 2>/dev/null || true
  xset s off 2>/dev/null || true
fi

# 3. systemd --user inhibitor for long-running session (optional —
#    only runs if HOST_POWER_MANAGEMENT_SESSION_INHIBIT=1 is set, to
#    avoid leaking inhibitors on every shell init).
if [[ "${HOST_POWER_MANAGEMENT_SESSION_INHIBIT:-0}" == "1" ]] \
   && command -v systemd-inhibit >/dev/null 2>&1; then
  pgrep -f "systemd-inhibit.*host-power-management-session-guard" >/dev/null 2>&1 || \
    nohup systemd-inhibit \
      --who="host-power-management-session-guard" \
      --why="parallel CLI agents + containers must not be interrupted by suspend" \
      --what=sleep:idle:handle-lid-switch \
      --mode=block \
      sleep infinity \
      >/dev/null 2>&1 &
  echo "[bootstrap] systemd-inhibit launched in background"
fi

echo "[bootstrap] user-scope no-suspend defences applied"
