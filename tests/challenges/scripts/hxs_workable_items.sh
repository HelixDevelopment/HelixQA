#!/bin/bash
# hxs_workable_items.sh — Capture all findings as HXS workable items
# Scans /tmp/hxs_*.results files, creates structured YAML items,
# updates DAB (Data Asset Base).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }
. "$LIB_AB"

ab_init "hxs_workable_items" "/tmp/hxs_workable_items.results"
ab_send_action "HXS Workable Items — capture findings, create items, update DAB"

ITEMS_DIR="$PROJECT_DIR/docs/workable-items"
DAB_FILE="$ITEMS_DIR/DAB.yaml"
RUN_ID="${1:-hxs_unknown}"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
TEST_PASSED=0

on_exit() {
    rc=$?
    [ "$TEST_PASSED" = "0" ] && [ $rc -ne 0 ] && [ $rc -ne 2 ] && \
        ab_fail "Workable items exited at line ${LINENO:-?} rc=$rc"
    exit $rc
}
trap on_exit EXIT

mkdir -p "$ITEMS_DIR"

echo "=== HXS Workable Items ==="

HAS_FAILURES=0
ISSUE_COUNT=0
RESULTS_FILES=$(ls /tmp/hxs_*.results 2>/dev/null || echo "")

if [ -z "$RESULTS_FILES" ]; then
    ab_skip "No result files found in /tmp/hxs_*.results — no tests ran yet"
    TEST_PASSED=1
    ab_summary
    exit 2
fi

echo "Scanning result files..."
for rf in $RESULTS_FILES; do
    script_name=$(basename "$rf" .results)
    echo "  $script_name"
    if grep -q '"decision":"FAIL"' "$rf" 2>/dev/null; then
        HAS_FAILURES=1
        ISSUE_COUNT=$((ISSUE_COUNT + 1))
        FAIL_LINE=$(grep -E '"decision":"FAIL"' "$rf" | head -1)
        FAIL_DESC=$(echo "$FAIL_LINE" | grep -oE '"message"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*: *"//;s/"//')
        [ -z "$FAIL_DESC" ] && FAIL_DESC="Unspecified failure"

        ITEM_ID="HXS-$(printf '%03d' $ISSUE_COUNT)"
        ITEM_FILE="$ITEMS_DIR/$ITEM_ID.yaml"

        cat > "$ITEM_FILE" << EOF
---
id: $ITEM_ID
title: "$FAIL_DESC"
status: open
severity: important
source: "$script_name"
challenge_run: "$TIMESTAMP"
description: >
  Auto-detected failure from $script_name during run $RUN_ID.
  $(echo "$FAIL_DESC")
recordings: []
findings: []
root_cause: ""
fix:
  pr: ""
  files: []
  verified_by: ""
related_items: []
EOF
        ab_pass "Created $ITEM_ID: $FAIL_DESC"
    fi
done

if [ -f "$DAB_FILE" ]; then
    for f in "$ITEMS_DIR"/HXS-*.yaml; do
        [ -f "$f" ] || continue
        ITEM_ID=$(basename "$f" .yaml)
        if ! grep -q "$ITEM_ID" "$DAB_FILE" 2>/dev/null; then
            echo "  - id: $ITEM_ID" >> "$DAB_FILE"
            echo "    title: \"$(head -5 "$f" | grep 'title:' | sed 's/.*: *"//;s/"//')\"" >> "$DAB_FILE"
            echo "    status: open" >> "$DAB_FILE"
        fi
    done
else
    cat > "$DAB_FILE" << EOF
---
dab_id: HXS-DAB
title: "Helix Seller Workable Items Data Asset Base"
created: "$TIMESTAMP"
items:
EOF
    for f in "$ITEMS_DIR"/HXS-*.yaml; do
        [ -f "$f" ] || continue
        ITEM_ID=$(basename "$f" .yaml)
        TITLE=$(head -5 "$f" | grep 'title:' | sed 's/.*: *"//;s/"//')
        echo "  - id: $ITEM_ID" >> "$DAB_FILE"
        echo "    title: \"$TITLE\"" >> "$DAB_FILE"
        echo "    status: open" >> "$DAB_FILE"
    done
fi

ab_pass "DAB updated at $DAB_FILE"
echo "Issues found: $ISSUE_COUNT"

if [ "$HAS_FAILURES" = "1" ]; then
    echo "$ISSUE_COUNT" > /tmp/hxs_has_issues.flag
    ab_warn "Found $ISSUE_COUNT issue(s) — created workable items"
else
    rm -f /tmp/hxs_has_issues.flag
    ab_pass "Zero issues — all clear"
fi

echo
echo "=== hxs_workable_items complete ==="
TEST_PASSED=1
ab_summary
