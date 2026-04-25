#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# Smoke test for the challenges dashboard.
# Greps index.html for required markers and exits non-zero on any miss.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HTML="$SCRIPT_DIR/index.html"
JS="$SCRIPT_DIR/index.js"

fail=0

check_html() {
  local marker="$1"
  if grep -qF "$marker" "$HTML"; then
    echo "  OK  [html] $marker"
  else
    echo "  FAIL [html] missing: $marker"
    fail=1
  fi
}

check_js() {
  local marker="$1"
  if grep -qF "$marker" "$JS"; then
    echo "  OK  [js]   $marker"
  else
    echo "  FAIL [js]  missing: $marker"
    fail=1
  fi
}

echo "=== challenges-dashboard smoke test ==="

check_html 'id="pass-rate-trend"'
check_html 'id="pass-rate-per-bank"'
check_html '<table'
check_html 'chart.js'
check_html 'id="adv-table-body"'
check_html 'SPDX-License-Identifier: Apache-2.0'

check_js 'extractCounts'
check_js 'extractBanks'
check_js 'extractAdversarial'
check_js 'renderTrendChart'
check_js 'renderBankChart'
check_js 'renderAdvTable'
check_js 'SPDX-License-Identifier: Apache-2.0'

if [ "$fail" -eq 0 ]; then
  echo "=== PASSED ==="
  exit 0
else
  echo "=== FAILED ==="
  exit 1
fi
