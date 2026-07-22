#!/usr/bin/env bash
# pre_build_verification.sh — Constitution-inheritance gate.
#
# Purpose: verify the Helix Constitution submodule is present, intact,
# and that this project's own governance files actually reference it.
# Usage: bash tests/pre_build_verification.sh   (run from repo root or anywhere)
# Exit: 0 if every invariant holds, 1 on the first failed invariant (message on stderr).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

# Invariant 1: constitution/ directory exists
[[ -d "${ROOT_DIR}/constitution" ]] || fail "constitution/ directory does not exist"

# Invariant 2: constitution/Constitution.md exists and contains the forensic anchor
CONST_MD="${ROOT_DIR}/constitution/Constitution.md"
[[ -f "${CONST_MD}" ]] || fail "constitution/Constitution.md does not exist"
grep -qF '§11.4 End-user quality guarantee — forensic anchor' "${CONST_MD}" \
    || fail "constitution/Constitution.md missing the §11.4 forensic anchor"

# Invariant 3: constitution/CLAUDE.md exists and contains the anti-bluff covenant anchor
CLAUDE_MD="${ROOT_DIR}/constitution/CLAUDE.md"
[[ -f "${CLAUDE_MD}" ]] || fail "constitution/CLAUDE.md does not exist"
grep -qF 'MANDATORY ANTI-BLUFF COVENANT' "${CLAUDE_MD}" \
    || fail "constitution/CLAUDE.md missing the MANDATORY ANTI-BLUFF COVENANT anchor"

# Invariant 4: constitution/AGENTS.md exists and contains the anti-bluff covenant anchor
AGENTS_MD="${ROOT_DIR}/constitution/AGENTS.md"
[[ -f "${AGENTS_MD}" ]] || fail "constitution/AGENTS.md does not exist"
grep -qF 'Anti-bluff covenant' "${AGENTS_MD}" \
    || fail "constitution/AGENTS.md missing the Anti-bluff covenant anchor"

# Invariant 5: parent CLAUDE.md / AGENTS.md / project constitution all reference the submodule
[[ -f "${ROOT_DIR}/CLAUDE.md" ]] || fail "root CLAUDE.md does not exist"
grep -qF 'constitution/CLAUDE.md' "${ROOT_DIR}/CLAUDE.md" \
    || fail "root CLAUDE.md does not reference constitution/CLAUDE.md"

[[ -f "${ROOT_DIR}/AGENTS.md" ]] || fail "root AGENTS.md does not exist"
grep -qF 'constitution/AGENTS.md' "${ROOT_DIR}/AGENTS.md" \
    || fail "root AGENTS.md does not reference constitution/AGENTS.md"

PROJECT_CONST="${ROOT_DIR}/docs/guides/HELIX_SELLER_CONSTITUTION.md"
[[ -f "${PROJECT_CONST}" ]] || fail "docs/guides/HELIX_SELLER_CONSTITUTION.md does not exist"
grep -qF 'constitution/Constitution.md' "${PROJECT_CONST}" \
    || fail "project constitution does not reference constitution/Constitution.md"

echo "PASS: all constitution-inheritance invariants hold"
exit 0
