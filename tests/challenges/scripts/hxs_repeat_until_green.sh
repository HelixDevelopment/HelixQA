#!/bin/bash
# hxs_repeat_until_green.sh — Endless autonomous loop orchestrator
# Runs setup → frontend → backend → recording → analysis → items
# Loops until zero issues found. Exits only when ALL GREEN.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }

RUN_COUNT=0
MAX_RUNS="${HXS_MAX_RUNS:-0}"
START_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
SCRIPT_LOG="$PROJECT_DIR/docs/challenges/orchestrator.log"

mkdir -p "$(dirname "$SCRIPT_LOG")"

echo "=== HXS Orchestrator Started at $START_TIME ===" | tee -a "$SCRIPT_LOG"
echo "Max runs: ${MAX_RUNS:-infinite}" | tee -a "$SCRIPT_LOG"

run_script() {
    local script="$1" name="$2"
    echo "[$RUN_COUNT] Running $name..." | tee -a "$SCRIPT_LOG"
    if [ ! -f "$SCRIPT_DIR/$script" ]; then
        echo "[$RUN_COUNT]  WARNING $script not found — skipping" | tee -a "$SCRIPT_LOG"
        return 0
    fi
    bash "$SCRIPT_DIR/$script" "hxs_run_${RUN_COUNT}"
    local rc=$?
    if [ "$rc" = "0" ]; then
        echo "[$RUN_COUNT]  PASS $name" | tee -a "$SCRIPT_LOG"
        return 0
    elif [ "$rc" = "2" ]; then
        echo "[$RUN_COUNT]  SKIP $name (infra not available)" | tee -a "$SCRIPT_LOG"
        return 0
    else
        echo "[$RUN_COUNT]  FAIL $name (rc=$rc)" | tee -a "$SCRIPT_LOG"
        return 1
    fi
}

while true; do
    RUN_COUNT=$((RUN_COUNT + 1))
    RUN_TIMESTAMP=$(date -u +"%Y%m%d_%H%M%S")
    RUN_ID="hxs_run_${RUN_COUNT}_${RUN_TIMESTAMP}"

    echo "" | tee -a "$SCRIPT_LOG"
    echo "=== HXS Run $RUN_COUNT: $RUN_ID ===" | tee -a "$SCRIPT_LOG"

    run_script "hxs_setup.sh" "Setup"

    bash "$SCRIPT_DIR/hxs_recording.sh" start "$RUN_ID" >> "$SCRIPT_LOG" 2>&1 || true

    run_script "hxs_frontend_e2e.sh" "Frontend E2E"
    run_script "hxs_backend_e2e.sh" "Backend E2E"

    bash "$SCRIPT_DIR/hxs_recording.sh" stop "$RUN_ID" >> "$SCRIPT_LOG" 2>&1 || true

    run_script "hxs_opencv_analysis.sh" "OpenCV Analysis"
    run_script "hxs_workable_items.sh" "Workable Items"

    if [ -f "/tmp/hxs_has_issues.flag" ]; then
        ISSUE_COUNT=$(cat /tmp/hxs_has_issues.flag 2>/dev/null || echo "?")
        echo "[$RUN_COUNT] WARNING $ISSUE_COUNT issue(s) detected — restarting loop" | tee -a "$SCRIPT_LOG"
        sleep 2
    else
        echo "[$RUN_COUNT] ALL GREEN — zero issues" | tee -a "$SCRIPT_LOG"
        break
    fi

    if [ "$MAX_RUNS" -gt 0 ] && [ "$RUN_COUNT" -ge "$MAX_RUNS" ]; then
        echo "[$RUN_COUNT] Reached max runs ($MAX_RUNS) — exiting" | tee -a "$SCRIPT_LOG"
        break
    fi
done

END_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "" | tee -a "$SCRIPT_LOG"
echo "=== HXS Orchestrator Finished ===" | tee -a "$SCRIPT_LOG"
echo "Started: $START_TIME" | tee -a "$SCRIPT_LOG"
echo "Ended:   $END_TIME" | tee -a "$SCRIPT_LOG"
echo "Runs:    $RUN_COUNT" | tee -a "$SCRIPT_LOG"

REPORT_FILE="$PROJECT_DIR/docs/challenges/HXS_FINAL_REPORT.md"
cat > "$REPORT_FILE" << EOF
# HXS Final Report — $RUN_ID

| Field | Value |
|-------|-------|
| Started | $START_TIME |
| Ended | $END_TIME |
| Total Runs | $RUN_COUNT |
| Status | $( [ -f /tmp/hxs_has_issues.flag ] && echo "ISSUES REMAINING" || echo "ALL GREEN" ) |

## Run Log

\`\`\`
$(cat "$SCRIPT_LOG")
\`\`\`

## Workable Items

$(ls "$PROJECT_DIR/docs/workable-items/" 2>/dev/null | grep HXS || echo "No items created")
EOF

echo "Final report: $REPORT_FILE" | tee -a "$SCRIPT_LOG"

if [ -f /tmp/hxs_has_issues.flag ]; then
    echo "ISSUES REMAINING — check workable-items for details" >&2
    exit 1
fi

echo "ALL GREEN — HXS challenge suite passed!"
exit 0
