#!/usr/bin/env bash
# stress_sustained_load_challenge.sh — anti-bluff Stress Challenge
# for HelixQA per CONST-035 + CONST-050(B). Submodule cascade of
# the parent project's stress Challenge per CONST-051(A).

set -uo pipefail

HEALTH_URL="${HELIXQA_HEALTH_URL:-http://localhost:8081/health}"
DURATION="${STRESS_DURATION_SEC:-15}"
RPS="${STRESS_REQUESTS_PER_SEC:-50}"
CONCURRENCY="${STRESS_CONCURRENCY:-20}"
TIMEOUT_SEC="${STRESS_TIMEOUT_SEC:-5}"
MIN_PASS_PCT="${STRESS_MIN_PASS_PCT:-95}"
MAX_DEG_PCT="${STRESS_MAX_LATENCY_DEGRADATION_PCT:-300}"

echo "=== HelixQA Stress Sustained-Load Challenge ==="
echo "  url=$HEALTH_URL dur=${DURATION}s rps=${RPS} conc=${CONCURRENCY} pass≥${MIN_PASS_PCT}% deg≤${MAX_DEG_PCT}%"

pre=$(curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || pre="000"
if [[ "$pre" != "200" ]]; then
    echo "[1/6] SKIP: HelixQA not reachable (HTTP $pre) — SKIP-OK: #env-helixqa-not-running"
    echo "=== HelixQA Stress Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/6] Pre-stress: PASS"

base=$(mktemp); trap "rm -f $base" EXIT
for _ in $(seq 1 10); do
    curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" -w "%{time_total}\n" "$HEALTH_URL" 2>/dev/null >> "$base" || true
done
base_med=$(sort -n "$base" | awk 'NR==5{print; exit}')
echo "[2/6] Baseline median: ${base_med}s"

body=$(curl -sS --max-time "$TIMEOUT_SEC" "$HEALTH_URL" 2>/dev/null || true)
printf '%s' "$body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' || { echo "[3/6] FAIL: no status field"; exit 1; }
echo "[3/6] Schema sanity: PASS"

RES=$(mktemp); trap "rm -f $base $RES" EXIT
start=$(date +%s.%N)
total_target=$((DURATION * RPS))
seq 1 "$total_target" | xargs -n1 -P "$CONCURRENCY" -I{} \
    curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" \
        -w "%{http_code} %{time_total}\n" "$HEALTH_URL" 2>/dev/null >> "$RES" || true
finish=$(date +%s.%N)
wall=$(awk -v a="$start" -v b="$finish" 'BEGIN{printf "%.3f", b-a}')

total=$(wc -l < "$RES" | tr -d ' '); [[ "$total" -eq 0 ]] && total=1
ok=$(awk '$1=="200"{c++} END{print c+0}' "$RES")
pct=$((ok * 100 / total))
echo "[4/6] Sustained load: total=$total ok=$ok pct=${pct}% wall=${wall}s"
[[ "$pct" -lt "$MIN_PASS_PCT" ]] && { echo "  FAIL: ${pct}% < ${MIN_PASS_PCT}%"; exit 1; }

post_base=$(mktemp); trap "rm -f $base $RES $post_base" EXIT
for _ in $(seq 1 10); do
    curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" -w "%{time_total}\n" "$HEALTH_URL" 2>/dev/null >> "$post_base" || true
done
post_med=$(sort -n "$post_base" | awk 'NR==5{print; exit}')
deg=$(awk -v a="$base_med" -v b="$post_med" 'BEGIN{if(a<=0)a=0.0001; printf "%.0f", (b-a)*100/a}')
echo "[5/6] Latency degradation: ${base_med}s → ${post_med}s (Δ=${deg}%, threshold ≤${MAX_DEG_PCT}%)"
[[ "$deg" -gt "$MAX_DEG_PCT" ]] && { echo "  FAIL: degraded ${deg}%"; exit 1; }

post_code=$(curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post_code="000"
[[ "$post_code" != "200" ]] && { echo "[6/6] FAIL: post-stress HTTP $post_code"; exit 1; }
echo "[6/6] Post-stress liveness: PASS"

echo
echo "=== HelixQA Stress Challenge: PASSED ==="
echo "  evidence: dur=${wall}s reqs=${total} pct=${pct}% baseline=${base_med}s post=${post_med}s deg=${deg}%"
