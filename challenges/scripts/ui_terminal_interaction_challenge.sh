#!/usr/bin/env bash
# ui_terminal_interaction_challenge.sh — anti-bluff UI Challenge for
# HelixQA per CONST-035 + CONST-050(B). Submodule cascade per CONST-051(A).
# Drives the HelixQA primary binary non-interactively + asserts on
# stdout schema sanity. No user-hostile pattern leakage.

set -uo pipefail

QA_BIN="${HELIXQA_BIN:-}"
if [[ -z "$QA_BIN" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    for cand in "$SCRIPT_DIR/../../bin/helixqa" "helix_qa/bin/helixqa" "$SCRIPT_DIR/../../../bin/helixqa"; do
        if [[ -x "$cand" ]]; then QA_BIN="$cand"; break; fi
    done
fi
TIMEOUT_SEC="${UI_TIMEOUT_SEC:-30}"

USER_HOSTILE=('panic:' 'goroutine [0-9]+ \[running\]:' 'runtime error:' 'segmentation fault' 'fatal error:')

echo "=== HelixQA UI Terminal-Interaction Challenge ==="
echo "  bin=$QA_BIN timeout=${TIMEOUT_SEC}s"

if [[ -z "$QA_BIN" ]] || [[ ! -x "$QA_BIN" ]]; then
    echo "[1/4] SKIP: helixqa binary not found — SKIP-OK: #env-binary-missing"
    echo "  (run \`cd HelixQA && make build\` to produce ./bin/helixqa)"
    echo "=== HelixQA UI Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/4] Binary present: PASS"

assert_no_panic() {
    local label="$1" body="$2"
    for pat in "${USER_HOSTILE[@]}"; do
        printf '%s' "$body" | grep -qE "$pat" && { echo "  FAIL: $label leaked: $pat"; return 1; }
    done
    return 0
}

help_out=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --help 2>&1 || \
           timeout "$TIMEOUT_SEC" "$QA_BIN" -h 2>&1 || \
           timeout "$TIMEOUT_SEC" "$QA_BIN" help 2>&1 || true)
assert_no_panic "--help" "$help_out" || exit 1
[[ -z "$help_out" ]] && { echo "[2/4] FAIL: empty help output"; exit 1; }
echo "[2/4] Help output: PASS ($(printf '%s' "$help_out" | wc -l) lines)"

ver_out=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --version 2>&1 || \
          timeout "$TIMEOUT_SEC" "$QA_BIN" -v 2>&1 || \
          timeout "$TIMEOUT_SEC" "$QA_BIN" version 2>&1 || true)
assert_no_panic "--version" "$ver_out" || exit 1
echo "[3/4] Version output: PASS (sanitized)"

set +e
bogus=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --this-flag-does-not-exist 2>&1)
bogus_exit=$?
set -e
[[ "$bogus_exit" -ge 124 ]] && { echo "[4/4] FAIL: bogus flag crashed (exit $bogus_exit)"; exit 1; }
assert_no_panic "bogus flag" "$bogus" || exit 1
echo "[4/4] Invalid-flag exit: PASS (exit $bogus_exit)"

echo
echo "=== HelixQA UI Challenge: PASSED ==="
echo "  evidence: bin=$QA_BIN help_lines=$(printf '%s' "$help_out" | wc -l) bogus_exit=$bogus_exit"
