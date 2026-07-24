#!/bin/bash
# run_challenges.sh — Top-level entrypoint for HXS challenge suite
# Usage: bash tests/challenges/run_challenges.sh [max_runs]

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bash "$SCRIPT_DIR/scripts/hxs_repeat_until_green.sh"
