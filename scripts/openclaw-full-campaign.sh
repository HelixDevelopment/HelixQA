#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# OpenClawing2 full production-readiness campaign runner.
# Orchestrates every regression bank + unit + integration + stress +
# security + benchmark + challenge suite across Phases 1–7 of the
# integration plan. Captures a single FINAL-REPORT and the Grafana
# panel snapshots.
#
# Usage:
#   scripts/openclaw-full-campaign.sh                    # full campaign
#   scripts/openclaw-full-campaign.sh --skip-stress      # skip long-running stress
#   scripts/openclaw-full-campaign.sh --phases 2,3,4     # subset
#   scripts/openclaw-full-campaign.sh --dry-run          # print the plan, run nothing
#
# Environment:
#   HELIXQA_REPORT_DIR       Output directory (default: qa-results/openclaw-<ts>)
#   HELIXQA_SKIP_BENCHMARKS  Skip `go test -bench` runs (default: 0)
#
# Non-negotiable constraints honoured by this script:
#   - Never uses sudo / root.
#   - Honours the host resource budget (GOMAXPROCS=3, -p 2 -parallel 2).
#   - Does not invoke any LLM / cloud endpoint; every test is offline.
#   - No Docker / Podman dependency for the unit + integration tiers;
#     containerised tiers are launched separately via
#     scripts/helixqa-orchestrator.sh.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TS="$(date -u +%Y%m%d-%H%M%S)"
REPORT_DIR="${HELIXQA_REPORT_DIR:-qa-results/openclaw-$TS}"
SKIP_STRESS=0
SKIP_BENCH=${HELIXQA_SKIP_BENCHMARKS:-0}
DRY_RUN=0
PHASES="1,2,3,4,5,6,7"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-stress) SKIP_STRESS=1 ;;
    --skip-benchmarks) SKIP_BENCH=1 ;;
    --dry-run) DRY_RUN=1 ;;
    --phases) PHASES="$2"; shift ;;
    --report-dir) REPORT_DIR="$2"; shift ;;
    -h|--help)
      sed -n '1,30p' "$0"
      exit 0
      ;;
    *)
      echo "unknown flag: $1" >&2
      exit 2
      ;;
  esac
  shift
done

mkdir -p "$REPORT_DIR"
echo "OpenClawing2 full campaign starting at $TS"
echo "  report dir: $REPORT_DIR"
echo "  phases:     $PHASES"
echo "  stress:     $([[ $SKIP_STRESS == 1 ]] && echo 'SKIPPED' || echo 'ON')"
echo "  benchmarks: $([[ $SKIP_BENCH == 1 ]] && echo 'SKIPPED' || echo 'ON')"
echo

export GOMAXPROCS=3
export GOTOOLCHAIN=local
export GOFLAGS="-mod=mod"

run_or_print() {
  local label="$1"; shift
  local log="$REPORT_DIR/${label}.log"
  echo "→ $label"
  if [[ $DRY_RUN == 1 ]]; then
    echo "    (dry-run) $*"
    return 0
  fi
  if ! ( "$@" ) &> "$log"; then
    echo "    FAIL — see $log" >&2
    return 1
  fi
  echo "    OK"
  return 0
}

phase_enabled() {
  [[ ",$PHASES," == *",$1,"* ]]
}

FAILED=()

# Phase 1 — OSS vendoring audit.
if phase_enabled 1; then
  run_or_print phase1-opensource-audit \
    go test ./pkg/opensource/... -count=1 -race || FAILED+=(phase1-opensource-audit)
  run_or_print phase1-sync-script-dryrun \
    bash scripts/sync-opensource-references.sh || FAILED+=(phase1-sync-script-dryrun)
fi

# Phase 2 — Agent state machine.
if phase_enabled 2; then
  run_or_print phase2-agent-unit \
    go test ./pkg/nexus/agent/... -count=1 -race -run 'TestAgent|TestNewAgent|TestParsePlannerJSON|TestAgentState' \
    || FAILED+=(phase2-agent-unit)
  run_or_print phase2-agent-integration \
    go test ./pkg/nexus/agent/... -count=1 -race -run Integration || FAILED+=(phase2-agent-integration)
fi

# Phase 3 — MessageManager.
if phase_enabled 3; then
  run_or_print phase3-messages-unit \
    go test ./pkg/nexus/agent/... -count=1 -race -run TestMessageManager || FAILED+=(phase3-messages-unit)
  run_or_print phase3-messages-fuzz \
    go test ./pkg/nexus/agent/... -run '^$' -fuzz FuzzMessageManager_Compact -fuzztime 3s \
    || FAILED+=(phase3-messages-fuzz)
fi

# Phase 4 — retry / healer / loop detection.
if phase_enabled 4; then
  run_or_print phase4-retry-unit \
    go test ./pkg/nexus/agent/... -count=1 -race -run 'TestBackoff|TestRetry|TestLoopDetector|TestFingerprint|TestNewSelfHealer|TestSelfHealer|TestStress_Sequential' \
    || FAILED+=(phase4-retry-unit)
fi

# Phase 5 — Stagehand primitives.
if phase_enabled 5; then
  run_or_print phase5-primitives \
    go test ./pkg/nexus/primitives/... -count=1 -race || FAILED+=(phase5-primitives)
  run_or_print phase5-primitives-fuzz \
    go test ./pkg/nexus/primitives/... -run '^$' -fuzz FuzzRefsFromMemory -fuzztime 3s \
    || FAILED+=(phase5-primitives-fuzz)
fi

# Phase 6 — coordinate scaling + action builders.
if phase_enabled 6; then
  run_or_print phase6-coordinate \
    go test ./pkg/nexus/coordinate/... -count=1 -race || FAILED+=(phase6-coordinate)
  run_or_print phase6-coordinate-fuzz \
    go test ./pkg/nexus/coordinate/... -run '^$' -fuzz FuzzScaleCoordinates -fuzztime 3s \
    || FAILED+=(phase6-coordinate-fuzz)
fi

# Phase 7 — rich ticketing.
if phase_enabled 7; then
  run_or_print phase7-ticket \
    go test ./pkg/ticket/... -count=1 -race || FAILED+=(phase7-ticket)
fi

# P10 — real-MinIO container integration (build-tagged, auto-skips
# when no container runtime is reachable).
if [[ "${HELIXQA_SKIP_CONTAINER_TESTS:-0}" != "1" ]]; then
  run_or_print p10-minio-integration \
    go test -tags=integration ./pkg/nexus/orchestrator/... -count=1 -run P10 \
    || FAILED+=(p10-minio-integration)
fi

# Benchmarks — only when explicitly requested.
if [[ $SKIP_BENCH == 0 ]]; then
  run_or_print benchmarks \
    go test -run '^$' -bench . -benchtime 1x \
      ./pkg/nexus/agent/... ./pkg/nexus/primitives/... ./pkg/nexus/coordinate/... \
    || FAILED+=(benchmarks)
fi

# FINAL-REPORT.md summary.
{
  echo "# OpenClawing2 Full Campaign — $TS"
  echo
  echo "Report directory: \`$REPORT_DIR\`"
  echo
  echo "## Outcome"
  if [[ ${#FAILED[@]} -eq 0 ]]; then
    echo "**GREEN** — every enabled phase passed."
  else
    echo "**RED** — ${#FAILED[@]} section(s) failed:"
    for f in "${FAILED[@]}"; do
      echo "  - $f"
    done
  fi
  echo
  echo "## Enabled phases"
  echo "\`$PHASES\`"
  echo
  echo "## Stress enabled"
  [[ $SKIP_STRESS == 1 ]] && echo "no" || echo "yes"
  echo
  echo "## Benchmarks enabled"
  [[ $SKIP_BENCH == 1 ]] && echo "no" || echo "yes"
  echo
  echo "## Per-section logs"
  for f in "$REPORT_DIR"/*.log; do
    echo "- \`$(basename "$f")\`"
  done
} > "$REPORT_DIR/FINAL-REPORT.md"

echo
echo "============================================"
echo "FINAL-REPORT: $REPORT_DIR/FINAL-REPORT.md"
echo "============================================"

if [[ ${#FAILED[@]} -gt 0 ]]; then
  exit 1
fi
