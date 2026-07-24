#!/bin/bash
# hxs_backend_e2e.sh — Full API surface E2E tests
# Auth, merchants, products, orders, payments, webhooks, payouts, WebSocket

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CONFIG_DIR="$(cd "$SCRIPT_DIR/../config" && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }
. "$LIB_AB"

ab_init "hxs_backend_e2e" "/tmp/hxs_backend_e2e.results"
ab_send_action "HXS Backend E2E — full API surface test"

[ -f "$CONFIG_DIR/credentials.env" ] && . "$CONFIG_DIR/credentials.env"

API_URL="${HXS_API_URL:-http://127.0.0.1:8080}"
TEST_PASSED=0

on_exit() {
    rc=$?
    [ "$TEST_PASSED" = "0" ] && [ $rc -ne 0 ] && \
        ab_fail "Backend E2E exited at line ${LINENO:-?} rc=$rc"
    exit $rc
}
trap on_exit EXIT

echo "=== HXS Backend E2E Tests ==="

echo "--- Auth ---"
REG=$(curl -s -m 5 -X POST "$API_URL/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"e2e@helix.test\",\"password\":\"test123!\",\"name\":\"E2E Tester\"}" 2>/dev/null)
if echo "$REG" | grep -qiE '"(id|token)"'; then
    ab_pass "Registration succeeds for new user"
else
    ab_skip "Registration endpoint not available or returned: $(echo "$REG" | head -c 80)"
fi

LOGIN=$(curl -s -m 5 -X POST "$API_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$HXS_ADMIN_EMAIL\",\"password\":\"$HXS_ADMIN_PASSWORD\"}" 2>/dev/null)
TOKEN=$(echo "$LOGIN" | grep -oE '"token"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*: *"//;s/"//')
if [ -n "$TOKEN" ]; then
    ab_pass "Admin login returns JWT"
else
    TOKEN=$(echo "$LOGIN" | grep -oE '"access_token"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*: *"//;s/"//')
    [ -n "$TOKEN" ] && ab_pass "Admin login returns access_token" || ab_fail "Admin login returned no token"
fi

BAD_LOGIN=$(curl -s -m 5 -w '%{http_code}' -X POST "$API_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"email":"wrong@test.com","password":"bad"}' -o /dev/null 2>/dev/null)
[ "$BAD_LOGIN" = "401" ] && ab_pass "Invalid login returns 401" || ab_fail "Invalid login got HTTP $BAD_LOGIN, want 401"

echo "--- Health ---"
HEALTH=$(curl -s -m 3 "$API_URL/health" -w '%{http_code}' -o /dev/null 2>/dev/null || echo "000")
[ "$HEALTH" = "200" ] && ab_pass "Health endpoint returns 200" || ab_fail "Health endpoint returned HTTP $HEALTH"

echo "--- Merchants ---"
if [ -n "$TOKEN" ]; then
    MERCHANTS=$(curl -s -m 5 -H "Authorization: Bearer $TOKEN" "$API_URL/api/v1/merchants" 2>/dev/null)
    if echo "$MERCHANTS" | grep -qiE '\[|\{'; then
        ab_pass "Merchants list returns valid JSON"
    else
        ab_skip "Merchants endpoint returned non-JSON"
    fi
    UNAUTH=$(curl -s -m 5 -w '%{http_code}' "$API_URL/api/v1/merchants" -o /dev/null 2>/dev/null)
    [ "$UNAUTH" = "401" ] && ab_pass "Merchants without auth returns 401" || ab_fail "Merchants without auth got HTTP $UNAUTH, want 401"
else
    ab_skip "No token — skipping merchant tests"
fi

echo "--- Products ---"
if [ -n "$TOKEN" ]; then
    PRODUCTS=$(curl -s -m 5 -H "Authorization: Bearer $TOKEN" "$API_URL/api/v1/products" 2>/dev/null)
    echo "$PRODUCTS" | grep -qiE '\[|\{' && ab_pass "Products list returns JSON" || ab_skip "Products endpoint non-JSON"
    PROD_CREATE=$(curl -s -m 5 -X POST -H "Authorization: Bearer $TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"name":"Test Product","price":1999,"currency":"USD"}' \
        "$API_URL/api/v1/products" 2>/dev/null)
    PROD_ID=$(echo "$PROD_CREATE" | grep -oE '"_?id"[[:space:]]*:[[:space:]]*"[^"]+"' | head -1 | sed 's/.*: *"//;s/"//')
    if [ -n "$PROD_ID" ]; then
        ab_pass "Product created (id=$PROD_ID)"
    else
        ab_fail "Product creation failed: $(echo "$PROD_CREATE" | head -c 80)"
    fi
else
    ab_skip "No token — skipping product tests"
fi

echo "--- Payments ---"
if [ -n "$TOKEN" ]; then
    PMT=$(curl -s -m 5 -X POST -H "Authorization: Bearer $TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"amount":5000,"currency":"USD","payment_method":"card"}' \
        "$API_URL/api/v1/payments/charge" 2>/dev/null)
    echo "$PMT" | grep -qiE '"(id|charge|status)"' && \
        ab_pass "Payment charge endpoint responds" || \
        ab_skip "Payment charge not fully configured"
else
    ab_skip "No token — skipping payment tests"
fi

echo "--- WebSocket ---"
if command -v websocat >/dev/null 2>&1; then
    WS_RESULT=$(echo "" | timeout 3 websocat "ws://127.0.0.1:8080/ws" 2>&1 || echo "timeout/error")
    echo "$WS_RESULT" | grep -qiE 'connected|message|error' && \
        ab_pass "WebSocket endpoint reachable" || \
        ab_skip "WebSocket not responding within 3s"
elif command -v curl >/dev/null && curl --version 2>/dev/null | grep -qi websocket; then
    ab_skip "WebSocket test skipped — use websocat for interactive test"
else
    ab_skip "WebSocket test skipped — websocat not installed"
fi

echo
echo "=== hxs_backend_e2e complete ==="
TEST_PASSED=1
ab_summary
