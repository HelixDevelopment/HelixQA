#!/usr/bin/env bash
# Installs the anti-bluff pre-commit hook into .git/hooks/pre-commit.
# Idempotent. Note: HelixQA also has a separate scripts/hooks/no-sudo.sh
# convention; this installer touches only .git/hooks/pre-commit.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

HOOK_TARGET="${ROOT_DIR}/.git/hooks/pre-commit"
HOOK_SOURCE="${SCRIPT_DIR}/pre-commit-hook.sh"

if [[ -e "${HOOK_TARGET}" && ! -L "${HOOK_TARGET}" ]]; then
  echo "Existing non-symlink pre-commit hook at ${HOOK_TARGET}; refusing to overwrite." >&2
  echo "Move it aside, then re-run." >&2
  exit 1
fi

ln -sf "${HOOK_SOURCE}" "${HOOK_TARGET}"
chmod +x "${HOOK_SOURCE}"
echo "Installed ${HOOK_TARGET} -> ${HOOK_SOURCE}"
