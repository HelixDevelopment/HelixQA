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
    # CONST-035 readiness fix: user api_keys.sh files commonly reference
    # variables that aren't defined in the same file (e.g.
    # `export VERTEX_API_KEY=${ApiKey_Google_Vertex_AI}` where
    # ApiKey_Google_Vertex_AI is set elsewhere). Under a caller's
    # `set -u`, that reference errors out and the loader silently aborts
    # the whole calling script. Disable -u for the user-file sourcing
    # so unbound expansions become empty strings (consistent with the
    # behaviour of running the file directly in an interactive shell
    # without -u). Restore the caller's -u state after.
    local _u_was_set=0
    case $- in *u*) _u_was_set=1 ;; esac
    set +u

    # Prefer ~/api_keys.sh (always honoured if present)
    if [ -f "$HOME/api_keys.sh" ]; then
        # shellcheck source=/dev/null
        . "$HOME/api_keys.sh"
        [ "$_u_was_set" = "1" ] && set -u
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
            [ "$_u_was_set" = "1" ] && set -u
            return 0
        fi
        if [ -f "$dir/.env" ]; then
            # any .env up the tree
            set -a
            # shellcheck source=/dev/null
            . "$dir/.env"
            set +a
            [ "$_u_was_set" = "1" ] && set -u
            return 0
        fi
        dir="$(dirname "$dir")"
    done

    # Neither found - silent (don't fail; caller may not need keys)
    [ "$_u_was_set" = "1" ] && set -u
    return 1
}

# helixcode_normalise_api_keys translates the ApiKey_<Provider> naming
# convention commonly used in $HOME/api_keys.sh into the
# <PROVIDER>_API_KEY canonical names that the Go provider constructors
# actually read via os.Getenv. Without this translation, a user with a
# populated $HOME/api_keys.sh would have keys exported as ApiKey_Groq,
# ApiKey_OpenAI, etc., but the HelixCode providers would not find them
# — a CONST-035 readiness bluff (canonical loader sources keys, but the
# product features can't see them).
#
# Round-41 readiness fix: only sets a canonical name if it's currently
# empty AND the ApiKey_<Provider> name has a non-empty value. Existing
# user-set GROQ_API_KEY etc. is preserved (caller takes precedence).
helixcode_normalise_api_keys() {
    local pair suffix canonical val
    for pair in \
        "Anthropic:ANTHROPIC_API_KEY" \
        "OpenAI:OPENAI_API_KEY" \
        "Groq:GROQ_API_KEY" \
        "Gemini:GEMINI_API_KEY" \
        "OpenRouter:OPENROUTER_API_KEY" \
        "XAI:XAI_API_KEY" \
        "Qwen:QWEN_API_KEY" \
        "GitHub:GITHUB_TOKEN" \
        "Copilot:GITHUB_TOKEN" \
        "DeepSeek:DEEPSEEK_API_KEY" \
        "Mistral_AiStudio:MISTRAL_API_KEY" \
        "Codestral:CODESTRAL_API_KEY" \
        "HuggingFace:HUGGINGFACE_API_KEY" \
        "Nvidia:NVIDIA_API_KEY" \
        "Cerebras:CEREBRAS_API_KEY" \
        "Fireworks_AI:FIREWORKS_API_KEY" \
        "Cloudflare_Workers_AI:CLOUDFLARE_API_KEY" \
        "Vercel_Ai_Gateway:VERCEL_AI_GATEWAY_API_KEY" \
        "SiliconFlow:SILICONFLOW_API_KEY" \
        "Kimi:KIMI_API_KEY" \
        "ZAI:ZAI_API_KEY" \
        "Chutes:CHUTES_API_KEY" \
        "Baseten:BASETEN_API_KEY" \
        "Novita_AI:NOVITA_API_KEY" \
        "Upstage_AI:UPSTAGE_API_KEY" \
        "Hyperbolic:HYPERBOLIC_API_KEY" \
        "SambaNova_AI:SAMBANOVA_API_KEY" \
        "Replicate:REPLICATE_API_KEY"
    do
        suffix="${pair%%:*}"
        canonical="${pair##*:}"
        if [ -z "${!canonical:-}" ]; then
            val="$(eval echo "\${ApiKey_${suffix}:-}")"
            if [ -n "$val" ]; then
                export "$canonical=$val"
            fi
        fi
    done
}

# Auto-run when sourced (allow opt-out via HELIXCODE_LOAD_API_KEYS=0)
if [ "${HELIXCODE_LOAD_API_KEYS:-1}" != "0" ]; then
    helixcode_load_api_keys
    helixcode_normalise_api_keys
fi
