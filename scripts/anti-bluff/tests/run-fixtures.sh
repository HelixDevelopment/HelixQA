#!/usr/bin/env bash
# Self-test for the scanner: each fixture must produce its expected verdict.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_GO="${SCRIPT_DIR}/../lib/go.sh"
source "${LIB_GO}"

run_fixture() {
  local name="$1" expected_id="$2" file="$3"
  local out
  out="$(scan_go "${name}" "${file}" || true)"
  if [[ -z "$expected_id" ]]; then
    if [[ -n "$out" ]]; then
      echo "FAIL ${name}: expected no hits, got: ${out}"
      return 1
    fi
  else
    if ! grep -q "${expected_id}" <<< "${out}"; then
      echo "FAIL ${name}: expected ${expected_id} hit, got: ${out}"
      return 1
    fi
  fi
  echo "OK ${name}: ${expected_id:-clean}"
}

failed=0
run_fixture "bluff_g_001_skip.go"    "BLUFF-G-001" "${SCRIPT_DIR}/fixtures/bluff_g_001_skip.go"    || failed=1
run_fixture "bluff_g_003_log.go"           "BLUFF-G-003" "${SCRIPT_DIR}/fixtures/bluff_g_003_log.go"           || failed=1
run_fixture "bluff_g_005_empty_subtest.go" "BLUFF-G-005" "${SCRIPT_DIR}/fixtures/bluff_g_005_empty_subtest.go" || failed=1
run_fixture "bluff_g_006_empty_body.go"    "BLUFF-G-006" "${SCRIPT_DIR}/fixtures/bluff_g_006_empty_body.go"    || failed=1
run_fixture "bluff_g_007_trivial.go" "BLUFF-G-007" "${SCRIPT_DIR}/fixtures/bluff_g_007_trivial.go" || failed=1
run_fixture "clean_test.go"          ""            "${SCRIPT_DIR}/fixtures/clean_test.go"          || failed=1

if (( failed )); then
  echo "Scanner self-test FAILED"
  exit 1
fi
echo "Scanner self-test PASSED"
