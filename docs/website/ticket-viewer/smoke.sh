#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# Smoke test for the ticket viewer static page.

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

echo "=== ticket-viewer smoke test ==="

check_html 'id="ticket-body"'
check_html 'marked'
check_html '?ticket='
check_html 'id="url-input"'
check_html 'id="file-input"'
check_html 'SPDX-License-Identifier: Apache-2.0'

check_js 'renderMarkdown'
check_js 'enhanceEvidence'
check_js 'buildImageWidget'
check_js 'buildVideoWidget'
check_js 'buildJSONWidget'
check_js 'buildTextCollapsible'
check_js 'buildReplayWidget'
check_js 'wireReplayBlocks'
check_js 'makeCopyButton'
check_js 'SPDX-License-Identifier: Apache-2.0'

if [ "$fail" -eq 0 ]; then
  echo "=== PASSED ==="
  exit 0
else
  echo "=== FAILED ==="
  exit 1
fi
