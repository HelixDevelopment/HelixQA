#!/bin/bash
# no_suspend_calls_challenge.sh — CONST-033 source-tree gate.
#
# Wraps check-no-suspend-calls.sh as a challenge. Asserts the project's
# source tree contains zero forbidden host-power-management invocations.
#
# Resolves the scanner relative to its own location, so it works
# whether executed from the project root or from challenges/scripts/.
#
# Exit:
#   0 = clean
#   1 = violations
#   2 = scanner missing

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The scanner is in scripts/host-power-management/, but we may be in
# challenges/scripts/. Resolve the project root by walking up until
# we find scripts/host-power-management/check-no-suspend-calls.sh.
find_project_root() {
  local d="$1"
  while [[ "$d" != "/" ]]; do
    if [[ -f "$d/scripts/host-power-management/check-no-suspend-calls.sh" ]]; then
      echo "$d"; return 0
    fi
    d=$(dirname "$d")
  done
  return 1
}

PROJECT_ROOT=$(find_project_root "$SCRIPT_DIR" || true)
if [[ -z "${PROJECT_ROOT:-}" ]]; then
  echo "FAIL: cannot locate scripts/host-power-management/check-no-suspend-calls.sh" >&2
  exit 2
fi

SCANNER="$PROJECT_ROOT/scripts/host-power-management/check-no-suspend-calls.sh"
echo "=== no_suspend_calls_challenge ==="
echo "Scanner: $SCANNER"
echo "Root:    $PROJECT_ROOT"
echo

bash "$SCANNER" "$PROJECT_ROOT"
rc=$?
echo
echo "=== summary: $([[ $rc -eq 0 ]] && echo PASS || echo FAIL) ==="
exit "$rc"
