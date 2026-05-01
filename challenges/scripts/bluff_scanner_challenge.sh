#!/usr/bin/env bash
# CONST-035 — wraps bluff-scanner.sh as a challenge.
#
# Runs in two phases, in order:
#   1. Scanner self-test: hand-crafted fixtures with known verdicts must
#      produce those verdicts. If the scanner's own pattern matchers are
#      broken (e.g., an awk regex regressed), this catches it before the
#      tree-wide scan runs. This satisfies CONST-035's "verification of
#      itself" requirement: "deliberately break the feature; the test
#      MUST fail. If it still passes, the test is non-conformant and
#      MUST be tightened."
#   2. Tree-wide scan: full source tree against the baseline.
#
# Either phase failing fails the challenge.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "[bluff_scanner_challenge] Phase 1/2 — scanner self-test"
bash "${ROOT_DIR}/scripts/anti-bluff/tests/run-fixtures.sh"

echo ""
echo "[bluff_scanner_challenge] Phase 2/2 — tree-wide scan"
bash "${ROOT_DIR}/scripts/anti-bluff/bluff-scanner.sh" --mode all
