#!/usr/bin/env bash
# Pre-commit hook — runs scanner + manifest check on staged files.
# Mutation gate is excluded (too slow for pre-commit).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Run scanner in changed-mode against staged files only.
"${SCRIPT_DIR}/bluff-scanner.sh" --mode changed

# Run anchor manifest check (cheap, < 1s).
if [[ -f "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh" ]]; then
  bash "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh"
fi
