#!/usr/bin/env bash
# meta_test_false_positive_proof.sh — Paired mutation proving
# tests/pre_build_verification.sh is not a bluff gate (Constitution §1.1).
#
# Purpose: strip the §11.4 forensic anchor from constitution/Constitution.md,
# assert the pre-build gate FAILs, restore the file, assert the gate PASSes
# again. A gate that stays green through the mutation is itself a
# constitution violation.
# Usage: bash scripts/testing/meta_test_false_positive_proof.sh
# Exit: 0 if the mutation is proven caught and the file is restored clean;
#       1 on any step that doesn't behave as the paired-mutation contract requires.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
GATE="${ROOT_DIR}/tests/pre_build_verification.sh"

CI_TARGET="${ROOT_DIR}/constitution/Constitution.md"
CI_ANCHOR='§11.4 End-user quality guarantee — forensic anchor'
CI_BACKUP="${CI_TARGET}.mut.bak"

cleanup() {
    if [[ -f "${CI_BACKUP}" ]]; then
        mv -- "${CI_BACKUP}" "${CI_TARGET}"
    fi
}
trap cleanup EXIT

echo "[1/4] Baseline: gate must PASS before mutation"
if ! bash "${GATE}" >/tmp/gate_baseline.log 2>&1; then
    echo "FAIL: gate does not pass on the unmutated tree — cannot proceed" >&2
    cat /tmp/gate_baseline.log >&2
    exit 1
fi
echo "  OK — gate PASSes on clean tree"

echo "[2/4] Mutating: stripping the §11.4 forensic anchor from Constitution.md"
[[ -f "${CI_TARGET}" ]] || { echo "FAIL: mutation target missing" >&2; exit 1; }
grep -qF "${CI_ANCHOR}" "${CI_TARGET}" || { echo "FAIL: anchor not present before mutation — nothing to strip" >&2; exit 1; }
cp -- "${CI_TARGET}" "${CI_BACKUP}"
sed -i "s|${CI_ANCHOR}|MUTATED_OUT|g" "${CI_TARGET}"

echo "[3/4] Asserting the gate now FAILs"
if bash "${GATE}" >/tmp/gate_mutated.log 2>&1; then
    echo "FAIL: gate still PASSes after the anchor was stripped — this is a bluff gate" >&2
    cat /tmp/gate_mutated.log >&2
    exit 1
fi
echo "  OK — gate FAILs as expected: $(tail -1 /tmp/gate_mutated.log)"

echo "[4/4] Restoring the file and re-asserting the gate PASSes"
mv -- "${CI_BACKUP}" "${CI_TARGET}"
if ! bash "${GATE}" >/tmp/gate_restored.log 2>&1; then
    echo "FAIL: gate does not pass after restore — restore was incomplete" >&2
    cat /tmp/gate_restored.log >&2
    exit 1
fi
echo "  OK — gate PASSes again after restore"

echo "PASS: CM-CONSTITUTION-INHERITANCE mutation caught by the gate; tree restored clean"
exit 0
