#!/usr/bin/env bash
# Pre-commit hook — runs scanner + manifest check on staged files.
# Mutation gate is excluded (too slow for pre-commit).
#
# This file is normally invoked via a symlink in .git/hooks/pre-commit.
# We resolve the symlink so SCRIPT_DIR points to the real script tree,
# not the .git/hooks directory.
set -euo pipefail
SOURCE="${BASH_SOURCE[0]}"
while [[ -L "$SOURCE" ]]; do
  TARGET="$(readlink "$SOURCE")"
  if [[ "$TARGET" == /* ]]; then
    SOURCE="$TARGET"
  else
    SOURCE="$(cd "$(dirname "$SOURCE")" && pwd)/$TARGET"
  fi
done
SCRIPT_DIR="$(cd "$(dirname "$SOURCE")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Run scanner in changed-mode against staged files only.
"${SCRIPT_DIR}/bluff-scanner.sh" --mode changed

# Run anchor manifest check (cheap, < 1s).
if [[ -f "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh" ]]; then
  bash "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh"
fi
