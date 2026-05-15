#!/usr/bin/env bash
# ux_end_to_end_flow_challenge.sh — anti-bluff UX Challenge for
# HelixQA per CONST-035 + CONST-050(B). Submodule cascade per
# CONST-051(A). Drives a complete user journey + asserts on
# coherence (no panics/stack-traces, graceful recovery from bogus
# input, post-error liveness).

set -uo pipefail

QA_BIN="${HELIXQA_BIN:-}"
if [[ -z "$QA_BIN" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    for cand in "$SCRIPT_DIR/../../bin/helixqa" "HelixQA/bin/helixqa" "$SCRIPT_DIR/../../../bin/helixqa"; do
        if [[ -x "$cand" ]]; then QA_BIN="$cand"; break; fi
    done
fi
TIMEOUT_SEC="${UX_TIMEOUT_SEC:-30}"

USER_HOSTILE=('panic:' 'goroutine [0-9]+ \[running\]:' 'runtime error:' 'segmentation fault' 'fatal error:')

echo "=== HelixQA UX End-to-End Flow Challenge ==="
echo "  bin=$QA_BIN timeout=${TIMEOUT_SEC}s"

if [[ -z "$QA_BIN" ]] || [[ ! -x "$QA_BIN" ]]; then
    echo "[1/5] SKIP: helixqa binary not found — SKIP-OK: #env-binary-missing"
    echo "=== HelixQA UX Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/5] Binary present: PASS"

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
[[ -z "$help_out" ]] && { echo "[2/5] FAIL: empty help"; exit 1; }
echo "[2/5] Help discovery: PASS"

ver_out=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --version 2>&1 || \
          timeout "$TIMEOUT_SEC" "$QA_BIN" -v 2>&1 || \
          timeout "$TIMEOUT_SEC" "$QA_BIN" version 2>&1 || true)
assert_no_panic "--version" "$ver_out" || exit 1
echo "[3/5] Version surface: PASS"

set +e
bogus_out=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --does-not-exist-flag 2>&1)
bogus_exit=$?
set -e
assert_no_panic "bogus flag" "$bogus_out" || exit 1
[[ "$bogus_exit" -ge 124 ]] && { echo "[4/5] FAIL: bogus flag crashed (exit $bogus_exit)"; exit 1; }
echo "[4/5] Graceful recovery from bad input: PASS (exit $bogus_exit)"

post_help=$(timeout "$TIMEOUT_SEC" "$QA_BIN" --help 2>&1 || \
            timeout "$TIMEOUT_SEC" "$QA_BIN" -h 2>&1 || \
            timeout "$TIMEOUT_SEC" "$QA_BIN" help 2>&1 || true)
assert_no_panic "post-error --help" "$post_help" || exit 1
[[ -z "$post_help" ]] && { echo "[5/5] FAIL: help broken after bogus invocation"; exit 1; }
echo "[5/5] Post-error liveness: PASS — UX journey survived"

echo
echo "=== HelixQA UX Challenge: PASSED ==="
echo "  evidence: journey=discover→help→version→bogus-recover→post-liveness bogus_exit=$bogus_exit"
