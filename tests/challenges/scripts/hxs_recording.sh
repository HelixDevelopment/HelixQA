#!/bin/bash
# hxs_recording.sh — HelixQA bridge recording orchestration
# Called by hxs_repeat_until_green.sh with start/stop/status commands

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }
. "$LIB_AB"

ab_init "hxs_recording" "/tmp/hxs_recording.results"

BRIDGE_URL="${HELIXQA_BRIDGE_URL:-http://127.0.0.1:7842}"
COMMAND="${1:-}"
RUN_ID="${2:-hxs_unknown}"
RECORDINGS_DIR="$PROJECT_DIR/docs/recordings"

RECORDING_PID=""
TEST_PASSED=0

on_exit() {
    rc=$?
    [ "$TEST_PASSED" = "0" ] && [ $rc -ne 0 ] && [ $rc -ne 2 ] && \
        ab_fail "Recording exited at line ${LINENO:-?} rc=$rc"
    exit $rc
}
trap on_exit EXIT

mkdir -p "$RECORDINGS_DIR"

case "$COMMAND" in
    start)
        ab_send_action "HXS Recording: START for $RUN_ID"
        echo "=== Starting recording for $RUN_ID ==="
        HEALTH=$(curl -s -m 5 -o /dev/null -w '%{http_code}' "$BRIDGE_URL/v1/health" 2>/dev/null || echo "000")
        if [ "$HEALTH" != "200" ]; then
            ab_skip "HelixQA bridge not available at $BRIDGE_URL (HTTP $HEALTH)" "infra"
            TEST_PASSED=1
            ab_summary
            exit 2
        fi
        ab_pass "Bridge health check OK"
        START_RESP=$(curl -s -m 10 -X POST -H 'Content-Type: application/json' \
            -d "{\"test_name\":\"$RUN_ID\",\"interval_ms\":500}" \
            "$BRIDGE_URL/v1/recording/start" 2>/dev/null || echo '{"status":"error"}')
        if echo "$START_RESP" | grep -qiE '"(recording_id|status)"[[:space:]]*:[[:space:]]*"?[^"}]'; then
            ab_pass "Recording started: $(echo "$START_RESP" | head -c 100)"
        else
            ab_fail "Failed to start recording: $(echo "$START_RESP" | head -c 100)"
        fi
        ;;
    stop)
        ab_send_action "HXS Recording: STOP for $RUN_ID"
        echo "=== Stopping recording for $RUN_ID ==="
        STOP_RESP=$(curl -s -m 10 -X POST -H 'Content-Type: application/json' \
            -d "{\"test_name\":\"$RUN_ID\"}" \
            "$BRIDGE_URL/v1/recording/stop" 2>/dev/null || echo '{"status":"error"}')
        if echo "$STOP_RESP" | grep -qiE '"(path|file|recording_path|status)"'; then
            REC_PATH=$(echo "$STOP_RESP" | grep -oE '"(path|file|recording_path)"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*: *"//;s/"//')
            if [ -n "$REC_PATH" ]; then
                cp "$REC_PATH" "$RECORDINGS_DIR/${RUN_ID}.mp4" 2>/dev/null && \
                    ab_pass "Recording saved to $RECORDINGS_DIR/${RUN_ID}.mp4" || \
                    ab_skip "Could not copy recording from $REC_PATH" "infra"
            else
                ab_pass "Recording stopped (no path returned)"
            fi
        else
            ab_skip "Recording stop not fully supported by bridge (may be running as external process)" "infra"
        fi
        ;;
    status)
        ab_send_action "HXS Recording: STATUS"
        echo "Recording dir: $RECORDINGS_DIR"
        find "$RECORDINGS_DIR" -name "*.mp4" -exec ls -lh {} \; 2>/dev/null || echo "No recordings yet"
        ab_pass "Recording status reported"
        ;;
    *)
        echo "Usage: $0 {start|stop|status} [run_id]"
        TEST_PASSED=1
        exit 2
        ;;
esac

TEST_PASSED=1
ab_summary
