#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# no-sudo.sh — pre-commit hook enforcing the "NO SUDO OR ROOT EXECUTION" rule
# from HelixQA/CLAUDE.md and Catalogizer/CLAUDE.md.
#
# Rationale: sudo in committed content (code, scripts, docs without strike-through)
# signals a design that requires runtime privilege escalation, which is forbidden
# by the Constitution. OpenClawing3.md had four such instances — Phase 0 of
# OpenClawing4 added this hook to prevent regressions.
#
# Policy:
#   - Forbid literal `sudo ` in source/build/test files.
#   - Allow in Markdown design docs only when strike-through'd (`~~sudo~~`) or
#     quoted as "sudo" (retraction notices).
#   - Allow inside `docs/openclawing/OpenClaw*` (retraction sources — not citable).
#
# Exit 0 = clean. Exit 1 = violation.

set -euo pipefail

# Run from the HelixQA repo root regardless of invocation directory.
cd "$(git rev-parse --show-toplevel)"

# Files pre-commit passes as args (or fall back to `git diff --cached`).
if [[ $# -gt 0 ]]; then
    files=("$@")
else
    mapfile -t files < <(git diff --cached --name-only --diff-filter=ACMR)
fi

declare -a violations=()

# Allow-listed paths (retracted docs + this hook itself + pre-commit config
# that references this hook). Patterns are POSIX extended regex, matched
# against the repository-relative path.
allow_pattern='^(scripts/hooks/no-sudo\.sh|docs/openclawing/(OpenClawing2|OpenClawing3|Starting_Point|OpenClawing4|OpenClawing4-Audit|OpenClawing4-Handover)\.md|\.pre-commit-config\.yaml|banks/docs-audit\.(yaml|json)|banks/fixes-validation\.(yaml|json)|challenges/config/helixqa-validation\.yaml)$'

for f in "${files[@]}"; do
    [[ -f "$f" ]] || continue
    # Skip binaries
    case "$f" in
        *.png|*.jpg|*.jpeg|*.mp4|*.pdf|*.docx|*.zip|*.tar|*.gz|*.jar|*.db) continue ;;
    esac
    if [[ "$f" =~ $allow_pattern ]]; then
        continue
    fi

    # Look for active sudo usage: `sudo <word>` that is NOT preceded by `~~`
    # (strike-through) and NOT enclosed in double quotes (e.g. "sudo" as a
    # retraction notice). Strike-through and quoted forms remain legal.
    #
    # grep -nE outputs line numbers; filter with awk to drop allowed forms.
    if matches=$(grep -nE '(^|[^~"[:alnum:]_])sudo[[:space:]]' "$f" 2>/dev/null || true); then
        [[ -z "$matches" ]] && continue
        # Second pass: drop lines where every occurrence is inside `~~sudo~~`
        # or `"sudo"`. Conservative — if any occurrence is bare, we flag.
        if printf '%s\n' "$matches" \
            | grep -vE '~~sudo|"sudo"' >/dev/null; then
            violations+=("$f")
            printf '  %s\n' "$matches" >&2
        fi
    fi
done

if (( ${#violations[@]} > 0 )); then
    echo >&2
    echo "✘ no-sudo hook: $(printf '%s\n' "${violations[@]}" | sort -u | wc -l) file(s) contain bare 'sudo' usage:" >&2
    printf '  - %s\n' "${violations[@]}" | sort -u >&2
    echo >&2
    echo "Allowed in design docs only as ~~sudo~~ (strike-through) or \"sudo\" (quoted reference)." >&2
    echo "See HelixQA/CLAUDE.md § NO SUDO OR ROOT EXECUTION." >&2
    exit 1
fi

exit 0
