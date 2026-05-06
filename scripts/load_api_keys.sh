#!/usr/bin/env bash
# scripts/load_api_keys.sh
# HelixCode API-key loader: prefers $HOME/api_keys.sh (with `export VAR=value`),
# falls back to local .env (VAR=value).
#
# Source this file from any subdirectory; it walks up to find the meta-repo root
# (presence of .gitmodules) for .env fallback location.
#
# Usage:
#   . scripts/load_api_keys.sh         # source from root
#   source $(git rev-parse --show-toplevel)/scripts/load_api_keys.sh  # from anywhere in repo

helixcode_load_api_keys() {
    # Prefer ~/api_keys.sh (always honoured if present)
    if [ -f "$HOME/api_keys.sh" ]; then
        # shellcheck source=/dev/null
        . "$HOME/api_keys.sh"
        return 0
    fi

    # Fallback: walk up to find .gitmodules (meta-repo root) for .env
    local dir
    dir="$(pwd)"
    while [ "$dir" != "/" ] && [ "$dir" != "" ]; do
        if [ -f "$dir/.gitmodules" ] && [ -f "$dir/.env" ]; then
            # source .env with auto-export
            set -a
            # shellcheck source=/dev/null
            . "$dir/.env"
            set +a
            return 0
        fi
        if [ -f "$dir/.env" ]; then
            # any .env up the tree
            set -a
            # shellcheck source=/dev/null
            . "$dir/.env"
            set +a
            return 0
        fi
        dir="$(dirname "$dir")"
    done

    # Neither found - silent (don't fail; caller may not need keys)
    return 1
}

# Auto-run when sourced (allow opt-out via HELIXCODE_LOAD_API_KEYS=0)
if [ "${HELIXCODE_LOAD_API_KEYS:-1}" != "0" ]; then
    helixcode_load_api_keys
fi
