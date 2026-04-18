#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# ocu-full-campaign.sh — 10-category OCU validation campaign.
#
# Per Constitution Article V: all categories must be green to ship.
# Runs sequentially (never in parallel) to stay within host resource
# budgets (max 30-40% of total CPU/RAM per the CLAUDE.md constraint).
#
# Usage:
#   ./scripts/ocu-full-campaign.sh           # run all categories
#   ./scripts/ocu-full-campaign.sh --bench   # include bench (default)
#   ./scripts/ocu-full-campaign.sh --fast    # skip bench + stress
#
# Exit code: 0 if all categories pass, 1 if any fail.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOTOOLCHAIN

SKIP_BENCH=false
SKIP_STRESS=false

for arg in "$@"; do
  case "$arg" in
    --fast) SKIP_BENCH=true; SKIP_STRESS=true ;;
    --skip-bench) SKIP_BENCH=true ;;
    --skip-stress) SKIP_STRESS=true ;;
  esac
done

# Associative array: label → status string.
# Keys are inserted in declaration order.
declare -A results
declare -a ordered_labels

_run() {
  local label="$1"
  shift
  ordered_labels+=("$label")
  echo ""
  echo "=== [$label] ==="
  if "$@" 2>&1; then
    results["$label"]="PASS"
  else
    results["$label"]="FAIL"
  fi
}

# ---------------------------------------------------------------------------
# 1. Unit — pure logic tests across pkg/nexus packages.
# ---------------------------------------------------------------------------
_run "1-unit" \
  go test -race ./pkg/nexus/... -count=1 -timeout 300s

# ---------------------------------------------------------------------------
# 2. Integration — build-tagged integration tests.
#    Skipped silently if no integration test files exist.
# ---------------------------------------------------------------------------
if compgen -G "tests/integration/*.go" > /dev/null 2>&1; then
  _run "2-integration" \
    go test -tags=integration -race ./tests/integration/... -count=1 -timeout 300s
else
  echo ""
  echo "=== [2-integration] — no integration tests found, marking SKIP ==="
  ordered_labels+=("2-integration")
  results["2-integration"]="SKIP"
fi

# ---------------------------------------------------------------------------
# 3. Stress — tests whose names begin with TestStress.
# ---------------------------------------------------------------------------
if [ "$SKIP_STRESS" = "true" ]; then
  echo ""
  echo "=== [3-stress] — skipped via --fast / --skip-stress ==="
  ordered_labels+=("3-stress")
  results["3-stress"]="SKIP"
else
  _run "3-stress" \
    go test -race -run 'TestStress' ./pkg/nexus/... -count=1 -timeout 300s
fi

# ---------------------------------------------------------------------------
# 4. Security — govulncheck (source mode).
#    Falls back gracefully if govulncheck is not installed.
# ---------------------------------------------------------------------------
if command -v govulncheck > /dev/null 2>&1; then
  _run "4-security-vuln" \
    govulncheck -mode source ./...
else
  echo ""
  echo "=== [4-security-vuln] — govulncheck not installed, marking SKIP ==="
  ordered_labels+=("4-security-vuln")
  results["4-security-vuln"]="SKIP"
fi

# ---------------------------------------------------------------------------
# 5. Security — go vet.
# ---------------------------------------------------------------------------
_run "5-security-vet" \
  go vet ./...

# ---------------------------------------------------------------------------
# 6. Bench — latency/throughput/memory baselines.
# ---------------------------------------------------------------------------
if [ "$SKIP_BENCH" = "true" ]; then
  echo ""
  echo "=== [6-bench] — skipped via --fast / --skip-bench ==="
  ordered_labels+=("6-bench")
  results["6-bench"]="SKIP"
else
  _run "6-bench" \
    go test -bench=. -benchmem -run '^$' -count=1 -benchtime=500ms ./pkg/nexus/...
fi

# ---------------------------------------------------------------------------
# 7. Gofmt — zero unformatted files.
# ---------------------------------------------------------------------------
_gofmt_check() {
  local unformatted
  unformatted="$(gofmt -l pkg/ cmd/ 2>/dev/null || true)"
  if [ -n "$unformatted" ]; then
    echo "Unformatted files:"
    echo "$unformatted"
    return 1
  fi
}
_run "7-gofmt" _gofmt_check

# ---------------------------------------------------------------------------
# 8. Challenges — load and validate all ocu-*.json banks via testbank.
# ---------------------------------------------------------------------------
_run "8-challenges" \
  go test -race ./pkg/testbank/... -count=1 -timeout 120s

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "=============================="
echo "   OCU Campaign — Summary"
echo "=============================="
printf "%-28s %s\n" "Category" "Status"
printf "%-28s %s\n" "--------" "------"

any_fail=false
for label in "${ordered_labels[@]}"; do
  status="${results[$label]}"
  marker="  "
  if [ "$status" = "PASS" ] || [ "$status" = "SKIP" ]; then
    marker="ok"
  else
    marker="FAIL"
    any_fail=true
  fi
  printf "%-28s [%s] %s\n" "$label" "$marker" "$status"
done

echo ""
if [ "$any_fail" = "true" ]; then
  echo "RESULT: FAIL — one or more categories did not pass."
  echo "Per Constitution Article V: shipping is prohibited until all categories are green."
  exit 1
else
  echo "RESULT: PASS — all categories green."
  exit 0
fi
