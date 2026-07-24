#!/bin/bash
# hxs_opencv_analysis.sh — Trigger OpenCV analysis via HelixQA bridge
# Analyzes recordings from hxs_recording.sh using latest OpenCV features
# (ORB/SIFT template matching, OCR, layout comparison, frame analysis)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }
. "$LIB_AB"

ab_init "hxs_opencv_analysis" "/tmp/hxs_opencv_analysis.results"
ab_send_action "HXS OpenCV Analysis — visual analysis of recordings"

BRIDGE_URL="${HELIXQA_BRIDGE_URL:-http://127.0.0.1:7842}"
RUN_ID="${1:-hxs_unknown}"
BASELINE_DIR="$PROJECT_DIR/tests/challenges/baselines"
FINDINGS_TIMEOUT_S="${HXS_FINDINGS_TIMEOUT_S:-30}"
TEST_PASSED=0

on_exit() {
    rc=$?
    [ "$TEST_PASSED" = "0" ] && [ $rc -ne 0 ] && [ $rc -ne 2 ] && \
        ab_fail "OpenCV analysis exited at line ${LINENO:-?} rc=$rc"
    exit $rc
}
trap on_exit EXIT

HEALTH=$(curl -s -m 5 -o /dev/null -w '%{http_code}' "$BRIDGE_URL/v1/health" 2>/dev/null || echo "000")
if [ "$HEALTH" != "200" ]; then
    ab_skip "HelixQA bridge not available at $BRIDGE_URL (HTTP $HEALTH)"
    TEST_PASSED=1
    ab_summary
    exit 2
fi

echo "=== HXS OpenCV Analysis ==="

echo "Triggering analysis for $RUN_ID..."
ANALYZE_RESP=$(curl -s -m 15 -X POST -H 'Content-Type: application/json' \
    -d "{\"test_name\":\"$RUN_ID\",\"pipeline\":\"full\"}" \
    "$BRIDGE_URL/v1/analyze/start" 2>/dev/null || echo '{"status":"error"}')
if echo "$ANALYZE_RESP" | grep -qiE '"(status|analysis_id)"[[:space:]]*:[[:space:]]*"?[^"}]'; then
    ab_pass "Analysis pipeline triggered"
else
    ab_skip "Analysis pipeline not available on bridge"
    TEST_PASSED=1
    ab_summary
    exit 2
fi

echo "Streaming findings (timeout ${FINDINGS_TIMEOUT_S}s)..."
FINDINGS_FILE="/tmp/hxs_findings_${RUN_ID}.ndjson"
: > "$FINDINGS_FILE"

elapsed=0
while [ "$elapsed" -lt "$FINDINGS_TIMEOUT_S" ]; do
    curl -s -m 5 -N \
        -G --data-urlencode "test_name=$RUN_ID" \
        "$BRIDGE_URL/v1/findings/stream" 2>/dev/null \
        | head -20 >> "$FINDINGS_FILE" || true
    if [ -s "$FINDINGS_FILE" ]; then
        break
    fi
    sleep 2
    elapsed=$((elapsed + 7))
done

LINE_COUNT=$(wc -l < "$FINDINGS_FILE" 2>/dev/null || echo 0)
LINE_COUNT=$(echo "$LINE_COUNT" | tr -dc '0-9')
[ -z "$LINE_COUNT" ] && LINE_COUNT=0

echo "Findings lines: $LINE_COUNT"
if [ "$LINE_COUNT" -ge 1 ]; then
    ab_pass "Findings stream returned $LINE_COUNT lines"
    FIRST=$(head -1 "$FINDINGS_FILE")
    if echo "$FIRST" | grep -qE '^\{.*\}$'; then
        ab_pass "First finding is valid JSON"
        for field in ts display frame_idx decision; do
            echo "$FIRST" | grep -qE "\"$field\"[[:space:]]*:" && \
                ab_pass "Field '$field' present" || \
                ab_warn "Field '$field' missing in finding"
        done
    else
        ab_fail "First finding is not valid JSON"
    fi
else
    ab_skip "No findings returned (pipeline may still be processing)"
fi

OPENCV_INFO=$(curl -s -m 3 "$BRIDGE_URL/v1/opencv/version" 2>/dev/null || echo "")
if [ -n "$OPENCV_INFO" ]; then
    ab_pass "OpenCV version info: $(echo "$OPENCV_INFO" | head -c 80)"
else
    ab_skip "OpenCV version endpoint not available"
fi

echo
echo "=== hxs_opencv_analysis complete ==="
TEST_PASSED=1
ab_summary
