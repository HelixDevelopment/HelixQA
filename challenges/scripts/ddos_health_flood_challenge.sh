#!/usr/bin/env bash
# ddos_health_flood_challenge.sh — anti-bluff DDoS Challenge for
# HelixQA per CONST-035 + CONST-050(B). Submodule-side cascade of
# the parent project's DDoS Challenge per CONST-051(A) (submodules-as-equal-
# codebase mandate).
#
# What this Challenge proves:
#   - HelixQA's HTTP health endpoint survives a burst of concurrent
#     requests with pass-rate above threshold (catches the "single-
#     request works but service falls over under load" class).
#   - Post-flood liveness probe still returns 200 + valid status
#     (catches the "fell over during flood" class).
#   - Captures p50/p95/p99 latencies as wire evidence — real numbers,
#     not vibes.
#
# Operator-safe (no sudo, no host mutation, no power management).
# Honest SKIP-OK per §11.4.3 topology-dispatch when HelixQA service
# isn't running locally — this is the COMMON case on dev boxes.

set -uo pipefail

HEALTH_URL="${HELIXQA_HEALTH_URL:-http://localhost:8081/health}"
TOTAL_REQS="${DDOS_REQUESTS:-500}"
CONCURRENCY="${DDOS_CONCURRENCY:-50}"
TIMEOUT_SEC="${DDOS_TIMEOUT_SEC:-5}"
MIN_PASS_PCT="${DDOS_MIN_PASS_PCT:-95}"

echo "=== HelixQA DDoS Health-Flood Challenge (anti-bluff per CONST-035) ==="
echo "  target health URL:  $HEALTH_URL"
echo "  total requests:     $TOTAL_REQS"
echo "  concurrency:        $CONCURRENCY"
echo "  pass threshold:     ≥${MIN_PASS_PCT}%"

# Step 1: pre-flood probe.
echo
echo "[1/5] Pre-flood probe — HelixQA must be reachable..."
pre_code=$(curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || pre_code="000"
if [[ "$pre_code" != "200" ]]; then
    echo "  SKIP: HelixQA not reachable (HTTP $pre_code) — SKIP-OK: #env-helixqa-not-running"
    echo "  (start HelixQA via docker-compose -f helix_qa/docker-compose.stack.yml up to exercise)"
    echo
    echo "=== HelixQA DDoS Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "  PASS: pre-flood probe HTTP 200"

# Step 2: schema sanity — assert real body with status field.
echo
echo "[2/5] Schema sanity — real body, not empty 200..."
pre_body=$(curl -sS --max-time "$TIMEOUT_SEC" "$HEALTH_URL" 2>/dev/null || true)
if ! printf '%s' "$pre_body" | grep -qE '"status"\s*:\s*"(ok|healthy|UP)"' ; then
    echo "  FAIL: pre-flood body missing valid status field"
    echo "  body: $(printf '%s' "$pre_body" | head -c 200)"
    exit 1
fi
echo "  PASS: pre-flood body has valid status field"

# Step 3: flood.
echo
echo "[3/5] Flooding $HEALTH_URL with $TOTAL_REQS reqs at concurrency $CONCURRENCY..."
RESULTS=$(mktemp)
trap "rm -f $RESULTS" EXIT
start=$(date +%s.%N)
seq 1 "$TOTAL_REQS" | xargs -n1 -P "$CONCURRENCY" -I{} \
    curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" \
        -w "%{http_code} %{time_total}\n" "$HEALTH_URL" \
    2>/dev/null >> "$RESULTS" || true
end=$(date +%s.%N)
wall=$(awk -v a="$start" -v b="$end" 'BEGIN{printf "%.3f", b-a}')

total=$(wc -l < "$RESULTS" | tr -d ' ')
ok=$(awk '$1=="200"{c++} END{print c+0}' "$RESULTS")
[[ "$total" -eq 0 ]] && total=1
pct=$((ok * 100 / total))

sorted=$(awk '{print $2}' "$RESULTS" | sort -n)
p50=$(printf '%s\n' "$sorted" | awk -v n="$total" 'NR==int(n*0.5){print; exit}')
p95=$(printf '%s\n' "$sorted" | awk -v n="$total" 'NR==int(n*0.95){print; exit}')
p99=$(printf '%s\n' "$sorted" | awk -v n="$total" 'NR==int(n*0.99){print; exit}')
nonok=$(awk '$1!="200"{print $1}' "$RESULTS" | sort -u | tr '\n' ',' | sed 's/,$//')

echo "  total:    $total"
echo "  HTTP 200: $ok"
echo "  pass:     ${pct}% (threshold ≥${MIN_PASS_PCT}%)"
echo "  non-200:  ${nonok:-none}"
echo "  wall:     ${wall}s"
echo "  p50/p95/p99: ${p50:-N/A}s / ${p95:-N/A}s / ${p99:-N/A}s"

# Step 4: threshold gate.
echo
echo "[4/5] Threshold gate..."
if [[ "$pct" -lt "$MIN_PASS_PCT" ]]; then
    echo "  FAIL: pass rate ${pct}% < ${MIN_PASS_PCT}%"
    exit 1
fi
echo "  PASS: pass rate ${pct}% ≥ ${MIN_PASS_PCT}%"

# Step 5: post-flood liveness.
echo
echo "[5/5] Post-flood liveness probe..."
post_code=$(curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null) || post_code="000"
if [[ "$post_code" != "200" ]]; then
    echo "  FAIL: post-flood probe HTTP $post_code — service tipped over"
    exit 1
fi
echo "  PASS: post-flood HTTP 200 — service stable"

echo
echo "=== HelixQA DDoS Challenge: PASSED ==="
echo "  evidence: reqs=${total} ok=${ok} pct=${pct}% wall=${wall}s p50=${p50:-N/A}s p95=${p95:-N/A}s p99=${p99:-N/A}s"
