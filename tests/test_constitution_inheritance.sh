#!/usr/bin/env bash
# test_constitution_inheritance.sh — Comprehensive constitution-inheritance test.
#
# Purpose: run the pre-build inheritance gate, then check that every
# recursively-owned application submodule (if any exist) also carries the
# inheritance pointer. Owned submodules are those directly under this
# project's control, NOT the constitution submodule's own internal
# reusable-engine submodules under constitution/submodules/ (those belong
# to the HelixConstitution project, not to this project).
# Usage: bash tests/test_constitution_inheritance.sh
# Exit: 0 if every invariant PASSes; 1 on the first failure.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=== Step 1: pre-build inheritance gate ==="
if ! bash "${ROOT_DIR}/tests/pre_build_verification.sh"; then
    echo "FAIL: pre_build_verification.sh reported a failing invariant" >&2
    exit 1
fi

echo
echo "=== Step 2: owned application submodules ==="
# Owned submodules = top-level entries in .gitmodules, excluding the
# constitution submodule itself and anything nested under it.
mapfile -t OWNED_SUBMODULES < <(
    git -C "${ROOT_DIR}" config -f "${ROOT_DIR}/.gitmodules" --get-regexp path 2>/dev/null \
        | awk '{print $2}' \
        | grep -v '^constitution$' \
        | grep -v '^constitution/'
)

if [[ ${#OWNED_SUBMODULES[@]} -eq 0 ]]; then
    echo "  No owned application submodules exist yet — invariant trivially satisfied."
else
    for sub in "${OWNED_SUBMODULES[@]}"; do
        SUB_PATH="${ROOT_DIR}/${sub}"
        echo "  Checking ${sub} ..."
        for f in CLAUDE.md AGENTS.md; do
            if [[ ! -f "${SUB_PATH}/${f}" ]]; then
                echo "FAIL: ${sub}/${f} does not exist — inheritance pointer missing" >&2
                exit 1
            fi
            if ! grep -qF 'INHERITED FROM Helix Constitution' "${SUB_PATH}/${f}"; then
                echo "FAIL: ${sub}/${f} does not carry the Helix Constitution inheritance pointer" >&2
                exit 1
            fi
        done
        echo "    OK"
    done
fi

echo
echo "PASS: constitution inheritance verified for the parent project and every owned submodule"
exit 0
