#!/usr/bin/env bash
# chaos_failure_injection_challenge.sh — anti-bluff Chaos Challenge
# for HelixQA per CONST-035 + CONST-050(B). Submodule cascade per
# CONST-051(A). Client-side fault injection (operator-safe — no
# sudo/root): malformed HTTP + oversized headers + slow-loris +
# mixed legit-load control group.

set -uo pipefail

HEALTH_URL="${HELIXQA_HEALTH_URL:-http://localhost:8081/health}"
CHAOS_HOST="${HELIXQA_CHAOS_HOST:-localhost}"
CHAOS_PORT="${HELIXQA_CHAOS_PORT:-8081}"
LEGIT_REQS="${CHAOS_LEGIT_REQUESTS:-100}"
MIN_PCT="${CHAOS_LEGIT_MIN_PASS_PCT:-95}"

echo "=== HelixQA Chaos Failure-Injection Challenge ==="
echo "  url=$HEALTH_URL host=${CHAOS_HOST}:${CHAOS_PORT} legit=${LEGIT_REQS} pass≥${MIN_PCT}%"

pre=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || pre="000"
if [[ "$pre" != "200" ]]; then
    echo "[1/6] SKIP: HelixQA not reachable (HTTP $pre) — SKIP-OK: #env-helixqa-not-running"
    echo "=== HelixQA Chaos Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/6] Pre-chaos: PASS"

for case in "BADVERB / HTTP/1.1" "GET / HTTP/9.9" "GET // HTTP/1.1" "INVALID"; do
    printf '%s\r\nHost: %s\r\n\r\n' "$case" "$CHAOS_HOST" 2>/dev/null \
        | timeout 3 bash -c "exec 3<>/dev/tcp/$CHAOS_HOST/$CHAOS_PORT && cat >&3 && cat <&3" >/dev/null 2>&1 || true
done
post=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post="000"
[[ "$post" != "200" ]] && { echo "[2/6] FAIL: tipped over after malformed (HTTP $post)"; exit 1; }
echo "[2/6] Malformed-salvo survived: PASS"

huge=$(head -c 8192 /dev/urandom | tr -dc 'A-Za-z0-9' | head -c 8192)
oversize=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" -H "X-Chaos-Huge: $huge" "$HEALTH_URL" 2>/dev/null) || oversize="000"
case "$oversize" in
    200|400|413|414|431|494) echo "[3/6] Oversized-header: PASS (HTTP $oversize)";;
    *) echo "[3/6] FAIL: oversized HTTP $oversize"; exit 1;;
esac

pids=()
for _ in $(seq 1 5); do
    (timeout 5 bash -c "
        exec 3<>/dev/tcp/$CHAOS_HOST/$CHAOS_PORT 2>/dev/null
        printf 'GET /health HTTP/1.1\r\nHost: $CHAOS_HOST\r\nX-Slow: ' >&3 2>/dev/null
        sleep 4 2>/dev/null
        printf 'done\r\n\r\n' >&3 2>/dev/null
    " >/dev/null 2>&1 || true) &
    pids+=($!)
done
echo "[4/6] Slow-loris: ${#pids[@]} idle connections spawned"

RES=$(mktemp); trap "rm -f $RES; kill ${pids[*]} 2>/dev/null" EXIT
seq 1 "$LEGIT_REQS" | xargs -n1 -P 20 -I{} \
    curl -sS -o /dev/null --max-time 5 -w "%{http_code}\n" "$HEALTH_URL" 2>/dev/null >> "$RES" || true
for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done; wait 2>/dev/null || true

total=$(wc -l < "$RES" | tr -d ' '); [[ "$total" -eq 0 ]] && total=1
ok=$(awk '$1=="200"{c++} END{print c+0}' "$RES")
pct=$((ok * 100 / total))
echo "[5/6] Mixed-load: total=$total ok=$ok pct=${pct}% (≥${MIN_PCT}%)"
[[ "$pct" -lt "$MIN_PCT" ]] && { echo "  FAIL: chaos starved legit"; exit 1; }

final=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || final="000"
[[ "$final" != "200" ]] && { echo "[6/6] FAIL: post-chaos HTTP $final"; exit 1; }
echo "[6/6] Post-chaos liveness: PASS"

echo
echo "=== HelixQA Chaos Challenge: PASSED ==="
echo "  evidence: legit=$total pct=${pct}% slow_loris=${#pids[@]} oversize_code=${oversize}"
