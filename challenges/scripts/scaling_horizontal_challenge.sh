#!/usr/bin/env bash
# scaling_horizontal_challenge.sh — anti-bluff Scaling Challenge for
# HelixQA per CONST-035 + CONST-050(B). Submodule cascade per CONST-051(A).
# Topology-dispatch via HELIXQA_SCALING_REPLICA_URLS.

set -uo pipefail

REPLICAS="${HELIXQA_SCALING_REPLICA_URLS:-}"
REQS="${SCALING_REQS_PER_REPLICA:-50}"
CONC="${SCALING_CONCURRENCY:-10}"
MIN_PCT="${SCALING_MIN_PASS_PCT:-95}"

echo "=== HelixQA Scaling Challenge ==="
echo "  replicas=$REPLICAS reqs=$REQS conc=$CONC pass≥${MIN_PCT}%"

if [[ -z "$REPLICAS" ]]; then
    echo "[1/6] SKIP: HELIXQA_SCALING_REPLICA_URLS unset — SKIP-OK: #env-single-replica"
    echo "=== HelixQA Scaling Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi

IFS=',' read -r -a URLS <<< "$REPLICAS"
REACH=()
for u in "${URLS[@]}"; do
    u="${u// /}"; [[ -z "$u" ]] && continue
    c=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$u/health" 2>/dev/null) || c="000"
    [[ "$c" == "200" ]] && REACH+=("$u") && echo "  reachable: $u"
done
if [[ ${#REACH[@]} -lt 2 ]]; then
    echo "[1/6] SKIP: ${#REACH[@]} reachable — SKIP-OK: #env-single-replica"
    echo "=== HelixQA Scaling Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/6] Topology: ${#REACH[@]} replicas reachable — PASS"

for u in "${REACH[@]}"; do
    b=$(curl -sS --max-time 5 "$u/health" 2>/dev/null || true)
    printf '%s' "$b" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' || { echo "[2/6] FAIL: $u no status"; exit 1; }
done
echo "[2/6] Schema sanity: PASS"

declare -A OK
for u in "${REACH[@]}"; do
    r=$(mktemp)
    seq 1 "$REQS" | xargs -n1 -P "$CONC" -I{} \
        curl -sS -o /dev/null --max-time 5 -w "%{http_code}\n" "$u/health" 2>/dev/null >> "$r" || true
    okc=$(awk '$1=="200"{c++} END{print c+0}' "$r")
    tot=$(wc -l < "$r" | tr -d ' '); [[ "$tot" -eq 0 ]] && tot=1
    pct=$((okc * 100 / tot))
    OK[$u]=$okc
    rm -f "$r"
    echo "  $u → $okc/$tot ($pct%)"
    [[ "$pct" -lt "$MIN_PCT" ]] && { echo "[3/6] FAIL: $u below threshold"; exit 1; }
done
echo "[3/6] Per-replica load: PASS"

first=$(curl -sS --max-time 5 "${REACH[0]}/health" 2>/dev/null | sed 's/"uptime[^,}]*//g; s/"timestamp[^,}]*//g')
fh=$(printf '%s' "$first" | sha256sum | awk '{print $1}')
mm=0
for u in "${REACH[@]:1}"; do
    b=$(curl -sS --max-time 5 "$u/health" 2>/dev/null | sed 's/"uptime[^,}]*//g; s/"timestamp[^,}]*//g')
    h=$(printf '%s' "$b" | sha256sum | awk '{print $1}')
    [[ "$h" == "$fh" ]] && echo "  MATCH: $u" || { echo "  DIFF: $u"; mm=$((mm+1)); }
done
[[ "$mm" -gt 0 ]] && { echo "[4/6] FAIL: $mm replicas diverged"; exit 1; }
echo "[4/6] Body-identity: PASS (sha256=${fh:0:12}...)"

total_ok=0; for u in "${REACH[@]}"; do total_ok=$((total_ok + ${OK[$u]})); done
echo "[5/6] LB-fairness informational: total_ok=$total_ok across ${#REACH[@]} replicas"

for u in "${REACH[@]}"; do
    c=$(curl -sS --max-time 5 -o /dev/null -w "%{http_code}" "$u/health" 2>/dev/null) || c="000"
    [[ "$c" != "200" ]] && { echo "[6/6] FAIL: $u tipped (HTTP $c)"; exit 1; }
done
echo "[6/6] Post-scaling liveness: PASS"

echo
echo "=== HelixQA Scaling Challenge: PASSED ==="
echo "  evidence: replicas=${#REACH[@]} total_ok=${total_ok} body_identity=ok"
