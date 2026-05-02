#!/usr/bin/env bash
# CONST-035 anchor-manifest challenge.
# Validates docs/behavior-anchors.md:
#   1. File exists and parses.
#   2. Every active row's anchor_test_path resolves to an existing
#      file containing the named test symbol.
#   3. Every pending-anchor row appears in baseline Section 3.
#   4. Cross-check against docs/CAPABILITIES.md if present (no-op if absent).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MANIFEST="${ROOT_DIR}/docs/behavior-anchors.md"
BASELINE="${ROOT_DIR}/challenges/baselines/bluff-baseline.txt"
CAPABILITIES="${ROOT_DIR}/docs/CAPABILITIES.md"

if [[ ! -f "${MANIFEST}" ]]; then
  echo "FAIL: ${MANIFEST} missing." >&2
  exit 1
fi

failed=0
# Extract data rows (Markdown table after header line containing "anchor_test_path").
mapfile -t ROWS < <(awk '
  /^\| *id *\|/ { in_table=1; next }
  in_table && /^\|[ ]*-/ { next }
  in_table && /^\|/ { print }
' "${MANIFEST}")

for row in "${ROWS[@]}"; do
  IFS='|' read -ra cols <<< "${row}"
  # cols[0] is empty (leading |), real columns are cols[1..6]
  id="$(echo "${cols[1]:-}"   | xargs || true)"
  layer="$(echo "${cols[2]:-}" | xargs || true)"
  capability="$(echo "${cols[3]:-}" | xargs || true)"
  anchor="$(echo "${cols[4]:-}" | xargs || true)"
  verifies="$(echo "${cols[5]:-}" | xargs || true)"
  status="$(echo "${cols[6]:-}" | xargs || true)"

  if [[ -z "${id}" ]]; then continue; fi

  case "${status}" in
    active)
      # anchor format: "path/to/file.go::TestName" or "path/to/file.kt::Class::method"
      file_part="${anchor%%::*}"
      sym_part="${anchor#*::}"
      if [[ ! -f "${ROOT_DIR}/${file_part}" ]]; then
        echo "FAIL: ${id}: anchor file ${file_part} not found." >&2
        failed=1
        continue
      fi
      # Crude symbol check: grep the file for the test name.
      first_sym="${sym_part%%::*}"
      if ! grep -qE "(func[[:space:]]+|fun[[:space:]]+)${first_sym}\b" "${ROOT_DIR}/${file_part}"; then
        echo "FAIL: ${id}: symbol ${first_sym} not found in ${file_part}." >&2
        failed=1
      fi
      ;;
    pending-anchor)
      if [[ -f "${BASELINE}" ]]; then
        if ! grep -qxF "${id}:MISSING_ANCHOR" "${BASELINE}"; then
          echo "FAIL: ${id}: pending-anchor row not in baseline Section 3." >&2
          failed=1
        fi
      fi
      ;;
    retired)
      ;;
    *)
      echo "FAIL: ${id}: unknown status '${status}'." >&2
      failed=1
      ;;
  esac
done

# Cross-check (no-op if CAPABILITIES.md absent).
if [[ -f "${CAPABILITIES}" ]]; then
  mapfile -t ACTIVE_IDS < <(awk -F'|' '
    /^\| *id *\|/ { in_table=1; next }
    in_table && /^\|[ ]*-/ { next }
    in_table && /^\|/ { gsub(/^[ ]+|[ ]+$/, "", $7); if ($7 == "active") { gsub(/^[ ]+|[ ]+$/, "", $2); print $2 } }
  ' "${MANIFEST}")
  mapfile -t DECLARED_IDS < <(grep -oE 'CAP-[0-9]{3}' "${CAPABILITIES}" | sort -u)
  for cid in "${DECLARED_IDS[@]}"; do
    if ! printf '%s\n' "${ACTIVE_IDS[@]}" | grep -qxF "${cid}"; then
      echo "FAIL: ${cid} declared in CAPABILITIES.md but no active anchor." >&2
      failed=1
    fi
  done
fi

if (( failed )); then
  echo "FAIL: anchor manifest challenge." >&2
  exit 1
fi
echo "OK: anchor manifest valid."
exit 0
