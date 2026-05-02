#!/usr/bin/env bash
# CONST-035 mutation ratchet challenge (Go).
# Modes:
#   default (no --mode): run on changed files vs main.
#   --mode all: run full project (slow).
# Compares against challenges/baselines/bluff-baseline.txt Section 2.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MODE="changed"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 3 ;;
  esac
done

BASELINE="${ROOT_DIR}/challenges/baselines/bluff-baseline.txt"

if ! command -v go-mutesting >/dev/null; then
  GOPATH_BIN="$(go env GOPATH)/bin"
  export PATH="${GOPATH_BIN}:${PATH}"
fi

if ! command -v go-mutesting >/dev/null; then
  echo "FAIL: go-mutesting not installed (run: go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest)" >&2
  exit 1
fi

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

# Determine base branch (main preferred, master fallback).
BASE_BRANCH="main"
if ! git -C "${ROOT_DIR}" rev-parse "${BASE_BRANCH}" >/dev/null 2>&1; then
  BASE_BRANCH="master"
fi

if [[ "$MODE" == "changed" ]]; then
  if ! git -C "${ROOT_DIR}" rev-parse "${BASE_BRANCH}" >/dev/null 2>&1; then
    echo "OK: no base branch (${BASE_BRANCH}) — skipping changed-file gate."
    exit 0
  fi
  mapfile -t CHANGED < <(git -C "${ROOT_DIR}" diff --name-only "${BASE_BRANCH}...HEAD" -- '*.go' 2>/dev/null || true)
  # Filter out test files, vendored, generated, and the scanner's own fixtures.
  FILTERED=()
  for f in "${CHANGED[@]}"; do
    [[ -z "$f" ]] && continue
    case "$f" in
      vendor/*|*_test.go|*.pb.go|*_mock.go|scripts/anti-bluff/*) continue ;;
    esac
    FILTERED+=("$f")
  done
  if (( ${#FILTERED[@]} == 0 )); then
    echo "OK: no Go source changes vs ${BASE_BRANCH}."
    exit 0
  fi
  PKGS=()
  declare -A SEEN
  for f in "${FILTERED[@]}"; do
    d="$(dirname "$f")"
    if [[ -z "${SEEN[$d]:-}" ]]; then
      SEEN[$d]=1
      PKGS+=("./$d")
    fi
  done
  : > "${OUT}"
  for p in "${PKGS[@]}"; do
    echo "=== PKG: $p ===" >> "${OUT}"
    go-mutesting --exec-timeout 60 "$p" >> "${OUT}" 2>&1 || true
  done
else
  # --mode all: enumerate buildable packages with tests, mutate each one serially.
  mapfile -t TEST_PKGS < <(find "${ROOT_DIR}" -name "*_test.go" \
    -not -path "*/vendor/*" \
    -not -path "*/scripts/anti-bluff/*" \
    -exec dirname {} \; 2>/dev/null | sort -u)
  : > "${OUT}"
  for d in "${TEST_PKGS[@]}"; do
    rel="${d#${ROOT_DIR}/}"
    # Only mutate packages that build successfully.
    if go build "./${rel}" >/dev/null 2>&1; then
      echo "=== PKG: ./${rel} ===" >> "${OUT}"
      go-mutesting --exec-timeout 60 "./${rel}" >> "${OUT}" 2>&1 || true
    fi
  done
fi

# Parse per-file kill rates and apply ratchet.
python3 - "${OUT}" "${BASELINE}" "${MODE}" <<'PYEOF'
import collections, re, sys
out_path, baseline_path, mode = sys.argv[1], sys.argv[2], sys.argv[3]
killed = collections.Counter()
total = collections.Counter()
pat = re.compile(r'^(PASS|FAIL) "[^"]*go-mutesting-\d+/(.+?)\.\d+"')
with open(out_path) as f:
    for line in f:
        m = pat.match(line)
        if not m: continue
        status, fn = m.group(1), m.group(2) + ""
        # Strip trailing .NN ordinal handled by pattern; ensure .go suffix.
        if not fn.endswith(".go"):
            continue
        total[fn] += 1
        if status == "PASS":
            killed[fn] += 1

baseline = {}
section = None
with open(baseline_path) as f:
    for line in f:
        line = line.rstrip("\n")
        if line.startswith("# === SECTION 2"): section = 2; continue
        if line.startswith("# === SECTION 3"): section = 3; continue
        if section == 2 and line and not line.startswith("#"):
            try:
                p, r, n = line.split(":")
                baseline[p] = int(r)
            except ValueError:
                continue

failed = False
for fn in sorted(total.keys()):
    rate = 100 * killed[fn] // total[fn] if total[fn] else 0
    if mode == "changed" and rate < 90:
        print(f"FAIL: {fn} kill rate {rate}% < 90% (changed-code threshold)")
        failed = True
    if fn in baseline and rate < baseline[fn]:
        print(f"FAIL: {fn} kill rate {rate}% < baseline {baseline[fn]}% (ratchet)")
        failed = True

if mode == "all":
    overall_killed = sum(killed.values())
    overall_total = sum(total.values())
    overall_rate = 100 * overall_killed // overall_total if overall_total else 0
    if overall_total > 0 and overall_rate < 80:
        print(f"FAIL: project-wide kill rate {overall_rate}% < 80%")
        failed = True

sys.exit(1 if failed else 0)
PYEOF

echo "OK: mutation ratchet (mode=${MODE})."
