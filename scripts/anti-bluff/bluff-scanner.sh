#!/usr/bin/env bash
# CONST-035 static scanner — entry point.
# Walks tracked source files, dispatches per-language matchers, applies
# baseline filter, prints new hits. Exit codes:
#   0 clean
#   1 new bluff outside baseline (gate failure)
#   2 baseline drift (a baselined hit is gone — baseline is stale)
#   3 invocation error
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MODE="all"
BASELINE="${ROOT_DIR}/challenges/baselines/bluff-baseline.txt"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --baseline) BASELINE="$2"; shift 2 ;;
    -h|--help)
      echo "usage: bluff-scanner.sh [--mode all|changed] [--baseline <path>]"
      exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 3 ;;
  esac
done

# Source language helpers (presence depends on which repo we're in)
[[ -f "${SCRIPT_DIR}/lib/kotlin.sh" ]] && source "${SCRIPT_DIR}/lib/kotlin.sh"
[[ -f "${SCRIPT_DIR}/lib/go.sh"     ]] && source "${SCRIPT_DIR}/lib/go.sh"

# Determine file list
if [[ "$MODE" == "changed" ]]; then
  # In a git checkout, "changed" = files that differ from main (or master if main absent).
  if git -C "${ROOT_DIR}" rev-parse main >/dev/null 2>&1; then
    mapfile -t FILES < <(git -C "${ROOT_DIR}" diff --name-only main...HEAD)
    # Plus any staged but not yet committed
    mapfile -t STAGED < <(git -C "${ROOT_DIR}" diff --name-only --cached)
    FILES+=("${STAGED[@]}")
  elif git -C "${ROOT_DIR}" rev-parse master >/dev/null 2>&1; then
    mapfile -t FILES < <(git -C "${ROOT_DIR}" diff --name-only master...HEAD)
    mapfile -t STAGED < <(git -C "${ROOT_DIR}" diff --name-only --cached)
    FILES+=("${STAGED[@]}")
  else
    mapfile -t FILES < <(git -C "${ROOT_DIR}" diff --name-only --cached)
  fi
elif [[ "$MODE" == "all" ]]; then
  mapfile -t FILES < <(git -C "${ROOT_DIR}" ls-files)
else
  echo "invalid --mode: ${MODE}" >&2; exit 3
fi

HITS_FILE="$(mktemp -t bluff-scanner.XXXXXX)"
BASELINE_KEYS_FILE="$(mktemp -t bluff-baseline.XXXXXX)"
SEEN_BASELINE_KEYS_FILE="$(mktemp -t bluff-seen.XXXXXX)"
trap 'rm -f "${HITS_FILE}" "${BASELINE_KEYS_FILE}" "${SEEN_BASELINE_KEYS_FILE}" "${BASELINE_KEYS_FILE}.sorted" "${SEEN_BASELINE_KEYS_FILE}.sorted"' EXIT

# Dedupe file list
declare -A SEEN_FILES
for f in "${FILES[@]}"; do
  [[ -z "$f" ]] && continue
  [[ -n "${SEEN_FILES[$f]:-}" ]] && continue
  SEEN_FILES[$f]=1
  fpath="${ROOT_DIR}/${f}"
  [[ ! -f "$fpath" ]] && continue

  # Skip the scanner's own fixtures from full-tree scans (they are deliberately bluff).
  case "$f" in
    scripts/anti-bluff/tests/fixtures/*) continue ;;
  esac

  case "$f" in
    *.kt|*.kts)
      if declare -F scan_kotlin >/dev/null; then
        scan_kotlin "$f" "$fpath" >>"${HITS_FILE}" || true
      fi
      ;;
    *.go)
      if declare -F scan_go >/dev/null; then
        scan_go "$f" "$fpath" >>"${HITS_FILE}" || true
      fi
      ;;
  esac
done

# Build baseline key set (Section 1 only).
if [[ -f "${BASELINE}" ]]; then
  awk '
    /^# === SECTION 2/ { exit }
    /^[^#[:space:]]/ && NF > 0 { print }
  ' "${BASELINE}" > "${BASELINE_KEYS_FILE}"
else
  : > "${BASELINE_KEYS_FILE}"
fi

# Filter: a hit line is "path:line:BLUFF-ID:context"; key is "path:BLUFF-ID".
NEW_HITS=0
: > "${SEEN_BASELINE_KEYS_FILE}"

while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  key=$(awk -F: '{print $1 ":" $3}' <<<"$line")
  if grep -qxF "${key}" "${BASELINE_KEYS_FILE}"; then
    echo "${key}" >> "${SEEN_BASELINE_KEYS_FILE}"
  else
    echo "$line"
    NEW_HITS=$((NEW_HITS+1))
  fi
done < "${HITS_FILE}"

# Drift detection: baseline keys not seen this run = stale baseline.
# Only meaningful in --mode all.
DRIFT=0
if [[ "$MODE" == "all" ]]; then
  sort -u "${SEEN_BASELINE_KEYS_FILE}" > "${SEEN_BASELINE_KEYS_FILE}.sorted"
  sort -u "${BASELINE_KEYS_FILE}"       > "${BASELINE_KEYS_FILE}.sorted"
  mapfile -t STALE < <(comm -23 "${BASELINE_KEYS_FILE}.sorted" "${SEEN_BASELINE_KEYS_FILE}.sorted")
  if (( ${#STALE[@]} > 0 )); then
    echo "" >&2
    echo "WARN: ${#STALE[@]} baseline entries are no longer present; baseline is stale." >&2
    printf '  %s\n' "${STALE[@]}" >&2
    DRIFT=1
  fi
fi

if (( NEW_HITS > 0 )); then
  echo "" >&2
  echo "FAIL: ${NEW_HITS} new bluff hit(s) outside baseline. Fix or add an exempt comment." >&2
  exit 1
fi

if (( DRIFT > 0 )); then
  echo "" >&2
  echo "FAIL: baseline is stale (${#STALE[@]} entries). Run 'make update-baseline' to refresh." >&2
  exit 2
fi

echo "OK: scanner clean (mode=${MODE})." >&2
exit 0
